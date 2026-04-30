package errors

import "fmt"

// APIError wraps AI/OpenAI API failures with status context.
type APIError struct {
	Op      string // operation: "complete", "stream", "image", etc.
	Message string
	Err     error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

func (e *APIError) Unwrap() error { return e.Err }

// MCPError wraps MCP server communication failures.
type MCPError struct {
	Server string
	Op     string // "start", "call", "read", "write"
	Err    error
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("mcp %s: %s: %v", e.Server, e.Op, e.Err)
}

func (e *MCPError) Unwrap() error { return e.Err }

// ToolError wraps tool execution failures.
type ToolError struct {
	Tool string
	Err  error
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("tool %s: %v", e.Tool, e.Err)
}

func (e *ToolError) Unwrap() error { return e.Err }

// ConfigError wraps configuration validation failures.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config %s: %s", e.Field, e.Message)
}

func (e *ConfigError) Unwrap() error { return nil }

// BudgetError signals cost limit exceeded.
type BudgetError struct {
	Limit   float64
	Current float64
}

func (e *BudgetError) Error() string {
	return fmt.Sprintf("monthly cost limit reached ($%.4f / $%.2f). Adjust with: gpterminal config set cost_limit <amount>", e.Current, e.Limit)
}

func (e *BudgetError) Unwrap() error { return nil }
