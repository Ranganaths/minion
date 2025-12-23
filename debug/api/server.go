// Package api provides the Debug API server for time-travel debugging.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Ranganaths/minion/debug/snapshot"
	"github.com/Ranganaths/minion/debug/timetravel"
)

// DebugServer is the HTTP server for the Debug API.
type DebugServer struct {
	store     snapshot.SnapshotStore
	branching *timetravel.BranchingEngine
	config    ServerConfig
	server    *http.Server
	startTime time.Time

	// Timeline cache
	mu        sync.RWMutex
	timelines map[string]*timetravel.ExecutionTimeline
}

// ServerConfig configures the debug server.
type ServerConfig struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxHeaderBytes int
	EnableCORS     bool
	CORSOrigins    []string
}

// DefaultServerConfig returns sensible default configuration.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:           ":8080",
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
		EnableCORS:     true,
		CORSOrigins:    []string{"*"},
	}
}

// NewDebugServer creates a new debug server.
func NewDebugServer(store snapshot.SnapshotStore, config ServerConfig) *DebugServer {
	s := &DebugServer{
		store:     store,
		branching: timetravel.NewBranchingEngine(store),
		config:    config,
		timelines: make(map[string]*timetravel.ExecutionTimeline),
		startTime: time.Now(),
	}

	mux := http.NewServeMux()

	// Health and stats
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/stats", s.handleStats)

	// Executions
	mux.HandleFunc("/api/v1/executions", s.handleExecutions)
	mux.HandleFunc("/api/v1/executions/", s.handleExecutionByID)

	// Timeline
	mux.HandleFunc("/api/v1/timeline/", s.handleTimeline)
	mux.HandleFunc("/api/v1/step/", s.handleStep)

	// State
	mux.HandleFunc("/api/v1/state/", s.handleState)
	mux.HandleFunc("/api/v1/compare-states/", s.handleCompareStates)

	// Replay
	mux.HandleFunc("/api/v1/replay", s.handleReplay)

	// Branches
	mux.HandleFunc("/api/v1/branches", s.handleBranches)
	mux.HandleFunc("/api/v1/branches/", s.handleBranchByID)
	mux.HandleFunc("/api/v1/compare-branches", s.handleCompareBranches)
	mux.HandleFunc("/api/v1/what-if", s.handleWhatIf)

	// Search and query
	mux.HandleFunc("/api/v1/search", s.handleSearch)
	mux.HandleFunc("/api/v1/query", s.handleQuery)

	// Export
	mux.HandleFunc("/api/v1/export/", s.handleExport)

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

// Start starts the server.
func (s *DebugServer) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *DebugServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Middleware

func (s *DebugServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := "*"
		if len(s.config.CORSOrigins) > 0 && s.config.CORSOrigins[0] != "*" {
			origin = s.config.CORSOrigins[0]
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *DebugServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = time.Since(start) // Log duration if needed
	})
}

func (s *DebugServer) recoveryMiddleware(next http.Handler) http.Handler {
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

func (s *DebugServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := s.store.Stats(ctx)

	resp := HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
		Uptime:  time.Since(s.startTime).String(),
		StoreOK: err == nil,
	}

	if err != nil {
		resp.Status = "degraded"
		resp.Message = err.Error()
	}

	_ = stats // Could include store info

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *DebugServer) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := s.store.Stats(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, StatsResponse{Stats: stats})
}

func (s *DebugServer) handleExecutions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := s.getIntParam(r, "limit", 100)
	offset := s.getIntParam(r, "offset", 0)

	executions, err := s.store.ListExecutions(ctx, limit, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, ExecutionListResponse{
		Executions: executions,
		TotalCount: int64(len(executions)),
		HasMore:    len(executions) == limit,
	})
}

func (s *DebugServer) handleExecutionByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	executionID := r.URL.Path[len("/api/v1/executions/"):]

	if executionID == "" {
		s.writeError(w, http.StatusBadRequest, "execution_id required")
		return
	}

	summary, err := s.store.GetExecutionSummary(ctx, executionID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	resp := ExecutionDetailResponse{
		Summary: summary,
	}

	// Include snapshots if requested
	if r.URL.Query().Get("include_snapshots") == "true" {
		snapshots, _ := s.store.GetByExecution(ctx, executionID)
		resp.Snapshots = snapshots
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *DebugServer) handleTimeline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	executionID := r.URL.Path[len("/api/v1/timeline/"):]

	if executionID == "" {
		s.writeError(w, http.StatusBadRequest, "execution_id required")
		return
	}

	timeline, err := s.getOrCreateTimeline(ctx, executionID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	resp := TimelineResponse{
		ExecutionID: executionID,
		Summary:     timeline.Summary(),
		Position:    timeline.Position(),
		Length:      timeline.Length(),
	}

	if r.URL.Query().Get("include_snapshots") == "true" {
		resp.Snapshots = timeline.All()
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *DebugServer) handleStep(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req StepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	timeline, err := s.getOrCreateTimeline(ctx, req.ExecutionID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var current *snapshot.ExecutionSnapshot

	switch req.Direction {
	case "forward":
		if req.Count > 1 {
			current = timeline.StepForwardN(req.Count)
		} else {
			current = timeline.StepForward()
		}
	case "backward":
		if req.Count > 1 {
			current = timeline.StepBackwardN(req.Count)
		} else {
			current = timeline.StepBackward()
		}
	case "first":
		current = timeline.First()
	case "last":
		current = timeline.Last()
	case "jump":
		if req.TargetSeq > 0 {
			current = timeline.JumpTo(req.TargetSeq)
		} else {
			current = timeline.JumpToIndex(req.TargetIndex)
		}
	case "next_checkpoint":
		current = timeline.JumpToNextCheckpoint(req.Checkpoint)
	case "prev_checkpoint":
		current = timeline.JumpToPrevCheckpoint(req.Checkpoint)
	case "next_error":
		current = timeline.JumpToNextError()
	case "prev_error":
		current = timeline.JumpToPrevError()
	default:
		s.writeError(w, http.StatusBadRequest, "invalid direction")
		return
	}

	resp := StepResponse{
		Current:     current,
		Position:    timeline.Position(),
		Total:       timeline.Length(),
		CanForward:  timeline.CanStepForward(),
		CanBackward: timeline.CanStepBackward(),
		Progress:    timeline.Progress(),
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *DebugServer) handleState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	executionID := r.URL.Path[len("/api/v1/state/"):]

	seqNum := s.getInt64Param(r, "sequence", 0)

	timeline, err := s.getOrCreateTimeline(ctx, executionID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	reconstructor := timetravel.NewStateReconstructor(timeline)

	var state *timetravel.ReconstructedState
	if seqNum > 0 {
		state, err = reconstructor.ReconstructAt(seqNum)
	} else {
		state, err = reconstructor.ReconstructCurrent()
	}

	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, StateResponse{State: state})
}

func (s *DebugServer) handleCompareStates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req CompareStatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	timeline, err := s.getOrCreateTimeline(ctx, req.ExecutionID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	reconstructor := timetravel.NewStateReconstructor(timeline)
	comparison, err := reconstructor.CompareStates(req.SequenceNum1, req.SequenceNum2)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, CompareStatesResponse{Comparison: comparison})
}

func (s *DebugServer) handleReplay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req ReplayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	timeline, err := s.getOrCreateTimeline(ctx, req.ExecutionID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	replayEngine := timetravel.NewReplayEngine(s.store, timeline)

	opts := &timetravel.ReplayOptions{
		Mode:                req.Mode,
		Modification:        req.Modification,
		CompareWithOriginal: req.Compare,
	}

	if req.StopAt != nil {
		opts.StopAtCheckpoint = req.StopAt.Checkpoint
		opts.StopAtSequence = req.StopAt.Sequence
		opts.MaxSteps = req.StopAt.MaxSteps
		if req.StopAt.Timeout != "" {
			if d, err := time.ParseDuration(req.StopAt.Timeout); err == nil {
				opts.Timeout = d
			}
		}
	}

	result, err := replayEngine.ReplayFrom(ctx, req.FromSequence, opts)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, ReplayResponse{Result: result})
}

func (s *DebugServer) handleBranches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		executionID := r.URL.Query().Get("execution_id")
		var branches []*timetravel.ExecutionBranch
		var tree *timetravel.BranchTree

		if executionID != "" {
			branches = s.branching.ListBranches(executionID)
			tree = s.branching.GetBranchTree(executionID)
		} else {
			branches = s.branching.ListAllBranches()
		}

		s.writeJSON(w, http.StatusOK, BranchListResponse{
			Branches: branches,
			Tree:     tree,
		})

	case http.MethodPost:
		var req BranchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		branch, err := s.branching.CreateBranch(ctx, req.ExecutionID, req.SequenceNum, &timetravel.CreateBranchOptions{
			Name:         req.Name,
			Description:  req.Description,
			Modification: req.Modification,
		})
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.writeJSON(w, http.StatusCreated, BranchResponse{Branch: branch})

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *DebugServer) handleBranchByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	branchID := r.URL.Path[len("/api/v1/branches/"):]

	// Check for sub-paths
	if len(branchID) > 0 {
		parts := splitPath(branchID)
		branchID = parts[0]

		if len(parts) > 1 && parts[1] == "execute" {
			s.executeBranch(ctx, w, r, branchID)
			return
		}
		if len(parts) > 1 && parts[1] == "compare" {
			s.compareBranchWithParent(ctx, w, r, branchID)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		branch, err := s.branching.GetBranch(branchID)
		if err != nil {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeJSON(w, http.StatusOK, BranchResponse{Branch: branch})

	case http.MethodDelete:
		if err := s.branching.DeleteBranch(branchID); err != nil {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *DebugServer) executeBranch(ctx context.Context, w http.ResponseWriter, r *http.Request, branchID string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req ExecuteBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body
		req = ExecuteBranchRequest{}
	}

	opts := &timetravel.ReplayOptions{
		Mode:                req.Mode,
		CompareWithOriginal: req.Compare,
	}

	result, err := s.branching.ExecuteBranch(ctx, branchID, opts)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := ExecuteBranchResponse{Result: result}

	if req.Compare {
		comparison, _ := s.branching.CompareWithParent(ctx, branchID)
		resp.Comparison = comparison
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *DebugServer) compareBranchWithParent(ctx context.Context, w http.ResponseWriter, r *http.Request, branchID string) {
	comparison, err := s.branching.CompareWithParent(ctx, branchID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, CompareBranchesResponse{Comparison: comparison})
}

func (s *DebugServer) handleCompareBranches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req CompareBranchesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	comparison, err := s.branching.CompareBranches(ctx, req.BranchID1, req.BranchID2)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, CompareBranchesResponse{Comparison: comparison})
}

func (s *DebugServer) handleWhatIf(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req WhatIfRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	comparisons, err := s.branching.WhatIfMultiple(ctx, req.ExecutionID, req.SequenceNum, req.Modifications)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, WhatIfResponse{Comparisons: comparisons})
}

func (s *DebugServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Build filter from search request
	filter := snapshot.SnapshotFilter{}
	if agentID, ok := req.Filters["agent_id"]; ok {
		filter.AgentID = agentID
	}
	if taskID, ok := req.Filters["task_id"]; ok {
		filter.TaskID = taskID
	}
	if cpType, ok := req.Filters["checkpoint_type"]; ok {
		filter.CheckpointType = snapshot.CheckpointType(cpType)
	}

	query := &snapshot.SnapshotQuery{
		Filter: filter,
		Limit:  req.Limit,
		Offset: req.Offset,
	}

	result, err := s.store.Query(ctx, query)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to search results
	var results []*SearchResult
	for _, snap := range result.Snapshots {
		results = append(results, &SearchResult{
			ExecutionID: snap.ExecutionID,
			SequenceNum: snap.SequenceNum,
			Snapshot:    snap,
			Score:       1.0, // Simple search doesn't score
		})
	}

	s.writeJSON(w, http.StatusOK, SearchResponse{
		Results:    results,
		TotalCount: result.TotalCount,
		HasMore:    result.HasMore,
	})
}

func (s *DebugServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req SnapshotQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	query := &snapshot.SnapshotQuery{
		Filter:  req.Filter,
		Limit:   req.Limit,
		Offset:  req.Offset,
		OrderBy: req.OrderBy,
	}

	result, err := s.store.Query(ctx, query)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, SnapshotQueryResponse{
		Snapshots:  result.Snapshots,
		TotalCount: result.TotalCount,
		HasMore:    result.HasMore,
	})
}

func (s *DebugServer) handleExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	executionID := r.URL.Path[len("/api/v1/export/"):]

	if executionID == "" {
		s.writeError(w, http.StatusBadRequest, "execution_id required")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	timeline, err := s.getOrCreateTimeline(ctx, executionID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	export := timeline.ToJSON()

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", executionID))
		json.NewEncoder(w).Encode(export)
	default:
		s.writeError(w, http.StatusBadRequest, "unsupported format")
	}
}

// Helper methods

func (s *DebugServer) getOrCreateTimeline(ctx context.Context, executionID string) (*timetravel.ExecutionTimeline, error) {
	s.mu.RLock()
	timeline, ok := s.timelines[executionID]
	s.mu.RUnlock()

	if ok {
		return timeline, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if timeline, ok := s.timelines[executionID]; ok {
		return timeline, nil
	}

	timeline, err := timetravel.NewExecutionTimeline(ctx, s.store, executionID)
	if err != nil {
		return nil, err
	}

	s.timelines[executionID] = timeline
	return timeline, nil
}

func (s *DebugServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *DebugServer) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, ErrorResponse{Error: message})
}

func (s *DebugServer) getIntParam(r *http.Request, name string, defaultValue int) int {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultValue
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	return defaultValue
}

func (s *DebugServer) getInt64Param(r *http.Request, name string, defaultValue int64) int64 {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultValue
	}
	if i, err := strconv.ParseInt(val, 10, 64); err == nil {
		return i
	}
	return defaultValue
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
