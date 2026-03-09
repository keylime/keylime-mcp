package agent

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpClientName    = "mcp-client"
	mcpClientVersion = "v1.0.0"

	DefaultMaxTokens = 2048
	DefaultMaxTurns  = 5

	DefaultSystemPrompt = `You are an AI assistant with access to Keylime system management tools. Your goal is to help users manage and monitor their Keylime infrastructure.

You have a maximum of 5 conversation turns to complete the task. When given a task:
1. Break it down into steps if needed
2. Use available tools to gather information and take actions
3. Chain multiple tool calls together to accomplish complex tasks
4. Provide clear explanations of what you're doing and what you found
5. If you encounter failures, investigate and suggest solutions
6. Work efficiently to complete tasks within the turn limit`
)

type Config struct {
	ServerPath   string
	Model        string
	MaxTokens    int64
	MaxTurns     int
	SystemPrompt string
}

type Agent struct {
	config     Config
	provider   LLMProvider
	mcpSession *mcp.ClientSession
	mcpCmd     *exec.Cmd
	tools      []*mcp.Tool

	mu        sync.Mutex
	messages  []Message
	toolQueue []ToolRequest
}

func NewAgent(cfg Config, provider LLMProvider) *Agent {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = DefaultMaxTokens
	}
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = DefaultMaxTurns
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = DefaultSystemPrompt
	}

	return &Agent{
		config:   cfg,
		provider: provider,
		messages: []Message{},
	}
}

func (a *Agent) Connect(ctx context.Context) error {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    mcpClientName,
		Version: mcpClientVersion,
	}, nil)
	cmd := exec.Command(a.config.ServerPath) // #nosec G204 -- ServerPath is from trusted config, not user input
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	a.mcpSession = session
	a.mcpCmd = cmd
	return nil
}

func (a *Agent) GetTools(ctx context.Context) error {
	result, err := a.mcpSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}
	a.tools = result.Tools
	return nil
}

func (a *Agent) Close() {
	if a.mcpSession != nil {
		if err := a.mcpSession.Close(); err != nil {
			log.Printf("Warning: failed to close MCP session: %v", err)
		}
	}
	if a.mcpCmd != nil && a.mcpCmd.Process != nil {
		if err := a.mcpCmd.Process.Kill(); err != nil {
			log.Printf("Warning: failed to kill MCP process: %v", err)
		}
	}
}

func (a *Agent) SendMessage(ctx context.Context, userMessage string, onMessage func(Message)) error {
	a.mu.Lock()
	a.messages = append(a.messages, Message{Role: RoleUser, Text: userMessage})
	a.mu.Unlock()
	return a.callLLM(ctx, onMessage)
}

func (a *Agent) callLLM(ctx context.Context, onMessage func(Message)) error {
	a.mu.Lock()
	opts := ChatOptions{
		Model:        a.config.Model,
		MaxTokens:    a.config.MaxTokens,
		SystemPrompt: a.config.SystemPrompt,
		Messages:     a.messages,
		Tools:        a.tools,
	}
	provider := a.provider
	a.mu.Unlock()

	response, err := provider.Chat(ctx, opts)
	if err != nil {
		return err
	}

	msg := Message{
		Role:      RoleAssistant,
		Text:      strings.Join(response.TextBlocks, "\n"),
		ToolCalls: response.ToolUses,
	}

	a.mu.Lock()
	a.messages = append(a.messages, msg)
	a.toolQueue = make([]ToolRequest, len(response.ToolUses))
	copy(a.toolQueue, response.ToolUses)

	var firstTool *ToolRequest
	if len(a.toolQueue) > 0 {
		firstTool = &a.toolQueue[0]
	}
	a.mu.Unlock()

	if msg.Text != "" {
		onMessage(Message{Role: RoleAssistant, Text: msg.Text})
	}
	if firstTool != nil {
		onMessage(Message{Role: RoleAssistant, ToolCalls: []ToolRequest{*firstTool}})
	}

	return nil
}

func (a *Agent) ExecuteTool(ctx context.Context, toolRequest *ToolRequest, onMessage func(Message)) error {
	result, err := a.mcpSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolRequest.Name,
		Arguments: toolRequest.Arguments,
	})

	var resultText string
	var isError bool

	switch {
	case err != nil:
		resultText = fmt.Sprintf("Error: %v", err)
		isError = true
	case result.IsError:
		resultText = fmt.Sprintf("Tool '%s' execution failed: %s", toolRequest.Name, extractTextContent(result.Content))
		isError = true
	default:
		resultText = extractTextContent(result.Content)
	}

	msg := Message{
		Role: RoleTool,
		ToolResult: &ToolResult{
			ToolID:  toolRequest.ID,
			Output:  resultText,
			IsError: isError,
		},
	}

	a.mu.Lock()
	a.messages = append(a.messages, msg)
	a.mu.Unlock()

	onMessage(msg)

	return a.advanceToolQueue(ctx, onMessage)
}

func (a *Agent) GetCurrentTool() *ToolRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.toolQueue) == 0 {
		return nil
	}
	return &a.toolQueue[0]
}

func (a *Agent) ToolDeny(ctx context.Context, tool *ToolRequest, onMessage func(Message)) error {
	a.mu.Lock()
	a.messages = append(a.messages, Message{
		Role: RoleTool,
		ToolResult: &ToolResult{
			ToolID: tool.ID,
			Output: "Tool execution denied by user.",
		},
	})
	a.mu.Unlock()

	return a.advanceToolQueue(ctx, onMessage)
}

func (a *Agent) advanceToolQueue(ctx context.Context, onMessage func(Message)) error {
	a.mu.Lock()
	if len(a.toolQueue) > 0 {
		a.toolQueue = a.toolQueue[1:]
	}
	var nextTool *ToolRequest
	if len(a.toolQueue) > 0 {
		nextTool = &a.toolQueue[0]
	}
	a.mu.Unlock()

	if nextTool != nil {
		onMessage(Message{Role: RoleAssistant, ToolCalls: []ToolRequest{*nextTool}})
		return nil
	}

	return a.callLLM(ctx, onMessage)
}

func extractTextContent(content []mcp.Content) string {
	var resultText strings.Builder

	for _, c := range content {
		if textContent, ok := c.(*mcp.TextContent); ok {
			resultText.WriteString(textContent.Text)
		}
	}

	return resultText.String()
}

func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = []Message{}
	a.toolQueue = nil
}

func (a *Agent) SetModel(provider LLMProvider, model string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.provider = provider
	a.config.Model = model
}

func (a *Agent) GetModel() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config.Model
}
