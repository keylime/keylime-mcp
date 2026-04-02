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


