package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaProviderName(t *testing.T) {
	p := NewOllamaProvider("http://localhost:11434")
	assert.Equal(t, "ollama", p.Name())
}

func TestOllamaListModels(t *testing.T) {
	t.Run("parses models", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/tags", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"models":[{"name":"llama3:latest"},{"name":"mistral:7b"}]}`))
		}))
		defer ts.Close()

		p := NewOllamaProvider(ts.URL)
		models, err := p.ListModels(context.Background())
		require.NoError(t, err)
		require.Len(t, models, 2)
		assert.Equal(t, "llama3:latest", models[0].ID)
		assert.Equal(t, "llama3:latest", models[0].DisplayName)
		assert.Equal(t, "ollama", models[0].Provider)
		assert.Equal(t, "mistral:7b", models[1].ID)
	})

	t.Run("empty models list", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"models":[]}`))
		}))
		defer ts.Close()

		p := NewOllamaProvider(ts.URL)
		models, err := p.ListModels(context.Background())
		require.NoError(t, err)
		assert.Empty(t, models)
	})

	t.Run("server error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`internal error`))
		}))
		defer ts.Close()

		p := NewOllamaProvider(ts.URL)
		_, err := p.ListModels(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("unreachable server", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		p := NewOllamaProvider("http://127.0.0.1:1")
		_, err := p.ListModels(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to reach Ollama")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not json`))
		}))
		defer ts.Close()

		p := NewOllamaProvider(ts.URL)
		_, err := p.ListModels(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse")
	})
}
