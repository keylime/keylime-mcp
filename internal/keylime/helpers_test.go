package keylime

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchAllAgentUUIDs(t *testing.T) {
	t.Run("returns uuid list", func(t *testing.T) {
		data := loadTestdata(t, "agent_list.json")
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
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
		data := loadTestdata(t, "agent_status.json")
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
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
		data := loadTestdata(t, "agent_status_failed.json")
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		}))

		status, err := svc.FetchAgentDetails(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
		require.NoError(t, err)
		assert.Equal(t, StateFailed, status.Results.OperationalState)
		assert.True(t, IsFailedState(status.Results.OperationalState))
	})

	t.Run("non-200 response returns error", func(t *testing.T) {
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"status":"agent not found"}`))
		}))

		_, err := svc.FetchAgentDetails(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent not found")
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
		data := loadTestdata(t, "registrar_agent_details.json")
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
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
		regData := loadTestdata(t, "registrar_agent_details.json")
		policyData := loadTestdata(t, "runtime_policy.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(regData)
		})
		mux.HandleFunc("GET /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(policyData)
		})
		svc := newTestService(t, mux)

		body, err := svc.PrepareEnrollmentBody(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", "test-policy", "")
		require.NoError(t, err)
		assert.NotEmpty(t, body["runtime_policy"]) // base64 encoded
		assert.Equal(t, `{"mask":"0x400"}`, body["tpm_policy"])
	})

	t.Run("with mb policy", func(t *testing.T) {
		data := loadTestdata(t, "registrar_agent_details.json")
		svc := newTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		}))

		body, err := svc.PrepareEnrollmentBody(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", "", "mb-policy")
		require.NoError(t, err)
		assert.Equal(t, "mb-policy", body["mb_policy_name"])
		// PCR mask includes PCRs 0-9, 11-15 = 0xfbff
		assert.Equal(t, `{"mask":"0xfbff"}`, body["tpm_policy"])
	})

	t.Run("with both policies", func(t *testing.T) {
		regData := loadTestdata(t, "registrar_agent_details.json")
		policyData := loadTestdata(t, "runtime_policy.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(regData)
		})
		mux.HandleFunc("GET /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(policyData)
		})
		svc := newTestService(t, mux)

		body, err := svc.PrepareEnrollmentBody(context.Background(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", "test-policy", "mb-policy")
		require.NoError(t, err)
		assert.NotEmpty(t, body["runtime_policy"])
		assert.Equal(t, "mb-policy", body["mb_policy_name"])
		// combined mask: PCRs 0-10, 11-15 = 0xffff
		assert.Equal(t, `{"mask":"0xffff"}`, body["tpm_policy"])
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

func TestSanitizeErrorBody(t *testing.T) {
	t.Run("short printable body unchanged", func(t *testing.T) {
		assert.Equal(t, "hello world", sanitizeErrorBody([]byte("hello world")))
	})

	t.Run("strips non-printable characters", func(t *testing.T) {
		assert.Equal(t, "hello", sanitizeErrorBody([]byte("he\x00ll\x01o")))
	})

	t.Run("preserves newlines and tabs", func(t *testing.T) {
		assert.Equal(t, "line1\nline2\tend", sanitizeErrorBody([]byte("line1\nline2\tend")))
	})

	t.Run("truncates at 512 bytes", func(t *testing.T) {
		long := strings.Repeat("a", 600)
		result := sanitizeErrorBody([]byte(long))
		assert.Contains(t, result, "... (truncated)")
		assert.LessOrEqual(t, len(result), 512+len("... (truncated)"))
	})

	t.Run("empty body", func(t *testing.T) {
		assert.Equal(t, "", sanitizeErrorBody([]byte{}))
	})

	t.Run("exactly 512 bytes not truncated", func(t *testing.T) {
		exact := strings.Repeat("a", 512)
		result := sanitizeErrorBody([]byte(exact))
		assert.NotContains(t, result, "truncated")
		assert.Equal(t, 512, len(result))
	})
}

func TestExtractAPIError(t *testing.T) {
	t.Run("json with status field", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(strings.NewReader(`{"status":"agent not found"}`)),
		}
		err := ExtractAPIError(resp)
		assert.EqualError(t, err, "API error (HTTP 404): agent not found")
	})

	t.Run("non-json body", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader("Internal Server Error")),
		}
		err := ExtractAPIError(resp)
		assert.EqualError(t, err, "API request failed with HTTP 500: Internal Server Error")
	})

	t.Run("json without status field", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader(`{"error":"something broke"}`)),
		}
		err := ExtractAPIError(resp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "API request failed with HTTP 500")
	})

	t.Run("large body does not OOM", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 20*1024))),
		}
		err := ExtractAPIError(resp)
		require.Error(t, err)
		assert.Less(t, len(err.Error()), 20*1024)
	})
}
