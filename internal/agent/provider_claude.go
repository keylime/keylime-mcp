package agent

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ClaudeProvider struct {
	client anthropic.Client
}

func NewClaudeProvider(apiKey string) *ClaudeProvider {
	return &ClaudeProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

func (p *ClaudeProvider) Chat(ctx context.Context, opts ChatOptions) (*LLMResponse, error) {
	messages := convertMessagesToClaude(opts.Messages)
	tools := convertToolsToClaude(opts.Tools)

	response, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(opts.Model),
		MaxTokens: opts.MaxTokens,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: opts.SystemPrompt}},
		Messages:  messages,
		Tools:     tools,
	})
	if err != nil {
		return nil, fmt.Errorf("claude API error: %w", err)
	}

	return parseClaudeResponse(response), nil
}

func convertMessagesToClaude(messages []Message) []anthropic.MessageParam {
	result := make([]anthropic.MessageParam, 0, len(messages))

	i := 0
	for i < len(messages) {
		msg := messages[i]

		switch msg.Role {
		case RoleUser:
			result = append(result, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Text)))
			i++

		case RoleAssistant:
			var blocks []anthropic.ContentBlockParamUnion
			if msg.Text != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Text))
			}
			for _, tc := range msg.ToolCalls {
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
			}
			result = append(result, anthropic.NewAssistantMessage(blocks...))
			i++

		case RoleTool:
			var blocks []anthropic.ContentBlockParamUnion
			for i < len(messages) && messages[i].Role == RoleTool && messages[i].ToolResult != nil {
				tr := messages[i].ToolResult
				blocks = append(blocks, anthropic.NewToolResultBlock(tr.ToolID, tr.Output, tr.IsError))
				i++
			}
			result = append(result, anthropic.NewUserMessage(blocks...))

		default:
			i++
		}
	}

	return result
}

func convertToolsToClaude(tools []*mcp.Tool) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		result = append(result, convertMCPToolToClaude(tool))
	}
	return result
}

func convertMCPToolToClaude(tool *mcp.Tool) anthropic.ToolUnionParam {
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

func parseClaudeResponse(response *anthropic.Message) *LLMResponse {
	result := &LLMResponse{}

	for _, block := range response.Content {
		switch content := block.AsAny().(type) {
		case anthropic.TextBlock:
			result.TextBlocks = append(result.TextBlocks, content.Text)
		case anthropic.ToolUseBlock:
			result.ToolUses = append(result.ToolUses, ToolRequest{
				ID:        content.ID,
				Name:      content.Name,
				Arguments: content.Input,
			})
		}
	}

	return result
}
