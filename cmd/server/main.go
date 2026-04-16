package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/keylime/keylime-mcp/internal/keylime"
	"github.com/keylime/keylime-mcp/internal/mcptools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	err1 := godotenv.Load(".env")
	err2 := godotenv.Load("../.env")
	if err1 != nil && err2 != nil {
		log.Printf("No .env file found, using defaults")
	}
	config := loadConfig()
	keylimeService, err := keylime.NewService(&config)
	if err != nil {
		log.Fatalf("Failed to initialize Keylime service: %v", err)
	}
	toolHandler := mcptools.NewToolHandler(keylimeService)

	server := mcp.NewServer(&mcp.Implementation{Name: "Keylime", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_version_and_health", Description: "Retrieves current and supported API Keylime Verifier and Registrar versions and checks if the services are reachable"}, toolHandler.GetVersionAndHealth)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_all_agents", Description: "Retrieves a list of all registered agent UUIDs from the registrar"}, toolHandler.GetAllAgents)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_verifier_enrolled_agents", Description: "Retrieves a list of agent UUIDs enrolled in the verifier for active attestation"}, toolHandler.GetVerifierEnrolledAgents)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_agent_status", Description: "Retrieves attestation status from the verifier: operational state, attestation count, severity, last quote timestamps, and algorithms."}, toolHandler.GetAgentStatus)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_failed_agents", Description: "Retrieves all agents currently in a failed operational state with their detailed status information including attestation history and failure reasons"}, toolHandler.GetFailedAgents)
	mcp.AddTool(server, &mcp.Tool{Name: "Reactivate_agent", Description: "Reactivates a failed agent identified by its UUID"}, toolHandler.ReactivateAgent)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_agent_policies", Description: "Retrieves policy configuration (TPM, vTPM, runtime policies) for a specific agent"}, toolHandler.AgentPolicies)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_agent_details", Description: "Retrieves hardware identity from the registrar: EK certificate, AIK, mTLS cert, IP and port. Not attestation status — use Get_agent_status for that."}, toolHandler.RegistrarGetAgentDetails)
	mcp.AddTool(server, &mcp.Tool{Name: "Registrar_remove_agent", Description: "Removes an agent from the registrar (NOT the verifier)"}, toolHandler.RegistrarRemoveAgent)
	mcp.AddTool(server, &mcp.Tool{Name: "Enroll_agent_to_verifier", Description: "Enrolls a registered agent into the verifier for active attestation. Optional runtime_policy_name (use List_runtime_policies for names) and mb_policy_name (use List_mb_policies for names) refer to existing policies on the verifier. Leave empty to enroll without policy."}, toolHandler.EnrollAgentToVerifier)
	mcp.AddTool(server, &mcp.Tool{Name: "Update_agent", Description: "Re-enrolls an agent with a new policy. Safely validates everything before unenrolling, then re-enrolls. Use this instead of manually calling Unenroll + Enroll."}, toolHandler.UpdateAgent)
	mcp.AddTool(server, &mcp.Tool{Name: "Unenroll_agent_from_verifier", Description: "Unenrolls an agent from the verifier (NOT the registrar)"}, toolHandler.UnenrollAgentFromVerifier)
	mcp.AddTool(server, &mcp.Tool{Name: "Stop_agent", Description: "Stop Verifier polling on an agent identified by its UUID, but does not remove the agent"}, toolHandler.StopAgent)
	mcp.AddTool(server, &mcp.Tool{Name: "List_runtime_policies", Description: "Lists names of runtime policies already uploaded to the verifier. These are policies available for assigning to agents during enrollment."}, toolHandler.ListRuntimePolicies)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_runtime_policy", Description: "Gets the content of a specific runtime policy stored on the verifier by name. Returns the policy JSON including digests, excludes, and keyrings. Use List_runtime_policies first to see available names."}, toolHandler.GetRuntimePolicy)
	mcp.AddTool(server, &mcp.Tool{Name: "Import_runtime_policy", Description: "Uploads a local runtime policy JSON file to the verifier. If the user has no policy file, ask whether they want to generate it from a local filesystem or a remote RPM repo. For local: 'sudo keylime-policy create runtime --rootfs / -o /tmp/runtime_policy.json'. For RPM repo: 'sudo keylime-policy create runtime --remote-rpm-repo <URL> -o /tmp/runtime_policy.json'. Then provide the output path to this tool."}, toolHandler.ImportRuntimePolicy)
	mcp.AddTool(server, &mcp.Tool{Name: "Update_runtime_policy", Description: "Updates an existing runtime policy on the verifier. Can add or remove excludes and digests. Fetches the current policy, applies changes, and re-uploads. Requires at least one of add_excludes, remove_excludes, add_digests, or remove_digests."}, toolHandler.UpdateRuntimePolicy)
	mcp.AddTool(server, &mcp.Tool{Name: "Delete_runtime_policy", Description: "Deletes a runtime policy from the verifier by name. Use List_runtime_policies first to see available names."}, toolHandler.DeleteRuntimePolicy)
	mcp.AddTool(server, &mcp.Tool{Name: "List_mb_policies", Description: "Lists names of measured boot policies already uploaded to the verifier. These are policies available for assigning to agents during enrollment."}, toolHandler.ListMBPolicies)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_mb_policy", Description: "Gets the content of a specific measured boot policy stored on the verifier by name. Returns the policy JSON including boot event logs and expected PCR values. Use List_mb_policies first to see available names."}, toolHandler.GetMBPolicy)
	mcp.AddTool(server, &mcp.Tool{Name: "Import_mb_policy", Description: "Uploads a local measured boot policy JSON file to the verifier. If the user has no policy file, tell them to generate one with: 'sudo keylime-policy create measured-boot -e /sys/kernel/security/tpm0/binary_bios_measurements -o /tmp/mb_policy.json'. If it fails with a SecureBoot error, add the -i flag to generate without SecureBoot validation. Then provide the output path to this tool."}, toolHandler.ImportMBPolicy)
	mcp.AddTool(server, &mcp.Tool{Name: "Delete_mb_policy", Description: "Deletes a measured boot policy from the verifier by name. Use List_mb_policies first to see available names."}, toolHandler.DeleteMBPolicy)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_verifier_logs", Description: "Investigates attestation failures and retrieves Keylime Verifier logs from journalctl. Requires co-located verifier. Filter by agent_uuid and use filter parameter: 'attestation_failures' for file mismatches, invalid quotes and policy violations, 'errors' for error-level messages, 'all' for unfiltered output (default). Lines parameter controls log window (default 50, max 200)."}, toolHandler.InvestigateVerifierLogs)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() keylime.Config {
	certDir := getEnv("KEYLIME_CERT_DIR", "/var/lib/keylime/cv_ca")

	return keylime.Config{
		VerifierURL:   getEnv("KEYLIME_VERIFIER_URL", "https://localhost:8881"),
		RegistrarURL:  getEnv("KEYLIME_REGISTRAR_URL", "https://localhost:8891"),
		CertDir:       certDir,
		TLSEnabled:    getEnv("KEYLIME_TLS_ENABLED", "true") == "true",
		TLSServerName: getEnv("KEYLIME_TLS_SERVER_NAME", "localhost"),
		APIVersion:    getEnv("KEYLIME_API_VERSION", "v2.5"),
		ClientCert:    getEnv("KEYLIME_CLIENT_CERT", certDir+"/client-cert.crt"),
		ClientKey:     getEnv("KEYLIME_CLIENT_KEY", certDir+"/client-private.pem"),
		CAPath:        getEnv("KEYLIME_CA_CERT", certDir+"/cacert.crt"),
		Port:          getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
