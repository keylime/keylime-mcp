package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Status:  "healthy",
		Service: "keylime-mcp-backend",
	}

	json.NewEncoder(w).Encode(response)
}

func getAllAgentsHandler(w http.ResponseWriter, r *http.Request) {
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
