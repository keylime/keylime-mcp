package main

import "net/http"

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type KeylimeClient struct {
	baseURL    string
	apiVersion string
	httpClient *http.Client
}

type getAllAgentsInput struct{}

type getAllAgentsOutput struct {
	Agents []string `json:"agents"`
}

type keylimeAgentListResponse struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Results struct {
		UUIDs []string `json:"uuids"`
	} `json:"results"`
}
