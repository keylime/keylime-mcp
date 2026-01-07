package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultServerPath = "../bin/server"
	mcpClientName     = "mcp-client"
	mcpClientVersion  = "v1.0.0"

	DefaultModel     = anthropic.ModelClaude3_5HaikuLatest
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
	APIKey       string
	ServerPath   string
	Model        anthropic.Model
	MaxTokens    int64
	MaxTurns     int
	SystemPrompt string
}

type Agent struct {
	config          Config
	anthropicClient anthropic.Client
	mcpSession      *mcp.ClientSession
	mcpCmd          *exec.Cmd
	tools           []anthropic.ToolUnionParam
	messages        []anthropic.MessageParam
}

type Message struct {
	Role    string // "user", "assistant", "tool_request", "tool_result"
	Content string
	ToolID  string
	Tool    *ToolRequest
}

type ToolRequest struct {
	ID        string
	Name      string
	Arguments any
}

func NewAgent(cfg Config) *Agent {
	// Apply defaults
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
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
		config:          cfg,
		anthropicClient: anthropic.NewClient(option.WithAPIKey(cfg.APIKey)),
		messages:        []anthropic.MessageParam{},
	}
}

func (a *Agent) Connect(ctx context.Context) error {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    mcpClientName,
		Version: mcpClientVersion,
	}, nil)
	cmd := exec.Command(a.config.ServerPath)
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
	tools, err := a.mcpSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}
	a.tools = make([]anthropic.ToolUnionParam, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		a.tools = append(a.tools, convertMCPToolToClaudeTool(tool))
	}
	return nil
}

func convertMCPToolToClaudeTool(tool *mcp.Tool) anthropic.ToolUnionParam {
	inputSchemaMap, ok := tool.InputSchema.(map[string]any)
	if !ok || inputSchemaMap == nil {
		inputSchemaMap = map[string]any{}
	}

	var properties any
	if p, ok := inputSchemaMap["properties"].(map[string]any); ok && p != nil {
		properties = p
	} else {
		properties = map[string]any{}
	}

	var required []string
	if r, ok := inputSchemaMap["required"].([]interface{}); ok {
		for _, v := range r {
			if s, ok := v.(string); ok {
				required = append(required, s)
			}
		}
	}

	toolParam := anthropic.ToolUnionParamOfTool(
		anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: properties,
			Required:   required,
		},
		tool.Name,
	)

	toolParam.OfTool.Description = anthropic.String(tool.Description)
	return toolParam
}

func (a *Agent) Close() {
	if a.mcpSession != nil {
		a.mcpSession.Close()
	}
	if a.mcpCmd != nil {
		a.mcpCmd.Process.Kill()
	}
}

func (a *Agent) SendMessage(ctx context.Context, userMessage string, onMessage func(Message)) error {
	a.messages = append(a.messages, anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)))
	// TODO: add maximun turns and add support for gemini
	return a.callClaude(ctx, onMessage)
}

func (a *Agent) callClaude(ctx context.Context, onMessage func(Message)) error {
	response, err := a.anthropicClient.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     a.config.Model,
		MaxTokens: a.config.MaxTokens,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: a.config.SystemPrompt}},
		Messages:  a.messages,
		Tools:     a.tools,
	})
	if err != nil {
		return fmt.Errorf("claude API error: %w", err)
	}

	assistantContent, toolRequests := a.processResponse(response, onMessage)

	a.messages = append(a.messages, anthropic.NewAssistantMessage(assistantContent...))

	for _, toolRequest := range toolRequests {
		onMessage(Message{
			Role:    "tool_request",
			Content: toolRequest.Name,
			ToolID:  toolRequest.ID,
			Tool:    toolRequest,
		})
	}
	return nil
}

func (a *Agent) processResponse(response *anthropic.Message, onMessage func(Message)) (
	[]anthropic.ContentBlockParamUnion,
	[]*ToolRequest,
) {
	var assistantContent []anthropic.ContentBlockParamUnion
	var toolRequests []*ToolRequest

	for _, block := range response.Content {
		switch content := block.AsAny().(type) {
		case anthropic.TextBlock:
			onMessage(Message{Role: "assistant", Content: content.Text})
			assistantContent = append(assistantContent, anthropic.NewTextBlock(content.Text))

		case anthropic.ToolUseBlock:
			assistantContent = append(assistantContent, anthropic.NewToolUseBlock(content.ID, content.Input, content.Name))
			toolRequests = append(toolRequests, &ToolRequest{
				ID:        content.ID,
				Name:      content.Name,
				Arguments: content.Input,
			})
		}
	}

	return assistantContent, toolRequests
}

func (a *Agent) ExecuteTool(ctx context.Context, toolRequest *ToolRequest, onMessage func(Message)) error {
	result, err := a.mcpSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolRequest.Name,
		Arguments: toolRequest.Arguments,
	})

	var resultText string
	var isError bool

	if err != nil {
		resultText = fmt.Sprintf("Error: %v", err)
		isError = true
	} else if result.IsError {
		resultText = fmt.Sprintf("Tool '%s' execution failed: %s", toolRequest.Name, extractTextContent(result.Content))
		isError = true
	} else {
		resultText = extractTextContent(result.Content)
	}

	onMessage(Message{
		Role:    "tool_result",
		Content: resultText,
		ToolID:  toolRequest.ID,
	})

	a.messages = append(a.messages, anthropic.NewUserMessage(
		anthropic.NewToolResultBlock(toolRequest.ID, resultText, isError),
	))

	return a.callClaude(ctx, onMessage)
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
