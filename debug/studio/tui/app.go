// Package tui provides a terminal user interface for the Minion debugger.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Ranganaths/minion/debug/snapshot"
	"github.com/Ranganaths/minion/debug/timetravel"
)

// App is the main application model for the TUI debugger.
type App struct {
	store     snapshot.SnapshotStore
	branching *timetravel.BranchingEngine

	// Current state
	mode       ViewMode
	executions []*snapshot.ExecutionSummary
	timeline   *timetravel.ExecutionTimeline
	state      *timetravel.ReconstructedState

	// UI state
	cursor       int
	width        int
	height       int
	ready        bool
	errorMessage string
	statusMessage string

	// View-specific state
	executionCursor int
	snapshotCursor  int
}

// ViewMode represents the current view mode.
type ViewMode int

const (
	ModeExecutionList ViewMode = iota
	ModeTimeline
	ModeStateInspector
	ModeDiff
	ModeHelp
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
)

// NewApp creates a new TUI application.
func NewApp(store snapshot.SnapshotStore) *App {
	return &App{
		store:     store,
		branching: timetravel.NewBranchingEngine(store),
		mode:      ModeExecutionList,
	}
}

// Init initializes the application.
func (a *App) Init() tea.Cmd {
	return a.loadExecutions
}

// Update handles messages and updates state.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		return a, nil

	case executionsMsg:
		a.executions = msg.executions
		a.errorMessage = ""
		return a, nil

	case timelineMsg:
		a.timeline = msg.timeline
		a.snapshotCursor = msg.timeline.Position()
		a.errorMessage = ""
		return a, nil

	case stateMsg:
		a.state = msg.state
		a.errorMessage = ""
		return a, nil

	case errorMsg:
		a.errorMessage = msg.err.Error()
		return a, nil
	}

	return a, nil
}

// View renders the application.
func (a *App) View() string {
	if !a.ready {
		return "Loading..."
	}

	var content string

	switch a.mode {
	case ModeExecutionList:
		content = a.renderExecutionList()
	case ModeTimeline:
		content = a.renderTimeline()
	case ModeStateInspector:
		content = a.renderStateInspector()
	case ModeDiff:
		content = a.renderDiff()
	case ModeHelp:
		content = a.renderHelp()
	}

	return a.renderLayout(content)
}

// Key handling

func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c", "q":
		return a, tea.Quit
	case "?":
		a.mode = ModeHelp
		return a, nil
	case "esc":
		if a.mode == ModeHelp {
			a.mode = ModeExecutionList
		} else if a.mode != ModeExecutionList {
			a.mode = ModeExecutionList
		}
		return a, nil
	}

	// Mode-specific keys
	switch a.mode {
	case ModeExecutionList:
		return a.handleExecutionListKeys(msg)
	case ModeTimeline:
		return a.handleTimelineKeys(msg)
	case ModeStateInspector:
		return a.handleStateInspectorKeys(msg)
	case ModeDiff:
		return a.handleDiffKeys(msg)
	case ModeHelp:
		return a.handleHelpKeys(msg)
	}

	return a, nil
}

func (a *App) handleExecutionListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if a.executionCursor < len(a.executions)-1 {
			a.executionCursor++
		}
	case "k", "up":
		if a.executionCursor > 0 {
			a.executionCursor--
		}
	case "enter":
		if len(a.executions) > 0 {
			return a, a.loadTimeline(a.executions[a.executionCursor].ExecutionID)
		}
	case "r":
		return a, a.loadExecutions
	}
	return a, nil
}

func (a *App) handleTimelineKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.timeline == nil {
		return a, nil
	}

	switch msg.String() {
	case "l", "right":
		a.timeline.StepForward()
		a.snapshotCursor = a.timeline.Position()
		return a, a.reconstructState
	case "h", "left":
		a.timeline.StepBackward()
		a.snapshotCursor = a.timeline.Position()
		return a, a.reconstructState
	case "g":
		a.timeline.First()
		a.snapshotCursor = a.timeline.Position()
		return a, a.reconstructState
	case "G":
		a.timeline.Last()
		a.snapshotCursor = a.timeline.Position()
		return a, a.reconstructState
	case "e":
		if snap := a.timeline.JumpToNextError(); snap != nil {
			a.snapshotCursor = a.timeline.Position()
			return a, a.reconstructState
		}
	case "E":
		if snap := a.timeline.JumpToPrevError(); snap != nil {
			a.snapshotCursor = a.timeline.Position()
			return a, a.reconstructState
		}
	case "s":
		a.mode = ModeStateInspector
		return a, a.reconstructState
	case "d":
		a.mode = ModeDiff
	case "enter":
		return a, a.reconstructState
	}

	return a, nil
}

func (a *App) handleStateInspectorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		a.cursor++
	case "k", "up":
		if a.cursor > 0 {
			a.cursor--
		}
	case "t":
		a.mode = ModeTimeline
	}
	return a, nil
}

func (a *App) handleDiffKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "t":
		a.mode = ModeTimeline
	}
	return a, nil
}

func (a *App) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "q", "esc":
		a.mode = ModeExecutionList
	}
	return a, nil
}

// Rendering

func (a *App) renderLayout(content string) string {
	header := a.renderHeader()
	statusBar := a.renderStatusBar()

	// Calculate available height for content
	availableHeight := a.height - 3 // header + status bar

	// Truncate or pad content to fit
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > availableHeight {
		contentLines = contentLines[:availableHeight]
	}
	content = strings.Join(contentLines, "\n")

	return header + "\n" + content + "\n" + statusBar
}

func (a *App) renderHeader() string {
	title := "🔍 Minion Debug Studio"

	modeStr := ""
	switch a.mode {
	case ModeExecutionList:
		modeStr = "Executions"
	case ModeTimeline:
		modeStr = "Timeline"
	case ModeStateInspector:
		modeStr = "State Inspector"
	case ModeDiff:
		modeStr = "Diff View"
	case ModeHelp:
		modeStr = "Help"
	}

	return headerStyle.Render(fmt.Sprintf("%s | %s", title, modeStr))
}

func (a *App) renderStatusBar() string {
	var status string

	if a.errorMessage != "" {
		status = errorStyle.Render("Error: " + a.errorMessage)
	} else if a.statusMessage != "" {
		status = a.statusMessage
	} else {
		switch a.mode {
		case ModeExecutionList:
			status = "j/k: navigate | enter: select | r: refresh | q: quit | ?: help"
		case ModeTimeline:
			status = "h/l: step | g/G: first/last | e/E: errors | s: state | esc: back"
		case ModeStateInspector:
			status = "j/k: scroll | t: timeline | esc: back"
		case ModeDiff:
			status = "t: timeline | esc: back"
		case ModeHelp:
			status = "Press any key to return"
		}
	}

	return statusBarStyle.Width(a.width).Render(status)
}

func (a *App) renderExecutionList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Recent Executions"))
	b.WriteString("\n\n")

	if len(a.executions) == 0 {
		b.WriteString(dimStyle.Render("No executions found. Run some agents to see them here."))
		return b.String()
	}

	for i, exec := range a.executions {
		line := a.formatExecutionLine(exec)

		if i == a.executionCursor {
			b.WriteString(selectedStyle.Render(fmt.Sprintf(" ▶ %s", line)))
		} else {
			b.WriteString(normalStyle.Render(fmt.Sprintf("   %s", line)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (a *App) formatExecutionLine(exec *snapshot.ExecutionSummary) string {
	statusIcon := "⏳"
	switch exec.Status {
	case "completed":
		statusIcon = "✅"
	case "failed":
		statusIcon = "❌"
	case "running":
		statusIcon = "🔄"
	}

	errorStr := ""
	if exec.ErrorCount > 0 {
		errorStr = errorStyle.Render(fmt.Sprintf(" [%d errors]", exec.ErrorCount))
	}

	return fmt.Sprintf("%s %s | %d steps | %s%s",
		statusIcon,
		truncate(exec.ExecutionID, 12),
		exec.TotalSteps,
		exec.Duration.Round(time.Millisecond),
		errorStr,
	)
}

func (a *App) renderTimeline() string {
	if a.timeline == nil {
		return dimStyle.Render("No timeline loaded")
	}

	var b strings.Builder

	// Summary
	summary := a.timeline.Summary()
	b.WriteString(titleStyle.Render(fmt.Sprintf("Execution: %s", truncate(summary.ExecutionID, 20))))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Duration: %s | Steps: %d | Errors: %d",
		summary.Duration.Round(time.Millisecond),
		summary.TotalSteps,
		summary.ErrorCount)))
	b.WriteString("\n\n")

	// Progress bar
	progress := a.timeline.Progress()
	b.WriteString(a.renderProgressBar(progress, a.width-10))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Step %d / %d (%.1f%%)",
		a.timeline.Position()+1,
		a.timeline.Length(),
		progress)))
	b.WriteString("\n\n")

	// Current snapshot
	current := a.timeline.Current()
	if current != nil {
		b.WriteString(a.renderSnapshotDetail(current))
	}

	return b.String()
}

func (a *App) renderProgressBar(progress float64, width int) string {
	if width < 10 {
		width = 10
	}

	filled := int(float64(width) * progress / 100)
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	return infoStyle.Render("[" + bar + "]")
}

func (a *App) renderSnapshotDetail(snap *snapshot.ExecutionSnapshot) string {
	var b strings.Builder

	// Checkpoint type with icon
	icon := a.getCheckpointIcon(snap.CheckpointType)
	b.WriteString(fmt.Sprintf("%s %s\n", icon, string(snap.CheckpointType)))

	// Timestamp
	b.WriteString(dimStyle.Render(fmt.Sprintf("Time: %s\n", snap.Timestamp.Format("15:04:05.000"))))

	// IDs
	if snap.AgentID != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Agent: %s\n", snap.AgentID)))
	}
	if snap.TaskID != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Task: %s\n", snap.TaskID)))
	}

	// Action details
	if snap.Action != nil {
		b.WriteString("\n")
		b.WriteString(infoStyle.Render("Action:\n"))
		b.WriteString(fmt.Sprintf("  Type: %s\n", snap.Action.Type))
		b.WriteString(fmt.Sprintf("  Name: %s\n", snap.Action.Name))
		if snap.Action.DurationMs > 0 {
			b.WriteString(fmt.Sprintf("  Duration: %dms\n", snap.Action.DurationMs))
		}
	}

	// Error
	if snap.Error != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error:\n"))
		b.WriteString(errorStyle.Render(fmt.Sprintf("  %s: %s\n", snap.Error.Type, snap.Error.Message)))
	}

	return boxStyle.Render(b.String())
}

func (a *App) getCheckpointIcon(cpType snapshot.CheckpointType) string {
	switch cpType {
	case snapshot.CheckpointTaskCreated:
		return "📝"
	case snapshot.CheckpointTaskStarted:
		return "▶️"
	case snapshot.CheckpointTaskCompleted:
		return "✅"
	case snapshot.CheckpointTaskFailed:
		return "❌"
	case snapshot.CheckpointToolCallStart:
		return "🔧"
	case snapshot.CheckpointToolCallEnd:
		return "🔨"
	case snapshot.CheckpointLLMCallStart:
		return "🤖"
	case snapshot.CheckpointLLMCallEnd:
		return "💬"
	case snapshot.CheckpointError:
		return "🚨"
	case snapshot.CheckpointDecisionPoint:
		return "🔀"
	default:
		return "📌"
	}
}

func (a *App) renderStateInspector() string {
	if a.state == nil {
		return dimStyle.Render("No state loaded. Press 's' in timeline view to inspect state.")
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("State Inspector"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Sequence: %d | Time: %s",
		a.state.SequenceNum,
		a.state.Timestamp.Format("15:04:05.000"))))
	b.WriteString("\n\n")

	// Session state
	if a.state.Session != nil {
		b.WriteString(infoStyle.Render("📋 Session:\n"))
		b.WriteString(fmt.Sprintf("  ID: %s\n", a.state.Session.ID))
		b.WriteString(fmt.Sprintf("  Status: %s\n", a.state.Session.Status))
		b.WriteString(fmt.Sprintf("  History: %d messages\n", len(a.state.Session.History)))
		b.WriteString("\n")
	}

	// Task state
	if a.state.Task != nil {
		b.WriteString(infoStyle.Render("📦 Task:\n"))
		b.WriteString(fmt.Sprintf("  ID: %s\n", a.state.Task.ID))
		b.WriteString(fmt.Sprintf("  Name: %s\n", a.state.Task.Name))
		b.WriteString(fmt.Sprintf("  Status: %s\n", a.state.Task.Status))
		b.WriteString("\n")
	}

	// Workspace
	if len(a.state.Workspace) > 0 {
		b.WriteString(infoStyle.Render("🗂️ Workspace:\n"))
		for k, v := range a.state.Workspace {
			b.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
		b.WriteString("\n")
	}

	// Action history
	if len(a.state.PreviousActions) > 0 {
		b.WriteString(infoStyle.Render(fmt.Sprintf("📜 Previous Actions (%d):\n", len(a.state.PreviousActions))))
		for i, action := range a.state.PreviousActions {
			if i >= 5 {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  ... and %d more\n", len(a.state.PreviousActions)-5)))
				break
			}
			b.WriteString(fmt.Sprintf("  - %s: %s\n", action.Type, action.Name))
		}
	}

	return b.String()
}

func (a *App) renderDiff() string {
	return titleStyle.Render("Diff View") + "\n\n" +
		dimStyle.Render("Compare states feature - select two points to compare.")
}

func (a *App) renderHelp() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Help"))
	b.WriteString("\n\n")

	sections := []struct {
		title string
		keys  [][]string
	}{
		{
			title: "Global",
			keys: [][]string{
				{"q", "Quit"},
				{"?", "Show help"},
				{"Esc", "Go back / Close"},
			},
		},
		{
			title: "Execution List",
			keys: [][]string{
				{"j/k", "Navigate up/down"},
				{"Enter", "Select execution"},
				{"r", "Refresh list"},
			},
		},
		{
			title: "Timeline",
			keys: [][]string{
				{"h/l", "Step backward/forward"},
				{"g/G", "Go to first/last"},
				{"e/E", "Jump to next/prev error"},
				{"s", "Open state inspector"},
				{"d", "Open diff view"},
			},
		},
		{
			title: "State Inspector",
			keys: [][]string{
				{"j/k", "Scroll"},
				{"t", "Back to timeline"},
			},
		},
	}

	for _, section := range sections {
		b.WriteString(infoStyle.Render(section.title))
		b.WriteString("\n")
		for _, kv := range section.keys {
			b.WriteString(fmt.Sprintf("  %s  %s\n",
				normalStyle.Render(fmt.Sprintf("%-10s", kv[0])),
				dimStyle.Render(kv[1])))
		}
		b.WriteString("\n")
	}

	return boxStyle.Render(b.String())
}

// Commands

type executionsMsg struct {
	executions []*snapshot.ExecutionSummary
}

type timelineMsg struct {
	timeline *timetravel.ExecutionTimeline
}

type stateMsg struct {
	state *timetravel.ReconstructedState
}

type errorMsg struct {
	err error
}

func (a *App) loadExecutions() tea.Msg {
	ctx := context.Background()
	executions, err := a.store.ListExecutions(ctx, 50, 0)
	if err != nil {
		return errorMsg{err: err}
	}
	return executionsMsg{executions: executions}
}

func (a *App) loadTimeline(executionID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		timeline, err := timetravel.NewExecutionTimeline(ctx, a.store, executionID)
		if err != nil {
			return errorMsg{err: err}
		}
		a.mode = ModeTimeline
		return timelineMsg{timeline: timeline}
	}
}

func (a *App) reconstructState() tea.Msg {
	if a.timeline == nil {
		return nil
	}

	reconstructor := timetravel.NewStateReconstructor(a.timeline)
	state, err := reconstructor.ReconstructCurrent()
	if err != nil {
		return errorMsg{err: err}
	}
	return stateMsg{state: state}
}

// Helpers

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Run starts the TUI application.
func Run(store snapshot.SnapshotStore) error {
	app := NewApp(store)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
