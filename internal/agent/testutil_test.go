package agent

import (
	"context"
	"sync"

	"github.com/keylime/keylime-mcp/internal/masking"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testTool1            = "tool1"
	testTool2            = "tool2"
	testUUID             = "d432fbb3-d2f1-4a97-9ef7-75bd81c00000"
	testCallID1          = "call1"
	testSchemaProperties = "properties"
	testSchemaRequired   = "required"
	testSchemaType       = "type"
	testArgUUID          = "agent_uuid"
	testToolGetStatus    = "Get_agent_status"
)

type mockProvider struct {
	mu        sync.Mutex
	name      string
	response  *LLMResponse
	err       error
	models    []ModelInfo
	modelsErr error
	chatCalls []ChatOptions
}

func (m *mockProvider) Chat(_ context.Context, opts ChatOptions) (*LLMResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatCalls = append(m.chatCalls, opts)
	return m.response, m.err
}

func (m *mockProvider) ListModels(_ context.Context) ([]ModelInfo, error) {
	return m.models, m.modelsErr
}

func (m *mockProvider) Name() string {
	if m.name == "" {
		return "mock"
	}
	return m.name
}

type callToolCall struct {
	Name      string
	Arguments any
}

type mockSession struct {
	mu            sync.Mutex
	tools         []*mcp.Tool
	toolsErr      error
	callResult    *mcp.CallToolResult
	callErr       error
	callToolCalls []callToolCall
	closed        bool
}

func (m *mockSession) ListTools(_ context.Context, _ *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	if m.toolsErr != nil {
		return nil, m.toolsErr
	}
	return &mcp.ListToolsResult{Tools: m.tools}, nil
}

func (m *mockSession) CallTool(_ context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callToolCalls = append(m.callToolCalls, callToolCall{
		Name:      params.Name,
		Arguments: params.Arguments,
	})
	return m.callResult, m.callErr
}

func (m *mockSession) Close() error {
	m.closed = true
	return nil
}

type testAgentOpts struct {
	provider *mockProvider
	session  *mockSession
	masker   *masking.Engine
	tools    []*mcp.Tool
	model    string
}

func newTestAgent(opts testAgentOpts) (*Agent, *mockProvider, *mockSession) {
	if opts.provider == nil {
		opts.provider = &mockProvider{
			response: &LLMResponse{TextBlocks: []string{"ok"}},
		}
	}
	if opts.session == nil {
		opts.session = &mockSession{
			callResult: &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "result"}},
			},
		}
	}
	if opts.masker == nil {
		opts.masker = masking.NewEngine(false)
	}

	a := NewAgent(Config{Model: opts.model}, opts.provider, opts.masker)
	a.mcpSession = opts.session
	a.tools = opts.tools

	return a, opts.provider, opts.session
}
