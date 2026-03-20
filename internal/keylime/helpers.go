package keylime

import (
	"encoding/json"
	"fmt"
	"log"
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

// MapAgentToOutput converts API response to standardized output format
func MapAgentToOutput(agentUUID string, agentStatus AgentStatusResponse) GetAgentStatusOutput {
	return GetAgentStatusOutput{
		AgentUUID:                   agentUUID,
		OperationalState:            agentStatus.Results.OperationalState,
		OperationalStateDescription: StateToString(agentStatus.Results.OperationalState),
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

func MapAgentToPolicies(agentUUID string, agentStatus AgentStatusResponse) GetAgentPoliciesOutput {
	// Ensure slices are never nil
	hashAlgs := agentStatus.Results.AcceptTPMHashAlgs
	if hashAlgs == nil {
		hashAlgs = []string{}
	}
	encAlgs := agentStatus.Results.AcceptTPMEncryptionAlgs
	if encAlgs == nil {
		encAlgs = []string{}
	}
	signAlgs := agentStatus.Results.AcceptTPMSigningAlgs
	if signAlgs == nil {
		signAlgs = []string{}
	}

	return GetAgentPoliciesOutput{
		AgentUUID:                 agentUUID,
		TPMPolicy:                 parseJSONString(agentStatus.Results.TPMPolicy),
		VTPMPolicy:                parseJSONString(agentStatus.Results.VTPMPolicy),
		MetaData:                  parseJSONString(agentStatus.Results.MetaData),
		HasMeasuredBootPolicy:     agentStatus.Results.HasMbRefstate != 0,
		HasRuntimePolicy:          agentStatus.Results.HasRuntimePolicy != 0,
		AcceptedTPMHashAlgs:       hashAlgs,
		AcceptedTPMEncryptionAlgs: encAlgs,
		AcceptedTPMSigningAlgs:    signAlgs,
	}
}

// parseJSONString converts a JSON string into a proper Go interface
func parseJSONString(jsonStr string) any {
	if jsonStr == "" {
		return map[string]any{}
	}

	var result any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("Warning: Invalid JSON string: %v", err)
		return map[string]any{}
	}

	return result
}
