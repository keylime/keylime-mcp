package keylime

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// newClient creates HTTP client for Keylime API with mTLS support
func newClient(baseURL string, config *Config) (*Client, error) {
	baseURL = strings.TrimPrefix(baseURL, "https://")
	baseURL = strings.TrimPrefix(baseURL, "http://")

	if !config.TLSEnabled {
		return &Client{
			baseURL:    "http://" + strings.TrimSuffix(baseURL, "/"),
			APIVersion: config.APIVersion,
			httpClient: &http.Client{Timeout: 30 * time.Second},
		}, nil
	}

	tlsConfig, err := createTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("TLS configuration failed: %w", err)
	}

	return &Client{
		baseURL:    "https://" + strings.TrimSuffix(baseURL, "/"),
		APIVersion: config.APIVersion,
		httpClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
			Timeout:   30 * time.Second,
		},
	}, nil
}

// createTLSConfig creates TLS configuration with mTLS support
func createTLSConfig(config *Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate (%s, %s): %w", config.ClientCert, config.ClientKey, err)
	}

	caCertPEM, err := os.ReadFile(config.CAPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate (%s): %w", config.CAPath, err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate from %s", config.CAPath)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	if config.TLSServerName != "" {
		tlsConfig.ServerName = config.TLSServerName
	}

	return tlsConfig, nil
}

func (kc *Client) Get(endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", kc.baseURL, kc.APIVersion, strings.TrimPrefix(endpoint, "/"))
	return kc.httpClient.Get(url) // #nosec G704 -- URL is built from trusted config, not user input
}

func (kc *Client) doRequestWithBody(method, endpoint string, body any) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", kc.baseURL, kc.APIVersion, strings.TrimPrefix(endpoint, "/"))
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return kc.httpClient.Do(req) // #nosec G704 -- URL is built from trusted config, not user input
}

func (kc *Client) Post(endpoint string, body any) (*http.Response, error) {
	return kc.doRequestWithBody("POST", endpoint, body)
}

func (kc *Client) Put(endpoint string, body any) (*http.Response, error) {
	return kc.doRequestWithBody("PUT", endpoint, body)
}

func (kc *Client) Delete(endpoint string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", kc.baseURL, kc.APIVersion, strings.TrimPrefix(endpoint, "/"))
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, err
	}
	return kc.httpClient.Do(req) // #nosec G704 -- URL is built from trusted config, not user input
}

// GetRaw sends a GET without the API version prefix. Used for /version endpoint.
func (kc *Client) GetRaw(path string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s", kc.baseURL, strings.TrimPrefix(path, "/"))
	return kc.httpClient.Get(url)
}
