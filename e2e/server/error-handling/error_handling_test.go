//go:build functional

package errorhandling_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/keylime/keylime-mcp/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fakeUUID = "00000000-0000-0000-0000-000000000000"

func TestErrorHandling(t *testing.T) {
	s := helpers.NewMCPTestServer(t, "MASKING_ENABLED=false")

	t.Run("invalid_uuid_format", func(t *testing.T) {
		s.CallToolExpectError("Get_agent_status", map[string]any{"agent_uuid": "not-a-uuid"})
	})

	t.Run("non_existent_agent", func(t *testing.T) {
		s.CallToolExpectError("Get_agent_status", map[string]any{"agent_uuid": fakeUUID})
	})

	t.Run("enroll_non_existent_agent", func(t *testing.T) {
		s.CallToolExpectError("Enroll_agent_to_verifier", map[string]any{
			"agent_uuid":          fakeUUID,
			"runtime_policy_name": "",
			"mb_policy_name":      "",
		})
	})

	t.Run("unenroll_non_enrolled_agent", func(t *testing.T) {
		s.CallToolExpectError("Unenroll_agent_from_verifier", map[string]any{"agent_uuid": fakeUUID})
	})

	t.Run("get_non_existent_policy", func(t *testing.T) {
		s.CallToolExpectError("Get_runtime_policy", map[string]any{"policy_name": "nonexistent-policy"})
	})

	t.Run("delete_non_existent_policy", func(t *testing.T) {
		s.CallToolExpectError("Delete_runtime_policy", map[string]any{"policy_name": "nonexistent-policy"})
	})

	t.Run("verifier_logs_attestation_failures", func(t *testing.T) {
		result := s.CallTool("Get_verifier_logs", map[string]any{
			"filter":     "attestation_failures",
			"agent_uuid": "",
			"lines":      10,
		})
		require.False(t, result.IsError)
		assert.NotEmpty(t, helpers.ExtractText(result))
	})

	t.Run("verifier_logs_errors", func(t *testing.T) {
		result := s.CallTool("Get_verifier_logs", map[string]any{
			"filter":     "errors",
			"agent_uuid": "",
			"lines":      10,
		})
		require.False(t, result.IsError)
		assert.NotEmpty(t, helpers.ExtractText(result))
	})

	t.Run("verifier_logs_all", func(t *testing.T) {
		result := s.CallTool("Get_verifier_logs", map[string]any{
			"filter":     "all",
			"agent_uuid": "",
			"lines":      10,
		})
		require.False(t, result.IsError)
		assert.NotEmpty(t, helpers.ExtractText(result))
	})

	t.Run("partial_service_failure", func(t *testing.T) {
		err := exec.Command("systemctl", "stop", "keylime_verifier").Run()
		require.NoError(t, err, "failed to stop keylime_verifier")
		defer func() {
			if err := exec.Command("systemctl", "start", "keylime_verifier").Run(); err != nil {
				t.Logf("WARNING: failed to restart keylime_verifier: %v", err)
			}
			time.Sleep(5 * time.Second)
		}()
		deadline := time.Now().Add(10 * time.Second)
		var text string
		for time.Now().Before(deadline) {
			result := s.CallTool("Get_version_and_health", map[string]any{})
			if result.IsError {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			text = helpers.ExtractText(result)
			if strings.Contains(text, `"reachable":false`) {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		assert.Contains(t, text, `"reachable":false`)
		assert.Contains(t, text, `"reachable":true`)
	})
}
