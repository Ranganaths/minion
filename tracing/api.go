// Package tracing provides the HTTP API for trace observability.
package tracing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TracingAPIServer provides HTTP endpoints for trace observability
type TracingAPIServer struct {
	store     TraceStore
	config    APIConfig
	server    *http.Server
	startTime time.Time
}

// APIConfig configures the tracing API server
type APIConfig struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxHeaderBytes int
	EnableCORS     bool
	CORSOrigins    []string
	BasePath       string
}

// DefaultAPIConfig returns sensible default configuration
func DefaultAPIConfig() APIConfig {
	return APIConfig{
		Addr:           ":8081",
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
		EnableCORS:     true,
		CORSOrigins:    []string{"*"},
		BasePath:       "/api/v1/traces",
	}
}

// NewTracingAPIServer creates a new tracing API server
func NewTracingAPIServer(store TraceStore, config APIConfig) *TracingAPIServer {
	s := &TracingAPIServer{
		store:     store,
		config:    config,
		startTime: time.Now(),
	}

	mux := http.NewServeMux()

	basePath := strings.TrimSuffix(config.BasePath, "/")

	// Health and stats
	mux.HandleFunc(basePath+"/health", s.handleHealth)
	mux.HandleFunc(basePath+"/stats", s.handleStats)

	// Traces
	mux.HandleFunc(basePath, s.handleTraces)                   // GET (list), POST (query)
	mux.HandleFunc(basePath+"/", s.handleTraceByID)            // GET, DELETE by ID
	mux.HandleFunc(basePath+"/summaries", s.handleSummaries)   // GET summaries

	// By agent and session
	mux.HandleFunc(basePath+"/by-agent/", s.handleTracesByAgent)     // GET by agent
	mux.HandleFunc(basePath+"/by-session/", s.handleTracesBySession) // GET by session

	// Spans
	mux.HandleFunc(basePath+"/spans/", s.handleSpan) // GET span by trace_id/span_id

	// Export
	mux.HandleFunc(basePath+"/export/", s.handleExport)

	// Cleanup
	mux.HandleFunc(basePath+"/cleanup", s.handleCleanup) // POST cleanup

	// Apply middleware
	handler := http.Handler(mux)
	if config.EnableCORS {
		handler = s.corsMiddleware(handler)
	}
	handler = s.loggingMiddleware(handler)
	handler = s.recoveryMiddleware(handler)

	s.server = &http.Server{
		Addr:           config.Addr,
		Handler:        handler,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}

	return s
}

// Start starts the server
func (s *TracingAPIServer) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *TracingAPIServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Middleware

func (s *TracingAPIServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := "*"
		if len(s.config.CORSOrigins) > 0 && s.config.CORSOrigins[0] != "*" {
			origin = s.config.CORSOrigins[0]
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *TracingAPIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = time.Since(start) // Log duration if needed
	})
}

func (s *TracingAPIServer) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("internal error: %v", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Handlers

func (s *TracingAPIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := s.store.Stats(ctx)

	resp := map[string]interface{}{
		"status":  "healthy",
		"version": "1.0.0",
		"uptime":  time.Since(s.startTime).String(),
		"storeOK": err == nil,
	}

	if err != nil {
		resp["status"] = "degraded"
		resp["message"] = err.Error()
	}

	if stats != nil {
		resp["totalTraces"] = stats.TotalTraces
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *TracingAPIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := s.store.Stats(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, stats)
}

func (s *TracingAPIServer) handleTraces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// List traces
		limit := s.getIntParam(r, "limit", 100)
		offset := s.getIntParam(r, "offset", 0)

		summaries, err := s.store.GetTraceSummaries(ctx, limit, offset)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"traces":  summaries,
			"limit":   limit,
			"offset":  offset,
			"hasMore": len(summaries) == limit,
		})

	case http.MethodPost:
		// Query traces with filters
		var query TraceQuery
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if query.Limit <= 0 {
			query.Limit = 100
		}

		result, err := s.store.QueryTraces(ctx, &query)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.writeJSON(w, http.StatusOK, result)

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *TracingAPIServer) handleTraceByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract trace ID from path
	basePath := strings.TrimSuffix(s.config.BasePath, "/") + "/"
	traceID := strings.TrimPrefix(r.URL.Path, basePath)

	// Handle trailing paths
	if strings.Contains(traceID, "/") {
		parts := strings.SplitN(traceID, "/", 2)
		traceID = parts[0]
		subPath := parts[1]

		// Handle sub-paths
		if subPath == "tree" {
			s.handleTraceTree(w, r, TraceID(traceID))
			return
		}
	}

	if traceID == "" {
		s.writeError(w, http.StatusBadRequest, "trace_id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		trace, err := s.store.GetTrace(ctx, TraceID(traceID))
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				s.writeError(w, http.StatusNotFound, err.Error())
			} else {
				s.writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}

		// Optionally include summary only
		if r.URL.Query().Get("summary") == "true" {
			s.writeJSON(w, http.StatusOK, trace.ToSummary())
			return
		}

		s.writeJSON(w, http.StatusOK, trace)

	case http.MethodDelete:
		if err := s.store.DeleteTrace(ctx, TraceID(traceID)); err != nil {
			if strings.Contains(err.Error(), "not found") {
				s.writeError(w, http.StatusNotFound, err.Error())
			} else {
				s.writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *TracingAPIServer) handleTraceTree(w http.ResponseWriter, r *http.Request, traceID TraceID) {
	ctx := r.Context()

	trace, err := s.store.GetTrace(ctx, traceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeError(w, http.StatusNotFound, err.Error())
		} else {
			s.writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	tree := trace.SpanTree()
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"traceID": traceID,
		"tree":    tree,
	})
}

func (s *TracingAPIServer) handleSummaries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := s.getIntParam(r, "limit", 100)
	offset := s.getIntParam(r, "offset", 0)

	summaries, err := s.store.GetTraceSummaries(ctx, limit, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"summaries": summaries,
		"limit":     limit,
		"offset":    offset,
		"hasMore":   len(summaries) == limit,
	})
}

func (s *TracingAPIServer) handleTracesByAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	basePath := strings.TrimSuffix(s.config.BasePath, "/") + "/by-agent/"
	agentID := strings.TrimPrefix(r.URL.Path, basePath)

	if agentID == "" {
		s.writeError(w, http.StatusBadRequest, "agent_id required")
		return
	}

	limit := s.getIntParam(r, "limit", 100)
	offset := s.getIntParam(r, "offset", 0)

	traces, err := s.store.GetTracesByAgent(ctx, agentID, limit, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to summaries for listing
	summaries := make([]*TraceSummary, len(traces))
	for i, trace := range traces {
		summaries[i] = trace.ToSummary()
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"agentID":   agentID,
		"traces":    summaries,
		"limit":     limit,
		"offset":    offset,
		"hasMore":   len(traces) == limit,
	})
}

func (s *TracingAPIServer) handleTracesBySession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	basePath := strings.TrimSuffix(s.config.BasePath, "/") + "/by-session/"
	sessionID := strings.TrimPrefix(r.URL.Path, basePath)

	if sessionID == "" {
		s.writeError(w, http.StatusBadRequest, "session_id required")
		return
	}

	limit := s.getIntParam(r, "limit", 100)
	offset := s.getIntParam(r, "offset", 0)

	traces, err := s.store.GetTracesBySession(ctx, sessionID, limit, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to summaries for listing
	summaries := make([]*TraceSummary, len(traces))
	for i, trace := range traces {
		summaries[i] = trace.ToSummary()
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessionID": sessionID,
		"traces":    summaries,
		"limit":     limit,
		"offset":    offset,
		"hasMore":   len(traces) == limit,
	})
}

func (s *TracingAPIServer) handleSpan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	basePath := strings.TrimSuffix(s.config.BasePath, "/") + "/spans/"
	path := strings.TrimPrefix(r.URL.Path, basePath)

	// Expected format: {trace_id}/{span_id}
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		s.writeError(w, http.StatusBadRequest, "path format: /spans/{trace_id}/{span_id}")
		return
	}

	traceID := TraceID(parts[0])
	spanID := SpanID(parts[1])

	span, err := s.store.GetSpan(ctx, traceID, spanID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeError(w, http.StatusNotFound, err.Error())
		} else {
			s.writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.writeJSON(w, http.StatusOK, span)
}

func (s *TracingAPIServer) handleExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	basePath := strings.TrimSuffix(s.config.BasePath, "/") + "/export/"
	traceID := strings.TrimPrefix(r.URL.Path, basePath)

	if traceID == "" {
		s.writeError(w, http.StatusBadRequest, "trace_id required")
		return
	}

	trace, err := s.store.GetTrace(ctx, TraceID(traceID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeError(w, http.StatusNotFound, err.Error())
		} else {
			s.writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=trace_%s.json", traceID))
		json.NewEncoder(w).Encode(trace)

	default:
		s.writeError(w, http.StatusBadRequest, "unsupported format: "+format)
	}
}

func (s *TracingAPIServer) handleCleanup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		OlderThanDays int `json:"older_than_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.OlderThanDays <= 0 {
		req.OlderThanDays = 30 // Default to 30 days
	}

	cutoff := time.Now().AddDate(0, 0, -req.OlderThanDays)
	deleted, err := s.store.DeleteTracesBefore(ctx, cutoff)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted":       deleted,
		"olderThanDays": req.OlderThanDays,
		"cutoff":        cutoff,
	})
}

// Helper methods

func (s *TracingAPIServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *TracingAPIServer) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

func (s *TracingAPIServer) getIntParam(r *http.Request, name string, defaultValue int) int {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultValue
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	return defaultValue
}

// API Response types for documentation

// TraceListResponse is returned when listing traces
type TraceListResponse struct {
	Traces  []*TraceSummary `json:"traces"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
	HasMore bool            `json:"has_more"`
}

// SpanResponse is returned when getting a single span
type SpanResponse struct {
	Span *Span `json:"span"`
}

// ErrorResponse is returned on errors
type ErrorResponse struct {
	Error string `json:"error"`
}

// CleanupRequest is the request body for cleanup
type CleanupRequest struct {
	OlderThanDays int `json:"older_than_days"`
}

// CleanupResponse is returned after cleanup
type CleanupResponse struct {
	Deleted       int64     `json:"deleted"`
	OlderThanDays int       `json:"older_than_days"`
	Cutoff        time.Time `json:"cutoff"`
}
