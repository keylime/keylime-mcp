//go:build functional

package policymanagement_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/keylime/keylime-mcp/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	policyName   = "test-mcp-policy"
	mbPolicyName = "test-mcp-mb-policy"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata", name)
}

func TestPolicyManagement(t *testing.T) {
	s := helpers.NewMCPTestServer(t, "MASKING_ENABLED=false")

	t.Run("List_runtime_policies_initial", func(t *testing.T) {
		result := s.CallTool("List_runtime_policies", map[string]any{})
		require.False(t, result.IsError)
	})

	t.Run("Import_runtime_policy", func(t *testing.T) {
		result := s.CallTool("Import_runtime_policy", map[string]any{
			"name":      policyName,
			"file_path": testdataPath("test_runtime_policy.json"),
		})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "imported")
	})

	t.Run("List_runtime_policies_after_import", func(t *testing.T) {
		result := s.CallTool("List_runtime_policies", map[string]any{})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), policyName)
	})

	t.Run("Get_runtime_policy", func(t *testing.T) {
		result := s.CallTool("Get_runtime_policy", map[string]any{"policy_name": policyName})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), policyName)
	})

	t.Run("Update_runtime_policy", func(t *testing.T) {
		result := s.CallTool("Update_runtime_policy", map[string]any{
			"policy_name":    policyName,
			"add_excludes":   []string{"/var/log/test"},
			"remove_excludes": []string{},
			"add_digests":    map[string]string{},
			"remove_digests": []string{},
		})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "updated")
	})

	t.Run("Get_runtime_policy_after_update", func(t *testing.T) {
		result := s.CallTool("Get_runtime_policy", map[string]any{"policy_name": policyName})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "var/log/test")
	})

	t.Run("Delete_runtime_policy", func(t *testing.T) {
		result := s.CallTool("Delete_runtime_policy", map[string]any{"policy_name": policyName})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "deleted")
	})

	t.Run("List_runtime_policies_after_delete", func(t *testing.T) {
		result := s.CallTool("List_runtime_policies", map[string]any{})
		require.False(t, result.IsError)
		assert.NotContains(t, helpers.ExtractText(result), policyName)
	})

	t.Run("List_mb_policies_initial", func(t *testing.T) {
		result := s.CallTool("List_mb_policies", map[string]any{})
		require.False(t, result.IsError)
	})

	t.Run("Import_mb_policy", func(t *testing.T) {
		result := s.CallTool("Import_mb_policy", map[string]any{
			"name":      mbPolicyName,
			"file_path": testdataPath("test_mb_policy.json"),
		})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "imported")
	})

	t.Run("Get_mb_policy", func(t *testing.T) {
		result := s.CallTool("Get_mb_policy", map[string]any{"policy_name": mbPolicyName})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), mbPolicyName)
	})

	t.Run("Delete_mb_policy", func(t *testing.T) {
		result := s.CallTool("Delete_mb_policy", map[string]any{"policy_name": mbPolicyName})
		require.False(t, result.IsError)
		assert.Contains(t, helpers.ExtractText(result), "deleted")
	})
}
