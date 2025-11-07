package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func getAllAgents(ctx context.Context, req *mcp.CallToolRequest, _ getAllAgentsInput) (
	*mcp.CallToolResult,
	getAllAgentsOutput,
	error,
) {
	resp, err := keylimeRegistrarClient.Get("agents")
	if err != nil {
		log.Printf("Error fetching agents: %v", err)
		return nil, getAllAgentsOutput{}, err
	}
	defer resp.Body.Close()

	var agents keylimeAgentListResponse
	err = json.NewDecoder(resp.Body).Decode(&agents)
	if err != nil {
		log.Printf("Error decoding agents: %v", err)
		return nil, getAllAgentsOutput{}, err
	}

	return nil, getAllAgentsOutput{Agents: agents.Results.UUIDs}, nil
}

func getAgentStatus(ctx context.Context, req *mcp.CallToolRequest, input getAgentStatusInput) (
	*mcp.CallToolResult,
	getAgentStatusOutput,
	error,
) {
	resp, err := keylimeVerifierClient.Get(fmt.Sprintf("agents/%s", input.AgentUUID))
	if err != nil {
		log.Printf("Error fetching agent status: %v", err)
		return nil, getAgentStatusOutput{}, err
	}
	defer resp.Body.Close()

	var agentStatus keylimeAgentStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&agentStatus)
	if err != nil {
		log.Printf("Error decoding agent status: %v", err)
		return nil, getAgentStatusOutput{}, err
	}

	return nil, getAgentStatusOutput{
		AgentUUID:                   input.AgentUUID,
		OperationalState:            agentStatus.Results.OperationalState,
		OperationalStateDescription: stateToString(agentStatus.Results.OperationalState),
		IP:                          agentStatus.Results.IP,
		Port:                        agentStatus.Results.Port,
		AttestationCount:            agentStatus.Results.AttestationCount,
		LastReceivedQuote:           agentStatus.Results.LastReceivedQuote,
		LastSuccessfulAttestation:   agentStatus.Results.LastSuccessfulAttestation,
		SeverityLevel:               agentStatus.Results.SeverityLevel,
		LastEventID:                 agentStatus.Results.LastEventID,
	}, nil
}

func getFailedAgents(ctx context.Context, req *mcp.CallToolRequest, input getFailedAgentsInput) (
	*mcp.CallToolResult,
	getFailedAgentsOutput,
	error,
) {
	resp, err := keylimeRegistrarClient.Get("agents")
	if err != nil {
		log.Printf("Error fetching agents: %v", err)
		return nil, getFailedAgentsOutput{}, err
	}
	defer resp.Body.Close()

	var agents keylimeAgentListResponse
	err = json.NewDecoder(resp.Body).Decode(&agents)
	if err != nil {
		log.Printf("Error decoding agents: %v", err)
		return nil, getFailedAgentsOutput{}, err
	}

	var failedAgents getFailedAgentsOutput
	for _, agentUUID := range agents.Results.UUIDs {
		agentResp, err := keylimeVerifierClient.Get(fmt.Sprintf("agents/%s", agentUUID))
		if err != nil {
			log.Printf("Error fetching agent status: %v", err)
			return nil, getFailedAgentsOutput{}, err
		}

		var agentStatus keylimeAgentStatusResponse
		err = json.NewDecoder(agentResp.Body).Decode(&agentStatus)
		agentResp.Body.Close() // Close immediately after use

		if err != nil {
			log.Printf("Error decoding agent status: %v", err)
			return nil, getFailedAgentsOutput{}, err
		}

		// Check if agent is in failed state
		if agentStatus.Results.OperationalState == StateFailed {
			failedAgents.FailedAgents = append(failedAgents.FailedAgents, getAgentStatusOutput{
				AgentUUID:                   agentUUID,
				OperationalState:            agentStatus.Results.OperationalState,
				OperationalStateDescription: stateToString(agentStatus.Results.OperationalState),
				IP:                          agentStatus.Results.IP,
				Port:                        agentStatus.Results.Port,
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
			})
		}
	}

	return nil, failedAgents, nil
}
