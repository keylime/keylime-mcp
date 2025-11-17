package main

import (
	"context"
	"log"
	"os"
	"os/exec"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverPath = "../backend/server"

func main() {
	ctx := context.Background()
	godotenv.Load("../.env")

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable not set")
	}

	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		log.Fatalf("Server not found: %s", serverPath)
	}

	// Connect to MCP server
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
	transport := &mcp.CommandTransport{Command: exec.Command(serverPath)}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	tools, _ := session.ListTools(ctx, &mcp.ListToolsParams{})

	claudeTools := make([]anthropic.ToolUnionParam, 0)
	for _, tool := range tools.Tools {
		inputSchemaMap, ok := tool.InputSchema.(map[string]interface{})
		if !ok || inputSchemaMap == nil {
			inputSchemaMap = map[string]interface{}{}
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

		claudeTools = append(claudeTools, toolParam)
	}
	anthropicClient := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)
	message, err := anthropicClient.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude_3_Haiku_20240307,
		MaxTokens: 256,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Give me all failed agents")),
		},
		Tools: claudeTools,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, block := range message.Content {
		switch block := block.AsAny().(type) {
		case anthropic.TextBlock:
			println(block.Text)
			println()
		case anthropic.ToolUseBlock:
			params := &mcp.CallToolParams{
				Name:      block.Name,
				Arguments: block.Input,
			}
			res, err := session.CallTool(ctx, params)
			if err != nil {
				log.Fatalf("CallTool failed: %v", err)
			}
			if res.IsError {
				log.Fatal("tool failed")
			}
			for _, c := range res.Content {
				log.Print(c.(*mcp.TextContent).Text)
				println()
			}
		}
	}
}
