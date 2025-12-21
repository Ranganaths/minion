package workers

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/chain"
	"github.com/Ranganaths/minion/core/multiagent"
)

// ChainWorker wraps any chain.Chain as a multiagent.TaskHandler.
// This allows any chain to be used within the multi-agent orchestration system.
type ChainWorker struct {
	name         string
	capabilities []string
	chain        chain.Chain
}

// ChainWorkerConfig configures the chain worker
type ChainWorkerConfig struct {
	// Name is the worker name (required)
	Name string

	// Capabilities are the capabilities this worker provides (required)
	Capabilities []string

	// Chain is the chain to wrap (required)
	Chain chain.Chain
}

// NewChainWorker creates a worker from any chain.
// The worker will execute the chain when handling tasks.
func NewChainWorker(cfg ChainWorkerConfig) (*ChainWorker, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(cfg.Capabilities) == 0 {
		return nil, fmt.Errorf("at least one capability is required")
	}
	if cfg.Chain == nil {
		return nil, fmt.Errorf("chain is required")
	}

	return &ChainWorker{
		name:         cfg.Name,
		capabilities: cfg.Capabilities,
		chain:        cfg.Chain,
	}, nil
}

// GetName implements multiagent.TaskHandler
func (w *ChainWorker) GetName() string {
	return w.name
}

// GetCapabilities implements multiagent.TaskHandler
func (w *ChainWorker) GetCapabilities() []string {
	return w.capabilities
}

// HandleTask implements multiagent.TaskHandler.
// It converts the task input to chain inputs and executes the chain.
func (w *ChainWorker) HandleTask(ctx context.Context, task *multiagent.Task) (interface{}, error) {
	// Convert task input to chain inputs
	inputs, err := w.convertInputs(task.Input)
	if err != nil {
		return nil, err
	}

	// Execute chain
	outputs, err := w.chain.Call(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("chain execution error: %w", err)
	}

	return outputs, nil
}

// convertInputs converts task input to chain input format
func (w *ChainWorker) convertInputs(input interface{}) (map[string]any, error) {
	switch v := input.(type) {
	case map[string]any:
		return v, nil

	case string:
		// Assume it's the primary input
		inputKeys := w.chain.InputKeys()
		if len(inputKeys) == 0 {
			return nil, fmt.Errorf("chain has no input keys")
		}
		return map[string]any{
			inputKeys[0]: v,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported task input type: %T; expected map[string]any or string", input)
	}
}

// Chain returns the underlying chain
func (w *ChainWorker) Chain() chain.Chain {
	return w.chain
}
