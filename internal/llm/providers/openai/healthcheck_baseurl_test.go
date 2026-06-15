package openai

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthCheck_HonorsInjectedBaseURL proves HXC-085: HealthCheck derives the
// models endpoint from the injected base URL (the chat-completions URL) instead
// of the hardcoded production constant, so a proxy / httptest base URL is honored.
func TestHealthCheck_HonorsInjectedBaseURL(t *testing.T) {
	t.Parallel()
	var hit int32
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hit, 1)
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	p := NewProvider("test-key", server.URL+"/v1/chat/completions", "gpt-4o")
	err := p.HealthCheck()
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hit), "HealthCheck must hit the injected server, not the production constant")
	assert.Equal(t, "/v1/models", gotPath, "models endpoint must be derived from the injected base URL")
}
