package keylime

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// Service handles Keylime operations with verifier and registrar clients
type Service struct {
	Verifier  *Client
	Registrar *Client
}

// NewService creates a new Keylime service with configured clients
func NewService(config *Config) (*Service, error) {
	verifier, err := newClient(config.VerifierURL, config)
	if err != nil {
		return nil, fmt.Errorf("verifier client: %w", err)
	}
	registrar, err := newClient(config.RegistrarURL, config)
	if err != nil {
		return nil, fmt.Errorf("registrar client: %w", err)
	}
	return &Service{
		Verifier:  verifier,
		Registrar: registrar,
	}, nil
}

func IsFailedState(state int) bool {
	return state == StateFailed || state == StateInvalidQuote || state == StateTenantFailed
}

// FetchAllAgentUUIDs retrieves list of all registered agent UUIDs from registrar
func (s *Service) FetchAllAgentUUIDs() ([]string, error) {
	resp, err := s.Registrar.Get("agents")
	if err != nil {
		log.Printf("Error fetching agents: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var agents AgentListResponse
	err = json.NewDecoder(resp.Body).Decode(&agents)
	if err != nil {
		log.Printf("Error decoding agents: %v", err)
		return nil, err
	}

	if agents.Results.UUIDs == nil {
		return []string{}, nil
	}

	return agents.Results.UUIDs, nil
}

// PrepareEnrollmentBody fetches registrar details and returns the enrollment body ready for POST to the verifier.
func (s *Service) PrepareEnrollmentBody(agentUUID, runtimePolicyName, mbPolicyName string) (map[string]any, error) {
	regResp, err := s.Registrar.Get(fmt.Sprintf("agents/%s", agentUUID))
	if err != nil {
		return nil, fmt.Errorf("agent not found in registrar: %w", err)
	}
	defer regResp.Body.Close()

	var regDetails RegistrarGetAgentDetailsOutput
	if err := json.NewDecoder(regResp.Body).Decode(&regDetails); err != nil {
		return nil, fmt.Errorf("failed to decode registrar response: %w", err)
	}

	runtimePolicyB64 := ""
	if runtimePolicyName != "" {
		policyResp, err := s.Verifier.Get(fmt.Sprintf("allowlists/%s", runtimePolicyName))
		if err != nil {
			return nil, fmt.Errorf("runtime policy %q not found: %w", runtimePolicyName, err)
		}
		defer policyResp.Body.Close()

		var policyData GetRuntimePolicyOutput
		if err := json.NewDecoder(policyResp.Body).Decode(&policyData); err != nil {
			return nil, fmt.Errorf("failed to decode runtime policy %q: %w", runtimePolicyName, err)
		}
		if policyData.Results.RuntimePolicy != "" {
			runtimePolicyB64 = base64.StdEncoding.EncodeToString([]byte(policyData.Results.RuntimePolicy))
		}
	}

	return map[string]any{
		"v":                          nil,
		"cloudagent_ip":              regDetails.Results.IP,
		"cloudagent_port":            regDetails.Results.Port,
		"tpm_policy":                 `{"mask":"0x0"}`,
		"runtime_policy":             runtimePolicyB64,
		"runtime_policy_name":        "",
		"runtime_policy_key":         "",
		"mb_policy":                  "",
		"mb_policy_name":             mbPolicyName,
		"ima_sign_verification_keys": "",
		"metadata":                   "{}",
		"revocation_key":             "",
		"accept_tpm_hash_algs":       []string{"sha256", "sha384", "sha512"},
		"accept_tpm_encryption_algs": []string{"rsa"},
		"accept_tpm_signing_algs":    []string{"rsassa"},
		"ak_tpm":                     regDetails.Results.AikTpm,
		"mtls_cert":                  regDetails.Results.MtlsCert,
		"supported_version":          strings.TrimPrefix(s.Verifier.APIVersion, "v"),
	}, nil
}

// FetchAgentDetails retrieves detailed status information for a specific agent
func (s *Service) FetchAgentDetails(agentUUID string) (AgentStatusResponse, error) {
	resp, err := s.Verifier.Get(fmt.Sprintf("agents/%s", agentUUID))
	if err != nil {
		log.Printf("Error fetching agent status: %v", err)
		return AgentStatusResponse{}, err
	}
	defer resp.Body.Close()

	var agentStatus AgentStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&agentStatus)
	if err != nil {
		log.Printf("Error decoding agent status: %v", err)
		return AgentStatusResponse{}, err
	}

	return agentStatus, nil
}
