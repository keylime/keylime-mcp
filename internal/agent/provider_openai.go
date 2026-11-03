package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

const providerOpenRouter = "openrouter"

// OpenAIProvider adapts OpenAI-compatible Chat Completions APIs.
type OpenAIProvider struct {
	client openai.Client
	name   string
}

// OpenRouter reasoning is replayed only to the provider and model that produced it.
type reasoningData struct {
	provider string
	model    string
	fields   map[string]any
}

type openAIError struct {
	Message string
	Code    json.RawMessage
}

func (e *openAIError) Error() string { return fmt.Sprintf("%s (code %s)", e.Message, e.Code) }

// Decode the original JSON once: the SDK's response types coerce invalid field
// types, including tool arguments, instead of rejecting them.
type openAIResponse struct {
	ID      string
	Error   *openAIError
	Usage   openai.CompletionUsage
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      openAIMessage
		Error        *openAIError
	}
}

type openAIMessage struct {
	Role, Content, Refusal string
	ToolCalls              []struct {
		ID, Type string
		Function struct{ Name, Arguments string }
	} `json:"tool_calls"`
	ReasoningDetails []json.RawMessage `json:"reasoning_details"`
	Reasoning        *string
	ReasoningContent *string `json:"reasoning_content"`
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return newOpenAIProvider("openai", apiKey, "https://api.openai.com/v1/")
}

func NewOpenRouterProvider(apiKey string) *OpenAIProvider {
	return newOpenAIProvider(providerOpenRouter, apiKey, "https://openrouter.ai/api/v1/")
}

func newOpenAIProvider(name, apiKey, baseURL string) *OpenAIProvider {
	opts := []option.RequestOption{option.WithAPIKey(apiKey), option.WithBaseURL(baseURL)}
	if name == providerOpenRouter {
		opts = append(opts, option.WithHeaderDel("OpenAI-Organization"), option.WithHeaderDel("OpenAI-Project"))
	}
	return &OpenAIProvider{
		name:   name,
		client: openai.NewClient(opts...),
	}
}

func (p *OpenAIProvider) Name() string { return p.name }

func (p *OpenAIProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var raw []byte
	_, err := p.client.Models.List(ctx, option.WithResponseBodyInto(&raw))
	if err != nil {
		return nil, fmt.Errorf("failed to list %s models: %w", p.name, err)
	}
	var page struct {
		Data  []struct{ ID string }
		Error *openAIError
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("%s returned an invalid model list: %w", p.name, err)
	}
	if page.Error != nil {
		return nil, fmt.Errorf("failed to list %s models: %w", p.name, page.Error)
	}
	if page.Data == nil {
		return nil, fmt.Errorf("%s returned an invalid model list", p.name)
	}
	models := make([]ModelInfo, 0, len(page.Data))
	for _, model := range page.Data {
		if strings.TrimSpace(model.ID) == "" {
			return nil, fmt.Errorf("%s returned an empty model ID", p.name)
		}
		models = append(models, ModelInfo{
			ID:          model.ID,
			DisplayName: model.ID,
			Provider:    p.name,
		})
	}
	return models, nil
}

func (p *OpenAIProvider) Chat(ctx context.Context, opts ChatOptions) (*LLMResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	messages, err := p.convertMessagesToOpenAI(opts.SystemPrompt, opts.Model, opts.Messages)
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
		if p.name == providerOpenRouter {
			params.MaxTokens = openai.Int(opts.MaxTokens)
		} else {
			params.MaxCompletionTokens = openai.Int(opts.MaxTokens)
		}
	}

	var raw []byte
	_, err = p.client.Chat.Completions.New(ctx, params, option.WithResponseBodyInto(&raw))
	if err != nil {
		return nil, fmt.Errorf("%s LLM API error: %w", p.name, err)
	}
	return parseOpenAIResponse(p.name, opts.Model, raw)
}

func parseOpenAIResponse(provider, model string, raw []byte) (*LLMResponse, error) {
	var response openAIResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("%s returned an invalid response: %w", provider, err)
	}
	apiErr := response.Error
	if apiErr == nil && len(response.Choices) > 0 {
		apiErr = response.Choices[0].Error
	}
	if apiErr != nil {
		return nil, fmt.Errorf("%s LLM API error: %w", provider, apiErr)
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("%s LLM API returned no choices", provider)
	}
	choice := response.Choices[0]
	log.Printf("[LLM] provider=%s model=%q response_id=%q finish_reason=%q prompt_tokens=%d completion_tokens=%d reasoning_tokens=%d",
		provider, model, response.ID, choice.FinishReason, response.Usage.PromptTokens,
		response.Usage.CompletionTokens, response.Usage.CompletionTokensDetails.ReasoningTokens)
	if err := validateOpenAIFinishReason(provider, choice.FinishReason, len(choice.Message.ToolCalls)); err != nil {
		return nil, err
	}

	message := choice.Message
	result := &LLMResponse{}
	if strings.TrimSpace(message.Content) != "" {
		result.TextBlocks = append(result.TextBlocks, message.Content)
	}
	if strings.TrimSpace(message.Refusal) != "" {
		result.TextBlocks = append(result.TextBlocks, message.Refusal)
	}
	tools, err := parseOpenAIToolCalls(provider, message)
	if err != nil {
		return nil, err
	}
	result.ToolUses = tools
	if len(result.TextBlocks) == 0 && len(tools) == 0 {
		return nil, fmt.Errorf("%s returned no text or tool calls (finish_reason=%q)", provider, choice.FinishReason)
	}
	result.reasoning = openAIReasoningData(provider, model, message)
	return result, nil
}

func openAIReasoningData(provider, model string, message openAIMessage) *reasoningData {
	if provider != providerOpenRouter {
		return nil
	}
	fields := make(map[string]any)
	if message.ReasoningDetails != nil {
		fields["reasoning_details"] = message.ReasoningDetails
	}
	if message.Reasoning != nil {
		fields["reasoning"] = *message.Reasoning
	}
	if message.ReasoningContent != nil {
		fields["reasoning_content"] = *message.ReasoningContent
	}
	return &reasoningData{provider: provider, model: model, fields: fields}
}

func validateOpenAIFinishReason(provider, reason string, toolCount int) error {
	switch reason {
	case "stop":
		return nil
	case "tool_calls":
		if toolCount == 0 {
			return fmt.Errorf("%s finished with tool_calls without tool calls", provider)
		}
		return nil
	case "length":
		return fmt.Errorf("%s reached the output token limit before completing its response (finish_reason=length); the budget includes reasoning tokens", provider)
	case "content_filter":
		return fmt.Errorf("%s response was interrupted by a content filter (finish_reason=content_filter)", provider)
	default:
		return fmt.Errorf("%s returned an unsuccessful or unsupported finish reason %q", provider, reason)
	}
}

func parseOpenAIToolCalls(provider string, message openAIMessage) ([]ToolRequest, error) {
	result := make([]ToolRequest, 0, len(message.ToolCalls))
	ids := make(map[string]bool, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		if call.Type != "function" || strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Function.Name) == "" {
			return nil, fmt.Errorf("%s returned an invalid or unsupported tool call %q", provider, call.ID)
		}
		if ids[call.ID] {
			return nil, fmt.Errorf("%s returned duplicate tool call ID %q", provider, call.ID)
		}
		ids[call.ID] = true
		var arguments map[string]json.RawMessage
		if err := json.Unmarshal([]byte(call.Function.Arguments), &arguments); err != nil || arguments == nil {
			return nil, fmt.Errorf("%s returned invalid JSON object arguments for tool call %s", provider, call.ID)
		}
		result = append(result, ToolRequest{ID: call.ID, Name: call.Function.Name, Arguments: json.RawMessage(call.Function.Arguments)})
	}
	return result, nil
}

func (p *OpenAIProvider) convertMessagesToOpenAI(systemPrompt, model string, messages []Message) ([]openai.ChatCompletionMessageParamUnion, error) {
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
			if data := message.reasoning; data != nil && data.provider == p.name && data.model == model {
				assistant.OfAssistant.SetExtraFields(data.fields)
			}
			for _, call := range message.ToolCalls {
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
