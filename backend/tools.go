package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func getAllAgents(ctx context.Context, req *mcp.CallToolRequest, _ getAllAgentsInput) (
	*mcp.CallToolResult,
	getAllAgentsOutput,
	error,
) {
	resp, err := keylimeClient.Get("agents")
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

// func getAgentStatus(ctx context.Context, req *mcp.CallToolRequest, input getAgentStatusInput) (
// 	*mcp.CallToolResult,
// 	AgentStatusResultsField,
// 	error,
// ) {
// 	resp, err := keylimeClient.Get(fmt.Sprintf("agents/%s", input.AgentUUID))
// 	if err != nil {
// 		log.Printf("Error fetching agent status: %v", err)
// 		return nil, AgentStatusResultsField{}, err
// 	}
// 	defer resp.Body.Close()

// 	var agentStatus getAgentStatusOutput
// 	err = json.NewDecoder(resp.Body).Decode(&agentStatus)
// 	if err != nil {
// 		log.Printf("Error decoding agent status: %v", err)
// 		return nil, AgentStatusResultsField{}, err
// 	}

// 	return nil, AgentStatusResultsField{}, nil
// }
