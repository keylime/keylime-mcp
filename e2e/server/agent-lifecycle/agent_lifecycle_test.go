//go:build functional

package agentlifecycle_test

import (
	"testing"
	"time"

	"github.com/keylime/keylime-mcp/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const maskedAgentID = "AGENT-1"

func TestAgentLifecycle(t *testing.T) {
	s := helpers.NewMCPTestServer(t)

	t.Run("Get_all_agents", func(t *testing.T) {
		result := s.CallTool("Get_all_agents", map[string]any{})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), maskedAgentID)
	})

	t.Run("Get_agent_details", func(t *testing.T) {
		result := s.CallTool("Get_agent_details", map[string]any{"agent_uuid": helpers.AgentID})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "aik_tpm")
	})

	t.Run("Get_verifier_enrolled_agents_before", func(t *testing.T) {
		result := s.CallTool("Get_verifier_enrolled_agents", map[string]any{})
		require.False(t, result.IsError)
		assert.NotContains(t, helpers.ExtractText(result), maskedAgentID)
	})

	if !t.Run("Enroll_agent_to_verifier", func(t *testing.T) {
		result := s.CallTool("Enroll_agent_to_verifier", map[string]any{
			"agent_uuid":          helpers.AgentID,
			"runtime_policy_name": "",
			"mb_policy_name":      "",
		})
		require.False(t, result.IsError)
	}) {
		t.Fatal("prerequisite failed: Enroll_agent_to_verifier")
	}

	t.Run("Get_agent_status_after_enrollment", func(t *testing.T) {
		result := s.PollUntilContains(t, "Get_agent_status",
			map[string]any{"agent_uuid": helpers.AgentID},
			"operational_state", 15*time.Second, 1*time.Second)
		text := helpers.ExtractText(result)
		assert.Contains(t, text, maskedAgentID)
	})

	t.Run("Get_verifier_enrolled_agents_after", func(t *testing.T) {
		result := s.CallTool("Get_verifier_enrolled_agents", map[string]any{})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), maskedAgentID)
	})

	t.Run("Get_agent_policies", func(t *testing.T) {
		result := s.CallTool("Get_agent_policies", map[string]any{"agent_uuid": helpers.AgentID})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "tpm_policy")
	})

	t.Run("Get_failed_agents", func(t *testing.T) {
		result := s.CallTool("Get_failed_agents", map[string]any{})
		require.False(t, result.IsError)
		assert.NotContains(t, helpers.ExtractText(result), maskedAgentID)
	})

	if !t.Run("Stop_agent", func(t *testing.T) {
		result := s.CallTool("Stop_agent", map[string]any{"agent_uuid": helpers.AgentID})
		require.False(t, result.IsError)
	}) {
		t.Fatal("prerequisite failed: Stop_agent")
	}

	t.Run("Reactivate_agent", func(t *testing.T) {
		result := s.CallTool("Reactivate_agent", map[string]any{"agent_uuid": helpers.AgentID})
		require.False(t, result.IsError)
	})

	t.Run("Update_agent", func(t *testing.T) {
		result := s.CallTool("Update_agent", map[string]any{
			"agent_uuid":          helpers.AgentID,
			"runtime_policy_name": "",
			"mb_policy_name":      "",
		})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "updated")
	})

	t.Run("Get_agent_status_after_update", func(t *testing.T) {
		result := s.PollUntilContains(t, "Get_agent_status",
			map[string]any{"agent_uuid": helpers.AgentID},
			maskedAgentID, 15*time.Second, 1*time.Second)
		assert.Contains(t, helpers.ExtractText(result), maskedAgentID)
	})

	if !t.Run("Unenroll_agent_from_verifier", func(t *testing.T) {
		result := s.CallTool("Unenroll_agent_from_verifier", map[string]any{"agent_uuid": helpers.AgentID})
		require.False(t, result.IsError)
	}) {
		t.Fatal("prerequisite failed: Unenroll_agent_from_verifier")
	}

	t.Run("Get_verifier_enrolled_agents_after_unenroll", func(t *testing.T) {
		s.PollUntilNotContains(t, "Get_verifier_enrolled_agents",
			map[string]any{}, maskedAgentID, 15*time.Second, 1*time.Second)
	})

	t.Run("Get_all_agents_still_in_registrar", func(t *testing.T) {
		result := s.CallTool("Get_all_agents", map[string]any{})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), maskedAgentID)
	})

	t.Run("Registrar_remove_agent", func(t *testing.T) {
		result := s.CallTool("Registrar_remove_agent", map[string]any{"agent_uuid": helpers.AgentID})
		require.False(t, result.IsError)
	})

	t.Run("Get_all_agents_after_removal", func(t *testing.T) {
		result := s.CallTool("Get_all_agents", map[string]any{})
		require.False(t, result.IsError)
		assert.NotContains(t, helpers.ExtractText(result), maskedAgentID)
	})
}
