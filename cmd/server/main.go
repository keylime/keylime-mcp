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
	keylimeService := keylime.NewService(&config)
	toolHandler := mcptools.NewToolHandler(keylimeService)

	server := mcp.NewServer(&mcp.Implementation{Name: "Keylime", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_all_agents", Description: "Retrieves a list of all registered agent UUIDs"}, toolHandler.GetAllAgents)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_agent_status", Description: "Retrieves the current status information for a specific agent identified by its UUID"}, toolHandler.GetAgentStatus)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_failed_agents", Description: "Retrieves all agents currently in a failed operational state with their detailed status information including attestation history and failure reasons"}, toolHandler.GetFailedAgents)
	mcp.AddTool(server, &mcp.Tool{Name: "Reactivate_agent", Description: "Reactivates a failed agent identified by its UUID"}, toolHandler.ReactivateAgent)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_agent_policies", Description: "Retrieves policy configuration (TPM, vTPM, runtime policies) for a specific agent"}, toolHandler.AgentPolicies)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}

}

func loadConfig() keylime.Config {
	certDir := getEnv("KEYLIME_CERT_DIR", "/var/lib/keylime/cv_ca")

	return keylime.Config{
		VerifierURL:    getEnv("KEYLIME_VERIFIER_URL", "https://localhost:8881"),
		RegistrarURL:   getEnv("KEYLIME_REGISTRAR_URL", "https://localhost:8891"),
		CertDir:        certDir,
		TLSEnabled:     getEnv("KEYLIME_TLS_ENABLED", "true") == "true",
		IgnoreHostname: getEnv("KEYLIME_IGNORE_HOSTNAME", "true") == "true",
		APIVersion:     getEnv("KEYLIME_API_VERSION", "v2.3"),
		ClientCert:     getEnv("KEYLIME_CLIENT_CERT", certDir+"/client-cert.crt"),
		ClientKey:      getEnv("KEYLIME_CLIENT_KEY", certDir+"/client-private.pem"),
		CAPath:         getEnv("KEYLIME_CA_CERT", certDir+"/cacert.crt"),
		Port:           getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
