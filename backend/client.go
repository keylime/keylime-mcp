package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// newKeylimeClient creates HTTP client for Keylime API
func newKeylimeClient(baseURL string) *KeylimeClient {
	apiVersion := getEnv("KEYLIME_API_VERSION", "v2.3")
	return &KeylimeClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiVersion: apiVersion,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // TODO TMP
			},
		},
	}
}

// Performs GET request to Keylime API endpoint
func (kc *KeylimeClient) Get(endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", kc.baseURL, kc.apiVersion, strings.TrimPrefix(endpoint, "/"))
	return kc.httpClient.Get(url)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
