package agent

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role
	Text       string
	ToolCalls  []ToolRequest
	ToolResult *ToolResult
}

type ToolRequest struct {
	ID        string
	Name      string
	Arguments any
}

type ToolResult struct {
	ToolID  string
	Output  string
	IsError bool
}
