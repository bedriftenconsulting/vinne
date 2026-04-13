package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ProviderFactory manages provider instances
type ProviderFactory struct {
	providers map[string]PaymentProvider
	mu        sync.RWMutex
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[string]PaymentProvider),
	}
}

// RegisterProvider registers a provider with the factory
func (f *ProviderFactory) RegisterProvider(name string, provider PaymentProvider) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	f.providers[name] = provider
	return nil
}

// GetProvider retrieves a provider by name (case-insensitive)
// Providers are registered with lowercase names, but can be queried with any case
func (f *ProviderFactory) GetProvider(name string) (PaymentProvider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Convert to lowercase for lookup (providers registered as lowercase)
	lookupName := strings.ToLower(name)

	provider, exists := f.providers[lookupName]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// GetProviderForOperation selects the best provider for an operation
func (f *ProviderFactory) GetProviderForOperation(
	ctx context.Context,
	opType OperationType,
	criteria *SelectionCriteria,
) (PaymentProvider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Selection logic based on criteria
	if criteria != nil && criteria.PreferredProvider != "" {
		if provider, err := f.GetProvider(criteria.PreferredProvider); err == nil {
			return provider, nil
		}
	}

	// Fallback to first available provider supporting the operation
	for _, provider := range f.providers {
		ops := provider.GetSupportedOperations()
		for _, op := range ops {
			if op == opType {
				return provider, nil
			}
		}
	}

	return nil, fmt.Errorf("no provider found for operation %s", opType)
}

// ListProviders returns all registered providers
func (f *ProviderFactory) ListProviders() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.providers))
	for name := range f.providers {
		names = append(names, name)
	}
	return names
}

// SelectionCriteria defines provider selection parameters
type SelectionCriteria struct {
	PreferredProvider string
	Amount            float64
	Currency          string
	RequireRealtime   bool
}

// ProviderUnavailableError indicates provider is unavailable
type ProviderUnavailableError struct {
	Provider string
	Reason   string
}

func (e *ProviderUnavailableError) Error() string {
	return fmt.Sprintf("provider %s unavailable: %s", e.Provider, e.Reason)
}
