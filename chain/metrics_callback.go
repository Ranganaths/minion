package chain

import (
	"context"
	"time"

	"github.com/Ranganaths/minion/metrics"
)

// MetricsCallback is a chain callback that records metrics for chain execution.
// It implements ChainCallback and tracks:
// - Chain call counts (total, success, error)
// - Chain call duration (histogram)
// - LLM calls and token usage
// - Retriever call counts
type MetricsCallback struct {
	// chainCalls tracks total chain call attempts per chain name
	chainCalls func(chainName string) metrics.Counter
	// chainErrors tracks chain errors per chain name
	chainErrors func(chainName string) metrics.Counter
	// chainDuration tracks chain execution duration per chain name
	chainDuration func(chainName string) metrics.Histogram
	// llmCalls tracks LLM call counts
	llmCalls metrics.Counter
	// llmTokens tracks LLM token usage
	llmTokens metrics.Counter
	// llmDuration tracks LLM call duration
	llmDuration metrics.Histogram
	// retrieverCalls tracks retriever call counts
	retrieverCalls metrics.Counter
	// retrieverDocs tracks documents retrieved (gauge per call)
	retrieverDocs metrics.Counter

	// Track active chain calls for duration measurement
	chainStarts map[string]time.Time
}

// NewMetricsCallback creates a new metrics callback using the global metrics provider.
func NewMetricsCallback() *MetricsCallback {
	m := metrics.GetMetrics()
	return &MetricsCallback{
		chainCalls: func(chainName string) metrics.Counter {
			return m.Counter(metrics.MetricChainCallsTotal, metrics.Labels{"chain": chainName})
		},
		chainErrors: func(chainName string) metrics.Counter {
			return m.Counter(metrics.MetricChainCallErrors, metrics.Labels{"chain": chainName})
		},
		chainDuration: func(chainName string) metrics.Histogram {
			return m.Histogram(metrics.MetricChainCallDuration, metrics.Labels{"chain": chainName})
		},
		llmCalls:       m.Counter(metrics.MetricLLMCallsTotal, nil),
		llmTokens:      m.Counter(metrics.MetricLLMTokensUsed, nil),
		llmDuration:    m.Histogram(metrics.MetricLLMCallDuration, nil),
		retrieverCalls: m.Counter(metrics.MetricVectorStoreSearches, nil),
		retrieverDocs:  m.Counter(metrics.MetricVectorStoreDocuments, nil),
		chainStarts:    make(map[string]time.Time),
	}
}

// NewMetricsCallbackWithProvider creates a metrics callback with a specific metrics provider.
func NewMetricsCallbackWithProvider(m metrics.Metrics) *MetricsCallback {
	return &MetricsCallback{
		chainCalls: func(chainName string) metrics.Counter {
			return m.Counter(metrics.MetricChainCallsTotal, metrics.Labels{"chain": chainName})
		},
		chainErrors: func(chainName string) metrics.Counter {
			return m.Counter(metrics.MetricChainCallErrors, metrics.Labels{"chain": chainName})
		},
		chainDuration: func(chainName string) metrics.Histogram {
			return m.Histogram(metrics.MetricChainCallDuration, metrics.Labels{"chain": chainName})
		},
		llmCalls:       m.Counter(metrics.MetricLLMCallsTotal, nil),
		llmTokens:      m.Counter(metrics.MetricLLMTokensUsed, nil),
		llmDuration:    m.Histogram(metrics.MetricLLMCallDuration, nil),
		retrieverCalls: m.Counter(metrics.MetricVectorStoreSearches, nil),
		retrieverDocs:  m.Counter(metrics.MetricVectorStoreDocuments, nil),
		chainStarts:    make(map[string]time.Time),
	}
}

// OnChainStart is called when a chain begins execution.
func (mc *MetricsCallback) OnChainStart(ctx context.Context, chainName string, inputs map[string]any) {
	mc.chainCalls(chainName).Inc()
	mc.chainStarts[chainName] = time.Now()
}

// OnChainEnd is called when a chain completes successfully.
func (mc *MetricsCallback) OnChainEnd(ctx context.Context, chainName string, outputs map[string]any) {
	if start, ok := mc.chainStarts[chainName]; ok {
		mc.chainDuration(chainName).Observe(time.Since(start).Seconds())
		delete(mc.chainStarts, chainName)
	}
}

// OnChainError is called when a chain encounters an error.
func (mc *MetricsCallback) OnChainError(ctx context.Context, chainName string, err error) {
	mc.chainErrors(chainName).Inc()
	if start, ok := mc.chainStarts[chainName]; ok {
		mc.chainDuration(chainName).Observe(time.Since(start).Seconds())
		delete(mc.chainStarts, chainName)
	}
}

// OnLLMStart is called before an LLM call.
func (mc *MetricsCallback) OnLLMStart(ctx context.Context, prompt string) {
	mc.llmCalls.Inc()
}

// OnLLMEnd is called after an LLM call completes.
func (mc *MetricsCallback) OnLLMEnd(ctx context.Context, response string, tokens int) {
	mc.llmTokens.Add(float64(tokens))
}

// OnRetrieverStart is called before document retrieval.
func (mc *MetricsCallback) OnRetrieverStart(ctx context.Context, query string) {
	mc.retrieverCalls.Inc()
}

// OnRetrieverEnd is called after documents are retrieved.
func (mc *MetricsCallback) OnRetrieverEnd(ctx context.Context, docs []Document) {
	mc.retrieverDocs.Add(float64(len(docs)))
}

// Ensure MetricsCallback implements ChainCallback
var _ ChainCallback = (*MetricsCallback)(nil)
