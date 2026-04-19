package keylime

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func loadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name) // #nosec G304 -- test helper, name is hardcoded in tests
	require.NoError(t, err, "failed to load testdata/%s", name)
	return data
}

func newTestService(t *testing.T, handler http.Handler) *Service {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	svc, err := NewService(&Config{
		VerifierURL:  ts.URL,
		RegistrarURL: ts.URL,
		TLSEnabled:   false,
		APIVersion:   "v2.5",
	})
	require.NoError(t, err)
	return svc
}
