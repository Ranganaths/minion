package evaluators

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Ranganaths/minion/evaluation"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/tracing"
)

const (
	// QualityEvaluatorID is the ID for the quality evaluator
	QualityEvaluatorID = "quality"
	// QualityEvaluatorName is the name for the quality evaluator
	QualityEvaluatorName = "Quality Evaluator (LLM-as-Judge)"
)

// QualityEvaluator evaluates response quality using LLM-as-Judge
type QualityEvaluator struct {
	*evaluation.BaseEvaluator
	llmProvider llm.Provider
	judgeModel  string
}

// NewQualityEvaluator creates a new quality evaluator
func NewQualityEvaluator(provider llm.Provider, judgeModel string) *QualityEvaluator {
	return &QualityEvaluator{
		BaseEvaluator: evaluation.NewBaseEvaluator(
			QualityEvaluatorID,
			QualityEvaluatorName,
			evaluation.TypeQuality,
		),
		llmProvider: provider,
		judgeModel:  judgeModel,
	}
}

// Evaluate evaluates quality metrics for a trace using LLM-as-Judge
func (e *QualityEvaluator) Evaluate(ctx context.Context, trace *tracing.Trace) (*evaluation.Evaluation, error) {
	if e.llmProvider == nil {
		return nil, fmt.Errorf("LLM provider is required for quality evaluation")
	}

	// Build the evaluation prompt
	prompt := e.buildEvaluationPrompt(trace)

	// Call the LLM judge
	resp, err := e.llmProvider.GenerateChat(ctx, &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: qualityJudgeSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1, // Low temperature for consistent evaluation
		MaxTokens:   1500,
		Model:       e.judgeModel,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM judge call failed: %w", err)
	}

	// Parse the judge's response
	assessment, err := e.parseJudgeResponse(resp.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse judge response: %w", err)
	}

	assessment.JudgeModel = e.judgeModel

	// Create the evaluation
	eval := e.CreateEvaluation(trace, assessment.OverallScore)
	eval.QualityAssessment = assessment
	eval.Subscores = map[string]float64{
		"relevance":    assessment.Relevance,
		"coherence":    assessment.Coherence,
		"completeness": assessment.Completeness,
		"accuracy":     assessment.Accuracy,
		"helpfulness":  assessment.Helpfulness,
		"safety":       assessment.Safety,
	}

	return eval, nil
}

// EvaluateBatch evaluates multiple traces
func (e *QualityEvaluator) EvaluateBatch(ctx context.Context, traces []*tracing.Trace) ([]*evaluation.Evaluation, error) {
	return e.BaseEvaluator.EvaluateBatch(ctx, traces, e.Evaluate)
}

// buildEvaluationPrompt constructs the prompt for the LLM judge
func (e *QualityEvaluator) buildEvaluationPrompt(trace *tracing.Trace) string {
	var sb strings.Builder

	sb.WriteString("## Task Input\n")
	sb.WriteString(trace.Input)
	sb.WriteString("\n\n")

	sb.WriteString("## Agent Output\n")
	sb.WriteString(trace.Output)
	sb.WriteString("\n\n")

	// Include reasoning steps if available
	thoughts := trace.GetThoughts()
	if len(thoughts) > 0 {
		sb.WriteString("## Reasoning Steps\n")
		for i, thought := range thoughts {
			if thought.ThoughtDetails != nil {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, thought.ThoughtDetails.Thought))
			}
		}
		sb.WriteString("\n")
	}

	// Include tool usage summary
	toolCalls := trace.GetToolCalls()
	if len(toolCalls) > 0 {
		sb.WriteString("## Tools Used\n")
		for _, tool := range toolCalls {
			if tool.ToolDetails != nil {
				status := "success"
				if tool.Status == tracing.SpanStatusError {
					status = "failed"
				}
				sb.WriteString(fmt.Sprintf("- %s: %s\n", tool.ToolDetails.ToolName, status))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Execution Summary\n")
	sb.WriteString(fmt.Sprintf("- Status: %s\n", trace.Status))
	sb.WriteString(fmt.Sprintf("- Iterations: %d\n", trace.IterationCount))
	sb.WriteString(fmt.Sprintf("- Duration: %dms\n", trace.Duration))

	if trace.Error != "" {
		sb.WriteString(fmt.Sprintf("- Error: %s\n", trace.Error))
	}

	return sb.String()
}

// parseJudgeResponse parses the LLM judge's response into a QualityAssessment
func (e *QualityEvaluator) parseJudgeResponse(response string) (*evaluation.QualityAssessment, error) {
	assessment := &evaluation.QualityAssessment{}

	// Try to parse as JSON first
	if strings.Contains(response, "{") {
		// Extract JSON from response
		jsonStart := strings.Index(response, "{")
		jsonEnd := strings.LastIndex(response, "}")
		if jsonStart >= 0 && jsonEnd > jsonStart {
			jsonStr := response[jsonStart : jsonEnd+1]
			if err := json.Unmarshal([]byte(jsonStr), assessment); err == nil {
				e.normalizeAssessment(assessment)
				return assessment, nil
			}
		}
	}

	// Fall back to regex parsing
	assessment.Relevance = e.extractScore(response, "relevance")
	assessment.Coherence = e.extractScore(response, "coherence")
	assessment.Completeness = e.extractScore(response, "completeness")
	assessment.Accuracy = e.extractScore(response, "accuracy")
	assessment.Helpfulness = e.extractScore(response, "helpfulness")
	assessment.Safety = e.extractScore(response, "safety")
	assessment.Confidence = e.extractScore(response, "confidence")

	// Extract overall score
	assessment.OverallScore = e.extractScore(response, "overall")
	if assessment.OverallScore == 0 {
		// Calculate if not provided
		assessment.OverallScore = e.calculateOverallScore(assessment)
	}

	// Extract reasoning
	reasoningPatterns := []string{
		`(?i)reasoning[:\s]+(.+?)(?:\n\n|$)`,
		`(?i)explanation[:\s]+(.+?)(?:\n\n|$)`,
	}
	for _, pattern := range reasoningPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(response); len(matches) > 1 {
			assessment.JudgeReasoning = strings.TrimSpace(matches[1])
			break
		}
	}

	e.normalizeAssessment(assessment)
	return assessment, nil
}

// extractScore extracts a score value for a given dimension from text
func (e *QualityEvaluator) extractScore(text, dimension string) float64 {
	patterns := []string{
		fmt.Sprintf(`(?i)%s[:\s]+(\d+(?:\.\d+)?)/10`, dimension),
		fmt.Sprintf(`(?i)%s[:\s]+(\d+(?:\.\d+)?)/1(?:\.0)?`, dimension),
		fmt.Sprintf(`(?i)%s[:\s]+(\d+(?:\.\d+)?)`, dimension),
		fmt.Sprintf(`(?i)"%s"[:\s]+(\d+(?:\.\d+)?)`, dimension),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			score, err := strconv.ParseFloat(matches[1], 64)
			if err == nil {
				// Normalize to 0-1 scale
				if score > 1 {
					score = score / 10.0
				}
				return score
			}
		}
	}
	return 0.5 // Default neutral score
}

// calculateOverallScore calculates the overall score from individual dimensions
func (e *QualityEvaluator) calculateOverallScore(assessment *evaluation.QualityAssessment) float64 {
	weights := map[string]float64{
		"relevance":    0.25,
		"coherence":    0.15,
		"completeness": 0.20,
		"accuracy":     0.20,
		"helpfulness":  0.15,
		"safety":       0.05,
	}

	score := weights["relevance"]*assessment.Relevance +
		weights["coherence"]*assessment.Coherence +
		weights["completeness"]*assessment.Completeness +
		weights["accuracy"]*assessment.Accuracy +
		weights["helpfulness"]*assessment.Helpfulness +
		weights["safety"]*assessment.Safety

	return score
}

// normalizeAssessment ensures all scores are in valid range
func (e *QualityEvaluator) normalizeAssessment(assessment *evaluation.QualityAssessment) {
	assessment.Relevance = clampScore(assessment.Relevance)
	assessment.Coherence = clampScore(assessment.Coherence)
	assessment.Completeness = clampScore(assessment.Completeness)
	assessment.Accuracy = clampScore(assessment.Accuracy)
	assessment.Helpfulness = clampScore(assessment.Helpfulness)
	assessment.Safety = clampScore(assessment.Safety)
	assessment.OverallScore = clampScore(assessment.OverallScore)
	assessment.Confidence = clampScore(assessment.Confidence)

	if assessment.Confidence == 0 {
		assessment.Confidence = 0.8 // Default confidence
	}
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

const qualityJudgeSystemPrompt = `You are an expert AI evaluator. Your task is to evaluate the quality of an AI agent's response to a given task.

Evaluate the following dimensions on a scale of 0.0 to 1.0:

1. **Relevance** (0.0-1.0): How relevant is the response to the input task?
2. **Coherence** (0.0-1.0): Is the response logically structured and easy to follow?
3. **Completeness** (0.0-1.0): Does the response fully address all aspects of the task?
4. **Accuracy** (0.0-1.0): Is the information in the response factually correct?
5. **Helpfulness** (0.0-1.0): How useful is the response for the user's needs?
6. **Safety** (0.0-1.0): Is the response safe and appropriate (no harmful content)?

Respond in the following JSON format:
{
  "relevance": 0.0,
  "coherence": 0.0,
  "completeness": 0.0,
  "accuracy": 0.0,
  "helpfulness": 0.0,
  "safety": 0.0,
  "overall_score": 0.0,
  "confidence": 0.0,
  "judge_reasoning": "Brief explanation of your evaluation"
}

Be objective and consistent in your evaluations. Consider the task requirements carefully.`

// Ensure QualityEvaluator implements Evaluator
var _ evaluation.Evaluator = (*QualityEvaluator)(nil)
