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
	uuids, err := fetchAllAgentUUIDs()
	if err != nil {
		return nil, getAllAgentsOutput{}, err
	}

	return nil, getAllAgentsOutput{Agents: uuids}, nil
}

func getAgentStatus(ctx context.Context, req *mcp.CallToolRequest, input getAgentStatusInput) (
	*mcp.CallToolResult,
	getAgentStatusOutput,
	error,
) {
	agentStatus, err := fetchAgentDetails(input.AgentUUID)
	if err != nil {
		return nil, getAgentStatusOutput{}, err
	}

	return nil, mapAgentToOutput(input.AgentUUID, agentStatus), nil
}

func getFailedAgents(ctx context.Context, req *mcp.CallToolRequest, input getFailedAgentsInput) (
	*mcp.CallToolResult,
	getFailedAgentsOutput,
	error,
) {
	uuids, err := fetchAllAgentUUIDs()
	if err != nil {
		return nil, getFailedAgentsOutput{}, err
	}

	failedAgents := getFailedAgentsOutput{
		FailedAgents: []getAgentStatusOutput{},
	}
	for _, agentUUID := range uuids {
		agentStatus, err := fetchAgentDetails(agentUUID)
		if err != nil {
			return nil, getFailedAgentsOutput{}, err
		}

		if agentStatus.Results.OperationalState == StateFailed {
			failedAgents.FailedAgents = append(failedAgents.FailedAgents, mapAgentToOutput(agentUUID, agentStatus))
		}
	}

	return nil, failedAgents, nil
}

func reactivate_agent(ctx context.Context, req *mcp.CallToolRequest, input reactivateAgentInput) (
	*mcp.CallToolResult,
	reactivateAgentOutput,
	error,
) {
	endpoint := fmt.Sprintf("agents/%s/reactivate", input.AgentUUID)
	resp, err := keylimeVerifierClient.Put(endpoint, nil)
	if err != nil {
		log.Printf("Error reactivating agent: %v", err)
		return nil, reactivateAgentOutput{}, err
	}
	defer resp.Body.Close()

	var response reactivateAgentOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, reactivateAgentOutput{}, err
	}

	return nil, response, nil
}

func agent_policies(ctx context.Context, req *mcp.CallToolRequest, input getAgentPoliciesInput) (
	*mcp.CallToolResult,
	getAgentPoliciesOutput,
	error,
) {
	agentDetails, err := fetchAgentDetails(input.AgentUUID)
	if err != nil {
		return nil, getAgentPoliciesOutput{}, err
	}
	return nil, mapAgentToPolicies(input.AgentUUID, agentDetails), nil
}
