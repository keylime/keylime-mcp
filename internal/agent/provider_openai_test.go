package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/keylime/keylime-mcp/internal/masking"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type openAITransportFunc func(*http.Request) (*http.Response, error)

func (f openAITransportFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// Replace only the network transport, preserving constructor URLs and credentials.
func stubOpenAIHTTP(p *OpenAIProvider, handler func(*http.Request) (int, string)) {
	opts := append([]option.RequestOption{}, p.client.Options...)
	p.client = openai.NewClient(append(opts,
		option.WithMaxRetries(0),
		option.WithHTTPClient(&http.Client{Transport: openAITransportFunc(func(r *http.Request) (*http.Response, error) {
			status, body := handler(r)
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		})}),
	)...)
}

func TestOpenAIListModels(t *testing.T) {
	for _, tt := range []struct {
		name string
		url  string
		new  func(string) *OpenAIProvider
	}{
		{"openai", "https://api.openai.com/v1/models", NewOpenAIProvider},
		{"openrouter", "https://openrouter.ai/api/v1/models", NewOpenRouterProvider},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// OpenAI environment defaults must not redirect OpenRouter or replace its key.
			t.Setenv("OPENAI_BASE_URL", "https://wrong.example/v1")
			t.Setenv("OPENAI_API_KEY", "wrong-key")
			p := tt.new("test-key")
			stubOpenAIHTTP(p, func(r *http.Request) (int, string) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, tt.url, r.URL.String())
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
				return http.StatusOK, `{"object":"list","data":[{"id":"vendor/model-b","object":"model","created":1,"owned_by":"vendor"},{"id":"model-a","object":"model","created":2,"owned_by":"vendor"}]}`
			})

			models, err := p.ListModels(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.name, p.Name())
			assert.Equal(t, []ModelInfo{
				{ID: "vendor/model-b", DisplayName: "vendor/model-b", Provider: tt.name},
				{ID: "model-a", DisplayName: "model-a", Provider: tt.name},
			}, models)
		})
	}
}

func TestOpenAIChatToolRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name       string
		url        string
		tokenParam string
		new        func(string) *OpenAIProvider
	}{
		{"openai", "https://api.openai.com/v1/chat/completions", "max_completion_tokens", NewOpenAIProvider},
		{"openrouter", "https://openrouter.ai/api/v1/chat/completions", "max_tokens", NewOpenRouterProvider},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.new("chat-key")
			schema := map[string]any{
				"type":       "object",
				"properties": map[string]any{"agent_uuid": map[string]any{"type": "string"}},
				"required":   []string{"agent_uuid"}, "additionalProperties": false,
			}
			opts := ChatOptions{
				SystemPrompt: "Manage Keylime", Model: "vendor/chat-model", MaxTokens: 2048,
				Messages: []Message{{Role: RoleUser, Text: "Check agent"}},
				Tools:    []*mcp.Tool{{Name: "Get_agent_status", Description: "Get status", InputSchema: schema}},
			}
			calls := 0
			stubOpenAIHTTP(p, func(r *http.Request) (int, string) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, tt.url, r.URL.String())
				assert.Equal(t, "Bearer chat-key", r.Header.Get("Authorization"))
				var body struct {
					Model    string           `json:"model"`
					Messages []map[string]any `json:"messages"`
					Tools    []map[string]any `json:"tools"`
				}
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(raw, &body))
				assert.Equal(t, "vendor/chat-model", body.Model)
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(raw, &fields))
				assert.JSONEq(t, "2048", string(fields[tt.tokenParam]))
				assert.Equal(t, map[string]any{"role": "system", "content": "Manage Keylime"}, body.Messages[0])
				require.Len(t, body.Tools, 1)
				toolJSON, err := json.Marshal(body.Tools[0])
				require.NoError(t, err)
				assert.JSONEq(t, `{"type":"function","function":{"name":"Get_agent_status","description":"Get status","parameters":{"type":"object","properties":{"agent_uuid":{"type":"string"}},"required":["agent_uuid"],"additionalProperties":false}}}`, string(toolJSON))
				calls++
				if calls == 1 {
					require.Len(t, body.Messages, 2)
					assert.Equal(t, map[string]any{"role": "user", "content": "Check agent"}, body.Messages[1])
					return http.StatusOK, `{"id":"chat-1","object":"chat.completion","created":1,"model":"vendor/chat-model","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":"Checking","tool_calls":[{"id":"call-1","type":"function","function":{"name":"Get_agent_status","arguments":"{\"agent_uuid\":\"AGENT-1\"}"}},{"id":"call-2","type":"function","function":{"name":"List_agents","arguments":"{}"}}]}}]}`
				}
				require.Len(t, body.Messages, 5)
				assistantJSON, err := json.Marshal(body.Messages[2])
				require.NoError(t, err)
				assert.JSONEq(t, `{"role":"assistant","content":"Checking","tool_calls":[{"id":"call-1","type":"function","function":{"name":"Get_agent_status","arguments":"{\"agent_uuid\":\"AGENT-1\"}"}},{"id":"call-2","type":"function","function":{"name":"List_agents","arguments":"{}"}}]}`, string(assistantJSON))
				assert.Equal(t, map[string]any{"role": "tool", "tool_call_id": "call-1", "content": "healthy"}, body.Messages[3])
				assert.Equal(t, map[string]any{"role": "tool", "tool_call_id": "call-2", "content": "Error: unavailable"}, body.Messages[4])
				return http.StatusOK, `{"id":"chat-2","object":"chat.completion","created":2,"model":"vendor/chat-model","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"Agent is healthy"}}]}`
			})

			resp, err := p.Chat(context.Background(), opts)
			require.NoError(t, err)
			assert.Equal(t, []string{"Checking"}, resp.TextBlocks)
			require.Len(t, resp.ToolUses, 2)
			assert.Equal(t, ToolRequest{ID: "call-1", Name: "Get_agent_status", Arguments: json.RawMessage(`{"agent_uuid":"AGENT-1"}`)}, resp.ToolUses[0])
			assert.Equal(t, ToolRequest{ID: "call-2", Name: "List_agents", Arguments: json.RawMessage(`{}`)}, resp.ToolUses[1])
			opts.Messages = append(opts.Messages,
				Message{Role: RoleAssistant, Text: "Checking", ToolCalls: resp.ToolUses},
				Message{Role: RoleTool, ToolResult: &ToolResult{ToolID: "call-1", Output: "healthy"}},
				Message{Role: RoleTool, ToolResult: &ToolResult{ToolID: "call-2", Output: "Error: unavailable", IsError: true}},
			)
			resp, err = p.Chat(context.Background(), opts)
			require.NoError(t, err)
			assert.Equal(t, []string{"Agent is healthy"}, resp.TextBlocks)
			assert.Empty(t, resp.ToolUses)
			assert.Equal(t, 2, calls)
		})
	}
}

func TestOpenAIChatErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"API failure", http.StatusUnauthorized, `{"error":{"message":"Invalid API key","type":"authentication_error"}}`, "Invalid API key"},
		{"malformed response", http.StatusOK, `{`, ""},
		{"empty choices", http.StatusOK, `{"choices":[]}`, "no choices"},
		{"malformed tool arguments", http.StatusOK, `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"bad-call","type":"function","function":{"name":"Get_agent_status","arguments":"{"}}]}}]}`, "bad-call"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := NewOpenAIProvider("test-key")
			stubOpenAIHTTP(p, func(*http.Request) (int, string) { return tt.status, tt.body })
			resp, err := p.Chat(context.Background(), ChatOptions{Model: "chat-model", Messages: []Message{{Role: RoleUser, Text: "hello"}}})
			require.Error(t, err)
			assert.Nil(t, resp)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestOpenAIListModelsErrors(t *testing.T) {
	for _, body := range []string{`{"error":{"message":"Invalid API key"}}`, `{`} {
		p := NewOpenRouterProvider("test-key")
		stubOpenAIHTTP(p, func(*http.Request) (int, string) { return http.StatusUnauthorized, body })
		models, err := p.ListModels(context.Background())
		require.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "openrouter")
	}
}

func TestOpenAICanceledContext(t *testing.T) {
	p := NewOpenAIProvider("test-key")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Chat(ctx, ChatOptions{Model: "chat-model", Messages: []Message{{Role: RoleUser, Text: "hello"}}})
	assert.ErrorIs(t, err, context.Canceled)
	_, err = p.ListModels(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestOpenRouterPreservesReasoningDuringToolExecution(t *testing.T) {
	const details = `[{"type":"reasoning.encrypted","data":"opaque-signed-data","id":"call-signed","format":"google-gemini-v1","index":0}]`
	masker := masking.NewEngine(true)
	maskedUUID := masker.Mask(testUUID)
	p := NewOpenRouterProvider("test-key")
	calls := 0
	stubOpenAIHTTP(p, func(r *http.Request) (int, string) {
		calls++
		if calls == 1 {
			return http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":null,"reasoning_details":` + details + `,"tool_calls":[{"id":"call-signed","type":"function","function":{"name":"Get_agent_status","arguments":"{\"agent_uuid\":\"` + maskedUUID + `\"}"}}]}}]}`
		}
		var body struct {
			Messages []struct {
				Role             string          `json:"role"`
				ReasoningDetails json.RawMessage `json:"reasoning_details"`
			} `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Len(t, body.Messages, 4)
		assert.Equal(t, "assistant", body.Messages[2].Role)
		assert.JSONEq(t, details, string(body.Messages[2].ReasoningDetails))
		return http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":"Agent is healthy"}}]}`
	})
	// Exercise the unchanged agent loop, including argument unmasking and MCP execution.
	a := NewAgent(Config{Model: "vendor/reasoning-model"}, p, masker)
	session := &mockSession{callResult: &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "healthy"}}}}
	a.mcpSession = session
	var texts []string
	onMessage := func(msg Message) {
		if msg.Text != "" {
			texts = append(texts, msg.Text)
		}
	}
	require.NoError(t, a.SendMessage(context.Background(), "Check agent", onMessage))
	tool := a.GetCurrentTool()
	require.NotNil(t, tool)
	args, err := json.Marshal(tool.Arguments)
	require.NoError(t, err)
	assert.JSONEq(t, `{"agent_uuid":"`+maskedUUID+`"}`, string(args))
	require.NoError(t, a.ExecuteTool(context.Background(), tool, onMessage))
	require.Len(t, session.callToolCalls, 1)
	args, err = json.Marshal(session.callToolCalls[0].Arguments)
	require.NoError(t, err)
	assert.JSONEq(t, `{"agent_uuid":"`+testUUID+`"}`, string(args))
	assert.Equal(t, []string{"Agent is healthy"}, texts)
	assert.Equal(t, 2, calls)
}
