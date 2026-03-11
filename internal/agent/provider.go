package agent

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LLMProvider defines the interface for LLM backends (Adapter pattern).
// Each provider converts generic Messages and MCP tools into its native format.
// Implementing a new provider requires only a new provider_X.go file.
type LLMProvider interface {
	Chat(ctx context.Context, opts ChatOptions) (*LLMResponse, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
	Name() string
}

// ChatOptions contains all parameters needed for an LLM API call.
type ChatOptions struct {
	SystemPrompt string
	Messages     []Message
	Tools        []*mcp.Tool
	MaxTokens    int64
	Model        string
}

// LLMResponse contains the parsed response from an LLM provider.
type LLMResponse struct {
	TextBlocks []string
	ToolUses   []ToolRequest
}
