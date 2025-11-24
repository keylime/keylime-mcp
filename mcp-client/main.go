package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultServerPath = "../backend/server"
	mcpClientName     = "mcp-client"
	mcpClientVersion  = "v1.0.0"

	claudeModel   = anthropic.ModelClaude3_5HaikuLatest
	maxTokens     = 2048
	maxAgentTurns = 5

	systemPrompt = `You are an AI assistant with access to Keylime system management tools. Your goal is to help users manage and monitor their Keylime infrastructure.

You have a maximum of 5 conversation turns to complete the task. When given a task:
1. Break it down into steps if needed
2. Use available tools to gather information and take actions
3. Chain multiple tool calls together to accomplish complex tasks
4. Provide clear explanations of what you're doing and what you found
5. If you encounter failures, investigate and suggest solutions
6. Work efficiently to complete tasks within the turn limit`
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\nShutting down gracefully...")
		cancel()
	}()

	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: .env file not loaded: %v", err)
	}

	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable not set or is empty after trimming whitespace")
	}

	if len(os.Args) <= 1 {
		log.Fatal("Usage: go run main.go <content>")
	}
	userQuery := strings.Join(os.Args[1:], " ")
	userQuery = strings.TrimSpace(userQuery)
	if userQuery == "" {
		log.Fatal("Error: user query cannot be empty")
	}

	session, cmd, err := connectToMCPServer(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MCP server: %v", err)
	}
	defer session.Close()

	// In main(), ensure process is killed on exit
	defer func() {
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

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
func connectToMCPServer(ctx context.Context) (*mcp.ClientSession, *exec.Cmd, error) {
	serverPath := os.Getenv("MCP_SERVER_PATH")
	if serverPath == "" {
		serverPath = defaultServerPath
	}

	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("server not found: %s", serverPath)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    mcpClientName,
		Version: mcpClientVersion,
	}, nil)

	cmd := exec.Command(serverPath)
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Monitor server process for unexpected exits
	if cmd.Process != nil {
		go func() {
			state, waitErr := cmd.Process.Wait()
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

	log.Printf("Connected to MCP server: %s", serverPath)
	return session, cmd, nil
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
			return nil
		}

		messages = append(messages, anthropic.NewAssistantMessage(assistantContent...))
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	log.Printf("\n=== Maximum turns reached, requesting summary ===")
	finalMessage(ctx, client, messages)

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

// extractTextContent returns the concatenated text from all mcp.TextContent elements in the given slice.
func extractTextContent(content []mcp.Content) string {
	var resultText strings.Builder

	for _, c := range content {
		if textContent, ok := c.(*mcp.TextContent); ok {
			resultText.WriteString(textContent.Text)
		}
	}

	return resultText.String()
}

// finalMessage asks LLM to summarize what happened during session if max turns are reached
func finalMessage(ctx context.Context, client anthropic.Client, messages []anthropic.MessageParam) {
	summaryPrompt := `I've reached the maximum number of allowed turns. Please provide a summary of:
1. What you accomplished so far
2. What still needs to be done
3. Any issues or blockers encountered`

	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(summaryPrompt)))

	finalMsg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
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
			fmt.Println(textBlock.Text)
		}
	}
}
