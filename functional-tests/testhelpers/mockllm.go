//go:build functional

package testhelpers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type MockToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type MockLLMResponse struct {
	StopReason string
	Text       string
	ToolCalls  []MockToolCall
}

type MockLLM struct {
	*httptest.Server
	mu        sync.Mutex
	responses []MockLLMResponse
	Requests  []json.RawMessage
	Models    []string
}

func NewMockLLM(responses []MockLLMResponse) *MockLLM {
	m := &MockLLM{
		responses: responses,
		Models:    []string{"mock-model:latest"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/messages", m.handleMessages)
	mux.HandleFunc("GET /api/tags", m.handleTags)
	m.Server = httptest.NewServer(mux)
	return m
}

func (m *MockLLM) handleMessages(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	m.Requests = append(m.Requests, body)

	if len(m.responses) == 0 {
		http.Error(w, "no more mock responses queued", http.StatusInternalServerError)
		return
	}

	resp := m.responses[0]
	m.responses = m.responses[1:]

	var content []map[string]any
	if resp.Text != "" {
		content = append(content, map[string]any{
			"type": "text",
			"text": resp.Text,
		})
	}
	for _, tc := range resp.ToolCalls {
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Name,
			"input": tc.Arguments,
		})
	}

	msg := map[string]any{
		"id":          fmt.Sprintf("msg_mock_%03d", len(m.Requests)),
		"type":        "message",
		"role":        "assistant",
		"model":       "mock-model",
		"stop_reason": resp.StopReason,
		"content":     content,
		"usage":       map[string]any{"input_tokens": 10, "output_tokens": 20},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func (m *MockLLM) handleTags(w http.ResponseWriter, _ *http.Request) {
	models := make([]map[string]string, len(m.Models))
	for i, name := range m.Models {
		models[i] = map[string]string{"name": name}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"models": models})
}

func (m *MockLLM) RequestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Requests)
}

type SSEEvent struct {
	Event string
	Data  string
}

type MCPTestClient struct {
	T       *testing.T
	URL     string
	MockLLM *MockLLM
	cmd     *exec.Cmd
}

func ClientBinaryPath(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(ProjectRoot(), "bin", "client")
	if _, err := os.Stat(bin); err != nil {
		if !os.IsNotExist(err) {
			require.NoError(t, err, "failed to stat client binary")
		}
		cmd := exec.Command("make", "-C", ProjectRoot(), "build")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "failed to build client: %s", string(out))
	}
	return bin
}

func NewMCPTestClient(t *testing.T, responses []MockLLMResponse, envOverrides ...string) *MCPTestClient {
	t.Helper()

	mockLLM := NewMockLLM(responses)

	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	bin := ClientBinaryPath(t)
	serverBin := ServerBinaryPath(t)

	cmd := exec.Command(bin)
	env := append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("OLLAMA_URL=%s", mockLLM.URL),
		"OLLAMA_MODEL=mock-model:latest",
		"ANTHROPIC_API_KEY=",
		fmt.Sprintf("MCP_SERVER_PATH=%s", serverBin),
		"MASKING_ENABLED=false",
	)
	env = append(env, envOverrides...)
	cmd.Env = env
	cmd.Dir = filepath.Dir(bin)

	require.NoError(t, cmd.Start(), "failed to start client")

	clientURL := fmt.Sprintf("http://localhost:%d", port)

	deadline := time.Now().Add(15 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(clientURL + "/")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.True(t, ready, "client did not become ready within 15s at %s", clientURL)

	c := &MCPTestClient{
		T:       t,
		URL:     clientURL,
		MockLLM: mockLLM,
		cmd:     cmd,
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func (c *MCPTestClient) Close() {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			c.cmd.Process.Kill()
			<-done
		}
		c.cmd = nil
	}
	if c.MockLLM != nil {
		c.MockLLM.Close()
		c.MockLLM = nil
	}
}

func (c *MCPTestClient) PostChat(message string) *http.Response {
	c.T.Helper()
	resp, err := http.PostForm(c.URL+"/chat", url.Values{"message": {message}})
	require.NoError(c.T, err)
	return resp
}

func (c *MCPTestClient) PostToolApprove() *http.Response {
	c.T.Helper()
	resp, err := http.Post(c.URL+"/tool/approve", "", nil)
	require.NoError(c.T, err)
	return resp
}

func (c *MCPTestClient) PostToolDeny() *http.Response {
	c.T.Helper()
	resp, err := http.Post(c.URL+"/tool/deny", "", nil)
	require.NoError(c.T, err)
	return resp
}

func (c *MCPTestClient) PostReset() *http.Response {
	c.T.Helper()
	resp, err := http.Post(c.URL+"/reset", "", nil)
	require.NoError(c.T, err)
	return resp
}

func (c *MCPTestClient) GetModels() *http.Response {
	c.T.Helper()
	resp, err := http.Get(c.URL + "/api/models")
	require.NoError(c.T, err)
	return resp
}

func (c *MCPTestClient) GetModel() *http.Response {
	c.T.Helper()
	resp, err := http.Get(c.URL + "/api/model")
	require.NoError(c.T, err)
	return resp
}

func (c *MCPTestClient) SetModel(provider, model string) *http.Response {
	c.T.Helper()
	body, _ := json.Marshal(map[string]string{"provider": provider, "model": model})
	resp, err := http.Post(c.URL+"/api/model", "application/json", bytes.NewReader(body))
	require.NoError(c.T, err)
	return resp
}

func (c *MCPTestClient) ConnectSSE() <-chan SSEEvent {
	c.T.Helper()
	resp, err := http.Get(c.URL + "/events")
	require.NoError(c.T, err)

	ch := make(chan SSEEvent, 100)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		scanner := bufio.NewScanner(resp.Body)
		var evt SSEEvent
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				evt.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data := strings.TrimPrefix(line, "data: ")
				evt.Data = strings.ReplaceAll(data, "\\n", "\n")
			case line == "":
				if evt.Event != "" {
					ch <- evt
				}
				evt = SSEEvent{}
			}
		}
	}()
	return ch
}

func (c *MCPTestClient) WaitForEvent(ch <-chan SSEEvent, eventType string, timeout time.Duration) SSEEvent {
	c.T.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				c.T.Fatalf("SSE channel closed while waiting for %q", eventType)
				return SSEEvent{} // unreachable but satisfies compiler
			}
			if evt.Event == eventType {
				return evt
			}
		case <-timer.C:
			c.T.Fatalf("timeout waiting for SSE event %q", eventType)
			return SSEEvent{} // unreachable but satisfies compiler
		}
	}
}

func (c *MCPTestClient) WaitForEvents(ch <-chan SSEEvent, eventTypes []string, timeout time.Duration) []SSEEvent {
	c.T.Helper()
	var events []SSEEvent
	for _, et := range eventTypes {
		events = append(events, c.WaitForEvent(ch, et, timeout))
	}
	return events
}
