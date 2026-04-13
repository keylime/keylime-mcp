package mcptools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/keylime/keylime-mcp/internal/keylime"
)

// extractAPIError reads the response body and returns a descriptive error.
func extractAPIError(resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(resp.Body)
	var apiErr struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(bodyBytes, &apiErr); err == nil && apiErr.Status != "" {
		return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, apiErr.Status)
	}
	return fmt.Errorf("API request failed with HTTP %d: %s", resp.StatusCode, string(bodyBytes))
}

// fetchAndDecode reads an HTTP response body and decodes it into a typed struct.
func fetchAndDecode[T any](resp *http.Response, err error) (T, error) {
	var zero T
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, extractAPIError(resp)
	}
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// deleteAndCheck verifies a DELETE request returned a success code.
func deleteAndCheck(resp *http.Response, err error) error {
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return extractAPIError(resp)
	}
	return nil
}

var (
	uuidRE     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	safeNameRE = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	digestRE   = regexp.MustCompile(`^[0-9a-f]{40,128}$`)
)

func validateAgentUUID(uuid string) error {
	if uuid == "" {
		return fmt.Errorf("agent_uuid is required")
	}
	if !uuidRE.MatchString(uuid) {
		return fmt.Errorf("agent_uuid must be a valid UUID")
	}
	return nil
}

func validatePolicyName(name string) error {
	if name == "" {
		return fmt.Errorf("policy_name is required")
	}
	if len(name) > 255 {
		return fmt.Errorf("policy_name exceeds 255 characters")
	}
	if !safeNameRE.MatchString(name) {
		return fmt.Errorf("policy_name contains invalid characters (use alphanumeric, hyphens, underscores, dots)")
	}
	return nil
}

func mapAgentToOutput(agentUUID string, agentStatus keylime.AgentStatusResponse) keylime.GetAgentStatusOutput {
	return keylime.GetAgentStatusOutput{
		AgentUUID:                   agentUUID,
		OperationalState:            agentStatus.Results.OperationalState,
		OperationalStateDescription: keylime.StateToString(agentStatus.Results.OperationalState),
		AttestationCount:            agentStatus.Results.AttestationCount,
		LastReceivedQuote:           agentStatus.Results.LastReceivedQuote,
		LastSuccessfulAttestation:   agentStatus.Results.LastSuccessfulAttestation,
		SeverityLevel:               agentStatus.Results.SeverityLevel,
		LastEventID:                 agentStatus.Results.LastEventID,
		HashAlgorithm:               agentStatus.Results.HashAlg,
		EncryptionAlgorithm:         agentStatus.Results.EncAlg,
		SigningAlgorithm:            agentStatus.Results.SignAlg,
		VerifierID:                  agentStatus.Results.VerifierID,
		VerifierAddress:             fmt.Sprintf("%s:%d", agentStatus.Results.VerifierIP, agentStatus.Results.VerifierPort),
		HasMeasuredBoot:             agentStatus.Results.HasMbRefstate != 0,
		HasRuntimePolicy:            agentStatus.Results.HasRuntimePolicy != 0,
	}
}

func normalizeDigest(digest, path string) (string, error) {
	digest = strings.TrimPrefix(digest, "sha256:")
	if !digestRE.MatchString(digest) {
		return "", fmt.Errorf("digest for %s must be a hex string (40-128 chars)", path)
	}
	return digest, nil
}

const maxPolicyFileSize = 50 * 1024 * 1024 // 50 MB

func validateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("file_path is required")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("file_path must be an absolute path")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("file_path must not contain path traversal")
	}
	if filepath.Ext(path) != ".json" {
		return fmt.Errorf("file_path must have .json extension")
	}
	return nil
}

func readPolicyFile(path string) ([]byte, error) {
	if err := validateFilePath(path); err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("file_path is a directory, not a file")
	}
	if info.Size() > maxPolicyFileSize {
		return nil, fmt.Errorf("file too large (%d bytes, max %d)", info.Size(), maxPolicyFileSize)
	}

	data, err := os.ReadFile(path) // #nosec G304 -- path is validated by validateFilePath above
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("file is empty: %s", path)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("file is not valid JSON: %s", path)
	}

	return data, nil
}

func parseJSONStr(s string) any {
	if s == "" {
		return map[string]any{}
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return s
	}
	return parsed
}

func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func filterLogLines(logs string, keywords []string) string {
	var filtered []string
	scanner := bufio.NewScanner(strings.NewReader(logs))
	for scanner.Scan() {
		line := scanner.Text()
		for _, kw := range keywords {
			if strings.Contains(line, kw) {
				filtered = append(filtered, line)
				break
			}
		}
	}
	return strings.Join(filtered, "\n")
}
