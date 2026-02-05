package evaluation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ranganaths/minion/tracing"
)

// APIServer provides HTTP endpoints for evaluation management
type APIServer struct {
	store    EvaluationStore
	runner   *BenchmarkRunner
	pipeline *Pipeline
	mux      *http.ServeMux
}

// APIConfig configures the API server
type APIConfig struct {
	// Store is the evaluation store
	Store EvaluationStore

	// Runner is the benchmark runner (optional)
	Runner *BenchmarkRunner

	// Pipeline is the evaluation pipeline (optional)
	Pipeline *Pipeline

	// EnableCORS enables CORS headers
	EnableCORS bool

	// AllowedOrigins for CORS (default: *)
	AllowedOrigins []string
}

// NewAPIServer creates a new API server
func NewAPIServer(config APIConfig) *APIServer {
	server := &APIServer{
		store:    config.Store,
		runner:   config.Runner,
		pipeline: config.Pipeline,
		mux:      http.NewServeMux(),
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures the HTTP routes
func (s *APIServer) setupRoutes() {
	// Evaluations
	s.mux.HandleFunc("/api/v1/evaluations", s.handleEvaluations)
	s.mux.HandleFunc("/api/v1/evaluations/", s.handleEvaluation)
	s.mux.HandleFunc("/api/v1/evaluations/query", s.handleEvaluationsQuery)

	// Agent summaries
	s.mux.HandleFunc("/api/v1/evaluations/agent/", s.handleAgentEvaluations)
	s.mux.HandleFunc("/api/v1/evaluations/trace/", s.handleTraceEvaluations)
	s.mux.HandleFunc("/api/v1/agents/", s.handleAgentSummary)

	// Benchmarks
	s.mux.HandleFunc("/api/v1/benchmarks", s.handleBenchmarks)
	s.mux.HandleFunc("/api/v1/benchmarks/", s.handleBenchmark)

	// Benchmark runs
	s.mux.HandleFunc("/api/v1/benchmark-runs", s.handleBenchmarkRuns)
	s.mux.HandleFunc("/api/v1/benchmark-runs/", s.handleBenchmarkRun)

	// Stats and health
	s.mux.HandleFunc("/api/v1/stats", s.handleStats)
	s.mux.HandleFunc("/api/v1/health", s.handleHealth)
}

// Handler returns the HTTP handler
func (s *APIServer) Handler() http.Handler {
	return s.mux
}

// ServeHTTP implements http.Handler
func (s *APIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mux.ServeHTTP(w, r)
}

// handleEvaluations handles GET /api/v1/evaluations
func (s *APIServer) handleEvaluations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse query parameters into filter
	filter := s.parseFilterFromQuery(r)

	result, err := s.store.ListEvaluations(ctx, filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

// handleEvaluationsQuery handles POST /api/v1/evaluations/query
func (s *APIServer) handleEvaluationsQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	var filter EvaluationFilter
	if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	result, err := s.store.ListEvaluations(ctx, &filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

// handleEvaluation handles GET/DELETE /api/v1/evaluations/{id}
func (s *APIServer) handleEvaluation(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/evaluations/")
	if id == "" {
		http.Error(w, "Missing evaluation ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		eval, err := s.store.GetEvaluation(ctx, EvaluationID(id))
		if err != nil {
			s.writeError(w, http.StatusNotFound, err)
			return
		}
		s.writeJSON(w, http.StatusOK, eval)

	case http.MethodDelete:
		if err := s.store.DeleteEvaluation(ctx, EvaluationID(id)); err != nil {
			s.writeError(w, http.StatusNotFound, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAgentEvaluations handles GET /api/v1/evaluations/agent/{agentId}
func (s *APIServer) handleAgentEvaluations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID := strings.TrimPrefix(r.URL.Path, "/api/v1/evaluations/agent/")
	if agentID == "" {
		http.Error(w, "Missing agent ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	limit := s.parseIntQuery(r, "limit", 100)

	evals, err := s.store.GetEvaluationsByAgent(ctx, agentID, limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id":    agentID,
		"evaluations": evals,
		"count":       len(evals),
	})
}

// handleTraceEvaluations handles GET /api/v1/evaluations/trace/{traceId}
func (s *APIServer) handleTraceEvaluations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	traceID := strings.TrimPrefix(r.URL.Path, "/api/v1/evaluations/trace/")
	if traceID == "" {
		http.Error(w, "Missing trace ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	evals, err := s.store.GetEvaluationsByTrace(ctx, tracing.TraceID(traceID))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"trace_id":    traceID,
		"evaluations": evals,
		"count":       len(evals),
	})
}

// handleAgentSummary handles GET /api/v1/agents/{agentId}/summary
func (s *APIServer) handleAgentSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "summary" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	agentID := parts[0]
	period := TimePeriod(r.URL.Query().Get("period"))
	if period == "" {
		period = Last24Hours
	}

	ctx := r.Context()

	summary, err := s.store.GetAgentSummary(ctx, agentID, period)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, http.StatusOK, summary)
}

// handleBenchmarks handles GET/POST /api/v1/benchmarks
func (s *APIServer) handleBenchmarks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		benchmarks, err := s.store.ListBenchmarks(ctx)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"benchmarks": benchmarks,
			"count":      len(benchmarks),
		})

	case http.MethodPost:
		var benchmark Benchmark
		if err := json.NewDecoder(r.Body).Decode(&benchmark); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}

		if err := s.store.SaveBenchmark(ctx, &benchmark); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}

		s.writeJSON(w, http.StatusCreated, benchmark)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBenchmark handles GET/DELETE /api/v1/benchmarks/{id}
func (s *APIServer) handleBenchmark(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/benchmarks/")
	parts := strings.Split(path, "/")
	id := parts[0]

	if id == "" {
		http.Error(w, "Missing benchmark ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check for sub-routes
	if len(parts) > 1 {
		switch parts[1] {
		case "runs":
			s.handleBenchmarkRunsForBenchmark(w, r, id)
			return
		case "run":
			s.handleRunBenchmark(w, r, id)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		benchmark, err := s.store.GetBenchmark(ctx, id)
		if err != nil {
			s.writeError(w, http.StatusNotFound, err)
			return
		}
		s.writeJSON(w, http.StatusOK, benchmark)

	case http.MethodDelete:
		if err := s.store.DeleteBenchmark(ctx, id); err != nil {
			s.writeError(w, http.StatusNotFound, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBenchmarkRunsForBenchmark handles GET /api/v1/benchmarks/{id}/runs
func (s *APIServer) handleBenchmarkRunsForBenchmark(w http.ResponseWriter, r *http.Request, benchmarkID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	runs, err := s.store.ListBenchmarkRuns(ctx, benchmarkID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"benchmark_id": benchmarkID,
		"runs":         runs,
		"count":        len(runs),
	})
}

// handleRunBenchmark handles POST /api/v1/benchmarks/{id}/run
func (s *APIServer) handleRunBenchmark(w http.ResponseWriter, r *http.Request, benchmarkID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.runner == nil {
		s.writeError(w, http.StatusNotImplemented, fmt.Errorf("benchmark runner not configured"))
		return
	}

	ctx := r.Context()

	// Get benchmark
	benchmark, err := s.store.GetBenchmark(ctx, benchmarkID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}

	// Parse run options
	var opts RunOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		// Use defaults if body is empty or invalid
		opts = *DefaultRunOptions()
	}

	// Note: In a real implementation, you'd need to provide the executor
	// This endpoint would need additional configuration or the executor
	// would be passed in the request body or configured in the runner
	s.writeError(w, http.StatusNotImplemented, fmt.Errorf("executor must be provided - use the runner directly"))
	_ = benchmark // Avoid unused variable
}

// handleBenchmarkRuns handles GET /api/v1/benchmark-runs
func (s *APIServer) handleBenchmarkRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	runs, err := s.store.ListBenchmarkRuns(ctx, "")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"runs":  runs,
		"count": len(runs),
	})
}

// handleBenchmarkRun handles GET /api/v1/benchmark-runs/{id}
func (s *APIServer) handleBenchmarkRun(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/benchmark-runs/")
	if id == "" {
		http.Error(w, "Missing run ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		run, err := s.store.GetBenchmarkRun(ctx, id)
		if err != nil {
			s.writeError(w, http.StatusNotFound, err)
			return
		}
		s.writeJSON(w, http.StatusOK, run)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStats handles GET /api/v1/stats
func (s *APIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	stats, err := s.store.GetStats(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, http.StatusOK, stats)
}

// handleHealth handles GET /api/v1/health
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Helper methods

func (s *APIServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *APIServer) writeError(w http.ResponseWriter, status int, err error) {
	s.writeJSON(w, status, map[string]string{
		"error": err.Error(),
	})
}

func (s *APIServer) parseFilterFromQuery(r *http.Request) *EvaluationFilter {
	q := r.URL.Query()

	filter := &EvaluationFilter{
		AgentID:     q.Get("agent_id"),
		SessionID:   q.Get("session_id"),
		BatchID:     q.Get("batch_id"),
		EvaluatorID: q.Get("evaluator_id"),
		OrderBy:     q.Get("order_by"),
		OrderDesc:   q.Get("order") == "desc",
	}

	if traceID := q.Get("trace_id"); traceID != "" {
		filter.TraceID = tracing.TraceID(traceID)
	}

	if evalType := q.Get("type"); evalType != "" {
		filter.Type = EvaluationType(evalType)
	}

	if scope := q.Get("scope"); scope != "" {
		filter.Scope = EvaluationScope(scope)
	}

	if minScore := q.Get("min_score"); minScore != "" {
		if v, err := strconv.ParseFloat(minScore, 64); err == nil {
			filter.MinScore = &v
		}
	}

	if maxScore := q.Get("max_score"); maxScore != "" {
		if v, err := strconv.ParseFloat(maxScore, 64); err == nil {
			filter.MaxScore = &v
		}
	}

	filter.Limit = s.parseIntQuery(r, "limit", DefaultLimit)
	filter.Offset = s.parseIntQuery(r, "offset", 0)

	return filter
}

func (s *APIServer) parseIntQuery(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	if v, err := strconv.Atoi(val); err == nil {
		return v
	}
	return defaultVal
}
