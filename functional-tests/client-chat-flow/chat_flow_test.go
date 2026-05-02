//go:build functional

package clientchatflow_test

import (
	"testing"
	"time"

	"github.com/keylime/keylime-mcp/functional-tests/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestChatFlow(t *testing.T) {
	responses := []testhelpers.MockLLMResponse{
		{StopReason: "end_turn", Text: "Hello! I am the mock assistant."},
		{StopReason: "tool_use", Text: "Let me check the health.", ToolCalls: []testhelpers.MockToolCall{
			{ID: "tool_001", Name: "Get_version_and_health", Arguments: map[string]any{}},
		}},
		{StopReason: "end_turn", Text: "The system is healthy."},
		{StopReason: "tool_use", Text: "Let me get the agents.", ToolCalls: []testhelpers.MockToolCall{
			{ID: "tool_002", Name: "Get_all_agents", Arguments: map[string]any{}},
		}},
		{StopReason: "end_turn", Text: "OK, I will not run that tool."},
	}

	c := testhelpers.NewMCPTestClient(t, responses)
	ch := c.ConnectSSE()
	c.WaitForEvent(ch, "sync", 5*time.Second)

	t.Run("basic_message", func(t *testing.T) {
		resp := c.PostChat("Hello")
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()

		userMsg := c.WaitForEvent(ch, "user-message", 10*time.Second)
		assert.Contains(t, userMsg.Data, "Hello")

		assistantMsg := c.WaitForEvent(ch, "assistant-message", 30*time.Second)
		assert.Contains(t, assistantMsg.Data, "Hello! I am the mock assistant.")
	})

	t.Run("tool_approval", func(t *testing.T) {
		requestsBefore := c.MockLLM.RequestCount()

		resp := c.PostChat("Check health")
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()

		c.WaitForEvent(ch, "user-message", 10*time.Second)
		c.WaitForEvent(ch, "assistant-message", 30*time.Second)

		toolReq := c.WaitForEvent(ch, "tool-request", 30*time.Second)
		assert.Contains(t, toolReq.Data, "Get_version_and_health")

		resp = c.PostToolApprove()
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()

		c.WaitForEvent(ch, "tool-executing", 10*time.Second)

		toolResult := c.WaitForEvent(ch, "tool-result", 30*time.Second)
		assert.Contains(t, toolResult.Data, "verifier")

		assistantMsg := c.WaitForEvent(ch, "assistant-message", 30*time.Second)
		assert.Contains(t, assistantMsg.Data, "The system is healthy.")

		assert.Equal(t, requestsBefore+2, c.MockLLM.RequestCount(),
			"mock should receive 2 requests: initial chat + follow-up with tool result")
	})

	t.Run("tool_denial", func(t *testing.T) {
		resp := c.PostChat("Get agents")
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()

		c.WaitForEvent(ch, "user-message", 10*time.Second)
		c.WaitForEvent(ch, "assistant-message", 30*time.Second)

		toolReq := c.WaitForEvent(ch, "tool-request", 30*time.Second)
		assert.Contains(t, toolReq.Data, "Get_all_agents")

		resp = c.PostToolDeny()
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()

		c.WaitForEvent(ch, "tool-denied", 10*time.Second)

		assistantMsg := c.WaitForEvent(ch, "assistant-message", 30*time.Second)
		assert.Contains(t, assistantMsg.Data, "OK, I will not run that tool.")
	})

	t.Run("reset", func(t *testing.T) {
		rc := testhelpers.NewMCPTestClient(t, []testhelpers.MockLLMResponse{
			{StopReason: "end_turn", Text: "Before reset."},
		})
		rch := rc.ConnectSSE()
		rc.WaitForEvent(rch, "sync", 5*time.Second)

		resp := rc.PostChat("Hi")
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()
		rc.WaitForEvent(rch, "user-message", 10*time.Second)
		rc.WaitForEvent(rch, "assistant-message", 30*time.Second)

		resp = rc.PostReset()
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()
		rc.WaitForEvent(rch, "reset", 10*time.Second)

		rch2 := rc.ConnectSSE()
		rc.WaitForEvent(rch2, "sync", 5*time.Second)
		select {
		case evt := <-rch2:
			assert.Equal(t, "ping", evt.Event,
				"expected ping after reset sync, got replayed event: %s", evt.Event)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for event after reset sync")
		}
	})

	t.Run("sse_history_replay", func(t *testing.T) {
		hc := testhelpers.NewMCPTestClient(t, []testhelpers.MockLLMResponse{
			{StopReason: "end_turn", Text: "Replayed response."},
		})
		hch := hc.ConnectSSE()
		hc.WaitForEvent(hch, "sync", 5*time.Second)

		resp := hc.PostChat("Test message")
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()
		hc.WaitForEvent(hch, "user-message", 10*time.Second)
		hc.WaitForEvent(hch, "assistant-message", 30*time.Second)

		hch2 := hc.ConnectSSE()
		hc.WaitForEvent(hch2, "sync", 5*time.Second)

		userEvt := hc.WaitForEvent(hch2, "user-message", 5*time.Second)
		assert.Contains(t, userEvt.Data, "Test message")

		assistantEvt := hc.WaitForEvent(hch2, "assistant-message", 5*time.Second)
		assert.Contains(t, assistantEvt.Data, "Replayed response.")
	})
}
