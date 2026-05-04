package mcptools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/keylime/keylime-mcp/internal/keylime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRuntimePolicies(t *testing.T) {
	t.Run("returns policy names", func(t *testing.T) {
		data := loadTestdata(t, "runtime_policy_list.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/allowlists/", func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.ListRuntimePolicies(context.Background(), nil, keylime.ListRuntimePoliciesInput{})
		require.NoError(t, err)

		result := output.(keylime.ListRuntimePoliciesOutput)
		assert.Equal(t, []string{testPolicyName, "another-policy"}, result.Results.RuntimePolicyNames)
	})
}

func TestGetRuntimePolicy(t *testing.T) {
	t.Run("returns policy content", func(t *testing.T) {
		data := loadTestdata(t, "runtime_policy.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.GetRuntimePolicy(context.Background(), nil, keylime.GetRuntimePolicyInput{
			PolicyName: testPolicyName,
		})
		require.NoError(t, err)

		result := output.(keylime.GetRuntimePolicyOutput)
		assert.Equal(t, testPolicyName, result.Results.Name)
		assert.NotEmpty(t, result.Results.RuntimePolicy)
	})

	t.Run("invalid name rejected", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.GetRuntimePolicy(context.Background(), nil, keylime.GetRuntimePolicyInput{
			PolicyName: pathTraversal,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "policy_name")
	})
}

func TestImportRuntimePolicy(t *testing.T) {
	t.Run("valid file imported", func(t *testing.T) {
		var receivedBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc("POST /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &receivedBody)
			w.WriteHeader(http.StatusOK)
		})
		h := newTestHandler(t, mux)

		// readPolicyFile requires absolute path
		dir := t.TempDir()
		policyPath := filepath.Join(dir, "policy.json")
		data := loadTestdata(t, "valid_runtime_policy.json")
		require.NoError(t, os.WriteFile(policyPath, data, 0600))

		_, output, err := h.ImportRuntimePolicy(context.Background(), nil, keylime.ImportRuntimePolicyInput{
			Name:     myPolicyName,
			FilePath: policyPath,
		})
		require.NoError(t, err)

		result := output.(keylime.ImportRuntimePolicyOutput)
		assert.Equal(t, myPolicyName, result.Name)
		assert.Equal(t, "imported", result.Status)

		// check base64-encoded policy in body
		assert.NotNil(t, receivedBody["runtime_policy"])
		policyB64 := receivedBody["runtime_policy"].(string)
		decoded, err := base64.StdEncoding.DecodeString(policyB64)
		require.NoError(t, err)
		assert.True(t, json.Valid(decoded))
	})

	t.Run("invalid policy name", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.ImportRuntimePolicy(context.Background(), nil, keylime.ImportRuntimePolicyInput{
			Name:     invalidPolicyName,
			FilePath: testPolicyPath,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "policy_name")
	})

	t.Run("file not found", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.ImportRuntimePolicy(context.Background(), nil, keylime.ImportRuntimePolicyInput{
			Name:     myPolicyName,
			FilePath: "/tmp/nonexistent-policy-12345.json",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("non-json file rejected", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		dir := t.TempDir()
		path := filepath.Join(dir, "policy.json")
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0600))

		_, _, err := h.ImportRuntimePolicy(context.Background(), nil, keylime.ImportRuntimePolicyInput{
			Name:     myPolicyName,
			FilePath: path,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not valid JSON")
	})

	t.Run("relative path rejected", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.ImportRuntimePolicy(context.Background(), nil, keylime.ImportRuntimePolicyInput{
			Name:     myPolicyName,
			FilePath: "relative/policy.json",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "absolute path")
	})

	t.Run("server error propagated", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"status":"policy already exists"}`))
		})
		h := newTestHandler(t, mux)

		dir := t.TempDir()
		policyPath := filepath.Join(dir, "policy.json")
		require.NoError(t, os.WriteFile(policyPath, loadTestdata(t, "valid_runtime_policy.json"), 0600))

		_, _, err := h.ImportRuntimePolicy(context.Background(), nil, keylime.ImportRuntimePolicyInput{
			Name:     myPolicyName,
			FilePath: policyPath,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "policy already exists")
	})
}

func TestUpdateRuntimePolicy(t *testing.T) {
	// serves existing policy on GET, captures PUT body
	setupMux := func(t *testing.T, capturedBody *map[string]any) *ToolHandler {
		t.Helper()
		policyData := loadTestdata(t, "runtime_policy.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(policyData)
		})
		mux.HandleFunc("PUT /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, capturedBody)
			w.WriteHeader(http.StatusOK)
		})
		return newTestHandler(t, mux)
	}

	// decode base64 policy from PUT body
	decodePutPolicy := func(t *testing.T, putBody map[string]any) map[string]any {
		t.Helper()
		policyB64 := putBody["runtime_policy"].(string)
		decoded, err := base64.StdEncoding.DecodeString(policyB64)
		require.NoError(t, err)
		var policy map[string]any
		require.NoError(t, json.Unmarshal(decoded, &policy))
		return policy
	}

	t.Run("add excludes", func(t *testing.T) {
		var putBody map[string]any
		h := setupMux(t, &putBody)

		_, output, err := h.UpdateRuntimePolicy(context.Background(), nil, keylime.UpdateRuntimePolicyInput{
			PolicyName:  testPolicyName,
			AddExcludes: []string{"/var/log"},
		})
		require.NoError(t, err)

		result := output.(keylime.UpdateRuntimePolicyOutput)
		assert.Equal(t, "updated", result.Status)

		policy := decodePutPolicy(t, putBody)
		excludes := policy["excludes"].([]any)
		assert.Contains(t, excludes, "/var/log(/.*)?") // auto-appended suffix
		assert.Contains(t, excludes, "/tmp(/.*)?")     // original preserved
	})

	t.Run("exclude with existing suffix not doubled", func(t *testing.T) {
		var putBody map[string]any
		h := setupMux(t, &putBody)

		_, _, err := h.UpdateRuntimePolicy(context.Background(), nil, keylime.UpdateRuntimePolicyInput{
			PolicyName:  testPolicyName,
			AddExcludes: []string{"/var/log(/.*)?"},
		})
		require.NoError(t, err)

		policy := decodePutPolicy(t, putBody)
		excludes := policy["excludes"].([]any)
		// suffix not doubled
		assert.Contains(t, excludes, "/var/log(/.*)?")
		assert.NotContains(t, excludes, "/var/log(/.*)?(/.*)?")
	})

	t.Run("remove excludes", func(t *testing.T) {
		var putBody map[string]any
		h := setupMux(t, &putBody)

		_, _, err := h.UpdateRuntimePolicy(context.Background(), nil, keylime.UpdateRuntimePolicyInput{
			PolicyName:     testPolicyName,
			RemoveExcludes: []string{"/tmp(/.*)?"},
		})
		require.NoError(t, err)

		policy := decodePutPolicy(t, putBody)
		excludes, _ := policy["excludes"].([]any)
		assert.NotContains(t, excludes, "/tmp(/.*)?")
	})

	t.Run("add digests", func(t *testing.T) {
		var putBody map[string]any
		h := setupMux(t, &putBody)

		digest := "ab0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550"

		_, _, err := h.UpdateRuntimePolicy(context.Background(), nil, keylime.UpdateRuntimePolicyInput{
			PolicyName: testPolicyName,
			AddDigests: map[string]string{"/usr/bin/new": digest},
		})
		require.NoError(t, err)

		policy := decodePutPolicy(t, putBody)
		digests := policy["digests"].(map[string]any)
		assert.Contains(t, digests, "/usr/bin/new")
		assert.Contains(t, digests, testBinBash) // original preserved
	})

	t.Run("remove digests", func(t *testing.T) {
		var putBody map[string]any
		h := setupMux(t, &putBody)

		_, _, err := h.UpdateRuntimePolicy(context.Background(), nil, keylime.UpdateRuntimePolicyInput{
			PolicyName:    testPolicyName,
			RemoveDigests: []string{testBinBash},
		})
		require.NoError(t, err)

		policy := decodePutPolicy(t, putBody)
		digests := policy["digests"].(map[string]any)
		assert.NotContains(t, digests, testBinBash)
	})

	t.Run("timestamp updated", func(t *testing.T) {
		var putBody map[string]any
		h := setupMux(t, &putBody)

		_, _, err := h.UpdateRuntimePolicy(context.Background(), nil, keylime.UpdateRuntimePolicyInput{
			PolicyName:  testPolicyName,
			AddExcludes: []string{testVarPath},
		})
		require.NoError(t, err)

		policy := decodePutPolicy(t, putBody)
		meta := policy["meta"].(map[string]any)
		ts, ok := meta["timestamp"].(string)
		require.True(t, ok, "timestamp must be a string")
		assert.NotEqual(t, "2024-01-01T00:00:00Z", ts)
	})

	t.Run("no operations returns error", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.UpdateRuntimePolicy(context.Background(), nil, keylime.UpdateRuntimePolicyInput{
			PolicyName: testPolicyName,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one of")
	})

	t.Run("invalid policy name", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.UpdateRuntimePolicy(context.Background(), nil, keylime.UpdateRuntimePolicyInput{
			PolicyName:  invalidPolicyName,
			AddExcludes: []string{testVarPath},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "policy_name")
	})

	t.Run("policy not found", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"status":"policy not found"}`))
		})
		h := newTestHandler(t, mux)

		_, _, err := h.UpdateRuntimePolicy(context.Background(), nil, keylime.UpdateRuntimePolicyInput{
			PolicyName:  "nonexistent",
			AddExcludes: []string{testVarPath},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})
}

func TestDeleteRuntimePolicy(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.DeleteRuntimePolicy(context.Background(), nil, keylime.DeleteRuntimePolicyInput{
			PolicyName: testPolicyName,
		})
		require.NoError(t, err)

		result := output.(keylime.DeletePolicyOutput)
		assert.Equal(t, testPolicyName, result.PolicyName)
		assert.Equal(t, "deleted", result.Status)
	})

	t.Run(invalidPolicyName, func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.DeleteRuntimePolicy(context.Background(), nil, keylime.DeleteRuntimePolicyInput{
			PolicyName: pathTraversal,
		})
		assert.Error(t, err)
	})
}

func TestListMBPolicies(t *testing.T) {
	t.Run("returns policy names", func(t *testing.T) {
		data := loadTestdata(t, "mb_policy_list.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/mbpolicies/", func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.ListMBPolicies(context.Background(), nil, keylime.ListMBPoliciesInput{})
		require.NoError(t, err)

		result := output.(keylime.ListMBPoliciesOutput)
		assert.Equal(t, []string{testMBPolicyName}, result.Results.MBPolicyNames)
	})
}

func TestGetMBPolicy(t *testing.T) {
	t.Run("returns policy content", func(t *testing.T) {
		data := loadTestdata(t, "mb_policy.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/mbpolicies/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.GetMBPolicy(context.Background(), nil, keylime.GetMBPolicyInput{
			PolicyName: testMBPolicyName,
		})
		require.NoError(t, err)

		result := output.(keylime.GetMBPolicyOutput)
		assert.Equal(t, 200, result.Code)
		assert.Equal(t, testMBPolicyName, result.Results["name"])
	})

	t.Run(invalidPolicyName, func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.GetMBPolicy(context.Background(), nil, keylime.GetMBPolicyInput{
			PolicyName: pathTraversal,
		})
		assert.Error(t, err)
	})
}

func TestImportMBPolicy(t *testing.T) {
	t.Run("valid file imported", func(t *testing.T) {
		var receivedBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc("POST /v2.5/mbpolicies/{name}", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &receivedBody)
			w.WriteHeader(http.StatusOK)
		})
		h := newTestHandler(t, mux)

		dir := t.TempDir()
		policyPath := filepath.Join(dir, "mb_policy.json")
		require.NoError(t, os.WriteFile(policyPath, loadTestdata(t, "valid_mb_policy.json"), 0600))

		_, output, err := h.ImportMBPolicy(context.Background(), nil, keylime.ImportMBPolicyInput{
			Name:     "my-mb-policy",
			FilePath: policyPath,
		})
		require.NoError(t, err)

		result := output.(keylime.ImportMBPolicyOutput)
		assert.Equal(t, "my-mb-policy", result.Name)
		assert.Equal(t, "imported", result.Status)

		// MB policy sent as raw string, not base64
		mbPolicy, ok := receivedBody["mb_policy"].(string)
		require.True(t, ok, "mb_policy must be a string")
		assert.True(t, json.Valid([]byte(mbPolicy)), "mb_policy must be valid JSON")
	})

	t.Run(invalidPolicyName, func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.ImportMBPolicy(context.Background(), nil, keylime.ImportMBPolicyInput{
			Name:     invalidPolicyName,
			FilePath: testPolicyPath,
		})
		assert.Error(t, err)
	})

	t.Run("file not found", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.ImportMBPolicy(context.Background(), nil, keylime.ImportMBPolicyInput{
			Name:     "my-mb-policy",
			FilePath: "/tmp/nonexistent-mb-policy-12345.json",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})
}

func TestDeleteMBPolicy(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /v2.5/mbpolicies/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.DeleteMBPolicy(context.Background(), nil, keylime.DeleteMBPolicyInput{
			PolicyName: testMBPolicyName,
		})
		require.NoError(t, err)

		result := output.(keylime.DeletePolicyOutput)
		assert.Equal(t, testMBPolicyName, result.PolicyName)
		assert.Equal(t, "deleted", result.Status)
	})

	t.Run(invalidPolicyName, func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.DeleteMBPolicy(context.Background(), nil, keylime.DeleteMBPolicyInput{
			PolicyName: pathTraversal,
		})
		assert.Error(t, err)
	})
}
