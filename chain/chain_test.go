package chain

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockChain is a simple mock chain for testing
type MockChain struct {
	*BaseChain
	callFunc   func(ctx context.Context, inputs map[string]any) (map[string]any, error)
	inputKeys  []string
	outputKeys []string
}

func NewMockChain(name string, inputKeys, outputKeys []string, callFunc func(ctx context.Context, inputs map[string]any) (map[string]any, error)) *MockChain {
	return &MockChain{
		BaseChain:  NewBaseChain(name),
		callFunc:   callFunc,
		inputKeys:  inputKeys,
		outputKeys: outputKeys,
	}
}

func (m *MockChain) InputKeys() []string  { return m.inputKeys }
func (m *MockChain) OutputKeys() []string { return m.outputKeys }

func (m *MockChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	return m.callFunc(ctx, inputs)
}

func (m *MockChain) Stream(ctx context.Context, inputs map[string]any) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 10)
	go func() {
		defer close(ch)
		result, err := m.Call(ctx, inputs)
		if err != nil {
			ch <- MakeStreamEvent(StreamEventError, "", nil, err)
			return
		}
		ch <- MakeStreamEvent(StreamEventComplete, "", result, nil)
	}()
	return ch, nil
}

// TestBaseChain tests basic chain functionality
func TestBaseChain(t *testing.T) {
	t.Run("NewBaseChain", func(t *testing.T) {
		bc := NewBaseChain("test_chain")
		if bc.Name() != "test_chain" {
			t.Errorf("expected name 'test_chain', got '%s'", bc.Name())
		}
		if bc.Options() == nil {
			t.Error("expected options to be non-nil")
		}
	})

	t.Run("WithOptions", func(t *testing.T) {
		timeout := 30 * time.Second
		bc := NewBaseChain("test_chain", WithTimeout(timeout), WithVerbose(true))
		if bc.Options().Timeout != timeout {
			t.Errorf("expected timeout %v, got %v", timeout, bc.Options().Timeout)
		}
		if !bc.Options().Verbose {
			t.Error("expected verbose to be true")
		}
	})

	t.Run("ValidateInputs", func(t *testing.T) {
		bc := NewBaseChain("test_chain")
		inputs := map[string]any{"key1": "value1", "key2": "value2"}

		err := bc.ValidateInputs(inputs, []string{"key1", "key2"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		err = bc.ValidateInputs(inputs, []string{"key1", "key3"})
		if err == nil {
			t.Error("expected error for missing key")
		}
	})

	t.Run("GetString", func(t *testing.T) {
		bc := NewBaseChain("test_chain")
		inputs := map[string]any{"text": "hello", "number": 123}

		val, err := bc.GetString(inputs, "text")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if val != "hello" {
			t.Errorf("expected 'hello', got '%s'", val)
		}

		_, err = bc.GetString(inputs, "number")
		if err == nil {
			t.Error("expected error for non-string value")
		}

		_, err = bc.GetString(inputs, "missing")
		if err == nil {
			t.Error("expected error for missing key")
		}
	})

	t.Run("GetStringOr", func(t *testing.T) {
		bc := NewBaseChain("test_chain")
		inputs := map[string]any{"text": "hello"}

		val := bc.GetStringOr(inputs, "text", "default")
		if val != "hello" {
			t.Errorf("expected 'hello', got '%s'", val)
		}

		val = bc.GetStringOr(inputs, "missing", "default")
		if val != "default" {
			t.Errorf("expected 'default', got '%s'", val)
		}
	})
}

// TestSequentialChain tests sequential chain execution
func TestSequentialChain(t *testing.T) {
	t.Run("Execute", func(t *testing.T) {
		chain1 := NewMockChain("chain1", []string{"input"}, []string{"output1"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output1": inputs["input"].(string) + "_chain1"}, nil
		})

		chain2 := NewMockChain("chain2", []string{"output1"}, []string{"output2"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output2": inputs["output1"].(string) + "_chain2"}, nil
		})

		seq, err := NewSequentialChain(SequentialChainConfig{
			Chains: []Chain{chain1, chain2},
		})
		if err != nil {
			t.Fatalf("failed to create sequential chain: %v", err)
		}

		result, err := seq.Call(context.Background(), map[string]any{"input": "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := "test_chain1_chain2"
		if result["output2"] != expected {
			t.Errorf("expected '%s', got '%v'", expected, result["output2"])
		}
	})

	t.Run("ErrorPropagation", func(t *testing.T) {
		chain1 := NewMockChain("chain1", []string{"input"}, []string{"output1"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return nil, errors.New("chain1 error")
		})

		seq, err := NewSequentialChain(SequentialChainConfig{
			Chains: []Chain{chain1},
		})
		if err != nil {
			t.Fatalf("failed to create sequential chain: %v", err)
		}

		_, err = seq.Call(context.Background(), map[string]any{"input": "test"})
		if err == nil {
			t.Error("expected error from chain")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		chain1 := NewMockChain("chain1", []string{"input"}, []string{"output1"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return map[string]any{"output1": "done"}, nil
			}
		})

		seq, err := NewSequentialChain(SequentialChainConfig{
			Chains:  []Chain{chain1},
			Options: []Option{WithTimeout(10 * time.Millisecond)},
		})
		if err != nil {
			t.Fatalf("failed to create sequential chain: %v", err)
		}

		_, err = seq.Call(context.Background(), map[string]any{"input": "test"})
		if err == nil {
			t.Error("expected context timeout error")
		}
	})

	t.Run("EmptyChains", func(t *testing.T) {
		_, err := NewSequentialChain(SequentialChainConfig{
			Chains: []Chain{},
		})
		if err == nil {
			t.Error("expected error for empty chains")
		}
	})

	t.Run("AddChain", func(t *testing.T) {
		chain1 := NewMockChain("chain1", []string{"input"}, []string{"output1"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output1": "value1"}, nil
		})

		seq, err := NewSequentialChain(SequentialChainConfig{
			Chains: []Chain{chain1},
		})
		if err != nil {
			t.Fatalf("failed to create sequential chain: %v", err)
		}

		chain2 := NewMockChain("chain2", []string{"output1"}, []string{"final"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"final": "complete"}, nil
		})

		seq.AddChain(chain2)

		if len(seq.Chains()) != 2 {
			t.Errorf("expected 2 chains, got %d", len(seq.Chains()))
		}
	})
}

// TestRouterChain tests router chain functionality
func TestRouterChain(t *testing.T) {
	t.Run("BasicRouting", func(t *testing.T) {
		mathChain := NewMockChain("math", []string{"input"}, []string{"output"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output": "math_result"}, nil
		})

		textChain := NewMockChain("text", []string{"input"}, []string{"output"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output": "text_result"}, nil
		})

		router, err := NewRouterChain(RouterChainConfig{
			Routes: map[string]Chain{
				"math": mathChain,
				"text": textChain,
			},
			RouterFunc: func(inputs map[string]any) (string, error) {
				return inputs["route"].(string), nil
			},
		})
		if err != nil {
			t.Fatalf("failed to create router chain: %v", err)
		}

		result, err := router.Call(context.Background(), map[string]any{"route": "math", "input": "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["output"] != "math_result" {
			t.Errorf("expected 'math_result', got '%v'", result["output"])
		}

		result, err = router.Call(context.Background(), map[string]any{"route": "text", "input": "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["output"] != "text_result" {
			t.Errorf("expected 'text_result', got '%v'", result["output"])
		}
	})

	t.Run("DefaultChain", func(t *testing.T) {
		mathChain := NewMockChain("math", []string{"input"}, []string{"output"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output": "math_result"}, nil
		})

		defaultChain := NewMockChain("default", []string{"input"}, []string{"output"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output": "default_result"}, nil
		})

		router, err := NewRouterChain(RouterChainConfig{
			Routes: map[string]Chain{
				"math": mathChain,
			},
			RouterFunc: func(inputs map[string]any) (string, error) {
				return "", nil // Return empty to use default
			},
			DefaultChain: defaultChain,
		})
		if err != nil {
			t.Fatalf("failed to create router chain: %v", err)
		}

		result, err := router.Call(context.Background(), map[string]any{"input": "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["output"] != "default_result" {
			t.Errorf("expected 'default_result', got '%v'", result["output"])
		}
	})

	t.Run("KeywordRouter", func(t *testing.T) {
		routerFunc := KeywordRouter(map[string][]string{
			"math": {"calculate", "compute", "add", "subtract"},
			"text": {"write", "edit", "format"},
		}, "query")

		route, err := routerFunc(map[string]any{"query": "please calculate 2+2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if route != "math" {
			t.Errorf("expected 'math', got '%s'", route)
		}

		route, err = routerFunc(map[string]any{"query": "write a poem"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if route != "text" {
			t.Errorf("expected 'text', got '%s'", route)
		}
	})

	t.Run("NoMatchNoDefault", func(t *testing.T) {
		chain1 := NewMockChain("chain1", []string{"input"}, []string{"output"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output": "result"}, nil
		})

		router, err := NewRouterChain(RouterChainConfig{
			Routes: map[string]Chain{
				"route1": chain1,
			},
			RouterFunc: func(inputs map[string]any) (string, error) {
				return "unknown_route", nil
			},
		})
		if err != nil {
			t.Fatalf("failed to create router chain: %v", err)
		}

		_, err = router.Call(context.Background(), map[string]any{"input": "test"})
		if err == nil {
			t.Error("expected error for unknown route without default")
		}
	})
}

// TestTransformChain tests transform chain functionality
func TestTransformChain(t *testing.T) {
	t.Run("BasicTransform", func(t *testing.T) {
		innerChain := NewMockChain("inner", []string{"transformed"}, []string{"output"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output": inputs["transformed"].(string) + "_processed"}, nil
		})

		transform, err := NewTransformChain(TransformChainConfig{
			InnerChain: innerChain,
			TransformFunc: func(inputs map[string]any) (map[string]any, error) {
				return map[string]any{"transformed": "prefix_" + inputs["input"].(string)}, nil
			},
		})
		if err != nil {
			t.Fatalf("failed to create transform chain: %v", err)
		}

		result, err := transform.Call(context.Background(), map[string]any{"input": "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := "prefix_test_processed"
		if result["output"] != expected {
			t.Errorf("expected '%s', got '%v'", expected, result["output"])
		}
	})

	t.Run("TransformError", func(t *testing.T) {
		innerChain := NewMockChain("inner", []string{"input"}, []string{"output"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output": "done"}, nil
		})

		transform, err := NewTransformChain(TransformChainConfig{
			InnerChain: innerChain,
			TransformFunc: func(inputs map[string]any) (map[string]any, error) {
				return nil, errors.New("transform failed")
			},
		})
		if err != nil {
			t.Fatalf("failed to create transform chain: %v", err)
		}

		_, err = transform.Call(context.Background(), map[string]any{"input": "test"})
		if err == nil {
			t.Error("expected error from transform")
		}
	})
}

// TestFuncChain tests function chain functionality
func TestFuncChain(t *testing.T) {
	t.Run("BasicFunc", func(t *testing.T) {
		funcChain, err := NewFuncChain(FuncChainConfig{
			Func: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				val := inputs["input"].(string)
				return map[string]any{"output": val + "_processed"}, nil
			},
			Name:       "my_func",
			InputKeys:  []string{"input"},
			OutputKeys: []string{"output"},
		})
		if err != nil {
			t.Fatalf("failed to create func chain: %v", err)
		}

		if funcChain.Name() != "my_func" {
			t.Errorf("expected name 'my_func', got '%s'", funcChain.Name())
		}

		result, err := funcChain.Call(context.Background(), map[string]any{"input": "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result["output"] != "test_processed" {
			t.Errorf("expected 'test_processed', got '%v'", result["output"])
		}
	})

	t.Run("FuncError", func(t *testing.T) {
		funcChain, err := NewFuncChain(FuncChainConfig{
			Func: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				return nil, errors.New("func error")
			},
		})
		if err != nil {
			t.Fatalf("failed to create func chain: %v", err)
		}

		_, err = funcChain.Call(context.Background(), map[string]any{})
		if err == nil {
			t.Error("expected error from func")
		}
	})
}

// TestPassthroughChain tests passthrough chain functionality
func TestPassthroughChain(t *testing.T) {
	t.Run("BasicPassthrough", func(t *testing.T) {
		chain := NewPassthroughChain([]string{"key1", "key2"})

		result, err := chain.Call(context.Background(), map[string]any{
			"key1":  "value1",
			"key2":  "value2",
			"extra": "ignored",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result["key1"] != "value1" {
			t.Errorf("expected 'value1', got '%v'", result["key1"])
		}
		if result["key2"] != "value2" {
			t.Errorf("expected 'value2', got '%v'", result["key2"])
		}
		if _, ok := result["extra"]; ok {
			t.Error("extra key should not be in output")
		}
	})
}

// TestSimpleSequentialChain tests simple sequential chain
func TestSimpleSequentialChain(t *testing.T) {
	t.Run("BasicExecution", func(t *testing.T) {
		chain1 := NewMockChain("chain1", []string{"input"}, []string{"text"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"text": inputs["input"].(string) + "_step1"}, nil
		})

		chain2 := NewMockChain("chain2", []string{"text"}, []string{"result"}, func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"result": inputs["text"].(string) + "_step2"}, nil
		})

		simple, err := NewSimpleSequentialChain(SimpleSequentialChainConfig{
			Chains: []Chain{chain1, chain2},
		})
		if err != nil {
			t.Fatalf("failed to create simple sequential chain: %v", err)
		}

		result, err := simple.Call(context.Background(), map[string]any{"input": "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result["result"] != "test_step1_step2" {
			t.Errorf("expected 'test_step1_step2', got '%v'", result["result"])
		}
	})
}

// TestOptions tests chain options
func TestOptions(t *testing.T) {
	t.Run("DefaultOptions", func(t *testing.T) {
		opts := DefaultOptions()
		if opts.Timeout != 60*time.Second {
			t.Errorf("expected 60s timeout, got %v", opts.Timeout)
		}
		if opts.MaxRetries != 3 {
			t.Errorf("expected 3 retries, got %d", opts.MaxRetries)
		}
		if opts.Verbose {
			t.Error("expected verbose to be false by default")
		}
	})

	t.Run("ApplyOptions", func(t *testing.T) {
		opts := ApplyOptions(
			WithTimeout(30*time.Second),
			WithMaxRetries(5),
			WithVerbose(true),
			WithMetadata(map[string]any{"key": "value"}),
		)

		if opts.Timeout != 30*time.Second {
			t.Errorf("expected 30s timeout, got %v", opts.Timeout)
		}
		if opts.MaxRetries != 5 {
			t.Errorf("expected 5 retries, got %d", opts.MaxRetries)
		}
		if !opts.Verbose {
			t.Error("expected verbose to be true")
		}
		if opts.Metadata["key"] != "value" {
			t.Errorf("expected metadata key 'value', got '%v'", opts.Metadata["key"])
		}
	})

	t.Run("WithTemperature", func(t *testing.T) {
		opts := ApplyOptions(WithTemperature(0.7))
		if opts.Temperature == nil || *opts.Temperature != 0.7 {
			t.Error("expected temperature 0.7")
		}
	})

	t.Run("WithMaxTokens", func(t *testing.T) {
		opts := ApplyOptions(WithMaxTokens(1000))
		if opts.MaxTokens == nil || *opts.MaxTokens != 1000 {
			t.Error("expected max tokens 1000")
		}
	})
}

// TestHelperFunctions tests helper functions
func TestHelperFunctions(t *testing.T) {
	t.Run("CopyInputs", func(t *testing.T) {
		original := map[string]any{"key1": "value1", "key2": "value2"}
		copied := CopyInputs(original)

		if copied["key1"] != "value1" || copied["key2"] != "value2" {
			t.Error("copied values don't match original")
		}

		copied["key1"] = "modified"
		if original["key1"] != "value1" {
			t.Error("modifying copy affected original")
		}
	})

	t.Run("MergeInputs", func(t *testing.T) {
		map1 := map[string]any{"key1": "value1"}
		map2 := map[string]any{"key2": "value2"}
		map3 := map[string]any{"key1": "overridden", "key3": "value3"}

		merged := MergeInputs(map1, map2, map3)

		if merged["key1"] != "overridden" {
			t.Errorf("expected 'overridden', got '%v'", merged["key1"])
		}
		if merged["key2"] != "value2" {
			t.Errorf("expected 'value2', got '%v'", merged["key2"])
		}
		if merged["key3"] != "value3" {
			t.Errorf("expected 'value3', got '%v'", merged["key3"])
		}
	})
}

// TestStreamEvent tests stream event creation
func TestStreamEvent(t *testing.T) {
	t.Run("MakeStreamEvent", func(t *testing.T) {
		event := MakeStreamEvent(StreamEventToken, "hello", map[string]any{"key": "value"}, nil)

		if event.Type != StreamEventToken {
			t.Errorf("expected StreamEventToken, got %v", event.Type)
		}
		if event.Content != "hello" {
			t.Errorf("expected 'hello', got '%s'", event.Content)
		}
		if event.Data["key"] != "value" {
			t.Errorf("expected data key 'value', got '%v'", event.Data["key"])
		}
		if event.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("ErrorEvent", func(t *testing.T) {
		err := errors.New("test error")
		event := MakeStreamEvent(StreamEventError, "", nil, err)

		if event.Type != StreamEventError {
			t.Errorf("expected StreamEventError, got %v", event.Type)
		}
		if event.Error != err {
			t.Error("expected error to be set")
		}
	})
}

// TestCallbackManager tests callback management
func TestCallbackManager(t *testing.T) {
	t.Run("MultipleCallbacks", func(t *testing.T) {
		startCount := 0
		endCount := 0

		cb1 := &testCallback{
			onStart: func() { startCount++ },
			onEnd:   func() { endCount++ },
		}
		cb2 := &testCallback{
			onStart: func() { startCount++ },
			onEnd:   func() { endCount++ },
		}

		manager := NewCallbackManager(cb1, cb2)

		ctx := context.Background()
		manager.OnChainStart(ctx, "test", nil)
		manager.OnChainEnd(ctx, "test", nil)

		if startCount != 2 {
			t.Errorf("expected 2 start callbacks, got %d", startCount)
		}
		if endCount != 2 {
			t.Errorf("expected 2 end callbacks, got %d", endCount)
		}
	})

	t.Run("AddCallback", func(t *testing.T) {
		manager := NewCallbackManager()
		count := 0

		cb := &testCallback{
			onStart: func() { count++ },
		}
		manager.Add(cb)

		ctx := context.Background()
		manager.OnChainStart(ctx, "test", nil)

		if count != 1 {
			t.Errorf("expected 1 callback, got %d", count)
		}
	})
}

// testCallback is a test implementation of ChainCallback
type testCallback struct {
	NoopCallback
	onStart func()
	onEnd   func()
}

func (t *testCallback) OnChainStart(ctx context.Context, chainName string, inputs map[string]any) {
	if t.onStart != nil {
		t.onStart()
	}
}

func (t *testCallback) OnChainEnd(ctx context.Context, chainName string, outputs map[string]any) {
	if t.onEnd != nil {
		t.onEnd()
	}
}
