package evaluation

import (
	"context"
	"sync"
	"time"

	"github.com/Ranganaths/minion/tracing"
)

// Worker processes traces for evaluation in the background
type Worker struct {
	pipeline    *Pipeline
	queue       chan *tracing.Trace
	concurrency int
	wg          sync.WaitGroup
	stopCh      chan struct{}
	running     bool
	mu          sync.Mutex
}

// WorkerConfig configures the evaluation worker
type WorkerConfig struct {
	// Pipeline is the evaluation pipeline to use
	Pipeline *Pipeline

	// QueueSize is the size of the trace queue (default: 100)
	QueueSize int

	// Concurrency is the number of concurrent evaluations (default: 1)
	Concurrency int
}

// NewWorker creates a new evaluation worker
func NewWorker(config WorkerConfig) *Worker {
	queueSize := config.QueueSize
	if queueSize <= 0 {
		queueSize = 100
	}

	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	return &Worker{
		pipeline:    config.Pipeline,
		queue:       make(chan *tracing.Trace, queueSize),
		concurrency: concurrency,
		stopCh:      make(chan struct{}),
	}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	// Start worker goroutines
	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.worker(ctx, i)
	}
}

// Stop stops the worker gracefully
func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	close(w.stopCh)
	w.mu.Unlock()

	// Wait for workers to finish
	w.wg.Wait()
}

// StopWithTimeout stops the worker with a timeout
func (w *Worker) StopWithTimeout(timeout time.Duration) {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	close(w.stopCh)
	w.mu.Unlock()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// Enqueue adds a trace to the evaluation queue
func (w *Worker) Enqueue(trace *tracing.Trace) bool {
	select {
	case w.queue <- trace:
		return true
	default:
		return false // Queue full
	}
}

// EnqueueBlocking adds a trace to the queue, blocking if full
func (w *Worker) EnqueueBlocking(ctx context.Context, trace *tracing.Trace) error {
	select {
	case w.queue <- trace:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// QueueLength returns the current queue length
func (w *Worker) QueueLength() int {
	return len(w.queue)
}

// IsRunning returns whether the worker is running
func (w *Worker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// worker processes traces from the queue
func (w *Worker) worker(ctx context.Context, id int) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case trace, ok := <-w.queue:
			if !ok {
				return
			}
			w.processTrace(ctx, trace)
		}
	}
}

// processTrace evaluates a single trace
func (w *Worker) processTrace(ctx context.Context, trace *tracing.Trace) {
	if w.pipeline == nil {
		return
	}

	// Create a timeout context for each trace
	evalCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	_, _ = w.pipeline.EvaluateTrace(evalCtx, trace)
}

// OnTraceEnd implements tracing.TraceHook for automatic enqueueing
func (w *Worker) OnTraceEnd(trace *tracing.Trace) {
	w.Enqueue(trace)
}

// OnTraceStart implements tracing.TraceHook (no-op)
func (w *Worker) OnTraceStart(trace *tracing.Trace) {}

// OnSpanStart implements tracing.TraceHook (no-op)
func (w *Worker) OnSpanStart(span *tracing.Span) {}

// OnSpanEnd implements tracing.TraceHook (no-op)
func (w *Worker) OnSpanEnd(span *tracing.Span) {}

// BatchWorker processes traces in batches
type BatchWorker struct {
	pipeline      *Pipeline
	buffer        []*tracing.Trace
	batchSize     int
	flushInterval time.Duration
	mu            sync.Mutex
	flushCh       chan struct{}
	stopCh        chan struct{}
	running       bool
}

// BatchWorkerConfig configures the batch worker
type BatchWorkerConfig struct {
	// Pipeline is the evaluation pipeline
	Pipeline *Pipeline

	// BatchSize is the number of traces per batch (default: 10)
	BatchSize int

	// FlushInterval is how often to flush incomplete batches (default: 30s)
	FlushInterval time.Duration
}

// NewBatchWorker creates a new batch worker
func NewBatchWorker(config BatchWorkerConfig) *BatchWorker {
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	flushInterval := config.FlushInterval
	if flushInterval <= 0 {
		flushInterval = 30 * time.Second
	}

	return &BatchWorker{
		pipeline:      config.Pipeline,
		buffer:        make([]*tracing.Trace, 0, batchSize),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
	}
}

// Start starts the batch worker
func (w *BatchWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	go w.flusher(ctx)
}

// Stop stops the batch worker
func (w *BatchWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	close(w.stopCh)
	w.mu.Unlock()

	// Flush remaining traces
	w.flush(context.Background())
}

// Add adds a trace to the batch
func (w *BatchWorker) Add(trace *tracing.Trace) {
	w.mu.Lock()
	w.buffer = append(w.buffer, trace)
	shouldFlush := len(w.buffer) >= w.batchSize
	w.mu.Unlock()

	if shouldFlush {
		select {
		case w.flushCh <- struct{}{}:
		default:
		}
	}
}

// flusher periodically flushes the batch
func (w *BatchWorker) flusher(ctx context.Context) {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.flush(ctx)
		case <-w.flushCh:
			w.flush(ctx)
		}
	}
}

// flush processes all buffered traces
func (w *BatchWorker) flush(ctx context.Context) {
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return
	}
	traces := w.buffer
	w.buffer = make([]*tracing.Trace, 0, w.batchSize)
	w.mu.Unlock()

	if w.pipeline != nil {
		_, _ = w.pipeline.EvaluateBatch(ctx, traces)
	}
}

// OnTraceEnd implements tracing.TraceHook
func (w *BatchWorker) OnTraceEnd(trace *tracing.Trace) {
	w.Add(trace)
}

// OnTraceStart implements tracing.TraceHook (no-op)
func (w *BatchWorker) OnTraceStart(trace *tracing.Trace) {}

// OnSpanStart implements tracing.TraceHook (no-op)
func (w *BatchWorker) OnSpanStart(span *tracing.Span) {}

// OnSpanEnd implements tracing.TraceHook (no-op)
func (w *BatchWorker) OnSpanEnd(span *tracing.Span) {}
