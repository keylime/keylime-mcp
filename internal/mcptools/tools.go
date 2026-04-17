package mcptools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/keylime/keylime-mcp/internal/keylime"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/errgroup"
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
	uuids, err := h.service.FetchAllAgentUUIDs(ctx)
	if err != nil {
		return nil, nil, err
	}

	return nil, keylime.GetAllAgentsOutput{Agents: uuids}, nil
}

func (h *ToolHandler) GetVerifierEnrolledAgents(ctx context.Context, req *mcp.CallToolRequest, _ keylime.GetVerifierEnrolledAgentsInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	resp, err := h.service.Verifier.Get(ctx, "agents/")
	if err != nil {
		log.Printf("Error fetching enrolled agents: %v", err)
		return nil, nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, keylime.ExtractAPIError(resp)
	}

	var parsed struct {
		Results struct {
			UUIDs [][]string `json:"uuids"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, nil, fmt.Errorf("failed to decode verifier response: %w", err)
	}

	var uuids []string
	for _, group := range parsed.Results.UUIDs {
		uuids = append(uuids, group...)
	}
	if uuids == nil {
		uuids = []string{}
	}

	return nil, keylime.GetVerifierEnrolledAgentsOutput{Agents: uuids}, nil
}

func (h *ToolHandler) GetAgentStatus(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetAgentStatusInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validateAgentUUID(input.AgentUUID); err != nil {
		return nil, nil, err
	}

	agentStatus, err := h.service.FetchAgentDetails(ctx, input.AgentUUID)
	if err != nil {
		return nil, nil, err
	}

	return nil, mapAgentToOutput(input.AgentUUID, agentStatus), nil
}

func (h *ToolHandler) GetFailedAgents(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetFailedAgentsInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	uuids, err := h.service.FetchAllAgentUUIDs(ctx)
	if err != nil {
		return nil, nil, err
	}

	var mu sync.Mutex
	var failed []keylime.GetAgentStatusOutput
	workers, _ := errgroup.WithContext(ctx)
	workers.SetLimit(10) // 10 was choosed as compromise between performance and resource usage

	for _, agentUUID := range uuids {
		agentUUID := agentUUID // capture loop variable for safe use in goroutine
		workers.Go(func() error {
			agentStatus, err := h.service.FetchAgentDetails(ctx, agentUUID)
			if err != nil || agentStatus.Code < 200 || agentStatus.Code >= 300 {
				return nil // skip agents not enrolled in verifier
			}
			if keylime.IsFailedState(agentStatus.Results.OperationalState) {
				output := mapAgentToOutput(agentUUID, agentStatus)
				mu.Lock()
				failed = append(failed, output)
				mu.Unlock()
			}
			return nil
		})
	}

	if err := workers.Wait(); err != nil {
		return nil, nil, err
	}

	return nil, keylime.GetFailedAgentsOutput{FailedAgents: failed}, nil
}

func (h *ToolHandler) GetAgentPolicies(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetAgentPoliciesInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validateAgentUUID(input.AgentUUID); err != nil {
		return nil, nil, err
	}

	agentDetails, err := h.service.FetchAgentDetails(ctx, input.AgentUUID)
	if err != nil {
		return nil, nil, err
	}

	r := agentDetails.Results
	return nil, keylime.GetAgentPoliciesOutput{
		AgentUUID:                 input.AgentUUID,
		TPMPolicy:                 parseJSONStr(r.TPMPolicy),
		VTPMPolicy:                parseJSONStr(r.VTPMPolicy),
		MetaData:                  parseJSONStr(r.MetaData),
		HasMeasuredBootPolicy:     r.HasMbRefstate != 0,
		HasRuntimePolicy:          r.HasRuntimePolicy != 0,
		AcceptedTPMHashAlgs:       nonNilSlice(r.AcceptTPMHashAlgs),
		AcceptedTPMEncryptionAlgs: nonNilSlice(r.AcceptTPMEncryptionAlgs),
		AcceptedTPMSigningAlgs:    nonNilSlice(r.AcceptTPMSigningAlgs),
	}, nil
}

func (h *ToolHandler) RegistrarGetAgentDetails(ctx context.Context, req *mcp.CallToolRequest, input keylime.RegistrarGetAgentDetailsInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validateAgentUUID(input.AgentUUID); err != nil {
		return nil, nil, err
	}
	result, err := fetchAndDecode[keylime.RegistrarGetAgentDetailsOutput](
		h.service.Registrar.Get(ctx, fmt.Sprintf("agents/%s", input.AgentUUID)),
	)
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

func (h *ToolHandler) GetVersionAndHealth(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetVersionAndHealthInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	var services []keylime.ServiceStatus

	verifierResp, err := fetchAndDecode[keylime.GetVersionOutput](h.service.Verifier.GetRaw(ctx, "version"))
	if err != nil {
		services = append(services, keylime.ServiceStatus{Service: "verifier", Reachable: false, Error: err.Error()})
	} else {
		services = append(services, keylime.ServiceStatus{Service: "verifier", CurrentVersion: verifierResp.Results.CurrentVersion, SupportedVersions: verifierResp.Results.SupportedVersions, Reachable: true})
	}

	registrarResp, err := fetchAndDecode[keylime.GetVersionOutput](h.service.Registrar.GetRaw(ctx, "version"))
	if err != nil {
		services = append(services, keylime.ServiceStatus{Service: "registrar", Reachable: false, Error: err.Error()})
	} else {
		services = append(services, keylime.ServiceStatus{Service: "registrar", CurrentVersion: registrarResp.Results.CurrentVersion, SupportedVersions: registrarResp.Results.SupportedVersions, Reachable: true})
	}

	return nil, keylime.GetVersionAndHealthOutput{Services: services}, nil
}

func (h *ToolHandler) RegistrarRemoveAgent(ctx context.Context, req *mcp.CallToolRequest, input keylime.RegistrarRemoveAgentInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validateAgentUUID(input.AgentUUID); err != nil {
		return nil, nil, err
	}
	result, err := fetchAndDecode[keylime.RegistrarRemoveAgentOutput](
		h.service.Registrar.Delete(ctx, fmt.Sprintf("agents/%s", input.AgentUUID)),
	)
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

func (h *ToolHandler) EnrollAgentToVerifier(ctx context.Context, req *mcp.CallToolRequest, input keylime.EnrollAgentToVerifierInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validateAgentUUID(input.AgentUUID); err != nil {
		return nil, nil, err
	}
	if input.RuntimePolicyName != "" {
		if err := validatePolicyName(input.RuntimePolicyName); err != nil {
			return nil, nil, fmt.Errorf("runtime_policy_name: %w", err)
		}
	}
	if input.MbPolicyName != "" {
		if err := validatePolicyName(input.MbPolicyName); err != nil {
			return nil, nil, fmt.Errorf("mb_policy_name: %w", err)
		}
	}

	body, err := h.service.PrepareEnrollmentBody(ctx, input.AgentUUID, input.RuntimePolicyName, input.MbPolicyName)
	if err != nil {
		return nil, nil, err
	}

	result, err := fetchAndDecode[keylime.EnrollAgentToVerifierOutput](
		h.service.Verifier.Post(ctx, fmt.Sprintf("agents/%s", input.AgentUUID), body),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("enrollment failed: %w", err)
	}
	return nil, result, nil
}

func (h *ToolHandler) UpdateAgent(ctx context.Context, req *mcp.CallToolRequest, input keylime.UpdateAgentInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validateAgentUUID(input.AgentUUID); err != nil {
		return nil, nil, err
	}
	if input.RuntimePolicyName != "" {
		if err := validatePolicyName(input.RuntimePolicyName); err != nil {
			return nil, nil, fmt.Errorf("runtime_policy_name: %w", err)
		}
	}
	if input.MbPolicyName != "" {
		if err := validatePolicyName(input.MbPolicyName); err != nil {
			return nil, nil, fmt.Errorf("mb_policy_name: %w", err)
		}
	}

	body, err := h.service.PrepareEnrollmentBody(ctx, input.AgentUUID, input.RuntimePolicyName, input.MbPolicyName)
	if err != nil {
		return nil, nil, err
	}

	if err := deleteAndCheck(h.service.Verifier.Delete(ctx, fmt.Sprintf("agents/%s", input.AgentUUID))); err != nil {
		return nil, nil, fmt.Errorf("failed to unenroll agent: %w", err)
	}

	resp, err := h.service.Verifier.Post(ctx, fmt.Sprintf("agents/%s", input.AgentUUID), body)
	if err != nil {
		return nil, nil, fmt.Errorf("CRITICAL: agent was unenrolled but re-enrollment failed: %w — manually re-enroll agent %s", err, input.AgentUUID)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	var enrollResp keylime.EnrollAgentToVerifierOutput
	if err := json.NewDecoder(resp.Body).Decode(&enrollResp); err != nil {
		return nil, nil, fmt.Errorf("CRITICAL: agent was unenrolled but response decode failed: %w — verify agent %s status", err, input.AgentUUID)
	}

	return nil, keylime.UpdateAgentOutput{AgentUUID: input.AgentUUID, Status: "updated"}, nil
}

func (h *ToolHandler) UnenrollAgentFromVerifier(ctx context.Context, req *mcp.CallToolRequest, input keylime.UnenrollAgentFromVerifierInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validateAgentUUID(input.AgentUUID); err != nil {
		return nil, nil, err
	}
	result, err := fetchAndDecode[keylime.UnenrollAgentFromVerifierOutput](
		h.service.Verifier.Delete(ctx, fmt.Sprintf("agents/%s", input.AgentUUID)),
	)
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

func (h *ToolHandler) ReactivateAgent(ctx context.Context, req *mcp.CallToolRequest, input keylime.ReactivateAgentInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validateAgentUUID(input.AgentUUID); err != nil {
		return nil, nil, err
	}
	result, err := fetchAndDecode[keylime.ReactivateAgentOutput](
		h.service.Verifier.Put(ctx, fmt.Sprintf("agents/%s/reactivate", input.AgentUUID), nil),
	)
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

func (h *ToolHandler) StopAgent(ctx context.Context, req *mcp.CallToolRequest, input keylime.StopAgentInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validateAgentUUID(input.AgentUUID); err != nil {
		return nil, nil, err
	}
	result, err := fetchAndDecode[keylime.StopAgentOutput](
		h.service.Verifier.Put(ctx, fmt.Sprintf("agents/%s/stop", input.AgentUUID), nil),
	)
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

func (h *ToolHandler) ListRuntimePolicies(ctx context.Context, req *mcp.CallToolRequest, input keylime.ListRuntimePoliciesInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	result, err := fetchAndDecode[keylime.ListRuntimePoliciesOutput](h.service.Verifier.Get(ctx, "allowlists/"))
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

func (h *ToolHandler) GetRuntimePolicy(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetRuntimePolicyInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validatePolicyName(input.PolicyName); err != nil {
		return nil, nil, err
	}
	result, err := fetchAndDecode[keylime.GetRuntimePolicyOutput](
		h.service.Verifier.Get(ctx, fmt.Sprintf("allowlists/%s", input.PolicyName)),
	)
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

func (h *ToolHandler) ImportRuntimePolicy(ctx context.Context, req *mcp.CallToolRequest, input keylime.ImportRuntimePolicyInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validatePolicyName(input.Name); err != nil {
		return nil, nil, err
	}

	data, err := readPolicyFile(input.FilePath)
	if err != nil {
		return nil, nil, err
	}

	body := map[string]any{
		"runtime_policy": base64.StdEncoding.EncodeToString(data),
	}

	endpoint := fmt.Sprintf("allowlists/%s", input.Name)
	resp, err := h.service.Verifier.Post(ctx, endpoint, body)
	if err != nil {
		log.Printf("Error importing runtime policy: %v", err)
		return nil, nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, keylime.ExtractAPIError(resp)
	}

	return nil, keylime.ImportRuntimePolicyOutput{Name: input.Name, Status: "imported"}, nil
}

//nolint:gocognit,gocyclo // sequential steps of a single read-modify-write operation
func (h *ToolHandler) UpdateRuntimePolicy(ctx context.Context, req *mcp.CallToolRequest, input keylime.UpdateRuntimePolicyInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validatePolicyName(input.PolicyName); err != nil {
		return nil, nil, err
	}
	if len(input.AddExcludes) == 0 && len(input.AddDigests) == 0 && len(input.RemoveExcludes) == 0 && len(input.RemoveDigests) == 0 {
		return nil, nil, fmt.Errorf("at least one of add_excludes, add_digests, remove_excludes or remove_digests is required")
	}

	// Fetch existing policy
	getResp, err := h.service.Verifier.Get(ctx, fmt.Sprintf("allowlists/%s", input.PolicyName))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch policy %q: %w", input.PolicyName, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, getResp.Body)
		_ = getResp.Body.Close()
	}()

	if getResp.StatusCode < 200 || getResp.StatusCode >= 300 {
		return nil, nil, keylime.ExtractAPIError(getResp)
	}

	var policyData keylime.GetRuntimePolicyOutput
	if err := json.NewDecoder(getResp.Body).Decode(&policyData); err != nil {
		return nil, nil, fmt.Errorf("failed to decode policy %q: %w", input.PolicyName, err)
	}
	var policy map[string]any
	policyStr := policyData.Results.RuntimePolicy
	if policyStr == "" {
		policyStr = "{}"
	}
	if err := json.Unmarshal([]byte(policyStr), &policy); err != nil {
		return nil, nil, fmt.Errorf("failed to parse policy JSON: %w", err)
	}
	// Remove excludes
	if len(input.RemoveExcludes) > 0 {
		oldExcludes, _ := policy["excludes"].([]any)
		var filtered []any
		for _, e := range oldExcludes {
			keep := true
			for _, r := range input.RemoveExcludes {
				if e == r {
					keep = false
					break
				}
			}
			if keep {
				filtered = append(filtered, e)
			}
		}
		policy["excludes"] = filtered
	}

	// Add excludes
	for _, newExclude := range input.AddExcludes {
		if !strings.HasSuffix(newExclude, ")?") {
			newExclude += "(/.*)?"
		}
		excludes, _ := policy["excludes"].([]any)
		var found bool
		for _, e := range excludes {
			if e == newExclude {
				found = true
				break
			}
		}
		if !found {
			policy["excludes"] = append(excludes, newExclude)
		}
	}

	// Remove digests
	if len(input.RemoveDigests) > 0 {
		digests, _ := policy["digests"].(map[string]any)
		for _, path := range input.RemoveDigests {
			delete(digests, path)
		}
	}

	// Add digests
	for newPath, newDigest := range input.AddDigests {
		normalized, err := normalizeDigest(newDigest, newPath)
		if err != nil {
			return nil, nil, err
		}
		digests, _ := policy["digests"].(map[string]any)
		if digests == nil {
			digests = map[string]any{}
			policy["digests"] = digests
		}
		digests[newPath] = []any{normalized}
	}

	// Update timestamp
	meta, _ := policy["meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
		policy["meta"] = meta
	}
	meta["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	// Re-upload
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal policy: %w", err)
	}

	body := map[string]any{
		"runtime_policy": base64.StdEncoding.EncodeToString(policyJSON),
	}

	reuploadResp, err := h.service.Verifier.Put(ctx, fmt.Sprintf("allowlists/%s", input.PolicyName), body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update policy: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, reuploadResp.Body)
		_ = reuploadResp.Body.Close()
	}()

	if reuploadResp.StatusCode < 200 || reuploadResp.StatusCode >= 300 {
		return nil, nil, keylime.ExtractAPIError(reuploadResp)
	}

	return nil, keylime.UpdateRuntimePolicyOutput{PolicyName: input.PolicyName, Status: "updated"}, nil
}

func (h *ToolHandler) DeleteRuntimePolicy(ctx context.Context, req *mcp.CallToolRequest, input keylime.DeleteRuntimePolicyInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validatePolicyName(input.PolicyName); err != nil {
		return nil, nil, err
	}
	if err := deleteAndCheck(h.service.Verifier.Delete(ctx, fmt.Sprintf("allowlists/%s", input.PolicyName))); err != nil {
		return nil, nil, err
	}
	return nil, keylime.DeletePolicyOutput{PolicyName: input.PolicyName, Status: "deleted"}, nil
}

func (h *ToolHandler) ListMBPolicies(ctx context.Context, req *mcp.CallToolRequest, input keylime.ListMBPoliciesInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	result, err := fetchAndDecode[keylime.ListMBPoliciesOutput](h.service.Verifier.Get(ctx, "mbpolicies/"))
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

func (h *ToolHandler) GetMBPolicy(ctx context.Context, req *mcp.CallToolRequest, input keylime.GetMBPolicyInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validatePolicyName(input.PolicyName); err != nil {
		return nil, nil, err
	}
	result, err := fetchAndDecode[keylime.GetMBPolicyOutput](
		h.service.Verifier.Get(ctx, fmt.Sprintf("mbpolicies/%s", input.PolicyName)),
	)
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

func (h *ToolHandler) ImportMBPolicy(ctx context.Context, req *mcp.CallToolRequest, input keylime.ImportMBPolicyInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validatePolicyName(input.Name); err != nil {
		return nil, nil, err
	}

	data, err := readPolicyFile(input.FilePath)
	if err != nil {
		return nil, nil, err
	}

	body := map[string]any{
		"mb_policy": string(data),
	}

	endpoint := fmt.Sprintf("mbpolicies/%s", input.Name)
	resp, err := h.service.Verifier.Post(ctx, endpoint, body)
	if err != nil {
		log.Printf("Error importing measured boot policy: %v", err)
		return nil, nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, keylime.ExtractAPIError(resp)
	}

	return nil, keylime.ImportMBPolicyOutput{Name: input.Name, Status: "imported"}, nil
}

func (h *ToolHandler) DeleteMBPolicy(ctx context.Context, req *mcp.CallToolRequest, input keylime.DeleteMBPolicyInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	if err := validatePolicyName(input.PolicyName); err != nil {
		return nil, nil, err
	}
	if err := deleteAndCheck(h.service.Verifier.Delete(ctx, fmt.Sprintf("mbpolicies/%s", input.PolicyName))); err != nil {
		return nil, nil, err
	}
	return nil, keylime.DeletePolicyOutput{PolicyName: input.PolicyName, Status: "deleted"}, nil
}

func (h *ToolHandler) InvestigateVerifierLogs(ctx context.Context, req *mcp.CallToolRequest, input keylime.InvestigateVerifierLogsInput) (
	*mcp.CallToolResult,
	any,
	error,
) {
	var logFilters = map[string][]string{
		"attestation_failures": {"FAIL", "fail", "not_in_allowlist", "invalid", "mismatch", "pcr", "quote", "policy", "not reachable", "terminated", "Revocation"},
		"errors":               {"ERROR", "Traceback", "Exception", "CRITICAL", "Unable"},
	}
	filter := input.Filter
	if filter == "" {
		filter = "all"
	}
	if filter != "all" {
		if _, ok := logFilters[filter]; !ok {
			return nil, nil, fmt.Errorf("invalid filter %q: must be 'all', 'attestation_failures', or 'errors'", filter)
		}
	}

	lines := input.Lines
	if lines <= 0 {
		lines = 50
	}
	if lines > 200 {
		lines = 200
	}

	if input.AgentUUID != "" {
		if err := validateAgentUUID(input.AgentUUID); err != nil {
			return nil, nil, err
		}
	}

	args := []string{"-u", "keylime_verifier", "--no-pager", "-n", fmt.Sprintf("%d", lines)}
	if input.AgentUUID != "" {
		args = append(args, "--grep", input.AgentUUID)
	}

	out, err := exec.CommandContext(ctx, "journalctl", args...).Output() // #nosec G204 -- args are validated (UUID regex, integer lines)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, keylime.InvestigateVerifierLogsOutput{Logs: "", FilterApplied: filter}, nil
		}
		return nil, nil, fmt.Errorf("failed to read verifier logs: %w", err)
	}

	logs := string(out)
	if keywords, ok := logFilters[filter]; ok {
		logs = filterLogLines(logs, keywords)
	}

	return nil, keylime.InvestigateVerifierLogsOutput{Logs: logs, FilterApplied: filter}, nil
}
