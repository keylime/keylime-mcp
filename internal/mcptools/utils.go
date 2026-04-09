package mcptools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/keylime/keylime-mcp/internal/keylime"
)

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

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if !json.Valid(data) {
		return nil, fmt.Errorf("file is not valid JSON")
	}

	return data, nil
}

func parseJSONStr(s string) any {
	if s == "" {
		return map[string]any{}
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return map[string]any{}
	}
	return v
}

func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
