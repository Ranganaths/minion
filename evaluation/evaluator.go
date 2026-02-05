package evaluation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Ranganaths/minion/tracing"
)

// Evaluator defines the interface for evaluation implementations
type Evaluator interface {
	// ID returns the unique identifier for this evaluator
	ID() string

	// Name returns the human-readable name
	Name() string

	// Type returns the evaluation type this evaluator produces
	Type() EvaluationType

	// Evaluate evaluates a single trace and returns an evaluation
	Evaluate(ctx context.Context, trace *tracing.Trace) (*Evaluation, error)

	// EvaluateBatch evaluates multiple traces
	EvaluateBatch(ctx context.Context, traces []*tracing.Trace) ([]*Evaluation, error)

	// Configure configures the evaluator with the given options
	Configure(config map[string]interface{}) error
}

// BaseEvaluator provides common functionality for evaluators
type BaseEvaluator struct {
	id     string
	name   string
	evType EvaluationType
	config map[string]interface{}
}

// NewBaseEvaluator creates a new base evaluator
func NewBaseEvaluator(id, name string, evType EvaluationType) *BaseEvaluator {
	return &BaseEvaluator{
		id:     id,
		name:   name,
		evType: evType,
		config: make(map[string]interface{}),
	}
}

// ID returns the evaluator ID
func (e *BaseEvaluator) ID() string {
	return e.id
}

// Name returns the evaluator name
func (e *BaseEvaluator) Name() string {
	return e.name
}

// Type returns the evaluation type
func (e *BaseEvaluator) Type() EvaluationType {
	return e.evType
}

// Configure configures the evaluator
func (e *BaseEvaluator) Configure(config map[string]interface{}) error {
	for k, v := range config {
		e.config[k] = v
	}
	return nil
}

// GetConfig returns a config value
func (e *BaseEvaluator) GetConfig(key string) (interface{}, bool) {
	v, ok := e.config[key]
	return v, ok
}

// GetConfigFloat returns a config value as float64
func (e *BaseEvaluator) GetConfigFloat(key string, defaultVal float64) float64 {
	v, ok := e.config[key]
	if !ok {
		return defaultVal
	}
	if f, ok := v.(float64); ok {
		return f
	}
	if i, ok := v.(int); ok {
		return float64(i)
	}
	return defaultVal
}

// GetConfigInt returns a config value as int
func (e *BaseEvaluator) GetConfigInt(key string, defaultVal int) int {
	v, ok := e.config[key]
	if !ok {
		return defaultVal
	}
	if i, ok := v.(int); ok {
		return i
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return defaultVal
}

// GetConfigBool returns a config value as bool
func (e *BaseEvaluator) GetConfigBool(key string, defaultVal bool) bool {
	v, ok := e.config[key]
	if !ok {
		return defaultVal
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return defaultVal
}

// EvaluateBatch provides a default batch implementation that calls Evaluate for each trace
func (e *BaseEvaluator) EvaluateBatch(ctx context.Context, traces []*tracing.Trace, evalFn func(context.Context, *tracing.Trace) (*Evaluation, error)) ([]*Evaluation, error) {
	results := make([]*Evaluation, 0, len(traces))
	for _, trace := range traces {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		eval, err := evalFn(ctx, trace)
		if err != nil {
			// Continue on error, but could make this configurable
			continue
		}
		results = append(results, eval)
	}
	return results, nil
}

// CreateEvaluation creates a new evaluation with common fields populated
func (e *BaseEvaluator) CreateEvaluation(trace *tracing.Trace, score float64) *Evaluation {
	return &Evaluation{
		ID:          NewEvaluationID(),
		TraceID:     trace.ID,
		AgentID:     trace.AgentID,
		SessionID:   trace.SessionID,
		Scope:       ScopeTrace,
		Type:        e.evType,
		EvaluatorID: e.id,
		Score:       score,
		CreatedAt:   now(),
	}
}

// EvaluatorRegistry manages registered evaluators
type EvaluatorRegistry struct {
	mu         sync.RWMutex
	evaluators map[string]Evaluator
}

// NewEvaluatorRegistry creates a new evaluator registry
func NewEvaluatorRegistry() *EvaluatorRegistry {
	return &EvaluatorRegistry{
		evaluators: make(map[string]Evaluator),
	}
}

// Register registers an evaluator
func (r *EvaluatorRegistry) Register(e Evaluator) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e == nil {
		return fmt.Errorf("cannot register nil evaluator")
	}

	id := e.ID()
	if id == "" {
		return fmt.Errorf("evaluator ID cannot be empty")
	}

	if _, exists := r.evaluators[id]; exists {
		return fmt.Errorf("evaluator already registered: %s", id)
	}

	r.evaluators[id] = e
	return nil
}

// MustRegister registers an evaluator and panics on error
func (r *EvaluatorRegistry) MustRegister(e Evaluator) {
	if err := r.Register(e); err != nil {
		panic(err)
	}
}

// Unregister removes an evaluator from the registry
func (r *EvaluatorRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.evaluators, id)
}

// Get retrieves an evaluator by ID
func (r *EvaluatorRegistry) Get(id string) (Evaluator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.evaluators[id]
	return e, ok
}

// List returns all registered evaluators
func (r *EvaluatorRegistry) List() []Evaluator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Evaluator, 0, len(r.evaluators))
	for _, e := range r.evaluators {
		result = append(result, e)
	}
	return result
}

// ListByType returns evaluators of a specific type
func (r *EvaluatorRegistry) ListByType(evType EvaluationType) []Evaluator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Evaluator
	for _, e := range r.evaluators {
		if e.Type() == evType {
			result = append(result, e)
		}
	}
	return result
}

// GetIDs returns all registered evaluator IDs
func (r *EvaluatorRegistry) GetIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.evaluators))
	for id := range r.evaluators {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of registered evaluators
func (r *EvaluatorRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.evaluators)
}

// DefaultRegistry is the global default evaluator registry
var DefaultRegistry = NewEvaluatorRegistry()

// Register registers an evaluator in the default registry
func Register(e Evaluator) error {
	return DefaultRegistry.Register(e)
}

// MustRegister registers an evaluator in the default registry, panics on error
func MustRegister(e Evaluator) {
	DefaultRegistry.MustRegister(e)
}

// GetEvaluator retrieves an evaluator from the default registry
func GetEvaluator(id string) (Evaluator, bool) {
	return DefaultRegistry.Get(id)
}

// ListEvaluators returns all evaluators from the default registry
func ListEvaluators() []Evaluator {
	return DefaultRegistry.List()
}

// now returns the current time (can be overridden for testing)
var now = func() time.Time {
	return time.Now()
}
