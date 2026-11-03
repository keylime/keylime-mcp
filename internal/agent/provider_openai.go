package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// OpenAIProvider adapts OpenAI-compatible Chat Completions APIs.
type OpenAIProvider struct {
	client openai.Client
	name   string
}

// Embedding RawMessage keeps MCP arguments unchanged when marshaled. OpenRouter's
// signed reasoning travels with the call in history without changing agent types.
type openAIToolArguments struct {
	json.RawMessage
	reasoningDetails json.RawMessage
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return newOpenAIProvider("openai", apiKey, "https://api.openai.com/v1/")
}

func NewOpenRouterProvider(apiKey string) *OpenAIProvider {
	return newOpenAIProvider("openrouter", apiKey, "https://openrouter.ai/api/v1/")
}

func newOpenAIProvider(name, apiKey, baseURL string) *OpenAIProvider {
	return &OpenAIProvider{
		name: name,
		client: openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseURL),
		),
	}
}

func (p *OpenAIProvider) Name() string { return p.name }

func (p *OpenAIProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	page, err := p.client.Models.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list %s models: %w", p.name, err)
	}
	models := make([]ModelInfo, 0, len(page.Data))
	for _, model := range page.Data {
		models = append(models, ModelInfo{
			ID:          model.ID,
			DisplayName: model.ID,
			Provider:    p.name,
		})
	}
	return models, nil
}

func (p *OpenAIProvider) Chat(ctx context.Context, opts ChatOptions) (*LLMResponse, error) {
	messages, err := p.convertMessagesToOpenAI(opts.SystemPrompt, opts.Messages)
	if err != nil {
		return nil, err
	}
	tools, err := convertToolsToOpenAI(opts.Tools)
	if err != nil {
		return nil, err
	}
	params := openai.ChatCompletionNewParams{
		Model: opts.Model, Messages: messages, Tools: tools,
	}
	if opts.MaxTokens > 0 {
		if p.name == "openrouter" {
			params.MaxTokens = openai.Int(opts.MaxTokens)
		} else {
			params.MaxCompletionTokens = openai.Int(opts.MaxTokens)
		}
	}

	response, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("%s LLM API error: %w", p.name, err)
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("%s LLM API returned no choices", p.name)
	}

	message := response.Choices[0].Message
	result := &LLMResponse{}
	if message.Content != "" {
		result.TextBlocks = append(result.TextBlocks, message.Content)
	}
	if message.Refusal != "" {
		result.TextBlocks = append(result.TextBlocks, message.Refusal)
	}
	for _, call := range message.ToolCalls {
		if !json.Valid([]byte(call.Function.Arguments)) {
			return nil, fmt.Errorf("%s returned invalid JSON arguments for tool call %s", p.name, call.ID)
		}
		var arguments any = json.RawMessage(call.Function.Arguments)
		if details := message.JSON.ExtraFields["reasoning_details"].Raw(); p.name == "openrouter" && details != "" && details != "null" {
			arguments = openAIToolArguments{
				RawMessage: json.RawMessage(call.Function.Arguments), reasoningDetails: json.RawMessage(details),
			}
		}
		result.ToolUses = append(result.ToolUses, ToolRequest{
			ID: call.ID, Name: call.Function.Name, Arguments: arguments,
		})
	}
	return result, nil
}

func (p *OpenAIProvider) convertMessagesToOpenAI(systemPrompt string, messages []Message) ([]openai.ChatCompletionMessageParamUnion, error) {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)+1)
	if systemPrompt != "" {
		result = append(result, openai.SystemMessage(systemPrompt))
	}
	for _, message := range messages {
		switch message.Role {
		case RoleUser:
			result = append(result, openai.UserMessage(message.Text))
		case RoleAssistant:
			assistant := openai.AssistantMessage(message.Text)
			for _, call := range message.ToolCalls {
				if args, ok := call.Arguments.(openAIToolArguments); ok && p.name == "openrouter" {
					assistant.OfAssistant.SetExtraFields(map[string]any{"reasoning_details": args.reasoningDetails})
				}
				args, err := json.Marshal(call.Arguments)
				if err != nil {
					return nil, fmt.Errorf("failed to encode tool call %s arguments: %w", call.ID, err)
				}
				assistant.OfAssistant.ToolCalls = append(assistant.OfAssistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: call.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name: call.Name, Arguments: string(args),
						},
					},
				})
			}
			result = append(result, assistant)
		case RoleTool:
			if message.ToolResult != nil {
				result = append(result, openai.ToolMessage(message.ToolResult.Output, message.ToolResult.ToolID))
			}
		}
	}
	return result, nil
}

func convertToolsToOpenAI(tools []*mcp.Tool) ([]openai.ChatCompletionToolUnionParam, error) {
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		parameters := shared.FunctionParameters{"type": "object", "properties": map[string]any{}}
		if tool.InputSchema != nil {
			schema, err := json.Marshal(tool.InputSchema)
			if err != nil {
				return nil, fmt.Errorf("failed to encode schema for tool %s: %w", tool.Name, err)
			}
			if err := json.Unmarshal(schema, &parameters); err != nil {
				return nil, fmt.Errorf("invalid schema for tool %s: %w", tool.Name, err)
			}
		}
		result = append(result, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name: tool.Name, Description: openai.String(tool.Description), Parameters: parameters,
		}))
	}
	return result, nil
}
