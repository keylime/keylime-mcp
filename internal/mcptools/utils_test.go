package mcptools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keylime/keylime-mcp/internal/keylime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAgentUUID(t *testing.T) {
	tests := []struct {
		name    string
		uuid    string
		wantErr string
	}{
		{"valid lowercase", "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", ""},
		{"valid uppercase", "D432FBB3-D2F1-4A97-9EF7-75BD81C00000", ""},
		{"valid mixed case", "d432FBB3-d2f1-4A97-9ef7-75bd81C00000", ""},
		{"empty", "", "agent_uuid is required"},
		{"too short", "d432fbb3-d2f1-4a97-9ef7", "agent_uuid must be a valid UUID"},
		{"no dashes", "d432fbb3d2f14a979ef775bd81c00000", "agent_uuid must be a valid UUID"},
		{"extra chars", "d432fbb3-d2f1-4a97-9ef7-75bd81c00000x", "agent_uuid must be a valid UUID"},
		{"non-hex chars", "g432fbb3-d2f1-4a97-9ef7-75bd81c00000", "agent_uuid must be a valid UUID"},
		{"spaces", "d432fbb3 d2f1 4a97 9ef7 75bd81c00000", "agent_uuid must be a valid UUID"},
		{"just text", "not-a-uuid", "agent_uuid must be a valid UUID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentUUID(tt.uuid)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePolicyName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"valid simple", "my-policy", ""},
		{"valid with dots", "policy_v2.1", ""},
		{"valid with underscores", "my_policy_name", ""},
		{"valid single char", "a", ""},
		{"empty", "", "policy_name is required"},
		{"too long", strings.Repeat("a", 256), "policy_name exceeds 255 characters"},
		{"max length OK", strings.Repeat("a", 255), ""},
		{"path traversal", "../evil", "policy_name contains invalid characters (use alphanumeric, hyphens, underscores, dots)"},
		{"semicolon", "name;rm", "policy_name contains invalid characters (use alphanumeric, hyphens, underscores, dots)"},
		{"spaces", "my policy", "policy_name contains invalid characters (use alphanumeric, hyphens, underscores, dots)"},
		{"slash", "path/name", "policy_name contains invalid characters (use alphanumeric, hyphens, underscores, dots)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePolicyName(tt.input)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{"valid absolute json", "/tmp/policy.json", ""},
		{"valid nested path", "/var/lib/keylime/policy.json", ""},
		{"empty", "", "file_path is required"},
		{"relative path", "policy.json", "file_path must be an absolute path"},
		{"path traversal", "/tmp/../etc/policy.json", "file_path must not contain path traversal"},
		{"non-json extension", "/tmp/policy.txt", "file_path must have .json extension"},
		{"no extension", "/tmp/policy", "file_path must have .json extension"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.path)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeDigest(t *testing.T) {
	validSHA256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	tests := []struct {
		name    string
		digest  string
		path    string
		want    string
		wantErr bool
	}{
		{"valid sha256", validSHA256, "/bin/bash", validSHA256, false},
		{"with sha256 prefix", "sha256:" + validSHA256, "/bin/bash", validSHA256, false},
		{"valid sha1 length (40 chars)", strings.Repeat("a", 40), "/bin/test", strings.Repeat("a", 40), false},
		{"max length 128", strings.Repeat("a", 128), "/bin/test", strings.Repeat("a", 128), false},
		{"too short", "abcdef1234", "/bin/bash", "", true},
		{"too long", strings.Repeat("a", 129), "/bin/bash", "", true},
		{"uppercase rejected", "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855", "/bin/bash", "", true},
		{"non-hex chars", "g3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "/bin/bash", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeDigest(tt.digest, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.path)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFilterLogLines(t *testing.T) {
	tests := []struct {
		name     string
		logs     string
		keywords []string
		want     string
	}{
		{
			"matches keywords",
			"line1 ERROR something\nline2 ok\nline3 FAIL here",
			[]string{"ERROR", "FAIL"},
			"line1 ERROR something\nline3 FAIL here",
		},
		{
			"no matches",
			"line1 ok\nline2 fine",
			[]string{"ERROR"},
			"",
		},
		{
			"empty input",
			"",
			[]string{"ERROR"},
			"",
		},
		{
			"no keywords returns nothing",
			"line1 ERROR\nline2 ok",
			[]string{},
			"",
		},
		{
			"case sensitive",
			"line1 error\nline2 ERROR",
			[]string{"ERROR"},
			"line2 ERROR",
		},
		{
			"line matches multiple keywords counted once",
			"ERROR and FAIL on same line",
			[]string{"ERROR", "FAIL"},
			"ERROR and FAIL on same line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, filterLogLines(tt.logs, tt.keywords))
		})
	}
}

func TestParseJSONStr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  any
	}{
		{"empty string", "", map[string]any{}},
		{"valid object", `{"key":"value"}`, map[string]any{"key": "value"}},
		{"valid array", `[1,2,3]`, []any{float64(1), float64(2), float64(3)}},
		{"invalid json returns raw string", "not json", "not json"},
		{"valid number", "42", float64(42)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseJSONStr(tt.input))
		})
	}
}

func TestNonNilSlice(t *testing.T) {
	t.Run("nil returns empty slice", func(t *testing.T) {
		result := nonNilSlice(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("existing slice returned as-is", func(t *testing.T) {
		input := []string{"a", "b"}
		assert.Equal(t, input, nonNilSlice(input))
	})

	t.Run("empty non-nil slice returned as-is", func(t *testing.T) {
		input := []string{}
		assert.Equal(t, input, nonNilSlice(input))
	})
}

func TestMapAgentToOutput(t *testing.T) {
	t.Run("full field mapping", func(t *testing.T) {
		agentUUID := "d432fbb3-d2f1-4a97-9ef7-75bd81c00000"
		severity := 5
		eventID := "event-123"
		lastQuote := 1700000000
		lastAttestation := 1700000000

		var status keylime.AgentStatusResponse
		status.Code = 200
		status.Status = "Success"
		status.Results.OperationalState = keylime.StateGetQuote
		status.Results.AttestationCount = 42
		status.Results.LastReceivedQuote = &lastQuote
		status.Results.LastSuccessfulAttestation = &lastAttestation
		status.Results.SeverityLevel = &severity
		status.Results.LastEventID = &eventID
		status.Results.HashAlg = "sha256"
		status.Results.EncAlg = "rsa"
		status.Results.SignAlg = "rsassa"
		status.Results.VerifierID = "default"
		status.Results.VerifierIP = "127.0.0.1"
		status.Results.VerifierPort = 8881
		status.Results.HasMbRefstate = 1
		status.Results.HasRuntimePolicy = 0

		output := mapAgentToOutput(agentUUID, status)

		assert.Equal(t, agentUUID, output.AgentUUID)
		assert.Equal(t, keylime.StateGetQuote, output.OperationalState)
		assert.Equal(t, "Get Quote", output.OperationalStateDescription)
		assert.Equal(t, 42, output.AttestationCount)
		assert.Equal(t, &lastQuote, output.LastReceivedQuote)
		assert.Equal(t, &lastAttestation, output.LastSuccessfulAttestation)
		assert.Equal(t, &severity, output.SeverityLevel)
		assert.Equal(t, &eventID, output.LastEventID)
		assert.Equal(t, "sha256", output.HashAlgorithm)
		assert.Equal(t, "rsa", output.EncryptionAlgorithm)
		assert.Equal(t, "rsassa", output.SigningAlgorithm)
		assert.Equal(t, "default", output.VerifierID)
		assert.Equal(t, "127.0.0.1:8881", output.VerifierAddress)
		assert.True(t, output.HasMeasuredBoot)
		assert.False(t, output.HasRuntimePolicy)
	})

	t.Run("boolean conversion", func(t *testing.T) {
		tests := []struct {
			name             string
			hasMbRefstate    int
			hasRuntimePolicy int
			wantMB           bool
			wantRuntime      bool
		}{
			{"both zero", 0, 0, false, false},
			{"both one", 1, 1, true, true},
			{"mb only", 1, 0, true, false},
			{"runtime only", 0, 1, false, true},
			{"non-zero counts as true", 5, 3, true, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var status keylime.AgentStatusResponse
				status.Results.HasMbRefstate = tt.hasMbRefstate
				status.Results.HasRuntimePolicy = tt.hasRuntimePolicy

				output := mapAgentToOutput("test-uuid", status)
				assert.Equal(t, tt.wantMB, output.HasMeasuredBoot)
				assert.Equal(t, tt.wantRuntime, output.HasRuntimePolicy)
			})
		}
	})
}

func TestFetchAndDecode(t *testing.T) {
	type testResult struct {
		Code   int    `json:"code"`
		Status string `json:"status"`
	}

	t.Run("success", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"code":200,"status":"Success"}`)),
		}
		result, err := fetchAndDecode[testResult](resp, nil)
		require.NoError(t, err)
		assert.Equal(t, 200, result.Code)
		assert.Equal(t, "Success", result.Status)
	})

	t.Run("http error passed through", func(t *testing.T) {
		_, err := fetchAndDecode[testResult](nil, fmt.Errorf("connection refused"))
		assert.ErrorContains(t, err, "connection refused")
	})

	t.Run("non-2xx returns API error", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(strings.NewReader(`{"status":"not found"}`)),
		}
		_, err := fetchAndDecode[testResult](resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("invalid json returns decode error", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("not json")),
		}
		_, err := fetchAndDecode[testResult](resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode")
	})
}

func TestDeleteAndCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("")),
		}
		assert.NoError(t, deleteAndCheck(resp, nil))
	})

	t.Run("http error", func(t *testing.T) {
		err := deleteAndCheck(nil, fmt.Errorf("connection refused"))
		assert.ErrorContains(t, err, "connection refused")
	})

	t.Run("non-2xx returns API error", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(strings.NewReader(`{"status":"not found"}`)),
		}
		err := deleteAndCheck(resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})
}

func TestReadPolicyFile(t *testing.T) {
	t.Run("valid json file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "policy.json")
		content := `{"key":"value"}`
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))

		data, err := readPolicyFile(path)
		require.NoError(t, err)
		assert.True(t, json.Valid(data))
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := readPolicyFile("/tmp/nonexistent-policy-12345.json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.json")
		require.NoError(t, os.WriteFile(path, []byte(""), 0600))

		_, err := readPolicyFile(path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file is empty")
	})

	t.Run("invalid json content", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.json")
		require.NoError(t, os.WriteFile(path, []byte("not json content"), 0600))

		_, err := readPolicyFile(path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not valid JSON")
	})

	t.Run("relative path rejected", func(t *testing.T) {
		_, err := readPolicyFile("relative/policy.json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "absolute path")
	})

	t.Run("directory instead of file", func(t *testing.T) {
		dir := t.TempDir()
		jsonDir := filepath.Join(dir, "fake.json")
		require.NoError(t, os.Mkdir(jsonDir, 0750))

		_, err := readPolicyFile(jsonDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory")
	})

	t.Run("non-json extension rejected", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "policy.txt")
		require.NoError(t, os.WriteFile(path, []byte(`{"key":"value"}`), 0600))

		_, err := readPolicyFile(path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ".json extension")
	})
}
