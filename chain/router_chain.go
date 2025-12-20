package chain

import (
	"context"
	"fmt"
	"sync"
)

// RouterChain routes inputs to different chains based on a routing function.
// RouterChain is safe for concurrent use by multiple goroutines.
type RouterChain struct {
	*BaseChain
	routes       map[string]Chain
	routerFunc   RouterFunc
	defaultChain Chain
	inputKeys    []string
	outputKeys   []string
	mu           sync.RWMutex
}

// RouterFunc determines which chain to route to based on inputs
// Returns the route name or empty string for default
type RouterFunc func(inputs map[string]any) (string, error)

// RouterChainConfig configures a router chain
type RouterChainConfig struct {
	// Routes maps route names to chains
	Routes map[string]Chain

	// RouterFunc determines which route to take
	RouterFunc RouterFunc

	// DefaultChain is used when no route matches (optional)
	DefaultChain Chain

	// InputKeys are the required input keys
	InputKeys []string

	// OutputKeys are the output keys (auto-detected from first chain if not provided)
	OutputKeys []string

	// Options are chain options
	Options []Option
}

// NewRouterChain creates a new router chain
func NewRouterChain(cfg RouterChainConfig) (*RouterChain, error) {
	if len(cfg.Routes) == 0 {
		return nil, fmt.Errorf("at least one route is required")
	}
	if cfg.RouterFunc == nil {
		return nil, fmt.Errorf("router function is required")
	}

	// Auto-detect output keys from first chain if not provided
	outputKeys := cfg.OutputKeys
	if len(outputKeys) == 0 {
		for _, chain := range cfg.Routes {
			outputKeys = chain.OutputKeys()
			break
		}
	}

	return &RouterChain{
		BaseChain:    NewBaseChain("router_chain", cfg.Options...),
		routes:       cfg.Routes,
		routerFunc:   cfg.RouterFunc,
		defaultChain: cfg.DefaultChain,
		inputKeys:    cfg.InputKeys,
		outputKeys:   outputKeys,
	}, nil
}

// InputKeys returns the required input keys
func (c *RouterChain) InputKeys() []string {
	return c.inputKeys
}

// OutputKeys returns the output keys
func (c *RouterChain) OutputKeys() []string {
	return c.outputKeys
}

// Call executes the appropriate chain based on routing
func (c *RouterChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	c.NotifyStart(ctx, inputs)

	// Apply timeout
	ctx, cancel := c.WithTimeout(ctx)
	defer cancel()

	// Determine route
	routeName, err := c.routerFunc(inputs)
	if err != nil {
		c.NotifyError(ctx, fmt.Errorf("routing error: %w", err))
		return nil, fmt.Errorf("routing error: %w", err)
	}

	// Find the chain to execute (with read lock)
	chain := c.getChain(routeName)

	if chain == nil {
		err := fmt.Errorf("no chain found for route: %s", routeName)
		c.NotifyError(ctx, err)
		return nil, err
	}

	// Execute the selected chain
	outputs, err := chain.Call(ctx, inputs)
	if err != nil {
		c.NotifyError(ctx, fmt.Errorf("chain %s error: %w", routeName, err))
		return nil, fmt.Errorf("chain %s error: %w", routeName, err)
	}

	c.NotifyEnd(ctx, outputs)
	return outputs, nil
}

// getChain safely retrieves a chain by route name
func (c *RouterChain) getChain(routeName string) Chain {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var chain Chain
	if routeName == "" {
		chain = c.defaultChain
	} else {
		chain = c.routes[routeName]
	}

	if chain == nil && c.defaultChain != nil {
		chain = c.defaultChain
	}

	return chain
}

// Stream executes and streams results from the selected chain
func (c *RouterChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 10)

	go func() {
		defer close(ch)

		ch <- MakeStreamEvent(StreamEventStart, "", map[string]any{"chain": c.Name()}, nil)

		// Determine route
		routeName, err := c.routerFunc(inputs)
		if err != nil {
			ch <- MakeStreamEvent(StreamEventError, "", nil, fmt.Errorf("routing error: %w", err))
			return
		}

		// Find the chain to execute (thread-safe)
		chain := c.getChain(routeName)

		if chain == nil {
			ch <- MakeStreamEvent(StreamEventError, "", nil, fmt.Errorf("no chain found for route: %s", routeName))
			return
		}

		// Stream from the selected chain
		streamCh, err := chain.Stream(ctx, inputs)
		if err != nil {
			ch <- MakeStreamEvent(StreamEventError, "", nil, err)
			return
		}

		// Forward all events with context cancellation check
		for {
			select {
			case <-ctx.Done():
				ch <- MakeStreamEvent(StreamEventError, "", nil, ctx.Err())
				return
			case event, ok := <-streamCh:
				if !ok {
					return
				}
				ch <- event
			}
		}
	}()

	return ch, nil
}

// AddRoute adds a new route to the router.
// This method is safe for concurrent use.
func (c *RouterChain) AddRoute(name string, chain Chain) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.routes[name] = chain
}

// RemoveRoute removes a route from the router.
// This method is safe for concurrent use.
func (c *RouterChain) RemoveRoute(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.routes, name)
}

// Routes returns a copy of the available routes.
// This method is safe for concurrent use.
func (c *RouterChain) Routes() map[string]Chain {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]Chain, len(c.routes))
	for k, v := range c.routes {
		result[k] = v
	}
	return result
}

// SetDefaultChain sets the default chain for unmatched routes.
// This method is safe for concurrent use.
func (c *RouterChain) SetDefaultChain(chain Chain) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultChain = chain
}

// KeywordRouter creates a router function that routes based on keyword matching
func KeywordRouter(keywordMap map[string][]string, inputKey string) RouterFunc {
	return func(inputs map[string]any) (string, error) {
		input, ok := inputs[inputKey]
		if !ok {
			return "", fmt.Errorf("missing input key: %s", inputKey)
		}

		inputStr := fmt.Sprintf("%v", input)
		inputLower := toLower(inputStr)

		for routeName, keywords := range keywordMap {
			for _, keyword := range keywords {
				if contains(inputLower, toLower(keyword)) {
					return routeName, nil
				}
			}
		}

		return "", nil // Use default chain
	}
}

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MapRouter creates a router function that uses a simple map lookup
func MapRouter(routeKey string) RouterFunc {
	return func(inputs map[string]any) (string, error) {
		route, ok := inputs[routeKey]
		if !ok {
			return "", nil // Use default chain
		}
		return fmt.Sprintf("%v", route), nil
	}
}
