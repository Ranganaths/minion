// Example demonstrating self-improving agents in the minion framework.
//
// This example shows:
// 1. Creating a self-improving agent wrapper
// 2. Recording experiences from task executions
// 3. Using multiple learning strategies (few-shot, pattern extraction, reflection)
// 4. Human approval workflow for improvements
// 5. A/B testing for proposed changes
// 6. Automatic rollback monitoring
// 7. Prompt versioning and rollback
// 8. Monitoring self-improvement metrics
//
// Note: Self-improvement is DISABLED by default and must be explicitly enabled.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Ranganaths/minion/core/selfimprove"
	"github.com/Ranganaths/minion/core/selfimprove/store"
	"github.com/Ranganaths/minion/core/selfimprove/strategies"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== Self-Improving Agent Demo ===")
	fmt.Println("Note: Self-improvement is disabled by default for safety.")
	fmt.Println()

	// ============================================================
	// Step 1: Create Experience Store
	// ============================================================
	fmt.Println("1. Creating experience store...")
	experienceStore := store.NewInMemoryExperienceStore()
	fmt.Println("   Created in-memory experience store")

	// ============================================================
	// Step 2: Create Proposal Store
	// ============================================================
	fmt.Println("\n2. Creating proposal store...")
	proposalStore := selfimprove.NewInMemoryProposalStore()
	fmt.Println("   Created in-memory proposal store")

	// ============================================================
	// Step 3: Create Learning Engine
	// ============================================================
	fmt.Println("\n3. Creating learning engine...")

	learningConfig := selfimprove.DefaultLearningConfig()
	learningConfig.Strategies = []selfimprove.LearningStrategy{
		selfimprove.StrategyFewShot,
		selfimprove.StrategyPromptRefinement,
		selfimprove.StrategyPatternExtraction,
		selfimprove.StrategyReflection,
	}
	learningConfig.MinExperiencesForLearn = 5 // Lower for demo
	learningConfig.FewShotExamples = 3

	learningEngine := selfimprove.NewLearningEngine(learningConfig, experienceStore)
	fmt.Println("   Created learning engine with strategies:")
	for _, s := range learningConfig.Strategies {
		fmt.Printf("     - %s\n", s)
	}

	// ============================================================
	// Step 4: Register Learning Strategies
	// ============================================================
	fmt.Println("\n4. Registering learning strategies...")

	// Few-shot strategy
	fewShotStrategy := strategies.NewFewShotStrategy(
		selfimprove.DefaultFewShotConfig(),
		experienceStore,
	)
	learningEngine.RegisterStrategy(fewShotStrategy)
	fmt.Println("   Registered few-shot strategy")

	// Pattern extraction strategy
	patternConfig := strategies.DefaultPatternExtractionConfig()
	patternConfig.MinPatternOccurrences = 2 // Lower for demo
	patternStrategy := strategies.NewPatternExtractionStrategy(patternConfig, experienceStore)
	learningEngine.RegisterStrategy(patternStrategy)
	fmt.Println("   Registered pattern extraction strategy")

	// ============================================================
	// Step 5: Setup Human Approval Workflow
	// ============================================================
	fmt.Println("\n5. Setting up approval workflow...")

	approvalConfig := selfimprove.DefaultApprovalConfig()
	approvalConfig.RequireApproval = false       // Auto-approve for demo
	approvalConfig.AutoApproveBelow = 0.5        // Auto-approve low-impact changes
	approvalConfig.TimeoutDuration = time.Hour

	approvalManager := selfimprove.NewApprovalManager(proposalStore, approvalConfig)

	// Add logging handler
	approvalManager.RegisterHandler(&selfimprove.LoggingApprovalHandler{
		LogFunc: func(format string, args ...interface{}) {
			fmt.Printf("   [Approval] "+format+"\n", args...)
		},
	})
	fmt.Println("   Created approval manager with auto-approve for low-impact changes")

	// ============================================================
	// Step 6: Setup Prompt Version Manager
	// ============================================================
	fmt.Println("\n6. Setting up prompt versioning...")

	versionManager := selfimprove.NewPromptVersionManager(10) // Keep 10 versions
	fmt.Println("   Created version manager (max 10 versions)")

	// ============================================================
	// Step 7: Setup Rollback Monitoring
	// ============================================================
	fmt.Println("\n7. Setting up rollback monitoring...")

	rollbackConfig := selfimprove.DefaultRollbackConfig()
	rollbackConfig.MinExecutionsForRollback = 5 // Lower for demo
	rollbackConfig.PerformanceDropThreshold = 0.2
	rollbackConfig.GracePeriodMinutes = 1

	rollbackMonitor := selfimprove.NewRollbackMonitor(
		versionManager,
		experienceStore,
		proposalStore,
		rollbackConfig,
	)

	// Add rollback handler
	rollbackMonitor.RegisterHandler(&selfimprove.LoggingRollbackHandler{
		LogFunc: func(format string, args ...interface{}) {
			fmt.Printf("   [Rollback] "+format+"\n", args...)
		},
	})
	fmt.Println("   Created rollback monitor with automatic rollback enabled")

	// ============================================================
	// Step 8: Setup A/B Testing Manager
	// ============================================================
	fmt.Println("\n8. Setting up A/B testing...")

	abConfig := selfimprove.DefaultABTestConfig()
	abConfig.MinSamples = 5 // Lower for demo
	abConfig.ConfidenceLevel = 0.90

	abTestManager := selfimprove.NewABTestManager(abConfig)
	fmt.Println("   Created A/B test manager (90% confidence level)")

	// ============================================================
	// Step 9: Add Metrics Hook
	// ============================================================
	fmt.Println("\n9. Setting up metrics collection...")

	metricsCollector := selfimprove.NewMetricsCollector()
	metricsHook := selfimprove.NewLearningMetricsHook(metricsCollector)
	learningEngine.AddHook(metricsHook)
	fmt.Println("   Added metrics hook to learning engine")

	// ============================================================
	// Step 10: Create Base Handler (Mock for demo)
	// ============================================================
	fmt.Println("\n10. Creating base task handler...")

	baseHandler := &MockTaskHandler{
		name:         "demo-worker",
		capabilities: []string{"math", "text_analysis", "code_review"},
	}
	fmt.Println("   Created mock task handler")

	// ============================================================
	// Step 11: Create Self-Improving Agent
	// ============================================================
	fmt.Println("\n11. Creating self-improving agent...")

	config := selfimprove.DefaultConfig()
	config.Enabled = true // Explicitly enable self-improvement
	config.LearnAfterEveryN = 5
	config.LearnOnFeedback = true

	agent := selfimprove.NewSelfImprovingAgent(
		baseHandler,
		"demo-agent-001",
		selfimprove.WithExperienceStore(experienceStore),
		selfimprove.WithLearningEngine(learningEngine),
		selfimprove.WithConfig(config),
	)

	fmt.Printf("   Created self-improving agent: %s\n", agent.GetName())
	fmt.Printf("   Self-improvement enabled: %v\n", agent.IsLearningEnabled())

	// ============================================================
	// Step 12: Create Prompt Version
	// ============================================================
	fmt.Println("\n12. Creating initial prompt version...")

	basePrompt := "You are a helpful assistant that excels at math, text analysis, and code review."
	version := versionManager.CreateVersion("demo-agent-001", basePrompt, "", "initial")
	versionManager.ActivateVersion("demo-agent-001", version.ID)
	fmt.Printf("   Created and activated version: %s\n", version.ID)

	// ============================================================
	// Step 13: Simulate Task Executions
	// ============================================================
	fmt.Println("\n13. Simulating task executions...")

	tasks := []map[string]interface{}{
		{"id": "task-1", "type": "math", "input": "2 + 2"},
		{"id": "task-2", "type": "math", "input": "5 * 3"},
		{"id": "task-3", "type": "text_analysis", "input": "Hello, world!"},
		{"id": "task-4", "type": "math", "input": "10 / 2"},
		{"id": "task-5", "type": "text_analysis", "input": "The quick brown fox"},
		{"id": "task-6", "type": "code_review", "input": "func main() { fmt.Println(\"Hello\") }"},
		{"id": "task-7", "type": "math", "input": "7 + 8"},
		{"id": "task-8", "type": "text_analysis", "input": "Self-improvement test"},
		{"id": "task-9", "type": "code_review", "input": "def hello(): print('Hi')"},
		{"id": "task-10", "type": "math", "input": "100 / 4"},
	}

	for i, task := range tasks {
		result, err := agent.HandleTask(ctx, task)
		success := err == nil
		score := 0.85 // Simulated score

		if err != nil {
			fmt.Printf("   Task %d failed: %v\n", i+1, err)
			score = 0.2
		} else {
			fmt.Printf("   Task %d completed: %v\n", i+1, result)
		}

		// Record execution for versioning and rollback monitoring
		versionManager.RecordExecution("demo-agent-001", score, success)
		rollbackMonitor.RecordExecution(ctx, "demo-agent-001", score, success, err != nil)

		// Small delay between tasks
		time.Sleep(50 * time.Millisecond)
	}

	// Give async operations time to complete
	time.Sleep(200 * time.Millisecond)

	// ============================================================
	// Step 14: Check Experience Store
	// ============================================================
	fmt.Println("\n14. Checking experience store...")

	count, _ := experienceStore.Count(ctx)
	fmt.Printf("   Total experiences stored: %d\n", count)

	stats, _ := experienceStore.GetGlobalStats(ctx)
	if stats != nil {
		fmt.Printf("   Average score: %.2f\n", stats.AvgScore)
		fmt.Printf("   Success rate: %.1f%%\n", stats.SuccessRate*100)
	}

	// ============================================================
	// Step 15: Manually Trigger Learning
	// ============================================================
	fmt.Println("\n15. Manually triggering learning...")

	proposals, err := learningEngine.AnalyzeAndPropose(ctx, "demo-agent-001")
	if err != nil {
		fmt.Printf("   Error during learning: %v\n", err)
	} else {
		fmt.Printf("   Generated %d improvement proposals\n", len(proposals))
		for _, p := range proposals {
			fmt.Printf("   - Strategy: %s, Type: %s, Confidence: %.2f\n",
				p.Strategy, p.ImprovementType, p.Confidence)
			fmt.Printf("     Rationale: %s\n", truncate(p.Rationale, 80))

			// Submit to approval manager
			approvalManager.SubmitForApproval(ctx, p)
		}
	}

	// ============================================================
	// Step 16: Check Approval Status
	// ============================================================
	fmt.Println("\n16. Checking approval status...")

	approvalStats := approvalManager.Stats()
	fmt.Printf("   Pending approvals: %d\n", approvalStats.TotalPending)
	fmt.Printf("   Agents with pending: %d\n", approvalStats.AgentsWithPending)

	pending, _ := approvalManager.GetPendingApprovals("", 5)
	for _, req := range pending {
		fmt.Printf("   Pending: %s (confidence: %.2f, risk: %.2f)\n",
			req.ProposalID, req.Confidence, req.RiskScore)
	}

	// ============================================================
	// Step 17: Demonstrate A/B Testing
	// ============================================================
	fmt.Println("\n17. Demonstrating A/B testing...")

	// Create a test proposal
	testProposal := &selfimprove.ImprovementProposal{
		ID:            "test-proposal-1",
		AgentID:       "demo-agent-001",
		CurrentValue:  "You are helpful",
		ProposedValue: "You are a highly skilled assistant",
	}

	abTest, _ := abTestManager.StartTest(testProposal)
	if abTest != nil {
		fmt.Printf("   Started A/B test: %s\n", abTest.ID)

		// Simulate some results
		for i := 0; i < 10; i++ {
			variant, _, _ := abTestManager.AssignVariant(abTest.ID)
			score := 0.8 + float64(i%3)*0.05
			success := i%4 != 0
			abTestManager.RecordResult(abTest.ID, variant, score, success)
		}

		results := abTestManager.GetResults(abTest.ID)
		if results != nil {
			fmt.Printf("   Control samples: %d (avg: %.2f)\n",
				results.ControlSamples, results.ControlAvgScore)
			fmt.Printf("   Treatment samples: %d (avg: %.2f)\n",
				results.TreatmentSamples, results.TreatmentAvgScore)
			fmt.Printf("   Winner: %s\n", results.Winner)
		}
	}

	// ============================================================
	// Step 18: Check Version History
	// ============================================================
	fmt.Println("\n18. Checking version history...")

	versions := versionManager.GetVersionHistory("demo-agent-001")
	fmt.Printf("   Total versions: %d\n", len(versions))
	for _, v := range versions {
		activeStr := ""
		if v.IsActive {
			activeStr = " (ACTIVE)"
		}
		fmt.Printf("   - %s: v%d, executions: %d%s\n",
			v.ID, v.Version, v.Executions, activeStr)
	}

	metrics := versionManager.GetVersionMetrics("demo-agent-001", version.ID)
	if metrics != nil {
		fmt.Printf("   Active version metrics: avg_score=%.2f, success_rate=%.1f%%\n",
			metrics.AvgScore, metrics.SuccessRate*100)
	}

	// ============================================================
	// Step 19: Check Rollback Monitor
	// ============================================================
	fmt.Println("\n19. Checking rollback monitor health...")

	healthReport := rollbackMonitor.HealthCheck(ctx)
	fmt.Printf("   Monitored agents: %d\n", healthReport.TotalAgents)
	fmt.Printf("   Healthy: %d, Unhealthy: %d\n",
		healthReport.HealthyAgents, healthReport.UnhealthyAgents)

	rollbackMetrics := rollbackMonitor.GetMetrics()
	fmt.Printf("   Total rollbacks: %d (auto: %d, manual: %d)\n",
		rollbackMetrics.TotalRollbacks,
		rollbackMetrics.AutomaticRollbacks,
		rollbackMetrics.ManualRollbacks)

	// ============================================================
	// Step 20: Check Agent Metrics
	// ============================================================
	fmt.Println("\n20. Checking self-improvement metrics...")

	agentMetrics := agent.GetMetrics()
	stats_map := agentMetrics.GetStats()
	fmt.Printf("   Total executions: %v\n", stats_map["total_executions"])
	fmt.Printf("   Successful: %v\n", stats_map["successful_executions"])
	fmt.Printf("   Learning cycles: %v\n", stats_map["learning_cycles"])
	fmt.Printf("   Improvements applied: %v\n", stats_map["improvements_applied"])

	// ============================================================
	// Step 21: Demonstrate Safety Features
	// ============================================================
	fmt.Println("\n21. Demonstrating safety features...")

	fmt.Println("   Testing agent-level disable:")
	agent.DisableLearning()
	fmt.Printf("   Learning enabled: %v\n", agent.IsLearningEnabled())

	agent.EnableLearning()
	fmt.Printf("   Learning re-enabled: %v\n", agent.IsLearningEnabled())

	fmt.Println("\n   Testing global kill switch:")
	selfimprove.DisableGlobally()
	fmt.Printf("   Globally disabled: %v\n", selfimprove.IsGloballyDisabled())
	fmt.Printf("   Agent learning enabled: %v (overridden by global)\n", agent.IsLearningEnabled())

	selfimprove.EnableGlobally()
	fmt.Printf("   Global re-enabled: %v\n", !selfimprove.IsGloballyDisabled())

	// ============================================================
	// Step 22: Check Global Metrics
	// ============================================================
	fmt.Println("\n22. Global metrics:")

	globalMetrics := metricsCollector.GetGlobalMetrics()
	fmt.Printf("   Total agents: %d\n", globalMetrics.TotalAgents)
	fmt.Printf("   Total executions: %d\n", globalMetrics.TotalExecutions)
	fmt.Printf("   Total learning cycles: %d\n", globalMetrics.TotalLearningCycles)
	fmt.Printf("   Uptime: %d seconds\n", globalMetrics.UptimeSecs)

	// ============================================================
	// Step 23: Pattern Extraction Results
	// ============================================================
	fmt.Println("\n23. Pattern extraction results...")

	allPatterns := patternStrategy.GetAllPatterns()
	fmt.Printf("   Patterns extracted for %d task types\n", len(allPatterns))
	for taskType, patterns := range allPatterns {
		fmt.Printf("   Task type '%s': %d patterns\n", taskType, len(patterns))
		for _, p := range patterns {
			fmt.Printf("     - %s: %s (occurrences: %d, confidence: %.2f)\n",
				p.PatternType, truncate(p.Description, 40), p.Occurrences, p.Confidence)
		}
	}

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\nSummary of Self-Improvement Components:")
	fmt.Println("  - Experience Store: Stores execution history for learning")
	fmt.Println("  - Learning Engine: Coordinates multiple learning strategies")
	fmt.Println("  - Strategies: Few-shot, Pattern Extraction, Reflection, Prompt Refinement")
	fmt.Println("  - Approval Manager: Human-in-the-loop for changes")
	fmt.Println("  - A/B Testing: Statistical validation of improvements")
	fmt.Println("  - Version Manager: Tracks prompt versions with rollback")
	fmt.Println("  - Rollback Monitor: Auto-reverts on performance regression")
}

// MockTaskHandler is a simple task handler for demonstration.
type MockTaskHandler struct {
	name         string
	capabilities []string
	execCount    int
}

func (h *MockTaskHandler) HandleTask(ctx context.Context, task interface{}) (interface{}, error) {
	h.execCount++

	// Simulate task processing
	time.Sleep(10 * time.Millisecond)

	taskMap, ok := task.(map[string]interface{})
	if !ok {
		return "processed", nil
	}

	taskType, _ := taskMap["type"].(string)
	input, _ := taskMap["input"].(string)

	switch taskType {
	case "math":
		return fmt.Sprintf("Math result for '%s': computed", input), nil
	case "text_analysis":
		return fmt.Sprintf("Text analysis for '%s': %d chars", input, len(input)), nil
	case "code_review":
		return fmt.Sprintf("Code review for '%s': looks good", truncate(input, 30)), nil
	default:
		return fmt.Sprintf("Processed: %s", input), nil
	}
}

func (h *MockTaskHandler) GetCapabilities() []string {
	return h.capabilities
}

func (h *MockTaskHandler) GetName() string {
	return h.name
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
