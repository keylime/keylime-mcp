package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// newKeylimeClient creates HTTP client for Keylime API with mTLS support
func newKeylimeClient(baseURL string) *KeylimeClient {
	// Remove "http(s)://" prefix if present
	baseURL = strings.TrimPrefix(baseURL, "https://")
	baseURL = strings.TrimPrefix(baseURL, "http://")

	var finalURL string
	var httpClient *http.Client

	if config.TLSEnabled {
		finalURL = "https://" + strings.TrimSuffix(baseURL, "/")
		tlsConfig := createTLSConfig()
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
	} else {
		finalURL = "http://" + strings.TrimSuffix(baseURL, "/")
		httpClient = &http.Client{}
	}

	return &KeylimeClient{
		baseURL:    finalURL,
		apiVersion: config.APIVersion,
		httpClient: httpClient,
	}
}

// createTLSConfig creates TLS configuration with mTLS support
// Equivalent to Python's HostNameIgnoreAdapter with SSL context
func createTLSConfig() *tls.Config {
	// Load client certificate and key
	cert, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
	if err != nil {
		log.Printf("Warning: Failed to load client certificate: %v", err)
		log.Printf("Attempting to connect without client cert (may fail with mTLS servers)")
		// Return basic TLS config without client cert
		return &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// Load CA certificate
	caCertPEM, err := os.ReadFile(config.CAPath)
	if err != nil {
		log.Printf("Warning: Failed to load CA certificate: %v", err)
		log.Printf("Using system CA pool")
	}

	// Create CA pool
	caCertPool := x509.NewCertPool()
	if caCertPEM != nil {
		if !caCertPool.AppendCertsFromPEM(caCertPEM) {
			log.Printf("Warning: Failed to append CA certificate to pool")
		}
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		// Ignore hostname verification (like Python's HostNameIgnoreAdapter)
		// This is needed because Keylime certs often don't have correct hostname
		InsecureSkipVerify: config.IgnoreHostname,
	}

	return tlsConfig
}

func (kc *KeylimeClient) Get(endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", kc.baseURL, kc.apiVersion, strings.TrimPrefix(endpoint, "/"))
	return kc.httpClient.Get(url)
}

func (kc *KeylimeClient) Post(endpoint string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", kc.baseURL, kc.apiVersion, strings.TrimPrefix(endpoint, "/"))
	// TODO: implement body marshaling
	return kc.httpClient.Post(url, "application/json", nil)
}

func (kc *KeylimeClient) Delete(endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", kc.baseURL, kc.apiVersion, strings.TrimPrefix(endpoint, "/"))
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, err
	}
	return kc.httpClient.Do(req)
}
