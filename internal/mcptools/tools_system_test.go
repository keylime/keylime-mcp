package mcptools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/keylime/keylime-mcp/internal/keylime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVersionAndHealth(t *testing.T) {
	t.Run("both reachable", func(t *testing.T) {
		data := loadTestdata(t, "version.json")
		mux := http.NewServeMux()
		mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
			w.Write(data)
		})
		h := newTestHandler(t, mux)

		_, output, err := h.GetVersionAndHealth(context.Background(), nil, keylime.GetVersionAndHealthInput{})
		require.NoError(t, err)

		result := output.(keylime.GetVersionAndHealthOutput)
		require.Len(t, result.Services, 2)

		assert.Equal(t, "verifier", result.Services[0].Service)
		assert.True(t, result.Services[0].Reachable)
		assert.Equal(t, "2.5", result.Services[0].CurrentVersion)
		assert.Equal(t, []string{"2.4", "2.5"}, result.Services[0].SupportedVersions)

		assert.Equal(t, "registrar", result.Services[1].Service)
		assert.True(t, result.Services[1].Reachable)
	})

	t.Run("verifier down registrar up", func(t *testing.T) {
		// closed server = unreachable
		downServer := httptest.NewServer(http.NotFoundHandler())
		downServer.Close()

		versionData := loadTestdata(t, "version.json")
		upServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(versionData)
		}))
		t.Cleanup(upServer.Close)

		svc, err := keylime.NewService(&keylime.Config{
			VerifierURL:  downServer.URL,
			RegistrarURL: upServer.URL,
			TLSEnabled:   false,
			APIVersion:   testAPIVersion,
		})
		require.NoError(t, err)
		h := NewToolHandler(svc)

		_, output, err := h.GetVersionAndHealth(context.Background(), nil, keylime.GetVersionAndHealthInput{})
		require.NoError(t, err)

		result := output.(keylime.GetVersionAndHealthOutput)
		require.Len(t, result.Services, 2)

		assert.False(t, result.Services[0].Reachable)
		assert.NotEmpty(t, result.Services[0].Error)

		assert.True(t, result.Services[1].Reachable)
		assert.Equal(t, "2.5", result.Services[1].CurrentVersion)
	})

	t.Run("both down", func(t *testing.T) {
		downServer := httptest.NewServer(http.NotFoundHandler())
		downServer.Close()

		svc, err := keylime.NewService(&keylime.Config{
			VerifierURL:  downServer.URL,
			RegistrarURL: downServer.URL,
			TLSEnabled:   false,
			APIVersion:   testAPIVersion,
		})
		require.NoError(t, err)
		h := NewToolHandler(svc)

		_, output, err := h.GetVersionAndHealth(context.Background(), nil, keylime.GetVersionAndHealthInput{})
		require.NoError(t, err)

		result := output.(keylime.GetVersionAndHealthOutput)
		require.Len(t, result.Services, 2)
		assert.False(t, result.Services[0].Reachable)
		assert.False(t, result.Services[1].Reachable)
	})
}

func TestInvestigateVerifierLogs(t *testing.T) {
	// validates input handling; journalctl exec not mocked

	t.Run("invalid filter rejected", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.InvestigateVerifierLogs(context.Background(), nil, keylime.InvestigateVerifierLogsInput{
			Filter: "invalid_filter",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid filter")
	})

	t.Run("invalid uuid rejected", func(t *testing.T) {
		h := newTestHandler(t, http.NotFoundHandler())
		_, _, err := h.InvestigateVerifierLogs(context.Background(), nil, keylime.InvestigateVerifierLogsInput{
			AgentUUID: "not-a-uuid",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "agent_uuid")
	})

	t.Run("valid filters accepted", func(t *testing.T) {
		if _, err := exec.LookPath("journalctl"); err != nil {
			t.Skip("journalctl not available")
		}

		h := newTestHandler(t, http.NotFoundHandler())
		for _, filter := range []string{testFilterAll, "attestation_failures", "errors"} {
			t.Run(filter, func(t *testing.T) {
				_, output, err := h.InvestigateVerifierLogs(context.Background(), nil, keylime.InvestigateVerifierLogsInput{
					Filter: filter,
					Lines:  10,
				})
				// journalctl exit code 1 (no matches) is handled gracefully
				require.NoError(t, err)

				result := output.(keylime.InvestigateVerifierLogsOutput)
				assert.Equal(t, filter, result.FilterApplied)
			})
		}
	})

	t.Run("empty filter defaults to all", func(t *testing.T) {
		if _, err := exec.LookPath("journalctl"); err != nil {
			t.Skip("journalctl not available")
		}

		h := newTestHandler(t, http.NotFoundHandler())
		_, output, err := h.InvestigateVerifierLogs(context.Background(), nil, keylime.InvestigateVerifierLogsInput{})
		require.NoError(t, err)

		result := output.(keylime.InvestigateVerifierLogsOutput)
		assert.Equal(t, testFilterAll, result.FilterApplied)
	})
}
