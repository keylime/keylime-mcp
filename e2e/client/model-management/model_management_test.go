//go:build functional

package clientmodelmanagement_test

import (
	"encoding/json"
	"testing"

	"github.com/keylime/keylime-mcp/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelManagement(t *testing.T) {
	c := helpers.NewMCPTestClient(t, nil)

	t.Run("list_models", func(t *testing.T) {
		resp := c.GetModels()
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var body struct {
			Models []struct {
				ID       string `json:"id"`
				Provider string `json:"provider"`
			} `json:"models"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

		found := false
		for _, m := range body.Models {
			if m.ID == "mock-model:latest" {
				assert.Equal(t, "ollama", m.Provider)
				found = true
			}
		}
		assert.True(t, found, "mock-model:latest not found in models list")
	})

	t.Run("get_model", func(t *testing.T) {
		resp := c.GetModel()
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		var body map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "mock-model:latest", body["model"])
		assert.Equal(t, "ollama", body["provider"])
	})

	t.Run("set_model", func(t *testing.T) {
		resp := c.SetModel("ollama", "mock-model:latest")
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)

		resp2 := c.GetModel()
		defer resp2.Body.Close()
		var body map[string]string
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&body))
		assert.Equal(t, "mock-model:latest", body["model"])
	})

	t.Run("invalid_provider", func(t *testing.T) {
		resp := c.SetModel("nonexistent", "x")
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)
	})
}
