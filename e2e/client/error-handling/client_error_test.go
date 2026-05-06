//go:build functional

package clienterrorhandling_test

import (
	"net/http"
	"testing"

	"github.com/keylime/keylime-mcp/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientErrors(t *testing.T) {
	c := helpers.NewMCPTestClient(t, nil)

	t.Run("index_page", func(t *testing.T) {
		resp, err := http.Get(c.URL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	})

	t.Run("empty_message", func(t *testing.T) {
		resp := c.PostChat("")
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("approve_no_pending_tool", func(t *testing.T) {
		resp := c.PostToolApprove()
		defer resp.Body.Close()
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("deny_no_pending_tool", func(t *testing.T) {
		resp := c.PostToolDeny()
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
	})
}
