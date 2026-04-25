package health

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/ports"
)

// TestLiveness_Lifecycle exercises the early-bind probe end-to-end:
// initial state (starting), transition to ready, payload shape on each.
//
// Uses a local-only port that's clearly outside the canonical band so
// the test never collides with a real binary on the same host.
func TestLiveness_Lifecycle(t *testing.T) {
	// Pick an off-band port so this never fights with a running binary.
	// We don't need ports.HelixAgentLiveness here — we're testing the type.
	const testPort = 18111
	t.Setenv(string(ports.HelixAgentLiveness), strconv.Itoa(testPort))

	probe := NewLiveness(nil)
	require.NoError(t, probe.Start())
	t.Cleanup(func() {
		_ = probe.Shutdown(t.Context())
	})

	// Give the listener a beat.
	time.Sleep(50 * time.Millisecond)

	url := "http://localhost:" + strconv.Itoa(testPort) + "/health"
	client := &http.Client{Timeout: 2 * time.Second}

	// Pre-ready: status=starting
	resp, err := client.Get(url)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var pre statusResp
	require.NoError(t, json.Unmarshal(body, &pre), "body=%s", string(body))
	assert.Equal(t, "starting", pre.Status)
	assert.NotEmpty(t, pre.StartedAt)
	assert.GreaterOrEqual(t, pre.ElapsedMs, int64(0))
	assert.Empty(t, pre.ReadyAt, "ReadyAt must be absent before SetReady")

	// Trigger transition.
	probe.SetReady()

	// Post-ready: status=ready
	resp2, err := client.Get(url)
	require.NoError(t, err)
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var post statusResp
	require.NoError(t, json.Unmarshal(body2, &post))
	assert.Equal(t, "ready", post.Status)
	assert.NotEmpty(t, post.ReadyAt, "ReadyAt must be set after SetReady")
	assert.GreaterOrEqual(t, post.ReadySince, int64(0))

	// /readyz and /livez aliases
	for _, path := range []string{"/readyz", "/livez"} {
		r, err := client.Get("http://localhost:" + strconv.Itoa(testPort) + path)
		require.NoError(t, err)
		_, _ = io.ReadAll(r.Body)
		r.Body.Close()
		assert.Equal(t, http.StatusOK, r.StatusCode, "%s should also return 200", path)
	}
}

// TestLiveness_BindFailure verifies that an in-use port surfaces as a
// clear error from Start (rather than a silent goroutine panic).
func TestLiveness_BindFailure(t *testing.T) {
	const testPort = 18112
	t.Setenv(string(ports.HelixAgentLiveness), strconv.Itoa(testPort))

	first := NewLiveness(nil)
	require.NoError(t, first.Start())
	t.Cleanup(func() {
		_ = first.Shutdown(t.Context())
	})
	time.Sleep(50 * time.Millisecond)

	second := NewLiveness(nil)
	err := second.Start()
	require.Error(t, err, "second Start on the same port must fail")
	assert.Contains(t, err.Error(), "liveness: bind")
}

// Ensure the package compiles cleanly even when the env var is absent.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
