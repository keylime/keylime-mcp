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
