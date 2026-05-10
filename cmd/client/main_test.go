package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const testAnthropicKey = "sk-test"

func TestGetEnv(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_KEYLIME_CLIENT_KEY", "test_value")
		assert.Equal(t, "test_value", getEnv("TEST_KEYLIME_CLIENT_KEY", "default"))
	})

	t.Run("returns default when unset", func(t *testing.T) {
		assert.Equal(t, "default", getEnv("KEYLIME_MCP_NONEXISTENT_VAR", "default"))
	})

	t.Run("returns default when empty string", func(t *testing.T) {
		t.Setenv("TEST_KEYLIME_CLIENT_EMPTY", "")
		assert.Equal(t, "default", getEnv("TEST_KEYLIME_CLIENT_EMPTY", "default"))
	})
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"", false},
		{"invalid", false},
		{"TRUE", true},
		{"True", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseBool(tt.input))
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		for _, key := range []string{
			"MCP_SERVER_PATH", "PORT", "OLLAMA_URL", "OLLAMA_MODEL",
			"ANTHROPIC_API_KEY", "MASKING_ENABLED",
		} {
			t.Setenv(key, "")
		}

		cfg := loadConfig()

		assert.Equal(t, "./server", cfg.ServerPath)
		assert.Equal(t, "3000", cfg.Port)
		assert.Equal(t, "http://localhost:11434", cfg.OllamaURL)
		assert.Equal(t, "", cfg.OllamaModel)
		assert.Equal(t, "", cfg.AnthropicKey)
		assert.True(t, cfg.MaskingEnabled)
	})

	t.Run("env vars override defaults", func(t *testing.T) {
		t.Setenv("MCP_SERVER_PATH", "/custom/server")
		t.Setenv("PORT", "8080")
		t.Setenv("OLLAMA_URL", "http://custom:11434")
		t.Setenv("OLLAMA_MODEL", "llama3")
		t.Setenv("ANTHROPIC_API_KEY", "sk-test-key")
		t.Setenv("MASKING_ENABLED", "false")

		cfg := loadConfig()

		assert.Equal(t, "/custom/server", cfg.ServerPath)
		assert.Equal(t, "8080", cfg.Port)
		assert.Equal(t, "http://custom:11434", cfg.OllamaURL)
		assert.Equal(t, "llama3", cfg.OllamaModel)
		assert.Equal(t, "sk-test-key", cfg.AnthropicKey)
		assert.False(t, cfg.MaskingEnabled)
	})

	t.Run("anthropic key whitespace trimmed", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "  sk-test-key  \n")
		t.Setenv("MCP_SERVER_PATH", "")
		t.Setenv("PORT", "")
		t.Setenv("OLLAMA_URL", "")
		t.Setenv("OLLAMA_MODEL", "")
		t.Setenv("MASKING_ENABLED", "")

		cfg := loadConfig()
		assert.Equal(t, "sk-test-key", cfg.AnthropicKey)
	})
}

func TestCreateProviders(t *testing.T) {
	t.Run("anthropic only", func(t *testing.T) {
		t.Setenv("OLLAMA_URL", "")
		t.Setenv("OLLAMA_MODEL", "")

		cfg := config{
			AnthropicKey: testAnthropicKey,
			OllamaURL:    "http://localhost:11434",
		}

		providers, initial, model := createProviders(cfg)

		assert.Len(t, providers, 2)
		assert.Equal(t, "anthropic", initial.Name())
		assert.Equal(t, "", model)
	})

	t.Run("ollama explicit via OLLAMA_URL env", func(t *testing.T) {
		t.Setenv("OLLAMA_URL", "http://custom:11434")
		t.Setenv("OLLAMA_MODEL", "")

		cfg := config{
			AnthropicKey: testAnthropicKey,
			OllamaURL:    "http://custom:11434",
		}

		_, initial, _ := createProviders(cfg)
		assert.Equal(t, "ollama", initial.Name())
	})

	t.Run("ollama with model", func(t *testing.T) {
		t.Setenv("OLLAMA_URL", "")
		t.Setenv("OLLAMA_MODEL", "")

		cfg := config{
			AnthropicKey: testAnthropicKey,
			OllamaURL:    "http://localhost:11434",
			OllamaModel:  "llama3",
		}

		_, initial, model := createProviders(cfg)
		assert.Equal(t, "ollama", initial.Name())
		assert.Equal(t, "llama3", model)
	})

	t.Run("both providers in list", func(t *testing.T) {
		t.Setenv("OLLAMA_URL", "")
		t.Setenv("OLLAMA_MODEL", "")

		cfg := config{
			AnthropicKey: testAnthropicKey,
			OllamaURL:    "http://localhost:11434",
		}

		providers, _, _ := createProviders(cfg)

		names := make([]string, len(providers))
		for i, p := range providers {
			names[i] = p.Name()
		}
		assert.Contains(t, names, "anthropic")
		assert.Contains(t, names, "ollama")
	})
}
