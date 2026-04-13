package transform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TransformRule defines a transformation rule
type TransformRule struct {
	Path      string                 `json:"path"`      // JSONPath or field path
	Transform string                 `json:"transform"` // Transformation type
	Options   map[string]interface{} `json:"options"`   // Transform options
}

// TransformConfig holds transformation configuration
type TransformConfig struct {
	RequestRules  []TransformRule `json:"request_rules"`
	ResponseRules []TransformRule `json:"response_rules"`
}

// Transformer handles request/response transformations
type Transformer struct {
	configs map[string]*TransformConfig // endpoint -> config
}

// NewTransformer creates a new transformer
func NewTransformer() *Transformer {
	return &Transformer{
		configs: make(map[string]*TransformConfig),
	}
}

// AddConfig adds transformation config for an endpoint
func (t *Transformer) AddConfig(endpoint string, config *TransformConfig) {
	t.configs[endpoint] = config
}

// TransformRequest transforms incoming request data
func (t *Transformer) TransformRequest(endpoint string, data []byte) ([]byte, error) {
	config, ok := t.configs[endpoint]
	if !ok || len(config.RequestRules) == 0 {
		return data, nil // No transformation needed
	}

	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	for _, rule := range config.RequestRules {
		obj = t.applyTransform(obj, rule)
	}

	return json.Marshal(obj)
}

// TransformResponse transforms outgoing response data
func (t *Transformer) TransformResponse(endpoint string, data []byte) ([]byte, error) {
	config, ok := t.configs[endpoint]
	if !ok || len(config.ResponseRules) == 0 {
		return data, nil // No transformation needed
	}

	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	for _, rule := range config.ResponseRules {
		obj = t.applyTransform(obj, rule)
	}

	return json.Marshal(obj)
}

// applyTransform applies a single transformation rule
func (t *Transformer) applyTransform(data interface{}, rule TransformRule) interface{} {
	switch rule.Transform {
	case "rename":
		return t.renameField(data, rule)
	case "remove":
		return t.removeField(data, rule)
	case "add":
		return t.addField(data, rule)
	case "format":
		return t.formatField(data, rule)
	case "mask":
		return t.maskField(data, rule)
	case "convert":
		return t.convertField(data, rule)
	case "filter":
		return t.filterField(data, rule)
	case "aggregate":
		return t.aggregateField(data, rule)
	default:
		return data
	}
}

// renameField renames a field in the data
func (t *Transformer) renameField(data interface{}, rule TransformRule) interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		newName, _ := rule.Options["new_name"].(string)
		if newName != "" {
			if value, exists := m[rule.Path]; exists {
				delete(m, rule.Path)
				m[newName] = value
			}
		}
	}
	return data
}

// removeField removes a field from the data
func (t *Transformer) removeField(data interface{}, rule TransformRule) interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		delete(m, rule.Path)
	}
	return data
}

// addField adds a new field to the data
func (t *Transformer) addField(data interface{}, rule TransformRule) interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		value := rule.Options["value"]
		if value != nil {
			m[rule.Path] = value
		}
	}
	return data
}

// formatField formats a field value
func (t *Transformer) formatField(data interface{}, rule TransformRule) interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		if value, exists := m[rule.Path]; exists {
			format, _ := rule.Options["format"].(string)
			switch format {
			case "date":
				if str, ok := value.(string); ok {
					if t, err := time.Parse(time.RFC3339, str); err == nil {
						m[rule.Path] = t.Format("2006-01-02")
					}
				}
			case "datetime":
				if str, ok := value.(string); ok {
					if t, err := time.Parse(time.RFC3339, str); err == nil {
						m[rule.Path] = t.Format("2006-01-02 15:04:05")
					}
				}
			case "uppercase":
				if str, ok := value.(string); ok {
					m[rule.Path] = strings.ToUpper(str)
				}
			case "lowercase":
				if str, ok := value.(string); ok {
					m[rule.Path] = strings.ToLower(str)
				}
			case "title":
				if str, ok := value.(string); ok {
					m[rule.Path] = strings.Title(str)
				}
			}
		}
	}
	return data
}

// maskField masks sensitive data
func (t *Transformer) maskField(data interface{}, rule TransformRule) interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		if value, exists := m[rule.Path]; exists {
			if str, ok := value.(string); ok {
				maskChar, _ := rule.Options["mask_char"].(string)
				if maskChar == "" {
					maskChar = "*"
				}
				showFirst, _ := rule.Options["show_first"].(float64)
				showLast, _ := rule.Options["show_last"].(float64)

				if len(str) > int(showFirst+showLast) {
					masked := str[:int(showFirst)]
					for i := 0; i < len(str)-int(showFirst+showLast); i++ {
						masked += maskChar
					}
					masked += str[len(str)-int(showLast):]
					m[rule.Path] = masked
				}
			}
		}
	}
	return data
}

// convertField converts field type
func (t *Transformer) convertField(data interface{}, rule TransformRule) interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		if value, exists := m[rule.Path]; exists {
			targetType, _ := rule.Options["type"].(string)
			switch targetType {
			case "string":
				m[rule.Path] = fmt.Sprintf("%v", value)
			case "int":
				if v, ok := value.(float64); ok {
					m[rule.Path] = int(v)
				}
			case "float":
				if v, ok := value.(string); ok {
					var f float64
					fmt.Sscanf(v, "%f", &f)
					m[rule.Path] = f
				}
			case "bool":
				switch v := value.(type) {
				case string:
					m[rule.Path] = v == "true" || v == "1"
				case float64:
					m[rule.Path] = v != 0
				case int:
					m[rule.Path] = v != 0
				}
			}
		}
	}
	return data
}

// filterField filters array fields
func (t *Transformer) filterField(data interface{}, rule TransformRule) interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		if value, exists := m[rule.Path]; exists {
			if arr, ok := value.([]interface{}); ok {
				condition, _ := rule.Options["condition"].(string)
				field, _ := rule.Options["field"].(string)
				compareValue := rule.Options["value"]

				var filtered []interface{}
				for _, item := range arr {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if fieldValue, exists := itemMap[field]; exists {
							if t.matchCondition(fieldValue, condition, compareValue) {
								filtered = append(filtered, item)
							}
						}
					}
				}
				m[rule.Path] = filtered
			}
		}
	}
	return data
}

// aggregateField aggregates array data
func (t *Transformer) aggregateField(data interface{}, rule TransformRule) interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		if value, exists := m[rule.Path]; exists {
			if arr, ok := value.([]interface{}); ok {
				operation, _ := rule.Options["operation"].(string)
				field, _ := rule.Options["field"].(string)

				switch operation {
				case "count":
					m[rule.Path+"_count"] = len(arr)
				case "sum":
					sum := 0.0
					for _, item := range arr {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if v, exists := itemMap[field]; exists {
								if num, ok := v.(float64); ok {
									sum += num
								}
							}
						}
					}
					m[rule.Path+"_sum"] = sum
				case "average":
					if len(arr) > 0 {
						sum := 0.0
						for _, item := range arr {
							if itemMap, ok := item.(map[string]interface{}); ok {
								if v, exists := itemMap[field]; exists {
									if num, ok := v.(float64); ok {
										sum += num
									}
								}
							}
						}
						m[rule.Path+"_avg"] = sum / float64(len(arr))
					}
				}
			}
		}
	}
	return data
}

// matchCondition checks if a value matches a condition
func (t *Transformer) matchCondition(value interface{}, condition string, compareValue interface{}) bool {
	switch condition {
	case "equals":
		return value == compareValue
	case "not_equals":
		return value != compareValue
	case "greater_than":
		if v1, ok1 := value.(float64); ok1 {
			if v2, ok2 := compareValue.(float64); ok2 {
				return v1 > v2
			}
		}
	case "less_than":
		if v1, ok1 := value.(float64); ok1 {
			if v2, ok2 := compareValue.(float64); ok2 {
				return v1 < v2
			}
		}
	case "contains":
		if str, ok := value.(string); ok {
			if substr, ok := compareValue.(string); ok {
				return strings.Contains(str, substr)
			}
		}
	}
	return false
}

// StandardizeResponse ensures all responses follow the standard format
func (t *Transformer) StandardizeResponse(data []byte, success bool, message string) ([]byte, error) {
	var result map[string]interface{}

	if success {
		var parsedData interface{}
		if err := json.Unmarshal(data, &parsedData); err != nil {
			parsedData = string(data)
		}

		result = map[string]interface{}{
			"success":   true,
			"message":   message,
			"data":      parsedData,
			"timestamp": time.Now().Format(time.RFC3339),
		}
	} else {
		result = map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"message": message,
			},
			"timestamp": time.Now().Format(time.RFC3339),
		}
	}

	return json.Marshal(result)
}

// CompressJSON removes unnecessary whitespace from JSON
func (t *Transformer) CompressJSON(data []byte) []byte {
	buffer := new(bytes.Buffer)
	if err := json.Compact(buffer, data); err != nil {
		return data
	}
	return buffer.Bytes()
}
