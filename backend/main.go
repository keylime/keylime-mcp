package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// KeylimeClient provides reusable HTTP client for Keylime API calls
type KeylimeClient struct {
	baseURL    string
	apiVersion string
	httpClient *http.Client
}

var keylimeClient *KeylimeClient

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Status:  "healthy",
		Service: "keylime-mcp-backend",
	}

	json.NewEncoder(w).Encode(response)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// newKeylimeClient creates HTTP client for Keylime API
func newKeylimeClient() *KeylimeClient {
	baseURL := getEnv("KEYLIME_VERIFIER_URL", "https://localhost:8891")
	apiVersion := getEnv("KEYLIME_API_VERSION", "v2.3")
	return &KeylimeClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiVersion: apiVersion,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// Performs GET request to Keylime API endpoint
func (kc *KeylimeClient) Get(endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", kc.baseURL, kc.apiVersion, strings.TrimPrefix(endpoint, "/"))
	return kc.httpClient.Get(url)
}

func getAllAgents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	resp, err := keylimeClient.Get("agents")
	if err != nil {
		log.Printf("Error fetching agents: %v", err)
		http.Error(w, "Failed to fetch agents: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var agents interface{}
	err = json.NewDecoder(resp.Body).Decode(&agents)
	if err != nil {
		log.Printf("Error decoding agents: %v", err)
		http.Error(w, "Failed to decode agents response"+err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(agents)
}

type Input struct {
	Name string `json:"name" jsonschema:"the name of the person to greet"`
}

type Output struct {
	Greeting string `json:"greeting" jsonschema:"the greeting to tell to the user"`
}

func SayHi(ctx context.Context, req *mcp.CallToolRequest, input Input) (
	*mcp.CallToolResult,
	Output,
	error,
) {
	return nil, Output{Greeting: "Hi " + input.Name}, nil
}

func main() {
	port := getEnv("PORT", "8080")
	keylimeClient = newKeylimeClient()

	server := mcp.NewServer(&mcp.Implementation{Name: "greeter", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/agents", getAllAgents)
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
