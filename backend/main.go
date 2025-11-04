package main

import (
	"context"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	keylimeVerifierClient  *KeylimeClient
	keylimeRegistrarClient *KeylimeClient
)

func main() {
	err1 := godotenv.Load(".env")
	err2 := godotenv.Load("../.env")
	if err1 != nil && err2 != nil {
		log.Printf("No .env file found, using defaults")
	}
	verifierBaseURL := getEnv("KEYLIME_VERIFIER_URL", "https://localhost:8881")
	registrarBaseURL := getEnv("KEYLIME_REGISTRAR_URL", "https://localhost:8891")
	keylimeVerifierClient = newKeylimeClient(verifierBaseURL)
	keylimeRegistrarClient = newKeylimeClient(registrarBaseURL)

	port := getEnv("PORT", "8080")
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/agents", getAllAgentsHandler)

	runMode := getEnv("RUN_MODE", "mcp")

	if runMode == "http" {
		// HTTP-only mode (for containers)
		log.Printf("Starting HTTP server on :%s", port)
		log.Fatal(http.ListenAndServe(":"+port, nil))
	} else {
		// MCP mode (for Claude Desktop)
		go func() {
			log.Printf("HTTP test server: http://localhost:%s", port)
			if err := http.ListenAndServe(":"+port, nil); err != nil {
				log.Printf("HTTP server error: %v", err)
			}
		}()

		server := mcp.NewServer(&mcp.Implementation{Name: "Keylime", Version: "v1.0.0"}, nil)
		mcp.AddTool(server, &mcp.Tool{Name: "Get_all_agents", Description: "Retrieves a list of all registered agent UUIDs"}, getAllAgents)
		mcp.AddTool(server, &mcp.Tool{Name: "Get_agent_status", Description: "Retrieves the current status information for a specific agent identified by its UUID"}, getAgentStatus)
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
	}
}
