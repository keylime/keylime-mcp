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
	apiVersion := getEnv("KEYLIME_API_VERSION", "v2.3")

	// Remove "http(s)://" prefix if present
	baseURL = strings.TrimPrefix(baseURL, "https://")
	baseURL = strings.TrimPrefix(baseURL, "http://")

	// Check if TLS is enabled
	tlsEnabled := getEnv("KEYLIME_TLS_ENABLED", "true") == "true"

	var finalURL string
	var httpClient *http.Client

	if tlsEnabled {
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
		apiVersion: apiVersion,
		httpClient: httpClient,
	}
}

// createTLSConfig creates TLS configuration with mTLS support
// Equivalent to Python's HostNameIgnoreAdapter with SSL context
func createTLSConfig() *tls.Config {
	// Default certificate paths (same as Keylime default)
	certDir := getEnv("KEYLIME_CERT_DIR", "/var/lib/keylime/cv_ca")
	clientCert := getEnv("KEYLIME_CLIENT_CERT", certDir+"/client-cert.crt")
	clientKey := getEnv("KEYLIME_CLIENT_KEY", certDir+"/client-private.pem")
	caCert := getEnv("KEYLIME_CA_CERT", certDir+"/cacert.crt")

	// Load client certificate and key
	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		log.Printf("Warning: Failed to load client certificate: %v", err)
		log.Printf("Attempting to connect without client cert (may fail with mTLS servers)")
		// Return basic TLS config without client cert
		return &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// Load CA certificate
	caCertPEM, err := os.ReadFile(caCert)
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

	// Check if hostname verification should be ignored
	ignoreHostname := getEnv("KEYLIME_IGNORE_HOSTNAME", "true") == "true"

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		// Ignore hostname verification (like Python's HostNameIgnoreAdapter)
		// This is needed because Keylime certs often don't have correct hostname
		InsecureSkipVerify: ignoreHostname,
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
