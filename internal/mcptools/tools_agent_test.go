package mcptools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/keylime/keylime-mcp/internal/keylime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllAgents(t *testing.T) {
	t.Run("returns uuid list", func(t *testing.T) {
		data := loadTestdata(t, "agent_list.json")
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		}))

		_, output, err := h.GetAllAgents(context.Background(), nil, keylime.GetAllAgentsInput{})
		require.NoError(t, err)

		result := output.(keylime.GetAllAgentsOutput)
		assert.Equal(t, []string{
			"d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
			"d432fbb3-d2f1-4a97-9ef7-75bd81c11111",
		}, result.Agents)
	})

	t.Run("empty list", func(t *testing.T) {
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"code":200,"status":"Success","results":{"uuids":null}}`))
		}))

		_, output, err := h.GetAllAgents(context.Background(), nil, keylime.GetAllAgentsInput{})
		require.NoError(t, err)

		result := output.(keylime.GetAllAgentsOutput)
		assert.Equal(t, []string{}, result.Agents)
	})
}

func TestGetVerifierEnrolledAgents(t *testing.T) {
	t.Run("returns flattened uuid list", func(t *testing.T) {
		data := loadTestdata(t, "enrolled_agents.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/", func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.GetVerifierEnrolledAgents(context.Background(), nil, keylime.GetVerifierEnrolledAgentsInput{})
		require.NoError(t, err)

		result := output.(keylime.GetVerifierEnrolledAgentsOutput)
		assert.Equal(t, []string{
			"d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
			"d432fbb3-d2f1-4a97-9ef7-75bd81c11111",
		}, result.Agents)
	})

	t.Run("empty list", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"code":200,"status":"Success","results":{"uuids":[]}}`))
		})
		h := newTestHandler(t, mux)

		_, output, err := h.GetVerifierEnrolledAgents(context.Background(), nil, keylime.GetVerifierEnrolledAgentsInput{})
		require.NoError(t, err)

		result := output.(keylime.GetVerifierEnrolledAgentsOutput)
		assert.Equal(t, []string{}, result.Agents)
	})

	t.Run("server error", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"status":"internal error"}`))
		})
		h := newTestHandler(t, mux)

		_, _, err := h.GetVerifierEnrolledAgents(context.Background(), nil, keylime.GetVerifierEnrolledAgentsInput{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})
}

func TestGetAgentStatus(t *testing.T) {
	t.Run("valid uuid returns status", func(t *testing.T) {
		data := loadTestdata(t, "agent_status.json")
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		}))

		_, output, err := h.GetAgentStatus(context.Background(), nil, keylime.GetAgentStatusInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		require.NoError(t, err)

		result := output.(keylime.GetAgentStatusOutput)
		assert.Equal(t, "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", result.AgentUUID)
		assert.Equal(t, keylime.StateGetQuote, result.OperationalState)
		assert.Equal(t, "Get Quote", result.OperationalStateDescription)
		assert.Equal(t, 42, result.AttestationCount)
		assert.Equal(t, "127.0.0.1:8881", result.VerifierAddress)
	})

	t.Run("invalid uuid rejected before HTTP call", func(t *testing.T) {
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("HTTP should not be called for invalid UUID")
		}))

		_, _, err := h.GetAgentStatus(context.Background(), nil, keylime.GetAgentStatusInput{
			AgentUUID: "not-valid",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "agent_uuid")
	})

	t.Run("empty uuid rejected", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.GetAgentStatus(context.Background(), nil, keylime.GetAgentStatusInput{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "agent_uuid is required")
	})
}

func TestGetFailedAgents(t *testing.T) {
	const (
		uuid1 = "d432fbb3-d2f1-4a97-9ef7-75bd81c00000" // healthy
		uuid2 = "d432fbb3-d2f1-4a97-9ef7-75bd81c11111" // failed (state 7)
		uuid3 = "d432fbb3-d2f1-4a97-9ef7-75bd81c22222" // invalid quote (state 9)
	)

	t.Run("returns only failed agents", func(t *testing.T) {
		statusHealthy := loadTestdata(t, "agent_status.json")
		statusFailed := loadTestdata(t, "agent_status_failed.json")
		statusInvalid := loadTestdata(t, "agent_status_invalid_quote.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w,
				`{"code":200,"status":"Success","results":{"uuids":["%s","%s","%s"]}}`,
				uuid1, uuid2, uuid3,
			)
		})
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			uuid := r.PathValue("uuid")
			switch uuid {
			case uuid1:
				w.Write(statusHealthy)
			case uuid2:
				w.Write(statusFailed)
			case uuid3:
				w.Write(statusInvalid)
			default:
				t.Errorf("unexpected uuid: %s", uuid)
				http.Error(w, "unexpected uuid", http.StatusInternalServerError)
			}
		})
		h := newTestHandler(t, mux)

		_, output, err := h.GetFailedAgents(context.Background(), nil, keylime.GetFailedAgentsInput{})
		require.NoError(t, err)

		result := output.(keylime.GetFailedAgentsOutput)
		assert.Len(t, result.FailedAgents, 2)

		// goroutine ordering is non-deterministic
		failedUUIDs := make([]string, 0, len(result.FailedAgents))
		for _, a := range result.FailedAgents {
			failedUUIDs = append(failedUUIDs, a.AgentUUID)
		}
		assert.ElementsMatch(t, []string{uuid2, uuid3}, failedUUIDs)
	})

	t.Run("no agents", func(t *testing.T) {
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"code":200,"status":"Success","results":{"uuids":null}}`))
		}))

		_, output, err := h.GetFailedAgents(context.Background(), nil, keylime.GetFailedAgentsInput{})
		require.NoError(t, err)

		result := output.(keylime.GetFailedAgentsOutput)
		assert.Empty(t, result.FailedAgents)
	})

	t.Run("all healthy returns empty", func(t *testing.T) {
		statusData := loadTestdata(t, "agent_status.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w,
				`{"code":200,"status":"Success","results":{"uuids":["%s"]}}`, uuid1,
			)
		})
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(statusData)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.GetFailedAgents(context.Background(), nil, keylime.GetFailedAgentsInput{})
		require.NoError(t, err)

		result := output.(keylime.GetFailedAgentsOutput)
		assert.Empty(t, result.FailedAgents)
	})

	t.Run("agent not enrolled in verifier skipped gracefully", func(t *testing.T) {
		failedData := loadTestdata(t, "agent_status_failed.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w,
				`{"code":200,"status":"Success","results":{"uuids":["%s","%s"]}}`,
				uuid1, uuid2,
			)
		})
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			uuid := r.PathValue("uuid")
			if uuid == uuid1 {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"code":404,"status":"agent not found"}`))
			} else {
				w.Write(failedData)
			}
		})
		h := newTestHandler(t, mux)

		_, output, err := h.GetFailedAgents(context.Background(), nil, keylime.GetFailedAgentsInput{})
		require.NoError(t, err)

		result := output.(keylime.GetFailedAgentsOutput)
		assert.Len(t, result.FailedAgents, 1)
	})
}

func TestReactivateAgent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		data := loadTestdata(t, "success.json")
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			w.Write(data)
		}))

		_, output, err := h.ReactivateAgent(context.Background(), nil, keylime.ReactivateAgentInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		require.NoError(t, err)

		result := output.(keylime.ReactivateAgentOutput)
		assert.Equal(t, 200, result.Code)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.ReactivateAgent(context.Background(), nil, keylime.ReactivateAgentInput{
			AgentUUID: "bad",
		})
		assert.Error(t, err)
	})
}

func TestGetAgentPolicies(t *testing.T) {
	t.Run("returns policy fields", func(t *testing.T) {
		data := loadTestdata(t, "agent_status.json")
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		}))

		_, output, err := h.GetAgentPolicies(context.Background(), nil, keylime.GetAgentPoliciesInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		require.NoError(t, err)

		result := output.(keylime.GetAgentPoliciesOutput)
		assert.Equal(t, "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", result.AgentUUID)
		assert.False(t, result.HasMeasuredBootPolicy) // has_mb_refstate: 0
		assert.True(t, result.HasRuntimePolicy)       // has_runtime_policy: 1
		assert.Equal(t, []string{"sha256"}, result.AcceptedTPMHashAlgs)
		assert.Equal(t, []string{"rsa"}, result.AcceptedTPMEncryptionAlgs)
		assert.Equal(t, []string{"rsassa"}, result.AcceptedTPMSigningAlgs)
		// parseJSONStr fields must be parsed objects, not raw strings
		assert.IsType(t, map[string]any{}, result.TPMPolicy)
		assert.IsType(t, map[string]any{}, result.VTPMPolicy)
		assert.IsType(t, map[string]any{}, result.MetaData)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.GetAgentPolicies(context.Background(), nil, keylime.GetAgentPoliciesInput{
			AgentUUID: "bad",
		})
		assert.Error(t, err)
	})
}

func TestRegistrarGetAgentDetails(t *testing.T) {
	t.Run("returns registrar fields", func(t *testing.T) {
		data := loadTestdata(t, "registrar_agent_details.json")
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		}))

		_, output, err := h.RegistrarGetAgentDetails(context.Background(), nil, keylime.RegistrarGetAgentDetailsInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		require.NoError(t, err)

		result := output.(keylime.RegistrarGetAgentDetailsOutput)
		assert.Equal(t, "test-aik-data", result.Results.AikTpm)
		assert.Equal(t, "test-ek-data", result.Results.EkTpm)
		assert.Equal(t, "test-mtls-cert", result.Results.MtlsCert)
		assert.Equal(t, "192.168.1.100", result.Results.IP)
		assert.Equal(t, 9002, result.Results.Port)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.RegistrarGetAgentDetails(context.Background(), nil, keylime.RegistrarGetAgentDetailsInput{
			AgentUUID: "bad",
		})
		assert.Error(t, err)
	})
}

func TestEnrollAgentToVerifier(t *testing.T) {
	t.Run("enroll without policies", func(t *testing.T) {
		regData := loadTestdata(t, "registrar_agent_details.json")
		successData := loadTestdata(t, "success.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(regData)
		})
		mux.HandleFunc("POST /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(successData)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.EnrollAgentToVerifier(context.Background(), nil, keylime.EnrollAgentToVerifierInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		require.NoError(t, err)

		result := output.(keylime.EnrollAgentToVerifierOutput)
		assert.Equal(t, 200, result.Code)
	})

	t.Run("enroll with runtime policy verifies POST body", func(t *testing.T) {
		regData := loadTestdata(t, "registrar_agent_details.json")
		policyData := loadTestdata(t, "runtime_policy.json")
		successData := loadTestdata(t, "success.json")
		var enrollBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(regData)
		})
		mux.HandleFunc("GET /v2.5/allowlists/{name}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(policyData)
		})
		mux.HandleFunc("POST /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &enrollBody)
			w.Write(successData)
		})
		h := newTestHandler(t, mux)

		_, _, err := h.EnrollAgentToVerifier(context.Background(), nil, keylime.EnrollAgentToVerifierInput{
			AgentUUID:         "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
			RuntimePolicyName: "test-policy",
		})
		require.NoError(t, err)

		// check enrollment body
		assert.Equal(t, "192.168.1.100", enrollBody["cloudagent_ip"])
		assert.Equal(t, "test-aik-data", enrollBody["ak_tpm"])
		assert.Equal(t, "test-mtls-cert", enrollBody["mtls_cert"])
		policyB64, ok := enrollBody["runtime_policy"].(string)
		require.True(t, ok)
		decoded, err := base64.StdEncoding.DecodeString(policyB64)
		require.NoError(t, err)
		assert.True(t, json.Valid(decoded))
		assert.Contains(t, enrollBody["tpm_policy"], "0x400") // PCR 10
	})

	t.Run("invalid uuid", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.EnrollAgentToVerifier(context.Background(), nil, keylime.EnrollAgentToVerifierInput{
			AgentUUID: "bad",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "agent_uuid")
	})

	t.Run("invalid runtime policy name", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.EnrollAgentToVerifier(context.Background(), nil, keylime.EnrollAgentToVerifierInput{
			AgentUUID:         "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
			RuntimePolicyName: "invalid name with spaces",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "runtime_policy_name")
	})

	t.Run("invalid mb policy name", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.EnrollAgentToVerifier(context.Background(), nil, keylime.EnrollAgentToVerifierInput{
			AgentUUID:    "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
			MbPolicyName: "../evil",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mb_policy_name")
	})
}

func TestUpdateAgent(t *testing.T) {
	t.Run("successful update", func(t *testing.T) {
		regData := loadTestdata(t, "registrar_agent_details.json")
		successData := loadTestdata(t, "success.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(regData)
		})
		mux.HandleFunc("DELETE /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(successData)
		})
		mux.HandleFunc("POST /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(successData)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.UpdateAgent(context.Background(), nil, keylime.UpdateAgentInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		require.NoError(t, err)

		result := output.(keylime.UpdateAgentOutput)
		assert.Equal(t, "d432fbb3-d2f1-4a97-9ef7-75bd81c00000", result.AgentUUID)
		assert.Equal(t, "updated", result.Status)
	})

	t.Run("unenroll fails", func(t *testing.T) {
		regData := loadTestdata(t, "registrar_agent_details.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(regData)
		})
		mux.HandleFunc("DELETE /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"status":"internal error"}`))
		})
		h := newTestHandler(t, mux)

		_, _, err := h.UpdateAgent(context.Background(), nil, keylime.UpdateAgentInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unenroll")
	})

	t.Run("re-enroll fails after unenroll", func(t *testing.T) {
		regData := loadTestdata(t, "registrar_agent_details.json")
		successData := loadTestdata(t, "success.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(regData)
		})
		mux.HandleFunc("DELETE /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.Write(successData)
		})
		mux.HandleFunc("POST /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		})
		h := newTestHandler(t, mux)

		_, _, err := h.UpdateAgent(context.Background(), nil, keylime.UpdateAgentInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CRITICAL")
		assert.Contains(t, err.Error(), "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
	})

	t.Run("prep failure prevents unenroll", func(t *testing.T) {
		successData := loadTestdata(t, "success.json")
		var deleteCalled bool
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		})
		mux.HandleFunc("DELETE /v2.5/agents/{uuid}", func(w http.ResponseWriter, r *http.Request) {
			deleteCalled = true
			w.Write(successData)
		})
		h := newTestHandler(t, mux)

		_, _, err := h.UpdateAgent(context.Background(), nil, keylime.UpdateAgentInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		assert.Error(t, err)
		assert.False(t, deleteCalled, "DELETE must not be called if PrepareEnrollmentBody fails")
	})

	t.Run("invalid uuid", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.UpdateAgent(context.Background(), nil, keylime.UpdateAgentInput{
			AgentUUID: "bad",
		})
		assert.Error(t, err)
	})
}

func TestUnenrollAgentFromVerifier(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		data := loadTestdata(t, "success.json")
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			w.Write(data)
		}))

		_, output, err := h.UnenrollAgentFromVerifier(context.Background(), nil, keylime.UnenrollAgentFromVerifierInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		require.NoError(t, err)

		result := output.(keylime.UnenrollAgentFromVerifierOutput)
		assert.Equal(t, 200, result.Code)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.UnenrollAgentFromVerifier(context.Background(), nil, keylime.UnenrollAgentFromVerifierInput{
			AgentUUID: "bad",
		})
		assert.Error(t, err)
	})
}

func TestStopAgent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		data := loadTestdata(t, "success.json")
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			w.Write(data)
		}))

		_, output, err := h.StopAgent(context.Background(), nil, keylime.StopAgentInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		require.NoError(t, err)

		result := output.(keylime.StopAgentOutput)
		assert.Equal(t, 200, result.Code)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.StopAgent(context.Background(), nil, keylime.StopAgentInput{
			AgentUUID: "bad",
		})
		assert.Error(t, err)
	})
}

func TestRegistrarRemoveAgent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		data := loadTestdata(t, "success.json")
		h := newTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			w.Write(data)
		}))

		_, output, err := h.RegistrarRemoveAgent(context.Background(), nil, keylime.RegistrarRemoveAgentInput{
			AgentUUID: "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		})
		require.NoError(t, err)

		result := output.(keylime.RegistrarRemoveAgentOutput)
		assert.Equal(t, 200, result.Code)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.RegistrarRemoveAgent(context.Background(), nil, keylime.RegistrarRemoveAgentInput{
			AgentUUID: "bad",
		})
		assert.Error(t, err)
	})
}
