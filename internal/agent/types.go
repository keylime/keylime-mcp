package agent

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
)

type Message struct {
	Role    Role      `json:"role"`
	Content []Content `json:"content"`
}

type Content struct {
	Type ContentType `json:"type"`
	Text string      `json:"text,omitempty"`

	ToolID    string `json:"tool_id,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	ToolInput any    `json:"tool_input,omitempty"`

	IsError bool `json:"is_error,omitempty"`
}

type ToolRequest struct {
	ID        string
	Name      string
	Arguments any
}
