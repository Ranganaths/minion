package chain

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/llm"
)

// LLMChain is a simple chain that formats a prompt and calls an LLM
type LLMChain struct {
	*BaseChain
	llmProvider  llm.Provider
	promptFunc   PromptFunc
	outputKey    string
	outputParser OutputParser
}

// PromptFunc is a function that formats inputs into a prompt
type PromptFunc func(inputs map[string]any) (string, error)

// OutputParser parses LLM output into structured data
type OutputParser interface {
	Parse(text string) (any, error)
	GetFormatInstructions() string
}

// LLMChainConfig configures the LLM chain
type LLMChainConfig struct {
	// LLM is the language model provider
	LLM llm.Provider

	// PromptFunc formats inputs into a prompt
	PromptFunc PromptFunc

	// PromptTemplate is a simple template string (alternative to PromptFunc)
	// Uses Go template syntax: {{.variable}}
	PromptTemplate string

	// OutputKey is the key for the output (default: "text")
	OutputKey string

	// OutputParser optionally parses the LLM output
	OutputParser OutputParser

	// InputKeys are the required input keys
	InputKeys []string

	// Options are chain options
	Options []Option
}

// NewLLMChain creates a new LLM chain
func NewLLMChain(cfg LLMChainConfig) (*LLMChain, error) {
	if cfg.LLM == nil {
		return nil, fmt.Errorf("LLM provider is required")
	}

	outputKey := cfg.OutputKey
	if outputKey == "" {
		outputKey = "text"
	}

	promptFunc := cfg.PromptFunc
	if promptFunc == nil && cfg.PromptTemplate != "" {
		promptFunc = CreateTemplatePromptFunc(cfg.PromptTemplate)
	}
	if promptFunc == nil {
		return nil, fmt.Errorf("either PromptFunc or PromptTemplate is required")
	}

	return &LLMChain{
		BaseChain:    NewBaseChain("llm_chain", cfg.Options...),
		llmProvider:  cfg.LLM,
		promptFunc:   promptFunc,
		outputKey:    outputKey,
		outputParser: cfg.OutputParser,
	}, nil
}

// InputKeys returns the required input keys
func (c *LLMChain) InputKeys() []string {
	// LLMChain accepts any inputs that the prompt function needs
	return []string{}
}

// OutputKeys returns the output keys
func (c *LLMChain) OutputKeys() []string {
	return []string{c.outputKey}
}

// Call executes the LLM chain
func (c *LLMChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	c.NotifyStart(ctx, inputs)

	// Apply timeout
	ctx, cancel := c.WithTimeout(ctx)
	defer cancel()

	// Format prompt
	prompt, err := c.promptFunc(inputs)
	if err != nil {
		c.NotifyError(ctx, fmt.Errorf("prompt format error: %w", err))
		return nil, fmt.Errorf("prompt format error: %w", err)
	}

	// Add format instructions if parser is present
	if c.outputParser != nil {
		instructions := c.outputParser.GetFormatInstructions()
		if instructions != "" {
			prompt = prompt + "\n\n" + instructions
		}
	}

	c.NotifyLLMStart(ctx, prompt)

	// Build LLM request
	req := &llm.CompletionRequest{
		UserPrompt: prompt,
	}

	// Apply option overrides
	if c.Options().Temperature != nil {
		req.Temperature = *c.Options().Temperature
	}
	if c.Options().MaxTokens != nil {
		req.MaxTokens = *c.Options().MaxTokens
	}

	// Call LLM
	resp, err := c.llmProvider.GenerateCompletion(ctx, req)
	if err != nil {
		c.NotifyError(ctx, fmt.Errorf("LLM error: %w", err))
		return nil, fmt.Errorf("LLM error: %w", err)
	}

	c.NotifyLLMEnd(ctx, resp.Text, resp.TokensUsed)

	// Parse output if parser is present
	var result any = resp.Text
	if c.outputParser != nil {
		parsed, err := c.outputParser.Parse(resp.Text)
		if err != nil {
			c.NotifyError(ctx, fmt.Errorf("parse error: %w", err))
			return nil, fmt.Errorf("parse error: %w", err)
		}
		result = parsed
	}

	outputs := map[string]any{
		c.outputKey: result,
	}

	c.NotifyEnd(ctx, outputs)
	return outputs, nil
}

// Stream executes the chain and streams results.
// The returned channel is closed when streaming completes or context is cancelled.
func (c *LLMChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 10)

	go func() {
		defer close(ch)

		// Helper to send with context check
		send := func(event StreamEvent) bool {
			select {
			case <-ctx.Done():
				return false
			case ch <- event:
				return true
			}
		}

		if !send(MakeStreamEvent(StreamEventStart, "", map[string]any{"chain": c.Name()}, nil)) {
			return
		}

		result, err := c.Call(ctx, inputs)
		if err != nil {
			// Check if it's a context error
			if ctx.Err() != nil {
				send(MakeStreamEvent(StreamEventError, "", nil, ctx.Err()))
				return
			}
			send(MakeStreamEvent(StreamEventError, "", nil, err))
			return
		}

		send(MakeStreamEvent(StreamEventComplete, "", result, nil))
	}()

	return ch, nil
}

// Predict is a convenience method for simple string input/output
func (c *LLMChain) Predict(ctx context.Context, input string) (string, error) {
	result, err := c.Call(ctx, map[string]any{"input": input})
	if err != nil {
		return "", err
	}
	output, ok := result[c.outputKey].(string)
	if !ok {
		return "", fmt.Errorf("output is not a string")
	}
	return output, nil
}

// CreateTemplatePromptFunc creates a prompt function from a template string
func CreateTemplatePromptFunc(template string) PromptFunc {
	return func(inputs map[string]any) (string, error) {
		result := template
		for key, val := range inputs {
			placeholder := fmt.Sprintf("{{.%s}}", key)
			strVal := fmt.Sprintf("%v", val)
			result = replaceAll(result, placeholder, strVal)
		}
		return result, nil
	}
}

// replaceAll replaces all occurrences of old with new in s
func replaceAll(s, old, new string) string {
	for {
		idx := indexOf(s, old)
		if idx == -1 {
			return s
		}
		s = s[:idx] + new + s[idx+len(old):]
	}
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// SimplePrompt creates a simple prompt function that uses the "input" key
func SimplePrompt(prefix string) PromptFunc {
	return func(inputs map[string]any) (string, error) {
		input, ok := inputs["input"]
		if !ok {
			return "", fmt.Errorf("missing 'input' key")
		}
		return fmt.Sprintf("%s%v", prefix, input), nil
	}
}

// QuestionAnswerPrompt creates a Q&A style prompt function
func QuestionAnswerPrompt() PromptFunc {
	return func(inputs map[string]any) (string, error) {
		question, ok := inputs["question"]
		if !ok {
			return "", fmt.Errorf("missing 'question' key")
		}
		return fmt.Sprintf("Question: %v\n\nAnswer:", question), nil
	}
}
