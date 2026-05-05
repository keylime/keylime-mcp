//go:build functional

package testhelpers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const AgentID = "d432fbb3-d2f1-4a97-9ef7-75bd81c00000"

func ProjectRoot() string {
	if root := os.Getenv("TMT_TREE"); root != "" {
		return root
	}
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func ServerBinaryPath(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(ProjectRoot(), "bin", "server")
	if _, err := os.Stat(bin); err != nil {
		if !os.IsNotExist(err) {
			require.NoError(t, err, "failed to stat server binary")
		}
		cmd := exec.Command("make", "-C", ProjectRoot(), "build-server")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "failed to build server: %s", string(out))
	}
	return bin
}

type MCPTestServer struct {
	T       *testing.T
	Session *mcp.ClientSession
	Cancel  context.CancelFunc
}

func NewMCPTestServer(t *testing.T, envOverrides ...string) *MCPTestServer {
	t.Helper()
	bin := ServerBinaryPath(t)
	cmd := exec.Command(bin)
	if len(envOverrides) > 0 {
		cmd.Env = append(os.Environ(), envOverrides...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err, "failed to connect to MCP server")
	s := &MCPTestServer{T: t, Session: session, Cancel: cancel}
	t.Cleanup(func() { s.Close() })
	return s
}

func (s *MCPTestServer) Close() {
	if s.Session != nil {
		_ = s.Session.Close()
		s.Session = nil
	}
	if s.Cancel != nil {
		s.Cancel()
	}
}

func (s *MCPTestServer) CallTool(name string, args map[string]any) *mcp.CallToolResult {
	s.T.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := s.Session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(s.T, err, "CallTool %s failed", name)
	return result
}

func (s *MCPTestServer) CallToolExpectError(name string, args map[string]any) *mcp.CallToolResult {
	s.T.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := s.Session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(s.T, err, "CallTool %s transport error", name)
	require.True(s.T, result.IsError, "expected tool error from %s, got: %s", name, ExtractText(result))
	return result
}

func (s *MCPTestServer) ListTools() []*mcp.Tool {
	s.T.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := s.Session.ListTools(ctx, nil)
	require.NoError(s.T, err)
	return result.Tools
}

func (s *MCPTestServer) PollUntilContains(t *testing.T, tool string, args map[string]any, expected string, timeout, interval time.Duration) *mcp.CallToolResult {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var result *mcp.CallToolResult
	for time.Now().Before(deadline) {
		result = s.CallTool(tool, args)
		if !result.IsError && strings.Contains(ExtractText(result), expected) {
			return result
		}
		time.Sleep(interval)
	}
	t.Fatalf("PollUntilContains(%s, %q): timed out after %v", tool, expected, timeout)
	return nil
}

func (s *MCPTestServer) PollUntilNotContains(t *testing.T, tool string, args map[string]any, unwanted string, timeout, interval time.Duration) *mcp.CallToolResult {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var result *mcp.CallToolResult
	for time.Now().Before(deadline) {
		result = s.CallTool(tool, args)
		if !result.IsError && !strings.Contains(ExtractText(result), unwanted) {
			return result
		}
		time.Sleep(interval)
	}
	t.Fatalf("PollUntilNotContains(%s, %q): timed out after %v", tool, unwanted, timeout)
	return nil
}

func ExtractText(result *mcp.CallToolResult) string {
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func ParseJSON[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	text := ExtractText(result)
	var out T
	require.NoError(t, json.Unmarshal([]byte(text), &out), "failed to parse: %s", text)
	return out
}

func StartServerExpectFail(t *testing.T, envOverrides ...string) string {
	t.Helper()
	bin := ServerBinaryPath(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Stdin = strings.NewReader("")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if len(envOverrides) > 0 {
		cmd.Env = append(os.Environ(), envOverrides...)
	}
	err := cmd.Run()
	require.Error(t, err, "server should exit with error")
	return stderr.String()
}
