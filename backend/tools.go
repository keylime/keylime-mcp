package main

import (
	"context"

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

	var failedAgents getFailedAgentsOutput
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
