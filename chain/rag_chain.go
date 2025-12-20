package chain

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/llm"
	"github.com/Ranganaths/minion/retriever"
	"github.com/Ranganaths/minion/vectorstore"
)

// RAGChain combines retrieval with LLM generation
type RAGChain struct {
	*BaseChain
	retriever     retriever.Retriever
	llmProvider   llm.Provider
	promptFunc    RAGPromptFunc
	inputKey      string
	outputKey     string
	contextKey    string
	combineFunc   CombineDocumentsFunc
	returnSources bool
}

// RAGPromptFunc formats the query and context into a prompt
type RAGPromptFunc func(query string, context string) (string, error)

// CombineDocumentsFunc combines documents into a context string
type CombineDocumentsFunc func(docs []vectorstore.Document) string

// RAGChainConfig configures the RAG chain
type RAGChainConfig struct {
	// Retriever retrieves relevant documents
	Retriever retriever.Retriever

	// LLM is the language model provider
	LLM llm.Provider

	// PromptFunc formats the prompt (optional, uses default if not provided)
	PromptFunc RAGPromptFunc

	// CombineFunc combines documents (optional, uses default if not provided)
	CombineFunc CombineDocumentsFunc

	// InputKey is the key for the query input (default: "question")
	InputKey string

	// OutputKey is the key for the output (default: "answer")
	OutputKey string

	// ContextKey is the key for the context in inputs (default: "context")
	ContextKey string

	// ReturnSources includes source documents in output
	ReturnSources bool

	// Options are chain options
	Options []Option
}

// NewRAGChain creates a new RAG chain
func NewRAGChain(cfg RAGChainConfig) (*RAGChain, error) {
	if cfg.Retriever == nil {
		return nil, fmt.Errorf("retriever is required")
	}
	if cfg.LLM == nil {
		return nil, fmt.Errorf("LLM provider is required")
	}

	inputKey := cfg.InputKey
	if inputKey == "" {
		inputKey = "question"
	}

	outputKey := cfg.OutputKey
	if outputKey == "" {
		outputKey = "answer"
	}

	contextKey := cfg.ContextKey
	if contextKey == "" {
		contextKey = "context"
	}

	promptFunc := cfg.PromptFunc
	if promptFunc == nil {
		promptFunc = DefaultRAGPromptFunc
	}

	combineFunc := cfg.CombineFunc
	if combineFunc == nil {
		combineFunc = DefaultCombineDocumentsFunc
	}

	return &RAGChain{
		BaseChain:     NewBaseChain("rag_chain", cfg.Options...),
		retriever:     cfg.Retriever,
		llmProvider:   cfg.LLM,
		promptFunc:    promptFunc,
		inputKey:      inputKey,
		outputKey:     outputKey,
		contextKey:    contextKey,
		combineFunc:   combineFunc,
		returnSources: cfg.ReturnSources,
	}, nil
}

// InputKeys returns the required input keys
func (c *RAGChain) InputKeys() []string {
	return []string{c.inputKey}
}

// OutputKeys returns the output keys
func (c *RAGChain) OutputKeys() []string {
	if c.returnSources {
		return []string{c.outputKey, "source_documents"}
	}
	return []string{c.outputKey}
}

// Call executes the RAG chain
func (c *RAGChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	c.NotifyStart(ctx, inputs)

	// Apply timeout
	ctx, cancel := c.WithTimeout(ctx)
	defer cancel()

	// Get query from inputs
	query, err := c.GetString(inputs, c.inputKey)
	if err != nil {
		c.NotifyError(ctx, err)
		return nil, err
	}

	// Check for pre-provided context
	var contextStr string
	var docs []vectorstore.Document

	if existingContext, ok := inputs[c.contextKey]; ok {
		contextStr, _ = existingContext.(string)
	}

	// If no context provided, retrieve documents
	if contextStr == "" {
		c.NotifyRetrieverStart(ctx, query)

		docs, err = c.retriever.GetRelevantDocuments(ctx, query)
		if err != nil {
			c.NotifyError(ctx, fmt.Errorf("retrieval error: %w", err))
			return nil, fmt.Errorf("retrieval error: %w", err)
		}

		c.NotifyRetrieverEnd(ctx, c.toChainDocs(docs))

		// Combine documents into context
		contextStr = c.combineFunc(docs)
	}

	// Format prompt
	prompt, err := c.promptFunc(query, contextStr)
	if err != nil {
		c.NotifyError(ctx, fmt.Errorf("prompt error: %w", err))
		return nil, fmt.Errorf("prompt error: %w", err)
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

	// Build output
	outputs := map[string]any{
		c.outputKey: resp.Text,
	}

	if c.returnSources && len(docs) > 0 {
		outputs["source_documents"] = docs
	}

	c.NotifyEnd(ctx, outputs)
	return outputs, nil
}

// Stream executes the RAG chain with streaming
func (c *RAGChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 10)

	go func() {
		defer close(ch)

		ch <- MakeStreamEvent(StreamEventStart, "", map[string]any{"chain": c.Name()}, nil)

		// Get query from inputs
		query, err := c.GetString(inputs, c.inputKey)
		if err != nil {
			ch <- MakeStreamEvent(StreamEventError, "", nil, err)
			return
		}

		// Check for pre-provided context
		var contextStr string
		var docs []vectorstore.Document

		if existingContext, ok := inputs[c.contextKey]; ok {
			contextStr, _ = existingContext.(string)
		}

		// If no context provided, retrieve documents
		if contextStr == "" {
			docs, err = c.retriever.GetRelevantDocuments(ctx, query)
			if err != nil {
				ch <- MakeStreamEvent(StreamEventError, "", nil, fmt.Errorf("retrieval error: %w", err))
				return
			}

			// Emit retrieval event
			ch <- MakeStreamEvent(StreamEventRetrieval, "", map[string]any{
				"documents": docs,
				"count":     len(docs),
			}, nil)

			contextStr = c.combineFunc(docs)
		}

		// Format prompt
		prompt, err := c.promptFunc(query, contextStr)
		if err != nil {
			ch <- MakeStreamEvent(StreamEventError, "", nil, fmt.Errorf("prompt error: %w", err))
			return
		}

		// Build LLM request
		req := &llm.CompletionRequest{
			UserPrompt: prompt,
		}

		// Call LLM
		resp, err := c.llmProvider.GenerateCompletion(ctx, req)
		if err != nil {
			ch <- MakeStreamEvent(StreamEventError, "", nil, fmt.Errorf("LLM error: %w", err))
			return
		}

		// Build output
		outputs := map[string]any{
			c.outputKey: resp.Text,
		}

		if c.returnSources && len(docs) > 0 {
			outputs["source_documents"] = docs
		}

		ch <- MakeStreamEvent(StreamEventComplete, "", outputs, nil)
	}()

	return ch, nil
}

// toChainDocs converts vectorstore documents to chain documents
func (c *RAGChain) toChainDocs(docs []vectorstore.Document) []Document {
	result := make([]Document, len(docs))
	for i, doc := range docs {
		result[i] = Document{
			ID:          doc.ID,
			PageContent: doc.PageContent,
			Metadata:    doc.Metadata,
		}
	}
	return result
}

// DefaultRAGPromptFunc is the default prompt function for RAG
func DefaultRAGPromptFunc(query string, context string) (string, error) {
	return fmt.Sprintf(`Use the following context to answer the question. If you cannot find the answer in the context, say "I don't have enough information to answer this question."

Context:
%s

Question: %s

Answer:`, context, query), nil
}

// DefaultCombineDocumentsFunc combines documents with newlines
func DefaultCombineDocumentsFunc(docs []vectorstore.Document) string {
	if len(docs) == 0 {
		return ""
	}

	var result string
	for i, doc := range docs {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		result += doc.PageContent
	}
	return result
}

// NumberedCombineDocumentsFunc combines documents with numbering
func NumberedCombineDocumentsFunc(docs []vectorstore.Document) string {
	if len(docs) == 0 {
		return ""
	}

	var result string
	for i, doc := range docs {
		if i > 0 {
			result += "\n\n"
		}
		result += fmt.Sprintf("[%d] %s", i+1, doc.PageContent)
	}
	return result
}

// WithSourcesCombineFunc combines documents with source metadata
func WithSourcesCombineFunc(docs []vectorstore.Document) string {
	if len(docs) == 0 {
		return ""
	}

	var result string
	for i, doc := range docs {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		source := doc.GetMetadataString("source")
		if source != "" {
			result += fmt.Sprintf("Source: %s\n\n", source)
		}
		result += doc.PageContent
	}
	return result
}

// ConversationalRAGChain extends RAG with conversation history
type ConversationalRAGChain struct {
	*RAGChain
	historyKey string
}

// ConversationalRAGChainConfig configures the conversational RAG chain
type ConversationalRAGChainConfig struct {
	RAGChainConfig

	// HistoryKey is the key for conversation history (default: "chat_history")
	HistoryKey string
}

// NewConversationalRAGChain creates a new conversational RAG chain
func NewConversationalRAGChain(cfg ConversationalRAGChainConfig) (*ConversationalRAGChain, error) {
	// Override prompt func to include history
	if cfg.PromptFunc == nil {
		cfg.PromptFunc = ConversationalRAGPromptFunc
	}

	ragChain, err := NewRAGChain(cfg.RAGChainConfig)
	if err != nil {
		return nil, err
	}

	historyKey := cfg.HistoryKey
	if historyKey == "" {
		historyKey = "chat_history"
	}

	return &ConversationalRAGChain{
		RAGChain:   ragChain,
		historyKey: historyKey,
	}, nil
}

// InputKeys returns the required input keys
func (c *ConversationalRAGChain) InputKeys() []string {
	return []string{c.inputKey, c.historyKey}
}

// Call executes the conversational RAG chain
func (c *ConversationalRAGChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	// Rewrite question based on history for better retrieval
	query, _ := c.GetString(inputs, c.inputKey)
	history, _ := inputs[c.historyKey].(string)

	if history != "" {
		// Create a condensed question
		condensedQuery := fmt.Sprintf("%s\n\nBased on the above conversation, the current question is: %s", history, query)
		inputs[c.inputKey] = condensedQuery
	}

	return c.RAGChain.Call(ctx, inputs)
}

// ConversationalRAGPromptFunc formats prompt with conversation history
func ConversationalRAGPromptFunc(query string, context string) (string, error) {
	return fmt.Sprintf(`Use the following context and conversation to answer the question. If you cannot find the answer in the context, say "I don't have enough information to answer this question."

Context:
%s

Question: %s

Answer:`, context, query), nil
}
