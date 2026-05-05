//go:build functional

package errorhandling_test

import (
	"os/exec"
	"testing"
	"time"

	"github.com/keylime/keylime-mcp/functional-tests/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fakeUUID = "00000000-0000-0000-0000-000000000000"

func TestErrorHandling(t *testing.T) {
	s := testhelpers.NewMCPTestServer(t, "MASKING_ENABLED=false")

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
	})

	t.Run("verifier_logs_errors", func(t *testing.T) {
		result := s.CallTool("Get_verifier_logs", map[string]any{
			"filter":     "errors",
			"agent_uuid": "",
			"lines":      10,
		})
		require.False(t, result.IsError)
	})

	t.Run("verifier_logs_all", func(t *testing.T) {
		result := s.CallTool("Get_verifier_logs", map[string]any{
			"filter":     "all",
			"agent_uuid": "",
			"lines":      10,
		})
		require.False(t, result.IsError)
	})

	t.Run("partial_service_failure", func(t *testing.T) {
		_ = exec.Command("systemctl", "stop", "keylime_verifier").Run()
		defer func() {
			_ = exec.Command("systemctl", "start", "keylime_verifier").Run()
			time.Sleep(5 * time.Second)
		}()
		time.Sleep(2 * time.Second)
		result := s.CallTool("Get_version_and_health", map[string]any{})
		require.False(t, result.IsError)
		text := testhelpers.ExtractText(result)
		assert.Contains(t, text, `"reachable":false`)
		assert.Contains(t, text, `"reachable":true`)
	})
}
