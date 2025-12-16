package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/agentql/agentql/pkg/minion/core"
	"github.com/agentql/agentql/pkg/minion/llm"
	"github.com/agentql/agentql/pkg/minion/models"
	"github.com/agentql/agentql/pkg/minion/storage"
)

// SentimentAnalysisBehavior is a custom behavior that analyzes sentiment
type SentimentAnalysisBehavior struct{}

func (b *SentimentAnalysisBehavior) GetSystemPrompt(agent *models.Agent) string {
	return fmt.Sprintf(`You are %s, a sentiment analysis expert.

Description: %s

Your task is to analyze the sentiment of user input and provide:
1. Overall sentiment (Positive, Negative, Neutral)
2. Confidence score (0-100)
3. Key emotional indicators
4. Brief explanation

Always respond in a structured format.
Personality: %s
`, agent.Name, agent.Description, agent.Config.Personality)
}

func (b *SentimentAnalysisBehavior) ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error) {
	// Preprocess input - clean and validate
	text := fmt.Sprintf("%v", input.Raw)
	text = strings.TrimSpace(text)

	if text == "" {
		return nil, fmt.Errorf("empty input provided")
	}

	return &models.ProcessedInput{
		Original:  input,
		Processed: fmt.Sprintf("Analyze the sentiment of the following text:\n\n\"%s\"", text),
		Instructions: "Provide sentiment analysis in structured format: " +
			"Sentiment, Confidence, Key Indicators, Explanation",
		ExtraContext: map[string]interface{}{
			"input_length": len(text),
			"timestamp":    time.Now(),
		},
	}, nil
}

func (b *SentimentAnalysisBehavior) ProcessOutput(ctx context.Context, agent *models.Agent, output *models.Output) (*models.ProcessedOutput, error) {
	// Post-process output - add metadata and enhancements
	enhanced := make(map[string]interface{})
	enhanced["analysis_type"] = "sentiment"
	enhanced["processed_at"] = time.Now()
	enhanced["agent_personality"] = agent.Config.Personality

	return &models.ProcessedOutput{
		Original:  output,
		Processed: output.Result,
		Enhanced:  enhanced,
	}, nil
}

// TranslationBehavior is a custom behavior for translation tasks
type TranslationBehavior struct{}

func (b *TranslationBehavior) GetSystemPrompt(agent *models.Agent) string {
	targetLang := "English"
	if lang, ok := agent.Metadata["target_language"].(string); ok {
		targetLang = lang
	}

	return fmt.Sprintf(`You are %s, a professional translator.

Description: %s

Your task is to translate text to %s with high accuracy.
Maintain the original tone and context.
Provide clear, natural translations.

Personality: %s
`, agent.Name, agent.Description, targetLang, agent.Config.Personality)
}

func (b *TranslationBehavior) ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error) {
	text := fmt.Sprintf("%v", input.Raw)

	sourceLang := "auto-detect"
	if lang, ok := agent.Metadata["source_language"].(string); ok {
		sourceLang = lang
	}

	targetLang := "English"
	if lang, ok := agent.Metadata["target_language"].(string); ok {
		targetLang = lang
	}

	return &models.ProcessedInput{
		Original:  input,
		Processed: fmt.Sprintf("Translate the following text from %s to %s:\n\n\"%s\"", sourceLang, targetLang, text),
		Instructions: fmt.Sprintf("Provide only the translation to %s, maintaining tone and context", targetLang),
		ExtraContext: map[string]interface{}{
			"source_language": sourceLang,
			"target_language": targetLang,
		},
	}, nil
}

func (b *TranslationBehavior) ProcessOutput(ctx context.Context, agent *models.Agent, output *models.Output) (*models.ProcessedOutput, error) {
	// Add translation metadata
	enhanced := map[string]interface{}{
		"translation_type": "text",
		"target_language":  agent.Metadata["target_language"],
		"processed_at":     time.Now(),
	}

	return &models.ProcessedOutput{
		Original:  output,
		Processed: output.Result,
		Enhanced:  enhanced,
	}, nil
}

// CodeReviewBehavior is a custom behavior for code review
type CodeReviewBehavior struct{}

func (b *CodeReviewBehavior) GetSystemPrompt(agent *models.Agent) string {
	return fmt.Sprintf(`You are %s, an expert code reviewer.

Description: %s

Your task is to review code and provide:
1. Code quality assessment (1-10)
2. Potential bugs or issues
3. Security concerns
4. Performance improvements
5. Best practice recommendations

Be thorough but constructive. Focus on actionable feedback.
Personality: %s
`, agent.Name, agent.Description, agent.Config.Personality)
}

func (b *CodeReviewBehavior) ProcessInput(ctx context.Context, agent *models.Agent, input *models.Input) (*models.ProcessedInput, error) {
	code := fmt.Sprintf("%v", input.Raw)

	language := "unknown"
	if lang, ok := input.Context["language"].(string); ok {
		language = lang
	}

	return &models.ProcessedInput{
		Original:  input,
		Processed: fmt.Sprintf("Review the following %s code:\n\n```%s\n%s\n```", language, language, code),
		Instructions: "Provide structured code review covering: quality, bugs, security, performance, and best practices",
		ExtraContext: map[string]interface{}{
			"language":   language,
			"code_lines": strings.Count(code, "\n") + 1,
		},
	}, nil
}

func (b *CodeReviewBehavior) ProcessOutput(ctx context.Context, agent *models.Agent, output *models.Output) (*models.ProcessedOutput, error) {
	enhanced := map[string]interface{}{
		"review_type": "code_quality",
		"reviewed_at": time.Now(),
	}

	return &models.ProcessedOutput{
		Original:  output,
		Processed: output.Result,
		Enhanced:  enhanced,
	}, nil
}

func main() {
	fmt.Println("🎨 Agent Framework - Custom Behavior Example")
	fmt.Println("============================================\n")

	// Check for OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// 1. Create framework
	fmt.Println("1. Initializing framework...")
	framework := core.NewFramework(
		core.WithStorage(storage.NewInMemory()),
		core.WithLLMProvider(llm.NewOpenAI(apiKey)),
	)
	defer framework.Close()
	fmt.Println("   ✓ Framework initialized\n")

	// 2. Register custom behaviors
	fmt.Println("2. Registering custom behaviors...")

	if err := framework.RegisterBehavior("sentiment_analysis", &SentimentAnalysisBehavior{}); err != nil {
		log.Fatalf("Failed to register sentiment behavior: %v", err)
	}
	fmt.Println("   ✓ Registered: sentiment_analysis")

	if err := framework.RegisterBehavior("translation", &TranslationBehavior{}); err != nil {
		log.Fatalf("Failed to register translation behavior: %v", err)
	}
	fmt.Println("   ✓ Registered: translation")

	if err := framework.RegisterBehavior("code_review", &CodeReviewBehavior{}); err != nil {
		log.Fatalf("Failed to register code review behavior: %v", err)
	}
	fmt.Println("   ✓ Registered: code_review\n")

	// 3. Create agent with sentiment analysis behavior
	fmt.Println("3. Creating sentiment analysis agent...")
	sentimentAgent, err := framework.CreateAgent(context.Background(), &models.CreateAgentRequest{
		Name:         "Sentiment Analyzer",
		Description:  "Analyzes the emotional tone and sentiment of text",
		BehaviorType: "sentiment_analysis",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			Temperature: 0.3, // Lower temperature for more consistent analysis
			MaxTokens:   500,
			Personality: "analytical",
		},
		Capabilities: []string{"sentiment_analysis", "text_analysis"},
	})
	if err != nil {
		log.Fatalf("Failed to create sentiment agent: %v", err)
	}
	fmt.Printf("   ✓ Agent created: %s (Behavior: %s)\n", sentimentAgent.Name, sentimentAgent.BehaviorType)

	// Activate agent
	activeStatus := models.StatusActive
	sentimentAgent, _ = framework.UpdateAgent(context.Background(), sentimentAgent.ID, &models.UpdateAgentRequest{
		Status: &activeStatus,
	})

	// 4. Create translation agent
	fmt.Println("\n4. Creating translation agent...")
	translationAgent, err := framework.CreateAgent(context.Background(), &models.CreateAgentRequest{
		Name:         "Spanish Translator",
		Description:  "Translates text to Spanish",
		BehaviorType: "translation",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			Temperature: 0.3,
			MaxTokens:   1000,
			Personality: "precise",
		},
		Metadata: map[string]interface{}{
			"source_language": "English",
			"target_language": "Spanish",
		},
		Capabilities: []string{"translation"},
	})
	if err != nil {
		log.Fatalf("Failed to create translation agent: %v", err)
	}
	fmt.Printf("   ✓ Agent created: %s (Behavior: %s)\n", translationAgent.Name, translationAgent.BehaviorType)

	translationAgent, _ = framework.UpdateAgent(context.Background(), translationAgent.ID, &models.UpdateAgentRequest{
		Status: &activeStatus,
	})

	// 5. Create code review agent
	fmt.Println("\n5. Creating code review agent...")
	codeReviewAgent, err := framework.CreateAgent(context.Background(), &models.CreateAgentRequest{
		Name:         "Code Reviewer",
		Description:  "Reviews code for quality, bugs, and best practices",
		BehaviorType: "code_review",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			Temperature: 0.4,
			MaxTokens:   1500,
			Personality: "constructive",
		},
		Capabilities: []string{"code_review", "static_analysis"},
	})
	if err != nil {
		log.Fatalf("Failed to create code review agent: %v", err)
	}
	fmt.Printf("   ✓ Agent created: %s (Behavior: %s)\n", codeReviewAgent.Name, codeReviewAgent.BehaviorType)

	codeReviewAgent, _ = framework.UpdateAgent(context.Background(), codeReviewAgent.ID, &models.UpdateAgentRequest{
		Status: &activeStatus,
	})

	// 6. Execute agents with custom behaviors
	fmt.Println("\n6. Executing agents with custom behaviors...\n")

	// Sentiment analysis
	fmt.Println("   === Sentiment Analysis Agent ===")
	sentimentInput := "I absolutely love this product! It has exceeded all my expectations and made my life so much easier."
	fmt.Printf("   Input: %s\n\n", sentimentInput)

	output, err := framework.Execute(context.Background(), sentimentAgent.ID, &models.Input{
		Raw:  sentimentInput,
		Type: "text",
	})
	if err != nil {
		log.Printf("   ✗ Execution failed: %v\n", err)
	} else {
		fmt.Printf("   Analysis:\n   %v\n", output.Result)
	}

	// Translation
	fmt.Println("\n   === Translation Agent ===")
	translationInput := "Hello, how are you today? I hope you're having a wonderful day!"
	fmt.Printf("   Input (English): %s\n\n", translationInput)

	output, err = framework.Execute(context.Background(), translationAgent.ID, &models.Input{
		Raw:  translationInput,
		Type: "text",
	})
	if err != nil {
		log.Printf("   ✗ Execution failed: %v\n", err)
	} else {
		fmt.Printf("   Translation (Spanish):\n   %v\n", output.Result)
	}

	// Code review
	fmt.Println("\n   === Code Review Agent ===")
	codeInput := `func calculateTotal(items []Item) float64 {
    total := 0.0
    for i := 0; i < len(items); i++ {
        total = total + items[i].Price
    }
    return total
}`
	fmt.Printf("   Input:\n   %s\n\n", codeInput)

	output, err = framework.Execute(context.Background(), codeReviewAgent.ID, &models.Input{
		Raw:  codeInput,
		Type: "code",
		Context: map[string]interface{}{
			"language": "go",
		},
	})
	if err != nil {
		log.Printf("   ✗ Execution failed: %v\n", err)
	} else {
		fmt.Printf("   Review:\n   %v\n", output.Result)
	}

	// 7. Show metrics
	fmt.Println("\n7. Agent metrics summary...")
	agents := []*models.Agent{sentimentAgent, translationAgent, codeReviewAgent}

	for _, agent := range agents {
		metrics, err := framework.GetMetrics(context.Background(), agent.ID)
		if err != nil {
			continue
		}
		fmt.Printf("\n   %s:\n", agent.Name)
		fmt.Printf("   - Executions: %d\n", metrics.TotalExecutions)
		fmt.Printf("   - Success rate: %.1f%%\n", float64(metrics.SuccessfulExecutions)/float64(metrics.TotalExecutions)*100)
		fmt.Printf("   - Avg time: %.2fms\n", metrics.AvgExecutionTime)
	}

	fmt.Println("\n✅ Custom behavior example completed successfully!")
}
