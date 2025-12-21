package chain

import (
	"context"
	"fmt"
)

// TransformChain transforms inputs before passing to an inner chain
type TransformChain struct {
	*BaseChain
	innerChain    Chain
	transformFunc TransformFunc
	inputKeys     []string
	outputKeys    []string
}

// TransformFunc transforms inputs before they are passed to the inner chain
type TransformFunc func(inputs map[string]any) (map[string]any, error)

// TransformChainConfig configures a transform chain
type TransformChainConfig struct {
	// InnerChain is the chain to execute after transformation
	InnerChain Chain

	// TransformFunc transforms inputs before passing to inner chain
	TransformFunc TransformFunc

	// InputKeys are the required input keys (auto-detected if not provided)
	InputKeys []string

	// OutputKeys are the output keys (inherited from inner chain if not provided)
	OutputKeys []string

	// Options are chain options
	Options []Option
}

// NewTransformChain creates a new transform chain
func NewTransformChain(cfg TransformChainConfig) (*TransformChain, error) {
	if cfg.InnerChain == nil {
		return nil, fmt.Errorf("inner chain is required")
	}
	if cfg.TransformFunc == nil {
		return nil, fmt.Errorf("transform function is required")
	}

	outputKeys := cfg.OutputKeys
	if len(outputKeys) == 0 {
		outputKeys = cfg.InnerChain.OutputKeys()
	}

	return &TransformChain{
		BaseChain:     NewBaseChain("transform_chain", cfg.Options...),
		innerChain:    cfg.InnerChain,
		transformFunc: cfg.TransformFunc,
		inputKeys:     cfg.InputKeys,
		outputKeys:    outputKeys,
	}, nil
}

// InputKeys returns the required input keys
func (c *TransformChain) InputKeys() []string {
	return c.inputKeys
}

// OutputKeys returns the output keys
func (c *TransformChain) OutputKeys() []string {
	return c.outputKeys
}

// Call executes the transform chain
func (c *TransformChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	c.NotifyStart(ctx, inputs)

	// Apply timeout
	ctx, cancel := c.WithTimeout(ctx)
	defer cancel()

	// Transform inputs
	transformed, err := c.transformFunc(inputs)
	if err != nil {
		c.NotifyError(ctx, fmt.Errorf("transform error: %w", err))
		return nil, fmt.Errorf("transform error: %w", err)
	}

	// Execute inner chain
	outputs, err := c.innerChain.Call(ctx, transformed)
	if err != nil {
		c.NotifyError(ctx, err)
		return nil, err
	}

	c.NotifyEnd(ctx, outputs)
	return outputs, nil
}

// Stream executes and streams results.
// The returned channel is closed when streaming completes or context is cancelled.
func (c *TransformChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 10)

	go func() {
		defer close(ch)

		// Helper to send with context check
		send := func(event StreamEvent) bool {
			select {
			case <-ctx.Done():
				return false
			case ch <- event:
				return true
			}
		}

		if !send(MakeStreamEvent(StreamEventStart, "", map[string]any{"chain": c.Name()}, nil)) {
			return
		}

		// Transform inputs
		transformed, err := c.transformFunc(inputs)
		if err != nil {
			send(MakeStreamEvent(StreamEventError, "", nil, fmt.Errorf("transform error: %w", err)))
			return
		}

		// Stream from inner chain
		streamCh, err := c.innerChain.Stream(ctx, transformed)
		if err != nil {
			send(MakeStreamEvent(StreamEventError, "", nil, err))
			return
		}

		// Forward all events with context check
		for event := range streamCh {
			if !send(event) {
				return
			}
		}
	}()

	return ch, nil
}

// FuncChain is a simple chain that executes a function
type FuncChain struct {
	*BaseChain
	fn         ChainFunc
	inputKeys  []string
	outputKeys []string
}

// ChainFunc is a function that can be used as a chain
type ChainFunc func(ctx context.Context, inputs map[string]any) (map[string]any, error)

// FuncChainConfig configures a function chain
type FuncChainConfig struct {
	// Func is the function to execute
	Func ChainFunc

	// Name is the chain name
	Name string

	// InputKeys are the required input keys
	InputKeys []string

	// OutputKeys are the output keys
	OutputKeys []string

	// Options are chain options
	Options []Option
}

// NewFuncChain creates a new function chain
func NewFuncChain(cfg FuncChainConfig) (*FuncChain, error) {
	if cfg.Func == nil {
		return nil, fmt.Errorf("function is required")
	}

	name := cfg.Name
	if name == "" {
		name = "func_chain"
	}

	return &FuncChain{
		BaseChain:  NewBaseChain(name, cfg.Options...),
		fn:         cfg.Func,
		inputKeys:  cfg.InputKeys,
		outputKeys: cfg.OutputKeys,
	}, nil
}

// InputKeys returns the required input keys
func (c *FuncChain) InputKeys() []string {
	return c.inputKeys
}

// OutputKeys returns the output keys
func (c *FuncChain) OutputKeys() []string {
	return c.outputKeys
}

// Call executes the function chain
func (c *FuncChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	c.NotifyStart(ctx, inputs)

	// Apply timeout
	ctx, cancel := c.WithTimeout(ctx)
	defer cancel()

	outputs, err := c.fn(ctx, inputs)
	if err != nil {
		c.NotifyError(ctx, err)
		return nil, err
	}

	c.NotifyEnd(ctx, outputs)
	return outputs, nil
}

// Stream executes and streams results.
// The returned channel is closed when streaming completes or context is cancelled.
func (c *FuncChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 10)

	go func() {
		defer close(ch)

		// Helper to send with context check
		send := func(event StreamEvent) bool {
			select {
			case <-ctx.Done():
				return false
			case ch <- event:
				return true
			}
		}

		if !send(MakeStreamEvent(StreamEventStart, "", map[string]any{"chain": c.Name()}, nil)) {
			return
		}

		result, err := c.Call(ctx, inputs)
		if err != nil {
			send(MakeStreamEvent(StreamEventError, "", nil, err))
			return
		}

		send(MakeStreamEvent(StreamEventComplete, "", result, nil))
	}()

	return ch, nil
}

// PassthroughChain passes inputs directly to outputs
type PassthroughChain struct {
	*BaseChain
	keys []string
}

// NewPassthroughChain creates a chain that passes specified keys through
func NewPassthroughChain(keys []string, opts ...Option) *PassthroughChain {
	return &PassthroughChain{
		BaseChain: NewBaseChain("passthrough_chain", opts...),
		keys:      keys,
	}
}

// InputKeys returns the required input keys
func (c *PassthroughChain) InputKeys() []string {
	return c.keys
}

// OutputKeys returns the output keys
func (c *PassthroughChain) OutputKeys() []string {
	return c.keys
}

// Call passes inputs to outputs
func (c *PassthroughChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	c.NotifyStart(ctx, inputs)

	outputs := make(map[string]any)
	for _, key := range c.keys {
		if v, ok := inputs[key]; ok {
			outputs[key] = v
		}
	}

	c.NotifyEnd(ctx, outputs)
	return outputs, nil
}

// Stream executes and streams results.
// The returned channel is closed when streaming completes or context is cancelled.
func (c *PassthroughChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 10)

	go func() {
		defer close(ch)

		// Helper to send with context check
		send := func(event StreamEvent) bool {
			select {
			case <-ctx.Done():
				return false
			case ch <- event:
				return true
			}
		}

		result, err := c.Call(ctx, inputs)
		if err != nil {
			send(MakeStreamEvent(StreamEventError, "", nil, err))
			return
		}

		send(MakeStreamEvent(StreamEventComplete, "", result, nil))
	}()

	return ch, nil
}
