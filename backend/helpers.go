package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// fetchAllAgentUUIDs retrieves list of all registered agent UUIDs from registrar
func fetchAllAgentUUIDs() ([]string, error) {
	resp, err := keylimeRegistrarClient.Get("agents")
	if err != nil {
		log.Printf("Error fetching agents: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var agents keylimeAgentListResponse
	err = json.NewDecoder(resp.Body).Decode(&agents)
	if err != nil {
		log.Printf("Error decoding agents: %v", err)
		return nil, err
	}

	return agents.Results.UUIDs, nil
}

// fetchAgentDetails retrieves detailed status information for a specific agent
func fetchAgentDetails(agentUUID string) (keylimeAgentStatusResponse, error) {
	resp, err := keylimeVerifierClient.Get(fmt.Sprintf("agents/%s", agentUUID))
	if err != nil {
		log.Printf("Error fetching agent status: %v", err)
		return keylimeAgentStatusResponse{}, err
	}
	defer resp.Body.Close()

	var agentStatus keylimeAgentStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&agentStatus)
	if err != nil {
		log.Printf("Error decoding agent status: %v", err)
		return keylimeAgentStatusResponse{}, err
	}

	return agentStatus, nil
}

// mapAgentToOutput converts API response to standardized output format
func mapAgentToOutput(agentUUID string, agentStatus keylimeAgentStatusResponse) getAgentStatusOutput {
	return getAgentStatusOutput{
		AgentUUID:                   agentUUID,
		OperationalState:            agentStatus.Results.OperationalState,
		OperationalStateDescription: stateToString(agentStatus.Results.OperationalState),
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
