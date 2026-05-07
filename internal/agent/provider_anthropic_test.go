package agent

import (
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertMessagesToAnthropic(t *testing.T) {
	t.Run("user message", func(t *testing.T) {
		messages := []Message{
			{Role: RoleUser, Text: "hello"},
		}

		result := convertMessagesToAnthropic(messages)

		require.Len(t, result, 1)
		assert.Equal(t, anthropic.MessageParamRoleUser, result[0].Role)
		require.Len(t, result[0].Content, 1)
	})

	t.Run("assistant with text and tool calls", func(t *testing.T) {
		messages := []Message{
			{
				Role: RoleAssistant,
				Text: "Let me help",
				ToolCalls: []ToolRequest{
					{ID: testCallID1, Name: testToolGetStatus, Arguments: map[string]any{testArgUUID: testUUID}},
				},
			},
		}

		result := convertMessagesToAnthropic(messages)

		require.Len(t, result, 1)
		assert.Equal(t, anthropic.MessageParamRoleAssistant, result[0].Role)
		require.Len(t, result[0].Content, 2)
	})

	t.Run("assistant text only", func(t *testing.T) {
		messages := []Message{
			{Role: RoleAssistant, Text: "Got it"},
		}

		result := convertMessagesToAnthropic(messages)

		require.Len(t, result, 1)
		assert.Equal(t, anthropic.MessageParamRoleAssistant, result[0].Role)
		require.Len(t, result[0].Content, 1)
	})

	t.Run("consecutive tool results batched", func(t *testing.T) {
		messages := []Message{
			{
				Role:       RoleTool,
				ToolResult: &ToolResult{ToolID: testCallID1, Output: "sunny", IsError: false},
			},
			{
				Role:       RoleTool,
				ToolResult: &ToolResult{ToolID: "call2", Output: "windy", IsError: false},
			},
		}

		result := convertMessagesToAnthropic(messages)

		require.Len(t, result, 1)
		assert.Equal(t, anthropic.MessageParamRoleUser, result[0].Role)
		require.Len(t, result[0].Content, 2)
	})

	t.Run("empty list", func(t *testing.T) {
		messages := []Message{}

		result := convertMessagesToAnthropic(messages)

		assert.Empty(t, result)
	})

	t.Run("full conversation flow", func(t *testing.T) {
		messages := []Message{
			{Role: RoleUser, Text: "Check agent status"},
			{
				Role: RoleAssistant,
				Text: "Let me check",
				ToolCalls: []ToolRequest{
					{ID: testCallID1, Name: testToolGetStatus, Arguments: map[string]any{testArgUUID: testUUID}},
				},
			},
			{
				Role:       RoleTool,
				ToolResult: &ToolResult{ToolID: testCallID1, Output: "sunny", IsError: false},
			},
			{Role: RoleAssistant, Text: "It's sunny!"},
		}

		result := convertMessagesToAnthropic(messages)

		require.Len(t, result, 4)
		assert.Equal(t, anthropic.MessageParamRoleUser, result[0].Role)
		assert.Equal(t, anthropic.MessageParamRoleAssistant, result[1].Role)
		assert.Equal(t, anthropic.MessageParamRoleUser, result[2].Role)
		assert.Equal(t, anthropic.MessageParamRoleAssistant, result[3].Role)
	})
}

func TestConvertToolsToAnthropic(t *testing.T) {
	t.Run("converts with cache control on last", func(t *testing.T) {
		tools := []*mcp.Tool{
			{
				Name:        testTool1,
				Description: "First tool",
				InputSchema: map[string]any{
					testSchemaProperties: map[string]any{testArgUUID: map[string]any{testSchemaType: "string"}},
					testSchemaRequired:   []any{testArgUUID},
				},
			},
			{
				Name:        testTool2,
				Description: "Second tool",
				InputSchema: map[string]any{
					testSchemaProperties: map[string]any{"policy_name": map[string]any{testSchemaType: "number"}},
					testSchemaRequired:   []any{"policy_name"},
				},
			},
		}

		result := convertToolsToAnthropic(tools)

		require.Len(t, result, 2)
		assert.True(t, param.IsOmitted(result[0].OfTool.CacheControl))
		assert.False(t, param.IsOmitted(result[1].OfTool.CacheControl))
	})

	t.Run("single tool gets cache control", func(t *testing.T) {
		tools := []*mcp.Tool{
			{
				Name:        testTool1,
				Description: "Only tool",
				InputSchema: map[string]any{
					testSchemaProperties: map[string]any{testArgUUID: map[string]any{testSchemaType: "string"}},
				},
			},
		}

		result := convertToolsToAnthropic(tools)

		require.Len(t, result, 1)
		assert.False(t, param.IsOmitted(result[0].OfTool.CacheControl))
	})

	t.Run("empty list", func(t *testing.T) {
		tools := []*mcp.Tool{}

		result := convertToolsToAnthropic(tools)

		assert.Empty(t, result)
	})
}

func TestConvertMCPToolToAnthropic(t *testing.T) {
	t.Run("extracts properties and required", func(t *testing.T) {
		tool := &mcp.Tool{
			Name:        testToolGetStatus,
			Description: "Gets agent attestation status",
			InputSchema: map[string]any{
				testSchemaProperties: map[string]any{
					testArgUUID: map[string]any{testSchemaType: "string"},
				},
				testSchemaRequired: []any{testArgUUID},
			},
		}

		result := convertMCPToolToAnthropic(tool)

		require.NotNil(t, result.OfTool)
		assert.Equal(t, testToolGetStatus, result.OfTool.Name)
		assert.Equal(t, "Gets agent attestation status", result.OfTool.Description.Value)
		assert.Equal(t, "object", string(result.OfTool.InputSchema.Type))
		assert.NotNil(t, result.OfTool.InputSchema.Properties)
		require.Len(t, result.OfTool.InputSchema.Required, 1)
		assert.Equal(t, testArgUUID, result.OfTool.InputSchema.Required[0])
	})

	t.Run("nil schema", func(t *testing.T) {
		tool := &mcp.Tool{
			Name:        "simple_tool",
			Description: "No schema",
			InputSchema: nil,
		}

		result := convertMCPToolToAnthropic(tool)

		require.NotNil(t, result.OfTool)
		assert.Equal(t, "simple_tool", result.OfTool.Name)
		assert.Equal(t, "object", string(result.OfTool.InputSchema.Type))
		assert.NotNil(t, result.OfTool.InputSchema.Properties)
		assert.Empty(t, result.OfTool.InputSchema.Required)
	})

	t.Run("schema with no properties key", func(t *testing.T) {
		tool := &mcp.Tool{
			Name:        "minimal_tool",
			Description: "Minimal schema",
			InputSchema: map[string]any{
				testSchemaRequired: []any{"param"},
			},
		}

		result := convertMCPToolToAnthropic(tool)

		require.NotNil(t, result.OfTool)
		assert.Equal(t, "object", string(result.OfTool.InputSchema.Type))
		assert.NotNil(t, result.OfTool.InputSchema.Properties)
		require.Len(t, result.OfTool.InputSchema.Required, 1)
		assert.Equal(t, "param", result.OfTool.InputSchema.Required[0])
	})
}

func TestParseAnthropicResponse(t *testing.T) {
	t.Run("text and tool use blocks", func(t *testing.T) {
		responseJSON := `{
			"id": "msg_123",
			"type": "message",
			"role": "assistant",
			"model": "claude-3-5-sonnet-20241022",
			"content": [
				{"type": "text", "text": "Let me help", "citations": []},
				{"type": "tool_use", "id": "` + testCallID1 + `", "name": "` + testToolGetStatus + `", "input": {"agent_uuid": "` + testUUID + `"}}
			],
			"stop_reason": "tool_use",
			"stop_sequence": "",
			"usage": {"input_tokens": 10, "output_tokens": 20}
		}`

		var response anthropic.Message
		err := json.Unmarshal([]byte(responseJSON), &response)
		require.NoError(t, err)

		result := parseAnthropicResponse(&response)

		require.Len(t, result.TextBlocks, 1)
		assert.Equal(t, "Let me help", result.TextBlocks[0])
		require.Len(t, result.ToolUses, 1)
		assert.Equal(t, testCallID1, result.ToolUses[0].ID)
		assert.Equal(t, testToolGetStatus, result.ToolUses[0].Name)

		var inputMap map[string]any
		err = json.Unmarshal(result.ToolUses[0].Arguments.(json.RawMessage), &inputMap)
		require.NoError(t, err)
		assert.Equal(t, testUUID, inputMap[testArgUUID])
	})

	t.Run("empty content", func(t *testing.T) {
		responseJSON := `{
			"id": "msg_123",
			"type": "message",
			"role": "assistant",
			"model": "claude-3-5-sonnet-20241022",
			"content": [],
			"stop_reason": "end_turn",
			"stop_sequence": "",
			"usage": {"input_tokens": 10, "output_tokens": 0}
		}`

		var response anthropic.Message
		err := json.Unmarshal([]byte(responseJSON), &response)
		require.NoError(t, err)

		result := parseAnthropicResponse(&response)

		assert.Empty(t, result.TextBlocks)
		assert.Empty(t, result.ToolUses)
	})
}

func TestAnthropicProviderName(t *testing.T) {
	provider := &AnthropicProvider{}

	assert.Equal(t, "anthropic", provider.Name())
}
