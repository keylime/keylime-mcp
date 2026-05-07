package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/keylime/keylime-mcp/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleIndex(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.handleIndex(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Keylime")
}

func TestHandleChat(t *testing.T) {
	t.Run("sends user and assistant messages via SSE", func(t *testing.T) {
		s := newTestServer(t, &stubProvider{
			name:     providerStub,
			response: &agent.LLMResponse{TextBlocks: []string{"hello back"}},
		})
		ch := subscribeSSE(t, s)

		req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader("message=hi"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		s.handleChat(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		userEvent := waitForEvent(t, ch, 2*time.Second)
		assert.Equal(t, "user-message", userEvent.Event)
		assert.Contains(t, userEvent.Data, "hi")

		assistantEvent := waitForEvent(t, ch, 2*time.Second)
		assert.Equal(t, "assistant-message", assistantEvent.Event)
		assert.Contains(t, assistantEvent.Data, "hello back")
	})

	t.Run("empty message returns 400", func(t *testing.T) {
		s := newTestServer(t)
		req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader("message="))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		s.handleChat(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("tool use response sends tool-request event", func(t *testing.T) {
		s := newTestServer(t, &stubProvider{
			name: providerStub,
			response: &agent.LLMResponse{
				TextBlocks: []string{"checking"},
				ToolUses: []agent.ToolRequest{
					{ID: "t1", Name: "Get_agent_status", Arguments: map[string]any{"uuid": eventTest}},
				},
			},
		})
		ch := subscribeSSE(t, s)

		req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader("message=check"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		s.handleChat(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		events := drainEvents(ch, 2*time.Second)
		var hasToolRequest bool
		for _, e := range events {
			if e.Event == "tool-request" {
				hasToolRequest = true
				assert.Contains(t, e.Data, "Get_agent_status")
			}
		}
		assert.True(t, hasToolRequest, "should receive tool-request SSE event")
	})

	t.Run("provider error sends error event", func(t *testing.T) {
		s := newTestServer(t, &stubProvider{
			name: providerStub,
			err:  errors.New("API rate limited"),
		})
		ch := subscribeSSE(t, s)

		req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader("message=hi"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		s.handleChat(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		events := drainEvents(ch, 2*time.Second)
		var hasError bool
		for _, e := range events {
			if e.Event == eventError {
				hasError = true
				assert.Contains(t, e.Data, "API rate limited")
			}
		}
		assert.True(t, hasError, "should receive error SSE event")
	})
}

func TestHandleToolApprove(t *testing.T) {
	t.Run("no pending tool returns 400", func(t *testing.T) {
		s := newTestServer(t)
		req := httptest.NewRequest(http.MethodPost, "/tool/approve", nil)
		w := httptest.NewRecorder()

		s.handleToolApprove(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleToolDeny(t *testing.T) {
	t.Run("no pending tool returns 200", func(t *testing.T) {
		s := newTestServer(t)
		req := httptest.NewRequest(http.MethodPost, "/tool/deny", nil)
		w := httptest.NewRecorder()

		s.handleToolDeny(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("with pending tool sends denied event and continues", func(t *testing.T) {
		prov := &stubProvider{
			name: providerStub,
			response: &agent.LLMResponse{
				TextBlocks: []string{"let me check"},
				ToolUses: []agent.ToolRequest{
					{ID: "t1", Name: "tool1", Arguments: map[string]any{}},
				},
			},
		}
		s := newTestServer(t, prov)
		ch := subscribeSSE(t, s)

		chatReq := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader("message=check"))
		chatReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		s.handleChat(httptest.NewRecorder(), chatReq)

		events := drainEvents(ch, 2*time.Second)
		var hasToolRequest bool
		for _, e := range events {
			if e.Event == "tool-request" {
				hasToolRequest = true
			}
		}
		require.True(t, hasToolRequest, "tool queue should be populated")

		prov.mu.Lock()
		prov.response = &agent.LLMResponse{TextBlocks: []string{"ok, skipped"}}
		prov.mu.Unlock()

		denyReq := httptest.NewRequest(http.MethodPost, "/tool/deny", nil)
		w := httptest.NewRecorder()
		s.handleToolDeny(w, denyReq)
		assert.Equal(t, http.StatusOK, w.Code)

		denyEvents := drainEvents(ch, 2*time.Second)
		var hasDenied bool
		for _, e := range denyEvents {
			if e.Event == "tool-denied" {
				hasDenied = true
			}
		}
		assert.True(t, hasDenied, "should receive tool-denied SSE event")
	})
}

func TestHandleReset(t *testing.T) {
	s := newTestServer(t)
	ch := subscribeSSE(t, s)

	s.send(SSEvent{Event: eventTest, Data: "data"})
	drainEvents(ch, 100*time.Millisecond)

	req := httptest.NewRequest(http.MethodPost, "/reset", nil)
	w := httptest.NewRecorder()

	s.handleReset(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	event := waitForEvent(t, ch, time.Second)
	assert.Equal(t, "reset", event.Event)

	s.mu.Lock()
	assert.Nil(t, s.history)
	s.mu.Unlock()
}

func TestHandleListModels(t *testing.T) {
	t.Run("returns models from all providers", func(t *testing.T) {
		s := newTestServer(t,
			&stubProvider{
				name:   providerAnthropic,
				models: []agent.ModelInfo{{ID: "claude-3", DisplayName: "Claude 3", Provider: providerAnthropic}},
			},
			&stubProvider{
				name:   providerOllama,
				models: []agent.ModelInfo{{ID: "llama3", DisplayName: "Llama 3", Provider: providerOllama}},
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
		w := httptest.NewRecorder()

		s.handleListModels(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Models       []agent.ModelInfo `json:"models"`
			OllamaStatus string            `json:"ollama_status,omitempty"`
		}
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Len(t, resp.Models, 2)
		assert.Empty(t, resp.OllamaStatus)
	})

	t.Run("provider error returns partial results", func(t *testing.T) {
		s := newTestServer(t,
			&stubProvider{
				name:   providerAnthropic,
				models: []agent.ModelInfo{{ID: "claude-3", Provider: providerAnthropic}},
			},
			&stubProvider{
				name:      providerOllama,
				modelsErr: errors.New("connection refused"),
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
		w := httptest.NewRecorder()

		s.handleListModels(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Models       []agent.ModelInfo `json:"models"`
			OllamaStatus string            `json:"ollama_status,omitempty"`
		}
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Len(t, resp.Models, 1)
	})
}

func TestHandleGetModel(t *testing.T) {
	s := newTestServer(t, &stubProvider{name: providerStub})

	req := httptest.NewRequest(http.MethodGet, "/api/model", nil)
	w := httptest.NewRecorder()

	s.handleGetModel(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "test-model", resp["model"])
	assert.Equal(t, providerStub, resp["provider"])
}

func TestHandleSetModel(t *testing.T) {
	t.Run("switches model", func(t *testing.T) {
		s := newTestServer(t, &stubProvider{name: providerStub})

		body := `{"provider":"stub","model":"new-model"}`
		req := httptest.NewRequest(http.MethodPost, "/api/model", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		s.handleSetModel(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]string
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "new-model", resp["model"])
		assert.Equal(t, providerStub, resp["provider"])
	})

	t.Run("empty model returns 400", func(t *testing.T) {
		s := newTestServer(t)
		body := `{"provider":"stub","model":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/model", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		s.handleSetModel(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid body returns 400", func(t *testing.T) {
		s := newTestServer(t)
		req := httptest.NewRequest(http.MethodPost, "/api/model", strings.NewReader("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		s.handleSetModel(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unknown provider returns 400", func(t *testing.T) {
		s := newTestServer(t, &stubProvider{name: providerStub})
		body := `{"provider":"unknown","model":"m"}`
		req := httptest.NewRequest(http.MethodPost, "/api/model", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		s.handleSetModel(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Unknown provider")
	})
}

func TestSend(t *testing.T) {
	t.Run("broadcasts to all clients", func(t *testing.T) {
		s := newTestServer(t)
		ch1 := subscribeSSE(t, s)
		ch2 := subscribeSSE(t, s)

		s.send(SSEvent{Event: eventTest, Data: "hello"})

		e1 := waitForEvent(t, ch1, time.Second)
		e2 := waitForEvent(t, ch2, time.Second)
		assert.Equal(t, eventTest, e1.Event)
		assert.Equal(t, eventTest, e2.Event)
	})

	t.Run("history capped at 1000", func(t *testing.T) {
		s := newTestServer(t)
		for range 1010 {
			s.send(SSEvent{Event: eventTest, Data: "data"})
		}
		s.mu.Lock()
		assert.Len(t, s.history, 1000)
		s.mu.Unlock()
	})

	t.Run("full channel drops event without blocking", func(t *testing.T) {
		s := newTestServer(t)
		ch := make(chan SSEvent)
		s.mu.Lock()
		s.clients[ch] = struct{}{}
		s.mu.Unlock()
		t.Cleanup(func() {
			s.mu.Lock()
			delete(s.clients, ch)
			s.mu.Unlock()
		})

		done := make(chan struct{})
		go func() {
			s.send(SSEvent{Event: eventTest, Data: "data"})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("send blocked on full channel")
		}
	})
}
