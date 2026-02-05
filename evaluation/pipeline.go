package evaluation

import (
	"context"
	"fmt"
	"sync"

	"github.com/Ranganaths/minion/tracing"
)

// EvaluationHook defines callbacks for evaluation events
type EvaluationHook interface {
	// OnEvaluationStart is called when evaluation begins
	OnEvaluationStart(ctx context.Context, trace *tracing.Trace)

	// OnEvaluationComplete is called when all evaluations complete successfully
	OnEvaluationComplete(ctx context.Context, trace *tracing.Trace, evals []*Evaluation)

	// OnEvaluationError is called when evaluation fails
	OnEvaluationError(ctx context.Context, trace *tracing.Trace, err error)
}

// NoOpEvaluationHook is a no-op implementation of EvaluationHook
type NoOpEvaluationHook struct{}

func (h *NoOpEvaluationHook) OnEvaluationStart(ctx context.Context, trace *tracing.Trace)              {}
func (h *NoOpEvaluationHook) OnEvaluationComplete(ctx context.Context, trace *tracing.Trace, evals []*Evaluation) {}
func (h *NoOpEvaluationHook) OnEvaluationError(ctx context.Context, trace *tracing.Trace, err error)  {}

// Pipeline orchestrates automated evaluation of traces
type Pipeline struct {
	mu          sync.RWMutex
	store       EvaluationStore
	traceStore  tracing.TraceStore
	evaluators  []Evaluator
	hooks       []EvaluationHook
	parallel    bool
	stopOnError bool
}

// PipelineConfig configures the evaluation pipeline
type PipelineConfig struct {
	// Store is the evaluation store for persisting results
	Store EvaluationStore

	// TraceStore is the trace store (optional)
	TraceStore tracing.TraceStore

	// Evaluators are the evaluators to run
	Evaluators []Evaluator

	// Hooks are evaluation event hooks
	Hooks []EvaluationHook

	// Parallel enables parallel evaluator execution
	Parallel bool

	// StopOnError stops evaluation on first error
	StopOnError bool
}

// NewPipeline creates a new evaluation pipeline
func NewPipeline(config PipelineConfig) *Pipeline {
	return &Pipeline{
		store:       config.Store,
		traceStore:  config.TraceStore,
		evaluators:  config.Evaluators,
		hooks:       config.Hooks,
		parallel:    config.Parallel,
		stopOnError: config.StopOnError,
	}
}

// PipelineOption is a functional option for configuring a pipeline
type PipelineOption func(*Pipeline)

// WithStore sets the evaluation store
func WithStore(store EvaluationStore) PipelineOption {
	return func(p *Pipeline) {
		p.store = store
	}
}

// WithTraceStore sets the trace store
func WithTraceStore(store tracing.TraceStore) PipelineOption {
	return func(p *Pipeline) {
		p.traceStore = store
	}
}

// WithEvaluators sets the evaluators
func WithEvaluators(evaluators ...Evaluator) PipelineOption {
	return func(p *Pipeline) {
		p.evaluators = evaluators
	}
}

// WithHooks sets the evaluation hooks
func WithHooks(hooks ...EvaluationHook) PipelineOption {
	return func(p *Pipeline) {
		p.hooks = hooks
	}
}

// WithParallel enables parallel evaluation
func WithParallel(parallel bool) PipelineOption {
	return func(p *Pipeline) {
		p.parallel = parallel
	}
}

// WithStopOnError enables stop on error
func WithStopOnError(stop bool) PipelineOption {
	return func(p *Pipeline) {
		p.stopOnError = stop
	}
}

// NewPipelineWithOptions creates a pipeline with functional options
func NewPipelineWithOptions(opts ...PipelineOption) *Pipeline {
	p := &Pipeline{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// AddEvaluator adds an evaluator to the pipeline
func (p *Pipeline) AddEvaluator(evaluator Evaluator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evaluators = append(p.evaluators, evaluator)
}

// AddHook adds an evaluation hook
func (p *Pipeline) AddHook(hook EvaluationHook) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hooks = append(p.hooks, hook)
}

// EvaluateTrace evaluates a single trace with all configured evaluators
func (p *Pipeline) EvaluateTrace(ctx context.Context, trace *tracing.Trace) ([]*Evaluation, error) {
	if trace == nil {
		return nil, fmt.Errorf("trace cannot be nil")
	}

	p.mu.RLock()
	evaluators := make([]Evaluator, len(p.evaluators))
	copy(evaluators, p.evaluators)
	hooks := make([]EvaluationHook, len(p.hooks))
	copy(hooks, p.hooks)
	p.mu.RUnlock()

	if len(evaluators) == 0 {
		return nil, fmt.Errorf("no evaluators configured")
	}

	// Notify hooks of start
	for _, hook := range hooks {
		hook.OnEvaluationStart(ctx, trace)
	}

	// Run evaluators
	var evals []*Evaluation
	var evalErr error

	if p.parallel {
		evals, evalErr = p.evaluateParallel(ctx, trace, evaluators)
	} else {
		evals, evalErr = p.evaluateSequential(ctx, trace, evaluators)
	}

	if evalErr != nil {
		for _, hook := range hooks {
			hook.OnEvaluationError(ctx, trace, evalErr)
		}
		return evals, evalErr
	}

	// Store evaluations
	if p.store != nil {
		for _, eval := range evals {
			if err := p.store.SaveEvaluation(ctx, eval); err != nil {
				// Log but don't fail
				continue
			}
		}
	}

	// Notify hooks of completion
	for _, hook := range hooks {
		hook.OnEvaluationComplete(ctx, trace, evals)
	}

	return evals, nil
}

// evaluateSequential runs evaluators one at a time
func (p *Pipeline) evaluateSequential(ctx context.Context, trace *tracing.Trace, evaluators []Evaluator) ([]*Evaluation, error) {
	evals := make([]*Evaluation, 0, len(evaluators))

	for _, evaluator := range evaluators {
		select {
		case <-ctx.Done():
			return evals, ctx.Err()
		default:
		}

		eval, err := evaluator.Evaluate(ctx, trace)
		if err != nil {
			if p.stopOnError {
				return evals, fmt.Errorf("evaluator %s failed: %w", evaluator.ID(), err)
			}
			continue
		}

		evals = append(evals, eval)
	}

	return evals, nil
}

// evaluateParallel runs evaluators concurrently
func (p *Pipeline) evaluateParallel(ctx context.Context, trace *tracing.Trace, evaluators []Evaluator) ([]*Evaluation, error) {
	type result struct {
		eval *Evaluation
		err  error
	}

	results := make(chan result, len(evaluators))
	var wg sync.WaitGroup

	for _, evaluator := range evaluators {
		wg.Add(1)
		go func(e Evaluator) {
			defer wg.Done()
			eval, err := e.Evaluate(ctx, trace)
			results <- result{eval: eval, err: err}
		}(evaluator)
	}

	// Close results channel when all evaluators complete
	go func() {
		wg.Wait()
		close(results)
	}()

	var evals []*Evaluation
	var firstErr error

	for r := range results {
		if r.err != nil {
			if p.stopOnError && firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		if r.eval != nil {
			evals = append(evals, r.eval)
		}
	}

	return evals, firstErr
}

// EvaluateBatch evaluates multiple traces
func (p *Pipeline) EvaluateBatch(ctx context.Context, traces []*tracing.Trace) (map[tracing.TraceID][]*Evaluation, error) {
	results := make(map[tracing.TraceID][]*Evaluation)

	for _, trace := range traces {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		evals, err := p.EvaluateTrace(ctx, trace)
		if err != nil {
			if p.stopOnError {
				return results, err
			}
			continue
		}

		results[trace.ID] = evals
	}

	return results, nil
}

// OnTraceEnd implements tracing.TraceHook for automatic evaluation
func (p *Pipeline) OnTraceEnd(trace *tracing.Trace) {
	// Run evaluation in background
	go func() {
		ctx := context.Background()
		_, _ = p.EvaluateTrace(ctx, trace)
	}()
}

// OnTraceStart implements tracing.TraceHook (no-op)
func (p *Pipeline) OnTraceStart(trace *tracing.Trace) {}

// OnSpanStart implements tracing.TraceHook (no-op)
func (p *Pipeline) OnSpanStart(span *tracing.Span) {}

// OnSpanEnd implements tracing.TraceHook (no-op)
func (p *Pipeline) OnSpanEnd(span *tracing.Span) {}

// Ensure Pipeline implements tracing.TraceHook
// Note: This requires the TraceHook interface to exist in tracing package
// var _ tracing.TraceHook = (*Pipeline)(nil)
