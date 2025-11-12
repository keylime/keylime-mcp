package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	keylimeVerifierClient  *KeylimeClient
	keylimeRegistrarClient *KeylimeClient

	config Config
)

func main() {
	err1 := godotenv.Load(".env")
	err2 := godotenv.Load("../.env")
	if err1 != nil && err2 != nil {
		log.Printf("No .env file found, using defaults")
	}
	loadConfig()
	keylimeVerifierClient = newKeylimeClient(config.VerifierURL)
	keylimeRegistrarClient = newKeylimeClient(config.RegistrarURL)

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/agents", getAllAgentsHandler)

	runMode := getEnv("RUN_MODE", "mcp")

	if runMode == "http" {
		// HTTP-only mode (for containers)
		log.Printf("Starting HTTP server on :%s", config.Port)
		log.Fatal(http.ListenAndServe(":"+config.Port, nil))
	} else {
		// MCP mode (for Claude Desktop)
		go func() {
			log.Printf("HTTP test server: http://localhost:%s", config.Port)
			if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
				log.Printf("HTTP server error: %v", err)
			}
		}()

		server := mcp.NewServer(&mcp.Implementation{Name: "Keylime", Version: "v1.0.0"}, nil)
		mcp.AddTool(server, &mcp.Tool{Name: "Get_all_agents", Description: "Retrieves a list of all registered agent UUIDs"}, getAllAgents)
		mcp.AddTool(server, &mcp.Tool{Name: "Get_agent_status", Description: "Retrieves the current status information for a specific agent identified by its UUID"}, getAgentStatus)
		mcp.AddTool(server, &mcp.Tool{Name: "Get_failed_agents", Description: "Retrieves all agents currently in a failed operational state with their detailed status information including attestation history and failure reasons"}, getFailedAgents)
		mcp.AddTool(server, &mcp.Tool{Name: "Reactivate_agent", Description: "Reactivates a failed agent identified by its UUID"}, reactivate_agent)
		mcp.AddTool(server, &mcp.Tool{Name: "Get_agent_policies", Description: "Retrieves policy configuration (TPM, vTPM, runtime policies) for a specific agent"}, agent_policies)
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
	}
}

func loadConfig() {
	certDir := getEnv("KEYLIME_CERT_DIR", "/var/lib/keylime/cv_ca")

	config = Config{
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
