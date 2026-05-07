//go:build functional

package basicattestation_test

import (
	"testing"

	"github.com/keylime/keylime-mcp/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicAttestation(t *testing.T) {
	s := helpers.NewMCPTestServer(t, "MASKING_ENABLED=false")

	t.Run("tools_list", func(t *testing.T) {
		tools := s.ListTools()
		names := make([]string, len(tools))
		for i, tool := range tools {
			names[i] = tool.Name
		}
		assert.Contains(t, names, "Get_version_and_health")
		assert.Contains(t, names, "Get_all_agents")
		assert.Contains(t, names, "Get_agent_status")
		assert.Contains(t, names, "Enroll_agent_to_verifier")
		assert.Contains(t, names, "List_runtime_policies")
		assert.Contains(t, names, "Get_verifier_logs")
	})

	t.Run("Get_version_and_health", func(t *testing.T) {
		result := s.CallTool("Get_version_and_health", map[string]any{})
		require.False(t, result.IsError)
		text := helpers.ExtractText(result)
		assert.Contains(t, text, "verifier")
		assert.Contains(t, text, "registrar")
	})

	t.Run("Get_all_agents", func(t *testing.T) {
		result := s.CallTool("Get_all_agents", map[string]any{})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), helpers.AgentID)
	})

	t.Run("Get_agent_details", func(t *testing.T) {
		result := s.CallTool("Get_agent_details", map[string]any{"agent_uuid": helpers.AgentID})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "aik_tpm")
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
		result := s.CallTool("Get_agent_status", map[string]any{"agent_uuid": helpers.AgentID})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), helpers.AgentID)
	})

	t.Run("Get_verifier_enrolled_agents", func(t *testing.T) {
		result := s.CallTool("Get_verifier_enrolled_agents", map[string]any{})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), helpers.AgentID)
	})

	t.Run("List_runtime_policies", func(t *testing.T) {
		result := s.CallTool("List_runtime_policies", map[string]any{})
		require.False(t, result.IsError)
	})

	t.Run("Unenroll_agent_from_verifier", func(t *testing.T) {
		result := s.CallTool("Unenroll_agent_from_verifier", map[string]any{"agent_uuid": helpers.AgentID})
		require.False(t, result.IsError)
	})
}
