package agent

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

// func (a *Agent) callClaude(ctx context.Context, onMessage func(Message)) error {
// 	response, err := a.anthropicClient.Messages.New(ctx, anthropic.MessageNewParams{
// 		Model:     a.config.Model,
// 		MaxTokens: a.config.MaxTokens,
// 		System:    []anthropic.TextBlockParam{{Type: "text", Text: a.config.SystemPrompt}},
// 		Messages:  a.messages,
// 		Tools:     a.tools,
// 	})
// 	if err != nil {
// 		return fmt.Errorf("claude API error: %w", err)
// 	}

// 	assistantContent, toolRequests := a.processResponse(response, onMessage)

// 	a.messages = append(a.messages, anthropic.NewAssistantMessage(assistantContent...))

// 	for _, toolRequest := range toolRequests {
// 		onMessage(Message{
// 			Role:    "tool_request",
// 			Content: toolRequest.Name,
// 			ToolID:  toolRequest.ID,
// 			Tool:    toolRequest,
// 		})
// 	}
// 	return nil
// }

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
