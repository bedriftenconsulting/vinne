package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// AggregationRequest represents a request to aggregate data from multiple sources
type AggregationRequest struct {
	ID       string                 `json:"id"`
	Services []ServiceRequest       `json:"services"`
	Strategy AggregationStrategy    `json:"strategy"`
	Timeout  time.Duration          `json:"timeout"`
	Options  map[string]interface{} `json:"options"`
}

// ServiceRequest represents a request to a specific service
type ServiceRequest struct {
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers"`
	Body     json.RawMessage   `json:"body,omitempty"`
	Required bool              `json:"required"`
	Timeout  time.Duration     `json:"timeout"`
}

// ServiceResponse represents a response from a service
type ServiceResponse struct {
	Service   string          `json:"service"`
	Status    int             `json:"status"`
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error,omitempty"`
	Duration  time.Duration   `json:"duration"`
	Timestamp time.Time       `json:"timestamp"`
}

// AggregationStrategy defines how responses should be aggregated
type AggregationStrategy string

const (
	StrategyMerge     AggregationStrategy = "merge"     // Merge all responses into single object
	StrategyConcat    AggregationStrategy = "concat"    // Concatenate array responses
	StrategyFirst     AggregationStrategy = "first"     // Return first successful response
	StrategyAll       AggregationStrategy = "all"       // Return all responses as map
	StrategyCustom    AggregationStrategy = "custom"    // Custom aggregation logic
	StrategyPriority  AggregationStrategy = "priority"  // Return based on priority order
	StrategyComposite AggregationStrategy = "composite" // Compose response from multiple services
)

// Aggregator handles response aggregation from multiple services
type Aggregator struct {
	client         ServiceClient
	defaultTimeout time.Duration
	maxConcurrent  int
}

// ServiceClient interface for making service calls
type ServiceClient interface {
	Call(ctx context.Context, req ServiceRequest) (*ServiceResponse, error)
}

// NewAggregator creates a new aggregator
func NewAggregator(client ServiceClient) *Aggregator {
	return &Aggregator{
		client:         client,
		defaultTimeout: 30 * time.Second,
		maxConcurrent:  10,
	}
}

// Aggregate performs aggregation based on the request
func (a *Aggregator) Aggregate(ctx context.Context, req AggregationRequest) (interface{}, error) {
	// Set timeout
	timeout := req.Timeout
	if timeout == 0 {
		timeout = a.defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute service calls
	responses := a.executeServiceCalls(ctx, req.Services)

	// Check for required service failures
	if err := a.checkRequiredServices(req.Services, responses); err != nil {
		return nil, err
	}

	// Aggregate responses based on strategy
	return a.aggregateResponses(responses, req.Strategy, req.Options)
}

// executeServiceCalls executes all service calls concurrently
func (a *Aggregator) executeServiceCalls(ctx context.Context, services []ServiceRequest) []ServiceResponse {
	responses := make([]ServiceResponse, len(services))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, a.maxConcurrent)

	for i, service := range services {
		wg.Add(1)
		go func(index int, svc ServiceRequest) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Set service-specific timeout
			serviceCtx := ctx
			if svc.Timeout > 0 {
				var cancel context.CancelFunc
				serviceCtx, cancel = context.WithTimeout(ctx, svc.Timeout)
				defer cancel()
			}

			// Execute service call
			start := time.Now()
			resp, err := a.client.Call(serviceCtx, svc)

			if resp == nil {
				resp = &ServiceResponse{
					Service: svc.Name,
				}
			}

			resp.Duration = time.Since(start)
			resp.Timestamp = time.Now()

			if err != nil {
				resp.Error = err.Error()
				resp.Status = 500
			}

			responses[index] = *resp
		}(i, service)
	}

	wg.Wait()
	return responses
}

// checkRequiredServices checks if all required services succeeded
func (a *Aggregator) checkRequiredServices(services []ServiceRequest, responses []ServiceResponse) error {
	for i, service := range services {
		if service.Required && responses[i].Error != "" {
			return fmt.Errorf("required service %s failed: %s", service.Name, responses[i].Error)
		}
	}
	return nil
}

// aggregateResponses aggregates responses based on strategy
func (a *Aggregator) aggregateResponses(responses []ServiceResponse, strategy AggregationStrategy, options map[string]interface{}) (interface{}, error) {
	switch strategy {
	case StrategyMerge:
		return a.mergeResponses(responses)
	case StrategyConcat:
		return a.concatResponses(responses)
	case StrategyFirst:
		return a.firstResponse(responses)
	case StrategyAll:
		return a.allResponses(responses)
	case StrategyPriority:
		return a.priorityResponse(responses, options)
	case StrategyComposite:
		return a.compositeResponse(responses, options)
	case StrategyCustom:
		return a.customAggregation(responses, options)
	default:
		return nil, fmt.Errorf("unknown aggregation strategy: %s", strategy)
	}
}

// mergeResponses merges all successful responses into a single object
func (a *Aggregator) mergeResponses(responses []ServiceResponse) (interface{}, error) {
	result := make(map[string]interface{})

	for _, resp := range responses {
		if resp.Error == "" && resp.Data != nil {
			var data map[string]interface{}
			if err := json.Unmarshal(resp.Data, &data); err != nil {
				// If not an object, add as named field
				result[resp.Service] = resp.Data
			} else {
				// Merge object fields
				for key, value := range data {
					result[key] = value
				}
			}
		}
	}

	return result, nil
}

// concatResponses concatenates array responses
func (a *Aggregator) concatResponses(responses []ServiceResponse) (interface{}, error) {
	var result []interface{}

	for _, resp := range responses {
		if resp.Error == "" && resp.Data != nil {
			var data []interface{}
			if err := json.Unmarshal(resp.Data, &data); err != nil {
				// If not an array, skip or add as single element
				continue
			}
			result = append(result, data...)
		}
	}

	return result, nil
}

// firstResponse returns the first successful response
func (a *Aggregator) firstResponse(responses []ServiceResponse) (interface{}, error) {
	for _, resp := range responses {
		if resp.Error == "" && resp.Data != nil {
			var data interface{}
			if err := json.Unmarshal(resp.Data, &data); err != nil {
				return resp.Data, nil
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("no successful responses")
}

// allResponses returns all responses as a map
func (a *Aggregator) allResponses(responses []ServiceResponse) (interface{}, error) {
	result := make(map[string]interface{})

	for _, resp := range responses {
		serviceResult := map[string]interface{}{
			"status":    resp.Status,
			"duration":  resp.Duration.Milliseconds(),
			"timestamp": resp.Timestamp,
		}

		if resp.Error != "" {
			serviceResult["error"] = resp.Error
		} else if resp.Data != nil {
			var data interface{}
			if err := json.Unmarshal(resp.Data, &data); err != nil {
				serviceResult["data"] = resp.Data
			} else {
				serviceResult["data"] = data
			}
		}

		result[resp.Service] = serviceResult
	}

	return result, nil
}

// priorityResponse returns response based on priority order
func (a *Aggregator) priorityResponse(responses []ServiceResponse, options map[string]interface{}) (interface{}, error) {
	priority, ok := options["priority"].([]string)
	if !ok {
		return a.firstResponse(responses)
	}

	// Create service map
	serviceMap := make(map[string]ServiceResponse)
	for _, resp := range responses {
		serviceMap[resp.Service] = resp
	}

	// Return first successful response in priority order
	for _, serviceName := range priority {
		if resp, ok := serviceMap[serviceName]; ok {
			if resp.Error == "" && resp.Data != nil {
				var data interface{}
				if err := json.Unmarshal(resp.Data, &data); err != nil {
					return resp.Data, nil
				}
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("no successful responses in priority order")
}

// compositeResponse composes response from multiple services
func (a *Aggregator) compositeResponse(responses []ServiceResponse, options map[string]interface{}) (interface{}, error) {
	mapping, ok := options["mapping"].(map[string]interface{})
	if !ok {
		return a.mergeResponses(responses)
	}

	// Create service map
	serviceMap := make(map[string]json.RawMessage)
	for _, resp := range responses {
		if resp.Error == "" && resp.Data != nil {
			serviceMap[resp.Service] = resp.Data
		}
	}

	// Build composite response
	result := make(map[string]interface{})
	for targetField, sourceSpec := range mapping {
		if spec, ok := sourceSpec.(map[string]interface{}); ok {
			service := spec["service"].(string)
			field := spec["field"].(string)

			if data, ok := serviceMap[service]; ok {
				var parsed map[string]interface{}
				if err := json.Unmarshal(data, &parsed); err == nil {
					if value, exists := parsed[field]; exists {
						result[targetField] = value
					}
				}
			}
		}
	}

	return result, nil
}

// customAggregation allows for custom aggregation logic
func (a *Aggregator) customAggregation(responses []ServiceResponse, options map[string]interface{}) (interface{}, error) {
	// This would typically call a custom function or script
	// For now, return all responses
	return a.allResponses(responses)
}

// ParallelFetch fetches data from multiple endpoints in parallel
func (a *Aggregator) ParallelFetch(ctx context.Context, requests []ServiceRequest) map[string]*ServiceResponse {
	results := make(map[string]*ServiceResponse)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, req := range requests {
		wg.Add(1)
		go func(r ServiceRequest) {
			defer wg.Done()

			resp, err := a.client.Call(ctx, r)
			if err != nil {
				resp = &ServiceResponse{
					Service: r.Name,
					Error:   err.Error(),
					Status:  500,
				}
			}

			mu.Lock()
			results[r.Name] = resp
			mu.Unlock()
		}(req)
	}

	wg.Wait()
	return results
}
