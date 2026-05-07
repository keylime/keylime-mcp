package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/keylime/keylime-mcp/internal/masking"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgent(t *testing.T) {
	t.Run("applies default MaxTokens and SystemPrompt", func(t *testing.T) {
		a := NewAgent(Config{}, &mockProvider{}, nil)
		assert.Equal(t, int64(DefaultMaxTokens), a.config.MaxTokens)
		assert.Equal(t, DefaultSystemPrompt, a.config.SystemPrompt)
	})

	t.Run("preserves custom config", func(t *testing.T) {
		a := NewAgent(Config{MaxTokens: 4096, SystemPrompt: "custom"}, &mockProvider{}, nil)
		assert.Equal(t, int64(4096), a.config.MaxTokens)
		assert.Equal(t, "custom", a.config.SystemPrompt)
	})
}

func TestGetTools(t *testing.T) {
	tools := []*mcp.Tool{
		{Name: testTool1, Description: "desc1"},
		{Name: testTool2, Description: "desc2"},
	}

	t.Run("stores tools from session", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{
			session: &mockSession{tools: tools},
		})
		err := a.GetTools(context.Background())
		require.NoError(t, err)
		assert.Equal(t, tools, a.tools)
	})

	t.Run("propagates error", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{
			session: &mockSession{toolsErr: errors.New("list failed")},
		})
		err := a.GetTools(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list failed")
	})
}

func TestSendMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("text response", func(t *testing.T) {
		a, prov, _ := newTestAgent(testAgentOpts{
			provider: &mockProvider{
				response: &LLMResponse{TextBlocks: []string{"hello", "world"}},
			},
		})

		var received []Message
		err := a.SendMessage(ctx, "hi", func(m Message) { received = append(received, m) })
		require.NoError(t, err)

		assert.Len(t, a.messages, 2)
		assert.Equal(t, RoleUser, a.messages[0].Role)
		assert.Equal(t, "hi", a.messages[0].Text)
		assert.Equal(t, RoleAssistant, a.messages[1].Role)
		assert.Equal(t, "hello\nworld", a.messages[1].Text)

		require.Len(t, received, 1)
		assert.Equal(t, RoleAssistant, received[0].Role)
		assert.Equal(t, "hello\nworld", received[0].Text)

		prov.mu.Lock()
		assert.Len(t, prov.chatCalls, 1)
		assert.Len(t, prov.chatCalls[0].Messages, 1)
		prov.mu.Unlock()
	})

	t.Run("tool use response", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{
			provider: &mockProvider{
				response: &LLMResponse{
					TextBlocks: []string{"let me check"},
					ToolUses: []ToolRequest{
						{ID: "t1", Name: "Get_version_and_health", Arguments: map[string]any{}},
					},
				},
			},
		})

		var received []Message
		err := a.SendMessage(ctx, "check", func(m Message) { received = append(received, m) })
		require.NoError(t, err)

		require.Len(t, received, 2)
		assert.Equal(t, "let me check", received[0].Text)
		assert.Equal(t, "Get_version_and_health", received[1].ToolCalls[0].Name)

		assert.NotNil(t, a.GetCurrentTool())
		assert.Equal(t, "t1", a.GetCurrentTool().ID)
	})

	t.Run("provider error propagated", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{
			provider: &mockProvider{err: errors.New("API down")},
		})
		err := a.SendMessage(ctx, "hi", func(m Message) {})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "API down")
	})

	t.Run("masking applied to tool results", func(t *testing.T) {
		masker := masking.NewEngine(true)
		uuid := testUUID

		a, prov, _ := newTestAgent(testAgentOpts{
			masker: masker,
			provider: &mockProvider{
				response: &LLMResponse{TextBlocks: []string{"done"}},
			},
		})

		a.messages = []Message{
			{Role: RoleUser, Text: "check agent"},
			{Role: RoleAssistant, Text: "checking"},
			{Role: RoleTool, ToolResult: &ToolResult{
				ToolID: "t1",
				Output: `{"agent":"` + uuid + `"}`,
			}},
		}

		err := a.SendMessage(ctx, "next", func(m Message) {})
		require.NoError(t, err)

		prov.mu.Lock()
		defer prov.mu.Unlock()
		require.Len(t, prov.chatCalls, 1)

		msgs := prov.chatCalls[0].Messages
		var toolMsg *Message
		for i := range msgs {
			if msgs[i].Role == RoleTool && msgs[i].ToolResult != nil {
				toolMsg = &msgs[i]
				break
			}
		}
		require.NotNil(t, toolMsg)
		assert.NotContains(t, toolMsg.ToolResult.Output, uuid)
		assert.Contains(t, toolMsg.ToolResult.Output, "AGENT-1")
	})

	t.Run("masking disabled passes through", func(t *testing.T) {
		uuid := testUUID
		a, prov, _ := newTestAgent(testAgentOpts{
			masker: masking.NewEngine(false),
			provider: &mockProvider{
				response: &LLMResponse{TextBlocks: []string{"done"}},
			},
		})
		a.messages = []Message{
			{Role: RoleTool, ToolResult: &ToolResult{ToolID: "t1", Output: uuid}},
		}

		err := a.SendMessage(ctx, "next", func(m Message) {})
		require.NoError(t, err)

		prov.mu.Lock()
		defer prov.mu.Unlock()
		msgs := prov.chatCalls[0].Messages
		var toolMsg *Message
		for i := range msgs {
			if msgs[i].Role == RoleTool && msgs[i].ToolResult != nil {
				toolMsg = &msgs[i]
				break
			}
		}
		require.NotNil(t, toolMsg)
		assert.Contains(t, toolMsg.ToolResult.Output, uuid)
	})

	t.Run("text response unmasked for display", func(t *testing.T) {
		masker := masking.NewEngine(true)
		uuid := testUUID
		masker.Mask(uuid)

		a, _, _ := newTestAgent(testAgentOpts{
			masker: masker,
			provider: &mockProvider{
				response: &LLMResponse{TextBlocks: []string{"Agent AGENT-1 is healthy"}},
			},
		})

		var received []Message
		err := a.SendMessage(ctx, "check", func(m Message) { received = append(received, m) })
		require.NoError(t, err)

		require.Len(t, received, 1)
		assert.Contains(t, received[0].Text, uuid)
		assert.NotContains(t, received[0].Text, "AGENT-1")
	})
}

func TestExecuteTool(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		a, _, sess := newTestAgent(testAgentOpts{
			session: &mockSession{
				callResult: &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "tool output"}},
				},
			},
			provider: &mockProvider{
				response: &LLMResponse{TextBlocks: []string{"got it"}},
			},
		})
		a.toolQueue = []ToolRequest{{ID: "t1", Name: "Get_version_and_health", Arguments: map[string]any{}}}

		var received []Message
		err := a.ExecuteTool(ctx, &a.toolQueue[0], func(m Message) { received = append(received, m) })
		require.NoError(t, err)

		sess.mu.Lock()
		require.Len(t, sess.callToolCalls, 1)
		assert.Equal(t, "Get_version_and_health", sess.callToolCalls[0].Name)
		sess.mu.Unlock()

		require.NotEmpty(t, received)
		assert.Equal(t, RoleTool, received[0].Role)
		assert.Equal(t, "tool output", received[0].ToolResult.Output)
		assert.False(t, received[0].ToolResult.IsError)
	})

	t.Run("call error", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{
			session:  &mockSession{callErr: errors.New("connection lost")},
			provider: &mockProvider{response: &LLMResponse{TextBlocks: []string{"sorry"}}},
		})
		a.toolQueue = []ToolRequest{{ID: "t1", Name: testTool1}}

		var received []Message
		err := a.ExecuteTool(ctx, &a.toolQueue[0], func(m Message) { received = append(received, m) })
		require.NoError(t, err)

		require.NotEmpty(t, received)
		assert.True(t, received[0].ToolResult.IsError)
		assert.Contains(t, received[0].ToolResult.Output, "connection lost")
	})

	t.Run("tool-reported error", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{
			session: &mockSession{
				callResult: &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "agent not found"}},
				},
			},
			provider: &mockProvider{response: &LLMResponse{TextBlocks: []string{"sorry"}}},
		})
		a.toolQueue = []ToolRequest{{ID: "t1", Name: testToolGetStatus}}

		var received []Message
		err := a.ExecuteTool(ctx, &a.toolQueue[0], func(m Message) { received = append(received, m) })
		require.NoError(t, err)

		require.NotEmpty(t, received)
		assert.True(t, received[0].ToolResult.IsError)
		assert.Contains(t, received[0].ToolResult.Output, testToolGetStatus)
		assert.Contains(t, received[0].ToolResult.Output, "agent not found")
	})

	t.Run("advances to next tool in queue", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{
			session: &mockSession{
				callResult: &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
				},
			},
		})
		a.toolQueue = []ToolRequest{
			{ID: "t1", Name: testTool1},
			{ID: "t2", Name: testTool2},
		}

		var received []Message
		err := a.ExecuteTool(ctx, &a.toolQueue[0], func(m Message) { received = append(received, m) })
		require.NoError(t, err)

		var hasNextTool bool
		for _, m := range received {
			if len(m.ToolCalls) > 0 && m.ToolCalls[0].Name == testTool2 {
				hasNextTool = true
			}
		}
		assert.True(t, hasNextTool, "should signal next tool in queue")
	})

	t.Run("unmasks tool arguments before calling session", func(t *testing.T) {
		masker := masking.NewEngine(true)
		uuid := testUUID
		masker.Mask(uuid)

		sess := &mockSession{
			callResult: &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			},
		}
		a, _, _ := newTestAgent(testAgentOpts{
			masker:   masker,
			session:  sess,
			provider: &mockProvider{response: &LLMResponse{TextBlocks: []string{"done"}}},
		})
		a.toolQueue = []ToolRequest{{
			ID:        "t1",
			Name:      testToolGetStatus,
			Arguments: map[string]any{"agent_uuid": "AGENT-1"},
		}}

		err := a.ExecuteTool(ctx, &a.toolQueue[0], func(m Message) {})
		require.NoError(t, err)

		sess.mu.Lock()
		defer sess.mu.Unlock()
		require.Len(t, sess.callToolCalls, 1)
		args, ok := sess.callToolCalls[0].Arguments.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, uuid, args["agent_uuid"])
	})
}

func TestToolDeny(t *testing.T) {
	ctx := context.Background()

	t.Run("appends denied message and advances queue", func(t *testing.T) {
		a, prov, _ := newTestAgent(testAgentOpts{
			provider: &mockProvider{
				response: &LLMResponse{TextBlocks: []string{"ok, skipped"}},
			},
		})
		a.toolQueue = []ToolRequest{{ID: "t1", Name: testTool1}}

		var received []Message
		err := a.ToolDeny(ctx, &a.toolQueue[0], func(m Message) { received = append(received, m) })
		require.NoError(t, err)

		var hasDenied bool
		for _, m := range a.messages {
			if m.ToolResult != nil && m.ToolResult.Output == "Tool execution denied by user." {
				hasDenied = true
			}
		}
		assert.True(t, hasDenied)

		prov.mu.Lock()
		assert.NotEmpty(t, prov.chatCalls, "should call LLM after advancing empty queue")
		prov.mu.Unlock()
	})

	t.Run("with multiple tools advances to next", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{})
		a.toolQueue = []ToolRequest{
			{ID: "t1", Name: testTool1},
			{ID: "t2", Name: testTool2},
		}

		var received []Message
		err := a.ToolDeny(ctx, &a.toolQueue[0], func(m Message) { received = append(received, m) })
		require.NoError(t, err)

		var hasNextTool bool
		for _, m := range received {
			if len(m.ToolCalls) > 0 && m.ToolCalls[0].Name == testTool2 {
				hasNextTool = true
			}
		}
		assert.True(t, hasNextTool)
	})
}

func TestGetCurrentTool(t *testing.T) {
	t.Run("returns first tool", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{})
		a.toolQueue = []ToolRequest{{ID: "t1", Name: testTool1}, {ID: "t2", Name: testTool2}}
		tool := a.GetCurrentTool()
		require.NotNil(t, tool)
		assert.Equal(t, "t1", tool.ID)
	})

	t.Run("returns nil when empty", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{})
		assert.Nil(t, a.GetCurrentTool())
	})
}

func TestReset(t *testing.T) {
	a, _, _ := newTestAgent(testAgentOpts{})
	a.messages = []Message{{Role: RoleUser, Text: "hi"}}
	a.toolQueue = []ToolRequest{{ID: "t1"}}

	a.Reset()

	assert.Empty(t, a.messages)
	assert.Nil(t, a.toolQueue)
}

func TestSetModel(t *testing.T) {
	a, _, _ := newTestAgent(testAgentOpts{model: "old-model"})
	newProv := &mockProvider{name: "new-provider"}

	a.SetModel(newProv, "new-model")

	assert.Equal(t, "new-model", a.GetModel())
	assert.Equal(t, "new-provider", a.GetProvider())
}

func TestGetProvider(t *testing.T) {
	t.Run("returns provider name", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{
			provider: &mockProvider{name: "anthropic"},
		})
		assert.Equal(t, "anthropic", a.GetProvider())
	})

	t.Run("returns empty when nil", func(t *testing.T) {
		a := &Agent{}
		assert.Equal(t, "", a.GetProvider())
	})
}

func TestClose(t *testing.T) {
	a, _, sess := newTestAgent(testAgentOpts{})
	a.Close()
	assert.True(t, sess.closed)
}

func TestExtractTextContent(t *testing.T) {
	t.Run("extracts text blocks", func(t *testing.T) {
		content := []mcp.Content{
			&mcp.TextContent{Text: "hello "},
			&mcp.TextContent{Text: "world"},
		}
		assert.Equal(t, "hello world", extractTextContent(content))
	})

	t.Run("empty content", func(t *testing.T) {
		assert.Equal(t, "", extractTextContent(nil))
	})
}

func TestUnmaskToolRequest(t *testing.T) {
	uuid := testUUID

	t.Run("disabled returns original", func(t *testing.T) {
		a, _, _ := newTestAgent(testAgentOpts{masker: masking.NewEngine(false)})
		tr := ToolRequest{ID: "t1", Name: "tool", Arguments: map[string]any{"uuid": "AGENT-1"}}
		result := a.unmaskToolRequest(tr)
		assert.Equal(t, "AGENT-1", result.Arguments.(map[string]any)["uuid"])
	})

	t.Run("resolves aliases", func(t *testing.T) {
		masker := masking.NewEngine(true)
		masker.Mask(uuid)

		a, _, _ := newTestAgent(testAgentOpts{masker: masker})
		tr := ToolRequest{ID: "t1", Name: "tool", Arguments: map[string]any{"uuid": "AGENT-1"}}
		result := a.unmaskToolRequest(tr)
		assert.Equal(t, uuid, result.Arguments.(map[string]any)["uuid"])
	})

	t.Run("nil masker returns original", func(t *testing.T) {
		a := &Agent{}
		tr := ToolRequest{ID: "t1", Name: "tool", Arguments: map[string]any{"k": "v"}}
		result := a.unmaskToolRequest(tr)
		assert.Equal(t, "v", result.Arguments.(map[string]any)["k"])
	})
}
