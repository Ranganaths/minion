// Package api provides the Debug API server for time-travel debugging.
package api

import (
	"time"

	"github.com/Ranganaths/minion/debug/snapshot"
	"github.com/Ranganaths/minion/debug/timetravel"
)

// ExecutionListRequest is the request for listing executions.
type ExecutionListRequest struct {
	AgentID  string     `json:"agent_id,omitempty"`
	FromTime *time.Time `json:"from_time,omitempty"`
	ToTime   *time.Time `json:"to_time,omitempty"`
	HasError *bool      `json:"has_error,omitempty"`
	Status   string     `json:"status,omitempty"` // "running", "completed", "failed"
	Limit    int        `json:"limit,omitempty"`
	Offset   int        `json:"offset,omitempty"`
}

// ExecutionListResponse is the response for listing executions.
type ExecutionListResponse struct {
	Executions []*snapshot.ExecutionSummary `json:"executions"`
	TotalCount int64                        `json:"total_count"`
	HasMore    bool                         `json:"has_more"`
}

// ExecutionDetailResponse is the response for a single execution.
type ExecutionDetailResponse struct {
	Summary   *snapshot.ExecutionSummary     `json:"summary"`
	Timeline  *timetravel.TimelineExport     `json:"timeline,omitempty"`
	Snapshots []*snapshot.ExecutionSnapshot  `json:"snapshots,omitempty"`
}

// TimelineRequest is the request for timeline operations.
type TimelineRequest struct {
	ExecutionID string `json:"execution_id"`
	IncludeSnapshots bool `json:"include_snapshots,omitempty"`
}

// TimelineResponse is the response containing timeline data.
type TimelineResponse struct {
	ExecutionID string                         `json:"execution_id"`
	Summary     *snapshot.ExecutionSummary     `json:"summary"`
	Snapshots   []*snapshot.ExecutionSnapshot  `json:"snapshots,omitempty"`
	Position    int                            `json:"position"`
	Length      int                            `json:"length"`
}

// StepRequest is the request for stepping through a timeline.
type StepRequest struct {
	ExecutionID string                  `json:"execution_id"`
	Direction   string                  `json:"direction"` // "forward", "backward", "first", "last", "jump"
	TargetSeq   int64                   `json:"target_seq,omitempty"`
	TargetIndex int                     `json:"target_index,omitempty"`
	Checkpoint  snapshot.CheckpointType `json:"checkpoint,omitempty"` // Jump to next/prev of this type
	Count       int                     `json:"count,omitempty"`      // For stepping N times
}

// StepResponse is the response after stepping.
type StepResponse struct {
	Current     *snapshot.ExecutionSnapshot `json:"current"`
	Position    int                         `json:"position"`
	Total       int                         `json:"total"`
	CanForward  bool                        `json:"can_forward"`
	CanBackward bool                        `json:"can_backward"`
	Progress    float64                     `json:"progress"` // 0-100
}

// StateRequest is the request for state reconstruction.
type StateRequest struct {
	ExecutionID string `json:"execution_id"`
	SequenceNum int64  `json:"sequence_num,omitempty"`
	Index       int    `json:"index,omitempty"`
}

// StateResponse is the response with reconstructed state.
type StateResponse struct {
	State *timetravel.ReconstructedState `json:"state"`
}

// CompareStatesRequest is the request to compare two states.
type CompareStatesRequest struct {
	ExecutionID  string `json:"execution_id"`
	SequenceNum1 int64  `json:"sequence_num_1"`
	SequenceNum2 int64  `json:"sequence_num_2"`
}

// CompareStatesResponse is the response with state comparison.
type CompareStatesResponse struct {
	Comparison *timetravel.StateComparison `json:"comparison"`
}

// ReplayRequest is the request for replaying execution.
type ReplayRequest struct {
	ExecutionID  string                    `json:"execution_id"`
	FromSequence int64                     `json:"from_sequence"`
	Mode         timetravel.ReplayMode     `json:"mode,omitempty"` // "simulate", "execute", "hybrid"
	Modification *timetravel.Modification  `json:"modification,omitempty"`
	StopAt       *ReplayStopCondition      `json:"stop_at,omitempty"`
	Compare      bool                      `json:"compare,omitempty"`
}

// ReplayStopCondition defines when to stop replay.
type ReplayStopCondition struct {
	Checkpoint snapshot.CheckpointType `json:"checkpoint,omitempty"`
	Sequence   int64                   `json:"sequence,omitempty"`
	MaxSteps   int                     `json:"max_steps,omitempty"`
	Timeout    string                  `json:"timeout,omitempty"` // Duration string
}

// ReplayResponse is the response from a replay operation.
type ReplayResponse struct {
	Result *timetravel.ReplayResult `json:"result"`
}

// BranchRequest is the request for creating a branch.
type BranchRequest struct {
	ExecutionID  string                   `json:"execution_id"`
	SequenceNum  int64                    `json:"sequence_num"`
	Name         string                   `json:"name,omitempty"`
	Description  string                   `json:"description,omitempty"`
	Modification *timetravel.Modification `json:"modification,omitempty"`
}

// BranchResponse is the response after creating a branch.
type BranchResponse struct {
	Branch *timetravel.ExecutionBranch `json:"branch"`
}

// BranchListResponse is the response listing branches.
type BranchListResponse struct {
	Branches []*timetravel.ExecutionBranch `json:"branches"`
	Tree     *timetravel.BranchTree        `json:"tree,omitempty"`
}

// ExecuteBranchRequest is the request for executing a branch.
type ExecuteBranchRequest struct {
	BranchID string                   `json:"branch_id"`
	Mode     timetravel.ReplayMode    `json:"mode,omitempty"`
	Compare  bool                     `json:"compare,omitempty"`
}

// ExecuteBranchResponse is the response from branch execution.
type ExecuteBranchResponse struct {
	Result     *timetravel.ReplayResult      `json:"result"`
	Comparison *timetravel.BranchComparison  `json:"comparison,omitempty"`
}

// CompareBranchesRequest is the request to compare branches.
type CompareBranchesRequest struct {
	BranchID1 string `json:"branch_id_1"`
	BranchID2 string `json:"branch_id_2"`
}

// CompareBranchesResponse is the response with branch comparison.
type CompareBranchesResponse struct {
	Comparison *timetravel.BranchComparison `json:"comparison"`
}

// WhatIfRequest is the request for quick what-if analysis.
type WhatIfRequest struct {
	ExecutionID   string                     `json:"execution_id"`
	SequenceNum   int64                      `json:"sequence_num"`
	Modifications []*timetravel.Modification `json:"modifications"`
}

// WhatIfResponse is the response from what-if analysis.
type WhatIfResponse struct {
	Comparisons []*timetravel.BranchComparison `json:"comparisons"`
}

// SearchRequest is the request for searching snapshots.
type SearchRequest struct {
	Query   string            `json:"query"`
	Filters map[string]string `json:"filters,omitempty"`
	Limit   int               `json:"limit,omitempty"`
	Offset  int               `json:"offset,omitempty"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	ExecutionID string                      `json:"execution_id"`
	SequenceNum int64                       `json:"sequence_num"`
	Snapshot    *snapshot.ExecutionSnapshot `json:"snapshot"`
	Score       float64                     `json:"score"`
	Highlights  []string                    `json:"highlights,omitempty"`
}

// SearchResponse is the response from a search operation.
type SearchResponse struct {
	Results    []*SearchResult `json:"results"`
	TotalCount int64           `json:"total_count"`
	HasMore    bool            `json:"has_more"`
}

// SnapshotQueryRequest is the request for querying snapshots.
type SnapshotQueryRequest struct {
	Filter  snapshot.SnapshotFilter `json:"filter"`
	Limit   int                     `json:"limit,omitempty"`
	Offset  int                     `json:"offset,omitempty"`
	OrderBy string                  `json:"order_by,omitempty"`
}

// SnapshotQueryResponse is the response with snapshot query results.
type SnapshotQueryResponse struct {
	Snapshots  []*snapshot.ExecutionSnapshot `json:"snapshots"`
	TotalCount int64                         `json:"total_count"`
	HasMore    bool                          `json:"has_more"`
}

// ExportRequest is the request for exporting execution data.
type ExportRequest struct {
	ExecutionID string `json:"execution_id"`
	Format      string `json:"format,omitempty"` // "json", "csv"
	IncludeState bool  `json:"include_state,omitempty"`
}

// ExportResponse is the response with exported data.
type ExportResponse struct {
	Format   string `json:"format"`
	Data     any    `json:"data,omitempty"`
	Filename string `json:"filename,omitempty"`
	URL      string `json:"url,omitempty"` // For large exports
}

// StatsResponse is the response with store statistics.
type StatsResponse struct {
	Stats *snapshot.StoreStats `json:"stats"`
}

// HealthResponse is the health check response.
type HealthResponse struct {
	Status    string `json:"status"` // "healthy", "degraded", "unhealthy"
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	StoreOK   bool   `json:"store_ok"`
	Message   string `json:"message,omitempty"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// WebSocketMessage is the message format for WebSocket communication.
type WebSocketMessage struct {
	Type    string `json:"type"` // "snapshot", "step", "replay_progress", "branch_complete"
	Payload any    `json:"payload"`
}

// SnapshotEvent is sent when a new snapshot is recorded.
type SnapshotEvent struct {
	ExecutionID string                      `json:"execution_id"`
	Snapshot    *snapshot.ExecutionSnapshot `json:"snapshot"`
}

// ReplayProgressEvent is sent during replay.
type ReplayProgressEvent struct {
	BranchID    string `json:"branch_id,omitempty"`
	ExecutionID string `json:"execution_id"`
	CurrentSeq  int64  `json:"current_seq"`
	TotalSteps  int    `json:"total_steps"`
	Progress    float64 `json:"progress"` // 0-100
}

// Pagination provides standard pagination fields.
type Pagination struct {
	Limit      int   `json:"limit"`
	Offset     int   `json:"offset"`
	TotalCount int64 `json:"total_count"`
	HasMore    bool  `json:"has_more"`
}
