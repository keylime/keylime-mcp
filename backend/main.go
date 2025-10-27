package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
func newKeylimeClient(baseURL, apiVersion string) *KeylimeClient {
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
		http.Error(w, "Failed to fetch agents: " + err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var agents interface{}
	err = json.NewDecoder(resp.Body).Decode(&agents); 
	if err != nil {
		log.Printf("Error decoding agents: %v", err)
		http.Error(w, "Failed to decode agents response" + err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(agents)
}

func main() {
	port := getEnv("PORT", "8080")
	keylimeURL := getEnv("KEYLIME_VERIFIER_URL", "https://localhost:8891")
	apiVersion := getEnv("KEYLIME_API_VERSION", "v2.3")

	keylimeClient = newKeylimeClient(keylimeURL, apiVersion)

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/agents", getAllAgents)

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

