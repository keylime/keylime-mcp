package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/keylime/keylime-mcp/internal/masking"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpClientName    = "mcp-client"
	mcpClientVersion = "v1.0.0"

	DefaultMaxTokens = 2048
	DefaultSystemPrompt = `You are a Keylime infrastructure assistant with access to tools. You help users manage and monitor Keylime agents.

When users request information or actions, call the appropriate tool directly. You can call tools in sequence to complete multi-step tasks. After receiving tool results, summarize them for the user. If a tool returns an error, explain the issue and suggest a resolution.`
)

type Config struct {
	ServerPath   string
	Model        string
	MaxTokens    int64
	SystemPrompt string
}

type Agent struct {
	config     Config
	provider   LLMProvider
	masker     *masking.Engine
	mcpSession *mcp.ClientSession
	mcpCmd     *exec.Cmd
	tools      []*mcp.Tool

	mu        sync.Mutex
	messages  []Message
	toolQueue []ToolRequest
}

func NewAgent(cfg Config, provider LLMProvider, masker *masking.Engine) *Agent {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = DefaultMaxTokens
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = DefaultSystemPrompt
	}

	return &Agent{
		config:   cfg,
		provider: provider,
		masker:   masker,
		messages: []Message{},
	}
}

func (a *Agent) Connect(ctx context.Context) error {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    mcpClientName,
		Version: mcpClientVersion,
	}, nil)
	cmd := exec.Command(a.config.ServerPath) // #nosec G204 -- ServerPath is from trusted config, not user input
	cmd.Env = append(os.Environ(), "MASKING_ENABLED=false")
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

	messagesCopy := make([]Message, len(a.messages))
	copy(messagesCopy, a.messages)

	if a.masker != nil {
		for i := range messagesCopy {
			if messagesCopy[i].ToolResult != nil {
				messagesCopy[i].ToolResult = &ToolResult{
					ToolID:  messagesCopy[i].ToolResult.ToolID,
					Output:  a.masker.Mask(messagesCopy[i].ToolResult.Output),
					IsError: messagesCopy[i].ToolResult.IsError,
				}
			}
		}
	}

	opts := ChatOptions{
		Model:        a.config.Model,
		MaxTokens:    a.config.MaxTokens,
		SystemPrompt: a.config.SystemPrompt,
		Messages:     messagesCopy,
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
		displayText := msg.Text
		if a.masker != nil {
			displayText = a.masker.Unmask(displayText)
		}
		onMessage(Message{Role: RoleAssistant, Text: displayText})
	}
	if firstTool != nil {
		onMessage(Message{Role: RoleAssistant, ToolCalls: []ToolRequest{a.unmaskToolRequest(*firstTool)}})
	}

	return nil
}

func (a *Agent) ExecuteTool(ctx context.Context, toolRequest *ToolRequest, onMessage func(Message)) error {
	unmasked := a.unmaskToolRequest(*toolRequest)

	result, err := a.mcpSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      unmasked.Name,
		Arguments: unmasked.Arguments,
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
		onMessage(Message{Role: RoleAssistant, ToolCalls: []ToolRequest{a.unmaskToolRequest(*nextTool)}})
		return nil
	}

	return a.callLLM(ctx, onMessage)
}

func (a *Agent) unmaskToolRequest(tr ToolRequest) ToolRequest {
	if a.masker == nil || !a.masker.Enabled() {
		return tr
	}
	argsJSON, err := json.Marshal(tr.Arguments)
	if err != nil {
		return tr
	}
	unmasked := a.masker.Unmask(string(argsJSON))
	if unmasked == string(argsJSON) {
		return tr
	}
	var realArgs any
	if err := json.Unmarshal([]byte(unmasked), &realArgs); err != nil {
		return tr
	}
	return ToolRequest{ID: tr.ID, Name: tr.Name, Arguments: realArgs}
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

func (a *Agent) GetProvider() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.provider == nil {
		return ""
	}
	return a.provider.Name()
}
