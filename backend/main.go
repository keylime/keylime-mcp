package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"crypto/tls"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	
	response := HealthResponse{
		Status:  "healthy",
		Service: "keylime-mcp-backend",
	}
	
	json.NewEncoder(w).Encode(response)
}

func getAllAgents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Get("https://localhost:8891/v2.3/agents")
	if err != nil {
		http.Error(w, "Failed to fetch agents", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var agents interface{}
	err = json.NewDecoder(resp.Body).Decode(&agents)
	if err != nil {
		http.Error(w, "Failed to decode agents response", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(agents)
}

func main() {
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/agents", getAllAgents)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

