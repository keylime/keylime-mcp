package mcptools

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/keylime/keylime-mcp/internal/keylime"
)

var (
	uuidRE     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	safeNameRE = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
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
