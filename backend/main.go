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
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/health", healthHandler)
	// http.HandleFunc("/agents", getAllAgents)
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
