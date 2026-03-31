package mcptools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/keylime/keylime-mcp/internal/keylime"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolHandler struct {
	service *keylime.Service
}

func NewToolHandler(service *keylime.Service) *ToolHandler {
	return &ToolHandler{service: service}
}

func (h *ToolHandler) GetAllAgents(ctx context.Context, req *mcp.CallToolRequest, _ keylime.GetAllAgentsInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	uuids, err := h.service.FetchAllAgentUUIDs()
	if err != nil {
		return nil, nil, err
	}

	return nil, keylime.GetAllAgentsOutput{Agents: uuids}, nil
}

func (h *ToolHandler) GetAgentStatus(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetAgentStatusInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	agentStatus, err := h.service.FetchAgentDetails(input.AgentUUID)
	if err != nil {
		return nil, nil, err
	}

	return nil, keylime.MapAgentToOutput(input.AgentUUID, agentStatus), nil
}

func (h *ToolHandler) GetFailedAgents(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetFailedAgentsInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	uuids, err := h.service.FetchAllAgentUUIDs()
	if err != nil {
		return nil, nil, err
	}

	failedAgents := keylime.GetFailedAgentsOutput{
		FailedAgents: []keylime.GetAgentStatusOutput{},
	}
	for _, agentUUID := range uuids {
		agentStatus, err := h.service.FetchAgentDetails(agentUUID)
		if err != nil {
			return nil, nil, err
		}

		if agentStatus.Results.OperationalState == keylime.StateFailed {
			failedAgents.FailedAgents = append(failedAgents.FailedAgents, keylime.MapAgentToOutput(agentUUID, agentStatus))
		}
	}

	return nil, failedAgents, nil
}

func (h *ToolHandler) ReactivateAgent(ctx context.Context, req *mcp.CallToolRequest, input keylime.ReactivateAgentInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	endpoint := fmt.Sprintf("agents/%s/reactivate", input.AgentUUID)
	resp, err := h.service.Verifier.Put(endpoint, nil)
	if err != nil {
		log.Printf("Error reactivating agent: %v", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response keylime.ReactivateAgentOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, nil, err
	}

	return nil, response, nil
}

func (h *ToolHandler) AgentPolicies(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetAgentPoliciesInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	agentDetails, err := h.service.FetchAgentDetails(input.AgentUUID)
	if err != nil {
		return nil, nil, err
	}
	return nil, keylime.MapAgentToPolicies(input.AgentUUID, agentDetails), nil
}

func (h *ToolHandler) RegistrarGetAgentDetails(ctx context.Context, req *mcp.CallToolRequest, input keylime.RegistrarGetAgentDetailsInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	endpoint := fmt.Sprintf("agents/%s", input.AgentUUID)
	resp, err := h.service.Registrar.Get(endpoint)
	if err != nil {
		log.Printf("Error getting agent details: %v", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response keylime.RegistrarGetAgentDetailsOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, nil, err
	}

	return nil, response, nil

}

func (h *ToolHandler) GetAgentVersion(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetAgentVersionInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	endpoint := "version"
	resp, err := h.service.Registrar.GetRaw(endpoint)
	if err != nil {
		log.Printf("Error getting agent version: %v", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response keylime.GetAgentVersionOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, nil, err
	}

	return nil, response, nil
}

func (h *ToolHandler) RegistrarRemoveAgent(ctx context.Context, req *mcp.CallToolRequest, input keylime.RegistrarRemoveAgentInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	endpoint := fmt.Sprintf("agents/%s", input.AgentUUID)
	resp, err := h.service.Registrar.Delete(endpoint)
	if err != nil {
		log.Printf("Error getting agent version: %v", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response keylime.RegistrarRemoveAgentOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, nil, err
	}

	return nil, response, nil
}
