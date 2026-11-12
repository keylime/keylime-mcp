package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/keylime/keylime-mcp/internal/agent"
	"github.com/stretchr/testify/assert"
)

type providerTransportFunc func(*http.Request) (*http.Response, error)

func (f providerTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOpenRouterFailureReachesWeb(t *testing.T) {
	for _, tt := range []struct{ name, body, want string }{
		{"reasoning token limit", `{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":null,"reasoning":"Still thinking"}}]}`, "token limit"},
		{"empty completion", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":""}}]}`, "no text or tool calls"},
		{"provider failure", `{"error":{"code":503,"message":"Provider unavailable"}}`, "Provider unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Replace external HTTP only; exercise the real SDK, adapter, agent and SSE.
			previous := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: providerTransportFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK, Request: r,
					Header: http.Header{"Content-Type": []string{"application/json"}},
					Body:   io.NopCloser(strings.NewReader(tt.body)),
				}, nil
			})}
			t.Cleanup(func() { http.DefaultClient = previous })
			s := newTestServer(t)
			s.agent = agent.NewAgent(agent.Config{Model: "reasoning-model"}, agent.NewOpenRouterProvider("test-key"), nil)
			ch := subscribeSSE(t, s)
			req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader("message=check"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			s.handleChat(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "user-message", waitForEvent(t, ch, time.Second).Event)
			failure := waitForEvent(t, ch, time.Second)
			assert.Equal(t, "error", failure.Event)
			assert.Contains(t, failure.Data, tt.want)
			select {
			case extra := <-ch:
				t.Errorf("failed completion emitted an unexpected event: %s", extra.Event)
			default:
			}
		})
	}
}
