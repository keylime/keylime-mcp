package keylime

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchAllAgentUUIDs(t *testing.T) {
	t.Run("returns uuid list", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(loadTestdata(t, "agent_list.json"))
		}))

		uuids, err := svc.FetchAllAgentUUIDs(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []string{
			"d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
			"d432fbb3-d2f1-4a97-9ef7-75bd81c11111",
		}, uuids)
	})

	t.Run("empty list returns empty slice", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"code":200,"status":"Success","results":{"uuids":null}}`))
		}))

		uuids, err := svc.FetchAllAgentUUIDs(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []string{}, uuids)
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))

		_, err := svc.FetchAllAgentUUIDs(context.Background())
		assert.Error(t, err)
	})
}

func TestFetchAgentDetails(t *testing.T) {
	t.Run("returns agent status", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(loadTestdata(t, "agent_status.json"))
		}))

		status, err := svc.FetchAgentDetails(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
		require.NoError(t, err)
		assert.Equal(t, 200, status.Code)
		assert.Equal(t, StateGetQuote, status.Results.OperationalState)
		assert.Equal(t, 42, status.Results.AttestationCount)
		assert.Equal(t, "sha256", status.Results.HashAlg)
		assert.Equal(t, "127.0.0.1", status.Results.VerifierIP)
		assert.Equal(t, 8881, status.Results.VerifierPort)
	})

	t.Run("failed agent status", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(loadTestdata(t, "agent_status_failed.json"))
		}))

		status, err := svc.FetchAgentDetails(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
		require.NoError(t, err)
		assert.Equal(t, StateFailed, status.Results.OperationalState)
		assert.True(t, IsFailedState(status.Results.OperationalState))
	})

	t.Run("non-200 response decoded with zero code", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"status":"agent not found"}`))
		}))

		// FetchAgentDetails does not check HTTP status; 404 with valid JSON decodes to Code=0.
		status, err := svc.FetchAgentDetails(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
		require.NoError(t, err)
		assert.Equal(t, 0, status.Code)
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))

		_, err := svc.FetchAgentDetails(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
		assert.Error(t, err)
	})
}

func TestPrepareEnrollmentBody(t *testing.T) {
	t.Run("no policies", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(loadTestdata(t, "registrar_agent_details.json"))
		}))

		body, err := svc.PrepareEnrollmentBody(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", "", "")
		require.NoError(t, err)
		assert.Equal(t, "192.168.1.100", body["cloudagent_ip"])
		assert.Equal(t, 9002, body["cloudagent_port"])
		assert.Equal(t, `{"mask":"0x0"}`, body["tpm_policy"])
		assert.Equal(t, "", body["runtime_policy"])
		assert.Equal(t, "", body["mb_policy_name"])
		assert.Equal(t, "test-aik-data", body["ak_tpm"])
		assert.Equal(t, "test-mtls-cert", body["mtls_cert"])
	})

	t.Run("with runtime policy", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(loadTestdata(t, "registrar_agent_details.json"))
		})
		mux.HandleFunc("GET /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(loadTestdata(t, "runtime_policy.json"))
		})
		svc := newTestService(t, mux)

		body, err := svc.PrepareEnrollmentBody(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", "test-policy", "")
		require.NoError(t, err)
		assert.NotEmpty(t, body["runtime_policy"]) // base64 encoded
		assert.Contains(t, body["tpm_policy"].(string), "0x400")
	})

	t.Run("with mb policy", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(loadTestdata(t, "registrar_agent_details.json"))
		}))

		body, err := svc.PrepareEnrollmentBody(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", "", "mb-policy")
		require.NoError(t, err)
		assert.Equal(t, "mb-policy", body["mb_policy_name"])
		// PCR mask includes PCRs 0-9, 11-15 = 0xfbff
		assert.Contains(t, body["tpm_policy"].(string), "0xfbff")
	})

	t.Run("with both policies", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(loadTestdata(t, "registrar_agent_details.json"))
		})
		mux.HandleFunc("GET /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(loadTestdata(t, "runtime_policy.json"))
		})
		svc := newTestService(t, mux)

		body, err := svc.PrepareEnrollmentBody(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", "test-policy", "mb-policy")
		require.NoError(t, err)
		assert.NotEmpty(t, body["runtime_policy"])
		assert.Equal(t, "mb-policy", body["mb_policy_name"])
		// combined mask: PCRs 0-10, 11-15 = 0xffff
		assert.Contains(t, body["tpm_policy"].(string), "0xffff")
	})

	t.Run("registrar unreachable returns error", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}))

		_, err := svc.PrepareEnrollmentBody(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", "", "")
		assert.Error(t, err)
	})
}
