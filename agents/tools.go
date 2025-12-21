package agents

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"strconv"
	"strings"
)

// CalculatorTool performs mathematical calculations
type CalculatorTool struct{}

// NewCalculatorTool creates a new calculator tool
func NewCalculatorTool() *CalculatorTool {
	return &CalculatorTool{}
}

// Name returns the tool name
func (t *CalculatorTool) Name() string {
	return "Calculator"
}

// Description returns the tool description
func (t *CalculatorTool) Description() string {
	return "Useful for performing mathematical calculations. Input should be a mathematical expression like '2 + 2' or '(10 * 5) / 2'."
}

// Call executes the calculation
func (t *CalculatorTool) Call(ctx context.Context, input string) (string, error) {
	input = strings.TrimSpace(input)

	// Parse and evaluate the expression
	result, err := evaluateExpression(input)
	if err != nil {
		return "", fmt.Errorf("calculation error: %w", err)
	}

	return fmt.Sprintf("%v", result), nil
}

// evaluateExpression evaluates a simple mathematical expression
func evaluateExpression(expr string) (float64, error) {
	// Use Go's parser for safe evaluation
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, err
	}
	return eval(node)
}

func eval(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		return strconv.ParseFloat(n.Value, 64)
	case *ast.BinaryExpr:
		left, err := eval(n.X)
		if err != nil {
			return 0, err
		}
		right, err := eval(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op.String() {
		case "+":
			return left + right, nil
		case "-":
			return left - right, nil
		case "*":
			return left * right, nil
		case "/":
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("unsupported operator: %s", n.Op.String())
		}
	case *ast.ParenExpr:
		return eval(n.X)
	case *ast.UnaryExpr:
		val, err := eval(n.X)
		if err != nil {
			return 0, err
		}
		if n.Op.String() == "-" {
			return -val, nil
		}
		return val, nil
	default:
		return 0, fmt.Errorf("unsupported expression type: %T", node)
	}
}

// FunctionTool wraps a function as a tool
type FunctionTool struct {
	name        string
	description string
	fn          func(ctx context.Context, input string) (string, error)
}

// FunctionToolConfig configures a function tool
type FunctionToolConfig struct {
	Name        string
	Description string
	Func        func(ctx context.Context, input string) (string, error)
}

// NewFunctionTool creates a new function tool
func NewFunctionTool(cfg FunctionToolConfig) (*FunctionTool, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if cfg.Description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if cfg.Func == nil {
		return nil, fmt.Errorf("func is required")
	}

	return &FunctionTool{
		name:        cfg.Name,
		description: cfg.Description,
		fn:          cfg.Func,
	}, nil
}

// Name returns the tool name
func (t *FunctionTool) Name() string {
	return t.name
}

// Description returns the tool description
func (t *FunctionTool) Description() string {
	return t.description
}

// Call executes the function
func (t *FunctionTool) Call(ctx context.Context, input string) (string, error) {
	return t.fn(ctx, input)
}

// SearchTool is a placeholder for search functionality
type SearchTool struct {
	searchFunc func(ctx context.Context, query string) (string, error)
}

// NewSearchTool creates a new search tool with a custom search function
func NewSearchTool(searchFunc func(ctx context.Context, query string) (string, error)) *SearchTool {
	return &SearchTool{searchFunc: searchFunc}
}

// Name returns the tool name
func (t *SearchTool) Name() string {
	return "Search"
}

// Description returns the tool description
func (t *SearchTool) Description() string {
	return "Useful for searching for information. Input should be a search query."
}

// Call executes the search
func (t *SearchTool) Call(ctx context.Context, input string) (string, error) {
	if t.searchFunc == nil {
		return "", fmt.Errorf("search function not configured")
	}
	return t.searchFunc(ctx, input)
}

// ToolRegistry manages available tools
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[strings.ToLower(tool.Name())] = tool
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[strings.ToLower(name)]
	return tool, ok
}

// List returns all registered tools
func (r *ToolRegistry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Remove removes a tool from the registry
func (r *ToolRegistry) Remove(name string) {
	delete(r.tools, strings.ToLower(name))
}
