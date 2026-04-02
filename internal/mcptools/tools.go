package mcptools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
		log.Printf("Error removing agent from registrar: %v", err)
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

func (h *ToolHandler) EnrollAgentToVerifier(ctx context.Context, req *mcp.CallToolRequest, input keylime.EnrollAgentToVerifierInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	regEndpoint := fmt.Sprintf("agents/%s", input.AgentUUID)
	regResp, err := h.service.Registrar.Get(regEndpoint)
	if err != nil {
		log.Printf("Error fetching agent details: %v", err)
		return nil, nil, err
	}
	defer regResp.Body.Close()

	var regDetails keylime.RegistrarGetAgentDetailsOutput
	if err := json.NewDecoder(regResp.Body).Decode(&regDetails); err != nil {
		log.Printf("Error decoding registrar response: %v", err)
		return nil, nil, err
	}

	body := map[string]any{
		"v":                          nil,
		"cloudagent_ip":              regDetails.Results.IP,
		"cloudagent_port":            regDetails.Results.Port,
		"tpm_policy":                 "{}",
		"runtime_policy":             "",
		"runtime_policy_name":        "",
		"runtime_policy_key":         "",
		"mb_policy":                  "",
		"mb_policy_name":             "",
		"ima_sign_verification_keys": "",
		"metadata":                   "{}",
		"revocation_key":             "",
		"accept_tpm_hash_algs":       []string{"sha256", "sha384", "sha512"},
		"accept_tpm_encryption_algs": []string{"rsa"},
		"accept_tpm_signing_algs":    []string{"rsassa"},
		"ak_tpm":                     regDetails.Results.AikTpm,
		"mtls_cert":                  regDetails.Results.MtlsCert,
		"supported_version":          strings.TrimPrefix(h.service.Verifier.APIVersion, "v"),
	}

	endpoint := fmt.Sprintf("agents/%s", input.AgentUUID)
	resp, err := h.service.Verifier.Post(endpoint, body)
	if err != nil {
		log.Printf("Error enrolling agent to verifier: %v", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response keylime.EnrollAgentToVerifierOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, nil, err
	}

	return nil, response, nil
}

func (h *ToolHandler) UnenrollAgentFromVerifier(ctx context.Context, req *mcp.CallToolRequest, input keylime.UnenrollAgentFromVerifierInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	endpoint := fmt.Sprintf("agents/%s", input.AgentUUID)
	resp, err := h.service.Verifier.Delete(endpoint)
	if err != nil {
		log.Printf("Error removing agent from verifier: %v", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response keylime.UnenrollAgentFromVerifierOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, nil, err
	}

	return nil, response, nil
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

func (h *ToolHandler) StopAgent(ctx context.Context, req *mcp.CallToolRequest, input keylime.StopAgentInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	endpoint := fmt.Sprintf("agents/%s/stop", input.AgentUUID)
	resp, err := h.service.Verifier.Put(endpoint, nil)
	if err != nil {
		log.Printf("Error stopping agent: %v", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response keylime.StopAgentOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, nil, err
	}

	return nil, response, nil
}

func (h *ToolHandler) ListRuntimePolicies(ctx context.Context, req *mcp.CallToolRequest, input keylime.ListRuntimePoliciesInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	endpoint := "allowlists"
	resp, err := h.service.Verifier.Get(endpoint)
	if err != nil {
		log.Printf("Error listing runtime policies: %v", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response keylime.ListRuntimePoliciesOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, nil, err
	}

	return nil, response, nil
}

func (h *ToolHandler) GetRuntimePolicy(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetRuntimePolicyInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	endpoint := fmt.Sprintf("allowlists/%s", input.PolicyName)
	resp, err := h.service.Verifier.Get(endpoint)
	if err != nil {
		log.Printf("Error getting runtime policy: %v", err)
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response keylime.GetRuntimePolicyOutput
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return nil, nil, err
	}

	return nil, response, nil
}
