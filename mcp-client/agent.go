package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	ErrMissingAPIKey = errors.New("ANTHROPIC_API_KEY environment variable not set")
	ErrServerNotFound = errors.New("MCP server binary not found")
)

type OutputHandler func(message string)

type Agent struct {
	config          *Config
	session         *mcp.ClientSession
	cmd             *exec.Cmd
	anthropicClient anthropic.Client
	claudeTools     []anthropic.ToolUnionParam
	outputHandler   OutputHandler
}

func NewAgent(cfg *Config) *Agent {
	return &Agent{
		config:        cfg,
		outputHandler: defaultOutputHandler,
	}
}

func (a *Agent) SetOutputHandler(handler OutputHandler) {
	a.outputHandler = handler
}

func (a *Agent) output(message string) {
	if a.outputHandler != nil {
		a.outputHandler(message)
	}
}

func defaultOutputHandler(message string) {
	fmt.Println(message)
}

func (a *Agent) Initialize(ctx context.Context) error {
	if err := a.connectToMCPServer(ctx); err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	if err := a.loadMCPTools(ctx); err != nil {
		return fmt.Errorf("failed to load MCP tools: %w", err)
	}

	a.anthropicClient = anthropic.NewClient(option.WithAPIKey(a.config.AnthropicAPIKey))

	return nil
}

func (a *Agent) Close() {
	if a.session != nil {
		a.session.Close()
	}
	if a.cmd != nil && a.cmd.Process != nil {
		a.cmd.Process.Kill()
	}
}

func (a *Agent) connectToMCPServer(ctx context.Context) error {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    mcpClientName,
		Version: mcpClientVersion,
	}, nil)

	cmd := exec.Command(a.config.MCPServerPath)
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	a.session = session
	a.cmd = cmd

	a.monitorServerProcess(ctx)

	log.Printf("Connected to MCP server: %s", a.config.MCPServerPath)
	return nil
}

func (a *Agent) monitorServerProcess(ctx context.Context) {
	if a.cmd.Process != nil {
		go func() {
			state, waitErr := a.cmd.Process.Wait()
			if ctx.Err() != nil {
				return
			}
			if waitErr != nil {
				log.Printf("[Warning] MCP server process monitoring failed: %v", waitErr)
			} else if !state.Success() {
				log.Printf("[Error] MCP server process exited unexpectedly with status: %v", state)
			} else {
				log.Printf("[Info] MCP server process exited normally")
			}
		}()
	}
}

func (a *Agent) loadMCPTools(ctx context.Context) error {
	tools, err := a.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	a.claudeTools = make([]anthropic.ToolUnionParam, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		claudeTool := a.convertMCPToolToClaudeTool(tool)
		a.claudeTools = append(a.claudeTools, claudeTool)
	}

	return nil
}

func (a *Agent) convertMCPToolToClaudeTool(tool *mcp.Tool) anthropic.ToolUnionParam {
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

func (a *Agent) ProcessQuery(ctx context.Context, query string) error {
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(query)),
	}

	for _ = range maxAgentTurns {
		message, err := a.anthropicClient.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     claudeModel,
			MaxTokens: maxTokens,
			System:    []anthropic.TextBlockParam{{Type: "text", Text: systemPrompt}},
			Messages:  messages,
			Tools:     a.claudeTools,
		})
		if err != nil {
			return fmt.Errorf("claude API error: %w", err)
		}

		assistantContent, toolResults, hasToolUse := a.processClaudeResponse(ctx, message)

		if !hasToolUse {
			return nil
		}

		messages = append(messages, anthropic.NewAssistantMessage(assistantContent...))
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	a.output("\n=== Maximum turns reached, requesting summary ===")
	a.generateFinalSummary(ctx, messages)

	return nil
}

func (a *Agent) processClaudeResponse(
	ctx context.Context,
	message *anthropic.Message,
) (
	assistantContent []anthropic.ContentBlockParamUnion,
	toolResults []anthropic.ContentBlockParamUnion,
	hasToolUse bool,
) {
	assistantContent = []anthropic.ContentBlockParamUnion{}
	toolResults = []anthropic.ContentBlockParamUnion{}

	for _, block := range message.Content {
		switch block := block.AsAny().(type) {
		case anthropic.TextBlock:
			a.output(block.Text)
			assistantContent = append(assistantContent, anthropic.NewTextBlock(block.Text))

		case anthropic.ToolUseBlock:
			hasToolUse = true
			log.Printf("\n[Tool Use] %s", block.Name)

			assistantContent = append(assistantContent,
				anthropic.NewToolUseBlock(block.ID, block.Input, block.Name))

			toolResult := a.executeToolCall(ctx, block)
			toolResults = append(toolResults, toolResult)
		}
	}

	return assistantContent, toolResults, hasToolUse
}

func (a *Agent) executeToolCall(
	ctx context.Context,
	toolUse anthropic.ToolUseBlock,
) anthropic.ContentBlockParamUnion {
	result, err := a.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolUse.Name,
		Arguments: toolUse.Input,
	})

	if err != nil {
		log.Printf("[Error] CallTool failed: %v", err)
		return anthropic.NewToolResultBlock(
			toolUse.ID,
			fmt.Sprintf("Error: %v", err),
			true,
		)
	}

	if result.IsError {
		errorDetails := extractTextContent(result.Content)
		log.Printf("[Error] Tool execution failed for tool '%s': %s", toolUse.Name, errorDetails)
		return anthropic.NewToolResultBlock(
			toolUse.ID,
			fmt.Sprintf("Tool '%s' execution failed: %s", toolUse.Name, errorDetails),
			true,
		)
	}

	resultText := extractTextContent(result.Content)
	if resultText == "" {
		log.Printf("[Warning] Tool returned empty content - this might indicate an unexpected response from MCP server")
	}
	log.Printf("================================================")
	log.Printf("[Tool Result]\n%s", resultText)
	log.Printf("================================================")

	return anthropic.NewToolResultBlock(toolUse.ID, resultText, false)
}

func (a *Agent) generateFinalSummary(ctx context.Context, messages []anthropic.MessageParam) {
	summaryPrompt := `I've reached the maximum number of allowed turns. Please provide a summary of:
1. What you accomplished so far
2. What still needs to be done
3. Any issues or blockers encountered`

	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(summaryPrompt)))

	finalMsg, err := a.anthropicClient.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     claudeModel,
		MaxTokens: maxTokens,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: systemPrompt}},
		Messages:  messages,
	})
	if err != nil {
		log.Printf("failed to get final summary: %v", err)
		return
	}

	for _, block := range finalMsg.Content {
		if textBlock, ok := block.AsAny().(anthropic.TextBlock); ok {
			a.output(textBlock.Text)
		}
	}
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
