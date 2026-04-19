package mcptools

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/keylime/keylime-mcp/internal/keylime"
	"github.com/stretchr/testify/require"
)

const testAPIVersion = "v2.5"

func loadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name) // #nosec G304 -- test helper, name is hardcoded in tests
	require.NoError(t, err, "failed to load testdata/%s", name)
	return data
}

func newTestHandler(t *testing.T, handler http.Handler) *ToolHandler {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	svc, err := keylime.NewService(&keylime.Config{
		VerifierURL:  ts.URL,
		RegistrarURL: ts.URL,
		TLSEnabled:   false,
		APIVersion:   testAPIVersion,
	})
	require.NoError(t, err)
	return NewToolHandler(svc)
}
