package keylime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.Handler) *Client {
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client, err := newClient(ts.URL, &Config{
		TLSEnabled: false,
		APIVersion: "v2.5",
	})
	require.NoError(t, err)
	return client
}

func TestClientGet(t *testing.T) {
	t.Run("correct URL construction", func(t *testing.T) {
		var receivedPath string
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))

		resp, err := client.Get(context.Background(), "agents/test-uuid")
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, "/v2.5/agents/test-uuid", receivedPath)
	})

	t.Run("strips leading slash", func(t *testing.T) {
		var receivedPath string
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))

		resp, err := client.Get(context.Background(), "/agents/test-uuid")
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, "/v2.5/agents/test-uuid", receivedPath)
	})

	t.Run("uses GET method", func(t *testing.T) {
		var receivedMethod string
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			w.WriteHeader(http.StatusOK)
		}))

		resp, err := client.Get(context.Background(), "agents")
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, "GET", receivedMethod)
	})

	t.Run("context cancellation", func(t *testing.T) {
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		resp, err := client.Get(ctx, "agents")
		if resp != nil {
			_ = resp.Body.Close()
		}
		assert.Error(t, err)
	})
}

func TestClientPost(t *testing.T) {
	t.Run("sends json body with correct content type", func(t *testing.T) {
		var receivedBody map[string]any
		var receivedContentType string

		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &receivedBody)
			w.WriteHeader(http.StatusOK)
		}))

		body := map[string]string{"key": "value"}
		resp, err := client.Post(context.Background(), "agents/uuid", body)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, "application/json", receivedContentType)
		assert.Equal(t, "value", receivedBody["key"])
	})

	t.Run("uses POST method", func(t *testing.T) {
		var receivedMethod string
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			w.WriteHeader(http.StatusOK)
		}))

		resp, err := client.Post(context.Background(), "agents/uuid", nil)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, "POST", receivedMethod)
	})
}

func TestClientPut(t *testing.T) {
	t.Run("uses PUT method", func(t *testing.T) {
		var receivedMethod string
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			w.WriteHeader(http.StatusOK)
		}))

		resp, err := client.Put(context.Background(), "agents/uuid/reactivate", nil)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, "PUT", receivedMethod)
	})
}

func TestClientDelete(t *testing.T) {
	t.Run("uses DELETE method and correct path", func(t *testing.T) {
		var receivedMethod, receivedPath string
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))

		resp, err := client.Delete(context.Background(), "agents/uuid")
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, "DELETE", receivedMethod)
		assert.Equal(t, "/v2.5/agents/uuid", receivedPath)
	})
}

func TestClientGetRaw(t *testing.T) {
	t.Run("skips api version prefix", func(t *testing.T) {
		var receivedPath string
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))

		resp, err := client.GetRaw(context.Background(), "version")
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, "/version", receivedPath)
	})
}

func TestNewClient(t *testing.T) {
	t.Run("tls disabled uses http scheme", func(t *testing.T) {
		client, err := newClient("localhost:8881", &Config{
			TLSEnabled: false,
			APIVersion: "v2.5",
		})
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:8881", client.baseURL)
	})

	t.Run("strips https prefix", func(t *testing.T) {
		client, err := newClient("https://localhost:8881", &Config{
			TLSEnabled: false,
			APIVersion: "v2.5",
		})
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:8881", client.baseURL)
	})

	t.Run("strips http prefix", func(t *testing.T) {
		client, err := newClient("http://localhost:8881", &Config{
			TLSEnabled: false,
			APIVersion: "v2.5",
		})
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:8881", client.baseURL)
	})

	t.Run("strips trailing slash", func(t *testing.T) {
		client, err := newClient("localhost:8881/", &Config{
			TLSEnabled: false,
			APIVersion: "v2.5",
		})
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:8881", client.baseURL)
	})

	t.Run("tls enabled with invalid certs returns error", func(t *testing.T) {
		_, err := newClient("localhost:8881", &Config{
			TLSEnabled: true,
			ClientCert: "/nonexistent/cert.pem",
			ClientKey:  "/nonexistent/key.pem",
			CAPath:     "/nonexistent/ca.pem",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TLS configuration failed")
	})

	t.Run("sets api version", func(t *testing.T) {
		client, err := newClient("localhost:8881", &Config{
			TLSEnabled: false,
			APIVersion: "v2.4",
		})
		require.NoError(t, err)
		assert.Equal(t, "v2.4", client.APIVersion)
	})
}
