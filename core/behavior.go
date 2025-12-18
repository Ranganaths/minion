package core

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Ranganaths/minion/models"
)

var (
	ErrBehaviorNotFound      = errors.New("behavior not found")
	ErrBehaviorAlreadyExists = errors.New("behavior already exists")
)

// DefaultBehavior implements a basic behavior
type DefaultBehavior struct{}

func (b *DefaultBehavior) GetSystemPrompt(agent *models.Agent) string {
	return fmt.Sprintf(`You are %s.

Description: %s

Behavior Type: %s
Personality: %s

Process user input and provide helpful responses.
`, agent.Name, agent.Description, agent.BehaviorType, agent.Config.Personality)
}

func (b *DefaultBehavior) ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error) {
	return &models.ProcessedInput{
		Original:     input,
		Processed:    input.Raw,
		Instructions: "Process this input",
		ExtraContext: make(map[string]interface{}),
	}, nil
}

func (b *DefaultBehavior) ProcessOutput(ctx context.Context, agent *models.Agent, output *models.Output) (*models.ProcessedOutput, error) {
	return &models.ProcessedOutput{
		Original:  output,
		Processed: output.Result,
		Enhanced:  make(map[string]interface{}),
	}, nil
}

// BehaviorRegistryImpl implements BehaviorRegistry
type BehaviorRegistryImpl struct {
	behaviors map[string]Behavior
	mu        sync.RWMutex
}

// NewBehaviorRegistry creates a new behavior registry
func NewBehaviorRegistry() *BehaviorRegistryImpl {
	registry := &BehaviorRegistryImpl{
		behaviors: make(map[string]Behavior),
	}

	// Register default behavior
	registry.behaviors["default"] = &DefaultBehavior{}

	return registry
}

// Register adds a behavior to the registry
func (r *BehaviorRegistryImpl) Register(behaviorType string, behavior Behavior) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.behaviors[behaviorType]; exists {
		return fmt.Errorf("%w: %s", ErrBehaviorAlreadyExists, behaviorType)
	}

	r.behaviors[behaviorType] = behavior
	return nil
}

// Get retrieves a behavior by type
func (r *BehaviorRegistryImpl) Get(behaviorType string) (Behavior, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	behavior, exists := r.behaviors[behaviorType]
	if !exists {
		// Return default behavior if not found
		if defaultBehavior, ok := r.behaviors["default"]; ok {
			return defaultBehavior, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrBehaviorNotFound, behaviorType)
	}

	return behavior, nil
}

// List returns all registered behavior types
func (r *BehaviorRegistryImpl) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.behaviors))
	for behaviorType := range r.behaviors {
		types = append(types, behaviorType)
	}

	return types
}
