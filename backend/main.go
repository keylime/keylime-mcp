package main

import (
	"context"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var keylimeClient *KeylimeClient

func main() {
	port := getEnv("PORT", "8080")
	keylimeClient = newKeylimeClient()

	server := mcp.NewServer(&mcp.Implementation{Name: "Keylime", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "Get_all_agents", Description: "Retrieves a list of all registered agent UUIDs"}, getAllAgents)
	//mcp.AddTool(server, &mcp.Tool{Name: "Get_agent_status", Description: "Retrieves the current status information for a specific agent identified by its UUID"}, getAgentStatus)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/agents", getAllAgentsHandler)
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
