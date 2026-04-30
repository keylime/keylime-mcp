//go:build functional

package tlsvalidation_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keylime/keylime-mcp/functional-tests/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSValidation(t *testing.T) {
	t.Run("valid_mTLS", func(t *testing.T) {
		s := testhelpers.NewMCPTestServer(t)
		result := s.CallTool("Get_version_and_health", map[string]any{})
		require.False(t, result.IsError)
		text := testhelpers.ExtractText(result)
		assert.Contains(t, text, "verifier")
		assert.Contains(t, text, "registrar")
	})

	t.Run("wrong_CA", func(t *testing.T) {
		tmpDir := t.TempDir()
		wrongCAPath := filepath.Join(tmpDir, "wrong-ca.crt")
		wrongKeyPath := filepath.Join(tmpDir, "wrong-ca.key")

		cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048",
			"-keyout", wrongKeyPath, "-out", wrongCAPath,
			"-days", "1", "-nodes", "-subj", "/CN=Wrong-CA")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "failed to generate wrong CA cert: %s", string(out))

		s := testhelpers.NewMCPTestServer(t, "KEYLIME_CA_CERT="+wrongCAPath)
		result := s.CallTool("Get_version_and_health", map[string]any{})
		text := testhelpers.ExtractText(result)
		assert.Contains(t, text, `"reachable":false`)
	})

	t.Run("missing_client_cert", func(t *testing.T) {
		stderr := testhelpers.StartServerExpectFail(t, "KEYLIME_CLIENT_CERT=/nonexistent/cert.pem")
		assert.True(t, strings.Contains(strings.ToLower(stderr), "certificate"),
			"expected 'certificate' in stderr, got: %s", stderr)
	})

	t.Run("SNI_mismatch", func(t *testing.T) {
		s := testhelpers.NewMCPTestServer(t, "KEYLIME_TLS_SERVER_NAME=wrong-hostname.example.com")
		result := s.CallTool("Get_version_and_health", map[string]any{})
		text := testhelpers.ExtractText(result)
		assert.Contains(t, text, `"reachable":false`)
	})

	t.Run("TLS_disabled", func(t *testing.T) {
		s := testhelpers.NewMCPTestServer(t, "KEYLIME_TLS_ENABLED=false")
		result := s.CallTool("Get_version_and_health", map[string]any{})
		require.False(t, result.IsError)
	})
}
