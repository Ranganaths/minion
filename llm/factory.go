package llm

import (
	"fmt"
	"os"
)

// ProviderType represents the type of LLM provider
type ProviderType string

const (
	ProviderTypeOpenAI    ProviderType = "openai"
	ProviderTypeAnthropic ProviderType = "anthropic"
	ProviderTypeOllama    ProviderType = "ollama"
	ProviderTypeTupleLeap ProviderType = "tupleleap"
)

// ProviderConfig contains configuration for creating providers
type ProviderConfig struct {
	Type   ProviderType
	APIKey string // For cloud providers
	BaseURL string // For Ollama or custom endpoints
}

// ProviderFactory creates LLM providers
type ProviderFactory struct {
	config *ProviderConfig
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(config *ProviderConfig) *ProviderFactory {
	return &ProviderFactory{config: config}
}

// NewProviderFactoryFromEnv creates a provider factory from environment variables
func NewProviderFactoryFromEnv() *ProviderFactory {
	providerType := os.Getenv("LLM_PROVIDER")
	if providerType == "" {
		providerType = "openai" // Default
	}

	config := &ProviderConfig{
		Type:    ProviderType(providerType),
		BaseURL: os.Getenv("LLM_BASE_URL"),
	}

	// Get API key based on provider
	switch config.Type {
	case ProviderTypeOpenAI:
		config.APIKey = os.Getenv("OPENAI_API_KEY")
	case ProviderTypeAnthropic:
		config.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	case ProviderTypeTupleLeap:
		config.APIKey = os.Getenv("TUPLELEAP_API_KEY")
	}

	return &ProviderFactory{config: config}
}

// CreateProvider creates a provider based on configuration
func (pf *ProviderFactory) CreateProvider() (Provider, error) {
	switch pf.config.Type {
	case ProviderTypeOpenAI:
		if pf.config.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key required")
		}
		return NewOpenAI(pf.config.APIKey), nil

	case ProviderTypeAnthropic:
		if pf.config.APIKey == "" {
			return nil, fmt.Errorf("Anthropic API key required")
		}
		return NewAnthropic(pf.config.APIKey), nil

	case ProviderTypeOllama:
		return NewOllama(pf.config.BaseURL), nil

	case ProviderTypeTupleLeap:
		if pf.config.APIKey == "" {
			return nil, fmt.Errorf("TupleLeap API key required")
		}
		if pf.config.BaseURL != "" {
			return NewTupleLeapWithBaseURL(pf.config.APIKey, pf.config.BaseURL), nil
		}
		return NewTupleLeap(pf.config.APIKey), nil

	default:
		return nil, fmt.Errorf("unknown provider type: %s", pf.config.Type)
	}
}

// MultiProviderFactory creates and manages multiple providers
type MultiProviderFactory struct {
	providers map[string]Provider
}

// NewMultiProviderFactory creates a factory that manages multiple providers
func NewMultiProviderFactory() *MultiProviderFactory {
	return &MultiProviderFactory{
		providers: make(map[string]Provider),
	}
}

// AddProvider adds a provider to the factory
func (mpf *MultiProviderFactory) AddProvider(name string, provider Provider) {
	mpf.providers[name] = provider
}

// GetProvider returns a provider by name
func (mpf *MultiProviderFactory) GetProvider(name string) (Provider, error) {
	provider, ok := mpf.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return provider, nil
}

// ListProviders returns all registered provider names
func (mpf *MultiProviderFactory) ListProviders() []string {
	names := make([]string, 0, len(mpf.providers))
	for name := range mpf.providers {
		names = append(names, name)
	}
	return names
}

// CreateDefaultProviders creates providers for all available credentials
func CreateDefaultProviders() *MultiProviderFactory {
	factory := NewMultiProviderFactory()

	// OpenAI
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		factory.AddProvider("openai", NewOpenAI(apiKey))
	}

	// Anthropic
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		factory.AddProvider("anthropic", NewAnthropic(apiKey))
	}

	// TupleLeap AI
	if apiKey := os.Getenv("TUPLELEAP_API_KEY"); apiKey != "" {
		baseURL := os.Getenv("TUPLELEAP_BASE_URL")
		if baseURL != "" {
			factory.AddProvider("tupleleap", NewTupleLeapWithBaseURL(apiKey, baseURL))
		} else {
			factory.AddProvider("tupleleap", NewTupleLeap(apiKey))
		}
	}

	// Ollama (always available if running locally)
	factory.AddProvider("ollama", NewOllama(""))

	return factory
}
