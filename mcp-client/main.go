package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverPath       = "../backend/server"
	mcpClientName    = "mcp-client"
	mcpClientVersion = "v1.0.0"

	claudeModel   = anthropic.ModelClaude_3_Haiku_20240307
	maxTokens     = 1024
	maxAgentTurns = 5

	systemPrompt = `You are an autonomous agent with access to Keylime system management tools. Your goal is to help users manage and monitor their Keylime infrastructure.

When given a task:
1. Break it down into steps if needed
2. Use available tools to gather information and take actions
3. Chain multiple tool calls together to accomplish complex tasks
4. Provide clear explanations of what you're doing and what you found
5. If you encounter failures, investigate and suggest solutions`
)

func main() {
	ctx := context.Background()

	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: .env file not loaded: %v", err)
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable not set")
	}

	if len(os.Args) <= 1 {
		log.Fatal("Usage: go run main.go <content>")
	}
	userQuery := strings.Join(os.Args[1:], " ")

	session, err := connectToMCPServer(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MCP server: %v", err)
	}
	defer session.Close()

	claudeTools, err := getMCPTools(ctx, session)
	if err != nil {
		log.Fatalf("Failed to get MCP tools: %v", err)
	}

	anthropicClient := anthropic.NewClient(option.WithAPIKey(apiKey))

	if err := runAgentLoop(ctx, anthropicClient, session, claudeTools, userQuery); err != nil {
		log.Fatalf("Agent loop failed: %v", err)
	}
}

// connectToMCPServer establishes connection to the MCP server
func connectToMCPServer(ctx context.Context) (*mcp.ClientSession, error) {
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("server not found: %s", serverPath)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    mcpClientName,
		Version: mcpClientVersion,
	}, nil)

	transport := &mcp.CommandTransport{Command: exec.Command(serverPath)}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	log.Printf("Connected to MCP server: %s", serverPath)
	return session, nil
}

// getMCPTools retrieves and converts MCP tools to Claude format
func getMCPTools(ctx context.Context, session *mcp.ClientSession) ([]anthropic.ToolUnionParam, error) {
	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	var claudeTools []anthropic.ToolUnionParam
	for _, tool := range tools.Tools {
		claudeTool := convertMCPToolToClaudeTool(tool)
		claudeTools = append(claudeTools, claudeTool)
	}

	return claudeTools, nil
}

// convertMCPToolToClaudeTool converts a single MCP tool to Claude format
func convertMCPToolToClaudeTool(tool *mcp.Tool) anthropic.ToolUnionParam {
	inputSchemaMap, ok := tool.InputSchema.(map[string]any)
	if !ok || inputSchemaMap == nil {
		inputSchemaMap = map[string]any{}
	}

	properties := inputSchemaMap["properties"]
	required, _ := inputSchemaMap["required"].([]string)

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

// runAgentLoop executes the main agent conversation loop
func runAgentLoop(ctx context.Context, client anthropic.Client, session *mcp.ClientSession, tools []anthropic.ToolUnionParam, userQuery string) error {
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userQuery)),
	}

	for _ = range maxAgentTurns {

		message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     claudeModel,
			MaxTokens: maxTokens,
			System:    []anthropic.TextBlockParam{{Type: "text", Text: systemPrompt}},
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			return fmt.Errorf("claude API error: %w", err)
		}

		assistantContent, toolResults, hasToolUse := processClaudeResponse(ctx, session, message)

		if !hasToolUse {
			break
		}

		messages = append(messages, anthropic.NewAssistantMessage(assistantContent...))
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return nil
}

// processClaudeResponse handles Claude's response and executes tool calls
func processClaudeResponse(
	ctx context.Context,
	session *mcp.ClientSession,
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
			fmt.Println(block.Text)
			assistantContent = append(assistantContent, anthropic.NewTextBlock(block.Text))

		case anthropic.ToolUseBlock:
			hasToolUse = true
			log.Printf("\n[Tool Use] %s", block.Name)

			assistantContent = append(assistantContent,
				anthropic.NewToolUseBlock(block.ID, block.Input, block.Name))

			toolResult := executeToolCall(ctx, session, block)
			toolResults = append(toolResults, toolResult)
		}
	}

	return assistantContent, toolResults, hasToolUse
}

// executeToolCall calls a tool via MCP and returns the result
func executeToolCall(
	ctx context.Context,
	session *mcp.ClientSession,
	toolUse anthropic.ToolUseBlock,
) anthropic.ContentBlockParamUnion {

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
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
		log.Printf("[Error] Tool execution failed")
		return anthropic.NewToolResultBlock(
			toolUse.ID,
			"Tool execution failed",
			true,
		)
	}

	resultText := extractTextContent(result.Content)
	log.Printf("================================================")
	log.Printf("[Tool Result]\n%s", resultText)
	log.Printf("================================================")

	return anthropic.NewToolResultBlock(toolUse.ID, resultText, false)
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
