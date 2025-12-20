package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/models"
	"github.com/Ranganaths/minion/storage"
	"github.com/Ranganaths/minion/tools"
)

// CalculatorTool implements basic math operations
type CalculatorTool struct{}

func (t *CalculatorTool) Name() string {
	return "calculator"
}

func (t *CalculatorTool) Description() string {
	return "Performs basic mathematical calculations (add, subtract, multiply, divide)"
}

func (t *CalculatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	operation, ok := input.Params["operation"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Missing operation parameter",
		}, nil
	}

	a, aOk := input.Params["a"].(float64)
	b, bOk := input.Params["b"].(float64)
	if !aOk || !bOk {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid numeric parameters",
		}, nil
	}

	var result float64
	switch operation {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return &models.ToolOutput{
				ToolName: t.Name(),
				Success:  false,
				Result:   "Division by zero",
			}, nil
		}
		result = a / b
	case "power":
		result = math.Pow(a, b)
	default:
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   fmt.Sprintf("Unknown operation: %s", operation),
		}, nil
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   result,
	}, nil
}

func (t *CalculatorTool) CanExecute(agent *models.Agent) bool {
	// This tool is available to agents with "math" or "calculation" capabilities
	for _, cap := range agent.Capabilities {
		if cap == "math" || cap == "calculation" {
			return true
		}
	}
	return false
}

// TextAnalysisTool analyzes text
type TextAnalysisTool struct{}

func (t *TextAnalysisTool) Name() string {
	return "text_analysis"
}

func (t *TextAnalysisTool) Description() string {
	return "Analyzes text and provides statistics (word count, character count, etc.)"
}

func (t *TextAnalysisTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	text, ok := input.Params["text"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Missing text parameter",
		}, nil
	}

	words := strings.Fields(text)
	sentences := strings.Split(text, ".")

	analysis := map[string]interface{}{
		"character_count": len(text),
		"word_count":      len(words),
		"sentence_count":  len(sentences),
		"avg_word_length": float64(len(text)) / float64(len(words)),
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *TextAnalysisTool) CanExecute(agent *models.Agent) bool {
	// This tool is available to agents with "text_analysis" capability
	for _, cap := range agent.Capabilities {
		if cap == "text_analysis" {
			return true
		}
	}
	return false
}

// WeatherTool simulates weather data retrieval
type WeatherTool struct{}

func (t *WeatherTool) Name() string {
	return "weather"
}

func (t *WeatherTool) Description() string {
	return "Gets weather information for a location (simulated)"
}

func (t *WeatherTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	location, ok := input.Params["location"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Missing location parameter",
		}, nil
	}

	// Simulated weather data
	weather := map[string]interface{}{
		"location":    location,
		"temperature": 72,
		"condition":   "Partly Cloudy",
		"humidity":    65,
		"wind_speed":  8,
	}

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   weather,
	}, nil
}

func (t *WeatherTool) CanExecute(agent *models.Agent) bool {
	// Available to all agents
	return true
}

func main() {
	fmt.Println("🛠️  Agent Framework - Tool Integration Example")
	fmt.Println("================================================")
	fmt.Println()

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
	fmt.Println("   ✓ Framework initialized")
	fmt.Println()

	// 2. Register tools
	fmt.Println("2. Registering tools...")

	calculatorTool := &CalculatorTool{}
	if err := framework.RegisterTool(calculatorTool); err != nil {
		log.Fatalf("Failed to register calculator tool: %v", err)
	}
	fmt.Printf("   ✓ Registered: %s\n", calculatorTool.Name())

	textAnalysisTool := &TextAnalysisTool{}
	if err := framework.RegisterTool(textAnalysisTool); err != nil {
		log.Fatalf("Failed to register text analysis tool: %v", err)
	}
	fmt.Printf("   ✓ Registered: %s\n", textAnalysisTool.Name())

	weatherTool := &WeatherTool{}
	if err := framework.RegisterTool(weatherTool); err != nil {
		log.Fatalf("Failed to register weather tool: %v", err)
	}
	fmt.Printf("   ✓ Registered: %s\n\n", weatherTool.Name())

	// 3. Create agent with math capabilities
	fmt.Println("3. Creating math-capable agent...")
	mathAgent, err := framework.CreateAgent(context.Background(), &models.CreateAgentRequest{
		Name:         "Math Assistant",
		Description:  "An agent that can perform mathematical calculations",
		BehaviorType: "default",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			Temperature: 0.7,
			MaxTokens:   500,
			Personality: "precise",
		},
		Capabilities: []string{"math", "calculation"},
	})
	if err != nil {
		log.Fatalf("Failed to create math agent: %v", err)
	}
	fmt.Printf("   ✓ Agent created: %s\n", mathAgent.Name)

	// Activate agent
	activeStatus := models.StatusActive
	mathAgent, _ = framework.UpdateAgent(context.Background(), mathAgent.ID, &models.UpdateAgentRequest{
		Status: &activeStatus,
	})

	// 4. Check available tools for math agent
	fmt.Println("\n4. Checking available tools for math agent...")
	availableTools := framework.GetToolsForAgent(mathAgent)
	fmt.Printf("   ✓ Found %d available tool(s):\n", len(availableTools))
	for _, toolInterface := range availableTools {
		if tool, ok := toolInterface.(tools.Tool); ok {
			fmt.Printf("   - %s: %s\n", tool.Name(), tool.Description())
		}
	}

	// 5. Create agent with text analysis capabilities
	fmt.Println("\n5. Creating text analysis agent...")
	textAgent, err := framework.CreateAgent(context.Background(), &models.CreateAgentRequest{
		Name:         "Text Analyzer",
		Description:  "An agent that can analyze text",
		BehaviorType: "default",
		Config: models.AgentConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			Temperature: 0.5,
			MaxTokens:   500,
			Personality: "analytical",
		},
		Capabilities: []string{"text_analysis"},
	})
	if err != nil {
		log.Fatalf("Failed to create text agent: %v", err)
	}
	fmt.Printf("   ✓ Agent created: %s\n", textAgent.Name)

	textAgent, _ = framework.UpdateAgent(context.Background(), textAgent.ID, &models.UpdateAgentRequest{
		Status: &activeStatus,
	})

	// Check available tools for text agent
	fmt.Println("\n   Checking available tools for text agent...")
	availableTools = framework.GetToolsForAgent(textAgent)
	fmt.Printf("   ✓ Found %d available tool(s):\n", len(availableTools))
	for _, toolInterface := range availableTools {
		if tool, ok := toolInterface.(tools.Tool); ok {
			fmt.Printf("   - %s: %s\n", tool.Name(), tool.Description())
		}
	}

	// 6. Execute agents
	fmt.Println("\n6. Executing agents...")

	fmt.Println("\n   Math Agent:")
	output, err := framework.Execute(context.Background(), mathAgent.ID, &models.Input{
		Raw:  "Calculate the result of 15 multiplied by 7",
		Type: "text",
	})
	if err != nil {
		log.Printf("   ✗ Execution failed: %v", err)
	} else {
		fmt.Printf("   Response: %v\n", output.Result)
	}

	fmt.Println("\n   Text Analysis Agent:")
	output, err = framework.Execute(context.Background(), textAgent.ID, &models.Input{
		Raw:  "Analyze this sample text: The quick brown fox jumps over the lazy dog",
		Type: "text",
	})
	if err != nil {
		log.Printf("   ✗ Execution failed: %v", err)
	} else {
		fmt.Printf("   Response: %v\n", output.Result)
	}

	// 7. Demonstrate direct tool execution
	fmt.Println("\n7. Direct tool execution demo...")

	fmt.Println("\n   Calculator Tool:")
	calcResult, err := calculatorTool.Execute(context.Background(), &models.ToolInput{
		Params: map[string]interface{}{
			"operation": "multiply",
			"a":         15.0,
			"b":         7.0,
		},
	})
	if err != nil {
		log.Printf("   ✗ Tool execution failed: %v", err)
	} else if calcResult.Success {
		fmt.Printf("   Result: 15 × 7 = %v\n", calcResult.Result)
	}

	fmt.Println("\n   Text Analysis Tool:")
	textResult, err := textAnalysisTool.Execute(context.Background(), &models.ToolInput{
		Params: map[string]interface{}{
			"text": "The quick brown fox jumps over the lazy dog",
		},
	})
	if err != nil {
		log.Printf("   ✗ Tool execution failed: %v", err)
	} else if textResult.Success {
		fmt.Printf("   Result: %+v\n", textResult.Result)
	}

	fmt.Println("\n✅ Tool integration example completed successfully!")
}
