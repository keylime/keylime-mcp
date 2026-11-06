package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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
		{"openrouter", "https://openrouter.ai/api/v1/models?supported_parameters=tools", NewOpenRouterProvider},
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
				return http.StatusOK, `{"object":"list","data":[{"id":"gpt-4o","object":"model","created":1,"owned_by":"vendor"},{"id":"gpt-4.1","object":"model","created":2,"owned_by":"vendor"}]}`
			})

			models, err := p.ListModels(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.name, p.Name())
			assert.Equal(t, []ModelInfo{
				{ID: "gpt-4o", DisplayName: "gpt-4o", Provider: tt.name},
				{ID: "gpt-4.1", DisplayName: "gpt-4.1", Provider: tt.name},
			}, models)
		})
	}
}

func TestOpenAIListModelsFiltersUnsupportedModels(t *testing.T) {
	p := NewOpenAIProvider("test-key")
	stubOpenAIHTTP(p, func(*http.Request) (int, string) {
		return http.StatusOK, `{"data":[{"id":"text-embedding-3-small"},{"id":"whisper-1"},{"id":"gpt-image-1"},{"id":"gpt-4o-realtime-preview"},{"id":"gpt-4o-audio-preview"},{"id":"gpt-5-pro"},{"id":"gpt-5-codex"},{"id":"gpt-4o-search-preview"},{"id":"o3-deep-research"},{"id":"chatgpt-4o-latest"},{"id":"o1-mini"},{"id":"o1-mini-2024-09-12"},{"id":"o1-preview"},{"id":"unknown-future-model"},{"id":"gpt-4o"},{"id":"gpt-4.1-mini-2025-04-14"},{"id":"gpt-5-mini"},{"id":"o4-mini"},{"id":"ft:gpt-4o-2024-08-06:org:custom:id"}]}`
	})
	models, err := p.ListModels(context.Background())
	require.NoError(t, err)
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	assert.Equal(t, []string{"gpt-4o", "gpt-4.1-mini-2025-04-14", "gpt-5-mini", "o4-mini", "ft:gpt-4o-2024-08-06:org:custom:id"}, ids)
}

func TestOpenAIInstructionRoles(t *testing.T) {
	for _, tt := range []struct {
		model string
		new   func(string) *OpenAIProvider
		role  string
	}{
		{"gpt-4o", NewOpenAIProvider, "system"},
		{"ft:gpt-4o-mini-2024-07-18:org:custom:id", NewOpenAIProvider, "system"},
		{"gpt-5-mini", NewOpenAIProvider, "developer"},
		{"gpt-5.2", NewOpenAIProvider, "developer"},
		{"o1-2024-12-17", NewOpenAIProvider, "developer"},
		{"o3-mini", NewOpenAIProvider, "developer"},
		{"o4-mini", NewOpenAIProvider, "developer"},
		{"ft:o4-mini-2025-04-16:org:custom:id", NewOpenAIProvider, "developer"},
		{"openai/o3", NewOpenRouterProvider, "system"},
	} {
		t.Run(tt.model, func(t *testing.T) {
			p := tt.new("test-key")
			stubOpenAIHTTP(p, func(r *http.Request) (int, string) {
				var body struct {
					Messages []struct{ Role, Content string }
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Len(t, body.Messages, 2)
				assert.Equal(t, tt.role, body.Messages[0].Role)
				assert.Equal(t, "Manage Keylime", body.Messages[0].Content)
				return http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"OK"}}]}`
			})
			_, err := p.Chat(context.Background(), ChatOptions{Model: tt.model, SystemPrompt: "Manage Keylime", Messages: []Message{{Role: RoleUser, Text: "check"}}})
			require.NoError(t, err)
		})
	}
}

func TestOpenAIUserPromptsRespectMasking(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, factory := range []struct {
			name string
			new  func(string) *OpenAIProvider
		}{{"openai", NewOpenAIProvider}, {"openrouter", NewOpenRouterProvider}} {
			t.Run(fmt.Sprintf("%s/masking=%t", factory.name, enabled), func(t *testing.T) {
				p := factory.new("test-key")
				prompt := "Check " + testUUID + " at 192.0.2.1"
				stubOpenAIHTTP(p, func(r *http.Request) (int, string) {
					var body struct{ Messages []struct{ Content string } }
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					require.Len(t, body.Messages, 2)
					if enabled {
						assert.Equal(t, "Check AGENT-1 at HOST-1", body.Messages[1].Content)
					} else {
						assert.Equal(t, prompt, body.Messages[1].Content)
					}
					return http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"OK"}}]}`
				})
				masker := masking.NewEngine(enabled)
				a := NewAgent(Config{Model: "gpt-4o"}, p, masker)
				require.NoError(t, a.SendMessage(context.Background(), prompt, func(Message) {}))
				assert.Equal(t, prompt, a.messages[0].Text, "local history must retain the original prompt")
				if enabled {
					assert.Equal(t, prompt, masker.Unmask("Check AGENT-1 at HOST-1"))
				}
			})
		}
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
		{"malformed tool arguments", http.StatusOK, `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"bad-call","type":"function","function":{"name":"Get_agent_status","arguments":"{"}}]}}]}`, "bad-call"},
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
			return http.StatusOK, `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"reasoning_details":` + details + `,"tool_calls":[{"id":"call-signed","type":"function","function":{"name":"Get_agent_status","arguments":"{\"agent_uuid\":\"` + maskedUUID + `\"}"}}]}}]}`
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
		return http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"Agent is healthy"}}]}`
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

func TestOpenAIRejectsUnusableCompletions(t *testing.T) {
	for _, factory := range []struct {
		name string
		new  func(string) *OpenAIProvider
	}{{"openai", NewOpenAIProvider}, {"openrouter", NewOpenRouterProvider}} {
		t.Run(factory.name, func(t *testing.T) {
			for _, tt := range []struct {
				name string
				body string
				want string
			}{
				{"null response", `null`, "no choices"},
				{"empty stop", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":""}}]}`, "no text or tool calls"},
				{"whitespace", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":" \n\t"}}]}`, "no text or tool calls"},
				{"reasoning exhausted budget", `{"id":"gen-budget","choices":[{"finish_reason":"length","message":{"role":"assistant","content":null,"reasoning":"Still thinking"}}],"usage":{"completion_tokens":2048,"completion_tokens_details":{"reasoning_tokens":2048}}}`, "token limit"},
				{"truncated text", `{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":"partial answer"}}]}`, "token limit"},
				{"truncated tool call with valid JSON", `{"choices":[{"finish_reason":"length","message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"Delete_agent","arguments":"{}"}}]}}]}`, "token limit"},
				{"content filtered", `{"choices":[{"finish_reason":"content_filter","message":{"role":"assistant","content":"partial"}}]}`, "content filter"},
				{"upstream failure", `{"error":{"code":503,"message":"Provider unavailable"}}`, "Provider unavailable"},
				{"upstream failure with partial text", `{"choices":[{"finish_reason":"error","message":{"role":"assistant","content":"partial"},"error":{"code":502,"message":"Provider disconnected"}}]}`, "Provider disconnected"},
				{"error overrides usable content", `{"error":{"message":"Generation failed"},"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"partial"}}]}`, "Generation failed"},
				{"unsupported finish reason", `{"choices":[{"finish_reason":"function_call","message":{"role":"assistant","content":"partial"}}]}`, "finish reason"},
				{"missing calls", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"Checking"}}]}`, "without tool calls"},
				{"missing tool id", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"type":"function","function":{"name":"List_agents","arguments":"{}"}}]}}]}`, "tool call"},
				{"missing tool name", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"arguments":"{}"}}]}}]}`, "tool call"},
				{"unsupported tool type", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"c1","type":"custom","custom":{"name":"List_agents","input":"{}"}}]}}]}`, "tool call"},
				{"array arguments", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"List_agents","arguments":"[]"}}]}}]}`, "arguments"},
				{"null arguments", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"List_agents","arguments":"null"}}]}}]}`, "arguments"},
				{"duplicate ids", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"List_agents","arguments":"{}"}},{"id":"c1","type":"function","function":{"name":"Delete_agent","arguments":"{}"}}]}}]}`, "duplicate"},
				{"object content coerced by SDK", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":{"unexpected":"object"}}}]}`, "invalid response"},
				{"malformed calls hidden by SDK", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"partial","tool_calls":{"unexpected":"object"}}}]}`, "invalid response"},
				{"object arguments coerced by SDK", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"Delete_agent","arguments":{}}}]}}]}`, "invalid response"},
				{"numeric tool ID coerced by SDK", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":123,"type":"function","function":{"name":"Delete_agent","arguments":"{}"}}]}}]}`, "invalid response"},
				{"explicit unfinished reason", `{"choices":[{"finish_reason":null,"message":{"role":"assistant","content":"partial"}}]}`, "finish reason"},
				{"explicit empty reason", `{"choices":[{"finish_reason":"","message":{"role":"assistant","content":"partial"}}]}`, "finish reason"},
				{"missing finish reason", `{"choices":[{"message":{"role":"assistant","content":"partial"}}]}`, "finish reason"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					p := factory.new("test-key")
					stubOpenAIHTTP(p, func(*http.Request) (int, string) { return http.StatusOK, tt.body })
					resp, err := p.Chat(context.Background(), ChatOptions{Model: "chat-model", MaxTokens: 2048, Messages: []Message{{Role: RoleUser, Text: "check"}}})
					require.Error(t, err)
					assert.Nil(t, resp, "failed or incomplete responses must never expose executable tools")
					assert.Contains(t, err.Error(), tt.want)
				})
			}
		})
	}
}

func TestOpenAIRejectsInvalidModelListings(t *testing.T) {
	for _, tt := range []struct{ body, want string }{
		{`null`, "model list"},
		{`{"error":{"code":503,"message":"Discovery unavailable"}}`, "Discovery unavailable"},
		{`{"data":[{"id":""}]}`, "model ID"},
		{`{"data":"invalid"}`, "model list"},
		{`{"data":[{"id":123}]}`, "model list"},
	} {
		t.Run(tt.body, func(t *testing.T) {
			p := NewOpenRouterProvider("test-key")
			stubOpenAIHTTP(p, func(*http.Request) (int, string) { return http.StatusOK, tt.body })
			models, err := p.ListModels(context.Background())
			require.Error(t, err)
			assert.Nil(t, models)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestOpenAIRequestDeadlines(t *testing.T) {
	for _, tt := range []struct {
		name  string
		limit time.Duration
		call  func(context.Context, *OpenAIProvider) error
	}{
		{"chat", 2 * time.Minute, func(ctx context.Context, p *OpenAIProvider) error {
			_, err := p.Chat(ctx, ChatOptions{Model: "chat-model", Messages: []Message{{Role: RoleUser, Text: "hi"}}})
			return err
		}},
		{"models", 5 * time.Second, func(ctx context.Context, p *OpenAIProvider) error {
			_, err := p.ListModels(ctx)
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := NewOpenAIProvider("test-key")
			requests := 0
			stubOpenAIHTTP(p, func(r *http.Request) (int, string) {
				requests++
				deadline, ok := r.Context().Deadline()
				assert.True(t, ok, "requests without a caller deadline must still be bounded")
				if ok {
					assert.Positive(t, time.Until(deadline))
					assert.LessOrEqual(t, time.Until(deadline), tt.limit)
				}
				return http.StatusOK, `{"data":[],"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"done"}}]}`
			})
			require.NoError(t, tt.call(context.Background(), p))
			assert.Equal(t, 1, requests)
		})
	}
}

func TestOpenRouterDoesNotForwardOpenAIAccountHeaders(t *testing.T) {
	t.Setenv("OPENAI_ORG_ID", "private-org")
	t.Setenv("OPENAI_PROJECT_ID", "private-project")
	for _, factory := range []struct {
		name string
		new  func(string) *OpenAIProvider
	}{{"openai", NewOpenAIProvider}, {"openrouter", NewOpenRouterProvider}} {
		t.Run(factory.name, func(t *testing.T) {
			p := factory.new("test-key")
			stubOpenAIHTTP(p, func(r *http.Request) (int, string) {
				if factory.name == "openrouter" {
					assert.Empty(t, r.Header.Get("OpenAI-Organization"))
					assert.Empty(t, r.Header.Get("OpenAI-Project"))
				} else {
					assert.Equal(t, "private-org", r.Header.Get("OpenAI-Organization"))
					assert.Equal(t, "private-project", r.Header.Get("OpenAI-Project"))
				}
				return http.StatusOK, `{"data":[]}`
			})
			_, err := p.ListModels(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestOpenRouterPreservesTextOnlyReasoning(t *testing.T) {
	for _, tt := range []struct {
		name       string
		new        func(string) *OpenAIProvider
		model      string
		wantReplay bool
		details    string
	}{
		{"same model with empty details", NewOpenRouterProvider, "source-model", true, `[]`},
		{"same model with signed details", NewOpenRouterProvider, "source-model", true, `[{"type":"reasoning.encrypted","data":"opaque-signature","index":0}]`},
		{"different model", NewOpenRouterProvider, "other-model", false, `[]`},
		{"different provider", NewOpenAIProvider, "source-model", false, `[]`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := NewOpenRouterProvider("test-key")
			stubOpenAIHTTP(p, func(*http.Request) (int, string) {
				return http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"Ready","reasoning_details":` + tt.details + `,"reasoning":"opaque reasoning","reasoning_content":"opaque alternate reasoning"}}]}`
			})
			a := NewAgent(Config{Model: "source-model"}, p, nil)
			require.NoError(t, a.SendMessage(context.Background(), "hello", func(Message) {}))
			next := tt.new("next-key")
			stubOpenAIHTTP(next, func(r *http.Request) (int, string) {
				var body struct {
					Messages []map[string]json.RawMessage `json:"messages"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Len(t, body.Messages, 4)
				previous := body.Messages[2]
				if tt.wantReplay {
					assert.JSONEq(t, tt.details, string(previous["reasoning_details"]))
					assert.JSONEq(t, `"opaque reasoning"`, string(previous["reasoning"]))
					assert.JSONEq(t, `"opaque alternate reasoning"`, string(previous["reasoning_content"]))
				} else {
					assert.NotContains(t, previous, "reasoning_details")
					assert.NotContains(t, previous, "reasoning")
					assert.NotContains(t, previous, "reasoning_content")
				}
				return http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"Done"}}]}`
			})
			a.SetModel(next, tt.model)
			require.NoError(t, a.SendMessage(context.Background(), "continue", func(Message) {}))
		})
	}
}

func TestOpenAIRefusalIsVisible(t *testing.T) {
	p := NewOpenAIProvider("test-key")
	stubOpenAIHTTP(p, func(*http.Request) (int, string) {
		return http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":null,"refusal":"I cannot complete this request"}}]}`
	})
	response, err := p.Chat(context.Background(), ChatOptions{Model: "chat-model", Messages: []Message{{Role: RoleUser, Text: "hello"}}})
	require.NoError(t, err)
	assert.Equal(t, []string{"I cannot complete this request"}, response.TextBlocks)
	assert.Empty(t, response.ToolUses)
}

func TestOpenAIHonorsDeadlineDuringRequest(t *testing.T) {
	for _, tt := range []struct {
		name string
		call func(context.Context, *OpenAIProvider) error
	}{
		{"chat", func(ctx context.Context, p *OpenAIProvider) error {
			_, err := p.Chat(ctx, ChatOptions{Model: "chat-model", Messages: []Message{{Role: RoleUser, Text: "hello"}}})
			return err
		}},
		{"models", func(ctx context.Context, p *OpenAIProvider) error {
			_, err := p.ListModels(ctx)
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := NewOpenAIProvider("test-key")
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			requests := 0
			opts := append([]option.RequestOption{}, p.client.Options...)
			p.client = openai.NewClient(append(opts, option.WithHTTPClient(&http.Client{Transport: openAITransportFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				deadline, ok := r.Context().Deadline()
				require.True(t, ok)
				parentDeadline, _ := ctx.Deadline()
				assert.Equal(t, parentDeadline, deadline)
				<-r.Context().Done()
				return nil, r.Context().Err()
			})}))...)
			assert.ErrorIs(t, tt.call(ctx, p), context.DeadlineExceeded)
			assert.Equal(t, 1, requests, "expired operations must not be retried")
		})
	}
}
