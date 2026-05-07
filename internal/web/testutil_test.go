package web

import (
	"context"
	"html/template"
	"sync"
	"testing"
	"time"

	"github.com/keylime/keylime-mcp/internal/agent"
	"github.com/keylime/keylime-mcp/internal/masking"
	"github.com/stretchr/testify/require"
)

const (
	providerStub      = "stub"
	providerAnthropic = "anthropic"
	eventTest         = "test"
)

type stubProvider struct {
	mu        sync.Mutex
	name      string
	response  *agent.LLMResponse
	err       error
	models    []agent.ModelInfo
	modelsErr error
}

func (p *stubProvider) Chat(_ context.Context, opts agent.ChatOptions) (*agent.LLMResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.response, p.err
}

func (p *stubProvider) ListModels(_ context.Context) ([]agent.ModelInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.models, p.modelsErr
}

func (p *stubProvider) Name() string {
	if p.name == "" {
		return providerStub
	}
	return p.name
}

func newTestServer(t *testing.T, providers ...*stubProvider) *Server {
	t.Helper()

	if len(providers) == 0 {
		providers = []*stubProvider{{
			name:     providerStub,
			response: &agent.LLMResponse{TextBlocks: []string{"test response"}},
			models:   []agent.ModelInfo{{ID: "model-1", DisplayName: "Model 1", Provider: providerStub}},
		}}
	}

	primary := providers[0]
	masker := masking.NewEngine(false)
	ag := agent.NewAgent(agent.Config{Model: "test-model"}, primary, masker)

	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	require.NoError(t, err)

	llmProviders := make([]agent.LLMProvider, len(providers))
	for i, p := range providers {
		llmProviders[i] = p
	}

	return &Server{
		agent:     ag,
		providers: llmProviders,
		templates: tmpl,
		clients:   make(map[chan SSEvent]struct{}),
		ctx:       context.Background(),
	}
}

func subscribeSSE(t *testing.T, s *Server) chan SSEvent {
	t.Helper()
	ch := make(chan SSEvent, 100)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	t.Cleanup(func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	})
	return ch
}

func waitForEvent(t *testing.T, ch chan SSEvent, timeout time.Duration) SSEvent {
	t.Helper()
	select {
	case e := <-ch:
		return e
	case <-time.After(timeout):
		t.Fatal("timed out waiting for SSE event")
		return SSEvent{}
	}
}

func drainEvents(ch chan SSEvent, timeout time.Duration) []SSEvent {
	var events []SSEvent
	for {
		select {
		case e := <-ch:
			events = append(events, e)
		case <-time.After(timeout):
			return events
		}
	}
}
