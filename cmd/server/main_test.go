package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_KEYLIME_MCP_KEY", "test_value")
		assert.Equal(t, "test_value", getEnv("TEST_KEYLIME_MCP_KEY", "default"))
	})

	t.Run("returns default when unset", func(t *testing.T) {
		assert.Equal(t, "default", getEnv("KEYLIME_MCP_NONEXISTENT_VAR", "default"))
	})

	t.Run("returns default when empty string", func(t *testing.T) {
		t.Setenv("TEST_KEYLIME_MCP_EMPTY", "")
		assert.Equal(t, "default", getEnv("TEST_KEYLIME_MCP_EMPTY", "default"))
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		// reset to defaults
		for _, key := range []string{
			"KEYLIME_VERIFIER_URL", "KEYLIME_REGISTRAR_URL", "KEYLIME_CERT_DIR",
			"KEYLIME_TLS_ENABLED", "KEYLIME_TLS_SERVER_NAME", "KEYLIME_API_VERSION",
			"KEYLIME_CLIENT_CERT", "KEYLIME_CLIENT_KEY", "KEYLIME_CA_CERT", "PORT",
		} {
			t.Setenv(key, "")
		}

		config := loadConfig()

		assert.Equal(t, "https://localhost:8881", config.VerifierURL)
		assert.Equal(t, "https://localhost:8891", config.RegistrarURL)
		assert.Equal(t, "/var/lib/keylime/cv_ca", config.CertDir)
		assert.True(t, config.TLSEnabled)
		assert.Equal(t, "localhost", config.TLSServerName)
		assert.Equal(t, "v2.5", config.APIVersion)
		assert.Equal(t, "/var/lib/keylime/cv_ca/client-cert.crt", config.ClientCert)
		assert.Equal(t, "/var/lib/keylime/cv_ca/client-private.pem", config.ClientKey)
		assert.Equal(t, "/var/lib/keylime/cv_ca/cacert.crt", config.CAPath)
		assert.Equal(t, "8080", config.Port)
	})

	t.Run("env vars override defaults", func(t *testing.T) {
		t.Setenv("KEYLIME_VERIFIER_URL", "https://custom:9999")
		t.Setenv("KEYLIME_REGISTRAR_URL", "https://custom:9998")
		t.Setenv("KEYLIME_CERT_DIR", "/custom/certs")
		t.Setenv("KEYLIME_TLS_ENABLED", "false")
		t.Setenv("KEYLIME_API_VERSION", "v3.0")
		t.Setenv("PORT", "9090")
		// rest use defaults
		t.Setenv("KEYLIME_TLS_SERVER_NAME", "")
		t.Setenv("KEYLIME_CLIENT_CERT", "")
		t.Setenv("KEYLIME_CLIENT_KEY", "")
		t.Setenv("KEYLIME_CA_CERT", "")

		config := loadConfig()

		assert.Equal(t, "https://custom:9999", config.VerifierURL)
		assert.Equal(t, "https://custom:9998", config.RegistrarURL)
		assert.Equal(t, "/custom/certs", config.CertDir)
		assert.False(t, config.TLSEnabled)
		assert.Equal(t, "v3.0", config.APIVersion)
		assert.Equal(t, "9090", config.Port)
		assert.Equal(t, "localhost", config.TLSServerName)
		assert.Equal(t, "/custom/certs/client-cert.crt", config.ClientCert)
		assert.Equal(t, "/custom/certs/client-private.pem", config.ClientKey)
		assert.Equal(t, "/custom/certs/cacert.crt", config.CAPath)
	})

	t.Run("cert paths derive from cert dir", func(t *testing.T) {
		t.Setenv("KEYLIME_CERT_DIR", "/custom/certs")
		// no cert overrides
		t.Setenv("KEYLIME_CLIENT_CERT", "")
		t.Setenv("KEYLIME_CLIENT_KEY", "")
		t.Setenv("KEYLIME_CA_CERT", "")
		// rest use defaults
		for _, key := range []string{
			"KEYLIME_VERIFIER_URL", "KEYLIME_REGISTRAR_URL",
			"KEYLIME_TLS_ENABLED", "KEYLIME_TLS_SERVER_NAME",
			"KEYLIME_API_VERSION", "PORT",
		} {
			t.Setenv(key, "")
		}

		config := loadConfig()

		assert.Equal(t, "/custom/certs/client-cert.crt", config.ClientCert)
		assert.Equal(t, "/custom/certs/client-private.pem", config.ClientKey)
		assert.Equal(t, "/custom/certs/cacert.crt", config.CAPath)
	})
}
