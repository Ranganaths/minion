package chain

import (
	"context"
	"fmt"
)

// SequentialChain executes multiple chains in sequence, passing outputs to inputs
type SequentialChain struct {
	*BaseChain
	chains     []Chain
	inputKeys  []string
	outputKeys []string
}

// SequentialChainConfig configures a sequential chain
type SequentialChainConfig struct {
	// Chains to execute in order
	Chains []Chain

	// InputKeys overrides auto-detected input keys
	InputKeys []string

	// OutputKeys overrides auto-detected output keys
	OutputKeys []string

	// Options are chain options
	Options []Option
}

// NewSequentialChain creates a new sequential chain
func NewSequentialChain(cfg SequentialChainConfig) (*SequentialChain, error) {
	if len(cfg.Chains) == 0 {
		return nil, fmt.Errorf("at least one chain is required")
	}

	// Auto-detect input keys from first chain if not provided
	inputKeys := cfg.InputKeys
	if len(inputKeys) == 0 && len(cfg.Chains) > 0 {
		inputKeys = cfg.Chains[0].InputKeys()
	}

	// Auto-detect output keys from last chain if not provided
	outputKeys := cfg.OutputKeys
	if len(outputKeys) == 0 && len(cfg.Chains) > 0 {
		outputKeys = cfg.Chains[len(cfg.Chains)-1].OutputKeys()
	}

	return &SequentialChain{
		BaseChain:  NewBaseChain("sequential_chain", cfg.Options...),
		chains:     cfg.Chains,
		inputKeys:  inputKeys,
		outputKeys: outputKeys,
	}, nil
}

// InputKeys returns the required input keys
func (c *SequentialChain) InputKeys() []string {
	return c.inputKeys
}

// OutputKeys returns the output keys
func (c *SequentialChain) OutputKeys() []string {
	return c.outputKeys
}

// Call executes all chains in sequence
func (c *SequentialChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	c.NotifyStart(ctx, inputs)

	// Apply timeout
	ctx, cancel := c.WithTimeout(ctx)
	defer cancel()

	// Accumulate all outputs
	allOutputs := CopyInputs(inputs)

	// Execute each chain
	for i, chain := range c.chains {
		// Check context
		select {
		case <-ctx.Done():
			err := ctx.Err()
			c.NotifyError(ctx, err)
			return nil, err
		default:
		}

		outputs, err := chain.Call(ctx, allOutputs)
		if err != nil {
			c.NotifyError(ctx, fmt.Errorf("chain %d (%s) error: %w", i, chain.Name(), err))
			return nil, fmt.Errorf("chain %d (%s) error: %w", i, chain.Name(), err)
		}

		// Merge outputs into accumulated state
		for k, v := range outputs {
			allOutputs[k] = v
		}
	}

	// Filter to requested output keys
	result := make(map[string]any)
	for _, key := range c.OutputKeys() {
		if v, ok := allOutputs[key]; ok {
			result[key] = v
		}
	}

	c.NotifyEnd(ctx, result)
	return result, nil
}

// Stream executes the chain and streams results
func (c *SequentialChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 10)

	go func() {
		defer close(ch)

		ch <- MakeStreamEvent(StreamEventStart, "", map[string]any{"chain": c.Name()}, nil)

		// Accumulate all outputs
		allOutputs := CopyInputs(inputs)

		// Execute each chain and stream intermediate results
		for i, chain := range c.chains {
			select {
			case <-ctx.Done():
				ch <- MakeStreamEvent(StreamEventError, "", nil, ctx.Err())
				return
			default:
			}

			// Stream from each chain
			streamCh, err := chain.Stream(ctx, allOutputs)
			if err != nil {
				ch <- MakeStreamEvent(StreamEventError, "", nil, err)
				return
			}

			// Forward events and capture final output
			var lastData map[string]any
			for event := range streamCh {
				// Forward intermediate events
				if event.Type != StreamEventComplete {
					ch <- event
				} else {
					lastData = event.Data
				}
			}

			// Merge outputs
			for k, v := range lastData {
				allOutputs[k] = v
			}

			// Emit chunk event for this chain's completion
			ch <- MakeStreamEvent(StreamEventChunk, "", map[string]any{
				"chain_index": i,
				"chain_name":  chain.Name(),
				"outputs":     lastData,
			}, nil)
		}

		// Filter to requested output keys
		result := make(map[string]any)
		for _, key := range c.OutputKeys() {
			if v, ok := allOutputs[key]; ok {
				result[key] = v
			}
		}

		ch <- MakeStreamEvent(StreamEventComplete, "", result, nil)
	}()

	return ch, nil
}

// AddChain appends a chain to the sequence
func (c *SequentialChain) AddChain(chain Chain) {
	c.chains = append(c.chains, chain)
	// Update output keys to match the new last chain
	c.outputKeys = chain.OutputKeys()
}

// Chains returns the list of chains
func (c *SequentialChain) Chains() []Chain {
	return c.chains
}

// SimpleSequentialChain is a simplified sequential chain for single input/output chains
type SimpleSequentialChain struct {
	*BaseChain
	chains   []Chain
	inputKey string
}

// SimpleSequentialChainConfig configures a simple sequential chain
type SimpleSequentialChainConfig struct {
	Chains   []Chain
	InputKey string
	Options  []Option
}

// NewSimpleSequentialChain creates a simple sequential chain
func NewSimpleSequentialChain(cfg SimpleSequentialChainConfig) (*SimpleSequentialChain, error) {
	if len(cfg.Chains) == 0 {
		return nil, fmt.Errorf("at least one chain is required")
	}

	inputKey := cfg.InputKey
	if inputKey == "" {
		inputKey = "input"
	}

	return &SimpleSequentialChain{
		BaseChain: NewBaseChain("simple_sequential_chain", cfg.Options...),
		chains:    cfg.Chains,
		inputKey:  inputKey,
	}, nil
}

// InputKeys returns the input keys
func (c *SimpleSequentialChain) InputKeys() []string {
	return []string{c.inputKey}
}

// OutputKeys returns the output keys
func (c *SimpleSequentialChain) OutputKeys() []string {
	if len(c.chains) > 0 {
		return c.chains[len(c.chains)-1].OutputKeys()
	}
	return []string{}
}

// Call executes all chains, passing each output as the next input
func (c *SimpleSequentialChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	c.NotifyStart(ctx, inputs)

	currentInputs := CopyInputs(inputs)

	for i, chain := range c.chains {
		outputs, err := chain.Call(ctx, currentInputs)
		if err != nil {
			c.NotifyError(ctx, fmt.Errorf("chain %d error: %w", i, err))
			return nil, fmt.Errorf("chain %d error: %w", i, err)
		}

		// Use outputs as next inputs
		currentInputs = outputs
	}

	c.NotifyEnd(ctx, currentInputs)
	return currentInputs, nil
}

// Stream executes and streams results
func (c *SimpleSequentialChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 10)

	go func() {
		defer close(ch)

		result, err := c.Call(ctx, inputs)
		if err != nil {
			ch <- MakeStreamEvent(StreamEventError, "", nil, err)
			return
		}

		ch <- MakeStreamEvent(StreamEventComplete, "", result, nil)
	}()

	return ch, nil
}
