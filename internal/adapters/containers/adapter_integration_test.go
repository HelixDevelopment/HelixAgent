//go:build integration

package containers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/config"
	"digital.vasic.containers/pkg/runtime"
)

// skipIfNoRuntime skips the test when no container runtime
// (Docker or Podman) is detected on the host.
func skipIfNoRuntime(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second,
	)
	defer cancel()
	_, err := runtime.AutoDetect(ctx)
	if err != nil {
		t.Skip("no container runtime available: ", err)  // SKIP-OK: #runtime-mock-only
	}
}

func TestIntegration_NewAdapterFromConfig_RealRuntimeDetection(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}
	skipIfNoRuntime(t)

	cfg := &config.Config{}
	adapter, err := NewAdapterFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, adapter)

	// When a runtime is detected, the adapter must have a
	// non-nil runtime and a non-empty runtime name.
	assert.NotNil(t, adapter.Runtime())
	name := adapter.Runtime().Name()
	assert.NotEmpty(t, name)
	assert.Contains(
		t, []string{"docker", "podman", "kubernetes"},
		name,
		"runtime name should be one of the supported runtimes",
	)

	// Health checker should always be set by NewAdapterFromConfig.
	assert.NotNil(t, adapter.healthChecker)
}

func TestIntegration_DetectRuntime_ActualContainerRuntime(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}
	skipIfNoRuntime(t)

	adapter, err := NewAdapter()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second,
	)
	defer cancel()

	name, err := adapter.DetectRuntime(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, name)
	assert.Contains(
		t, []string{"docker", "podman", "kubernetes"},
		name,
	)

	// After detection the runtime must be stored on the adapter.
	assert.NotNil(t, adapter.Runtime())
	assert.Equal(t, name, adapter.Runtime().Name())
}

func TestIntegration_RuntimeAvailable_RealSystem(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}
	skipIfNoRuntime(t)

	adapter, err := NewAdapter()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second,
	)
	defer cancel()

	available := adapter.RuntimeAvailable(ctx)
	assert.True(t, available)
}

func TestIntegration_HealthCheckHTTP_KnownEndpoint(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	// Start a local HTTP test server to provide a known
	// endpoint.
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		},
	))
	defer ts.Close()

	adapter, err := NewAdapter()
	require.NoError(t, err)

	err = adapter.HealthCheckHTTP(ts.URL)
	assert.NoError(t, err)
}

func TestIntegration_HealthCheckHTTP_NonOKStatus(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	))
	defer ts.Close()

	adapter, err := NewAdapter()
	require.NoError(t, err)

	err = adapter.HealthCheckHTTP(ts.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestIntegration_HealthCheckHTTP_Unreachable(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	adapter, err := NewAdapter()
	require.NoError(t, err)

	// Use a port that is almost certainly not listening.
	err = adapter.HealthCheckHTTP(
		"http://127.0.0.1:19/nonexistent",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot connect")
}

func TestIntegration_HealthCheckTCP_KnownPort(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	// Start a TCP listener on an ephemeral port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot start TCP listener: ", err)  // SKIP-OK: #legacy-untriaged
	}
	defer func() { _ = ln.Close() }()

	// Accept connections in the background so HealthCheckTCP
	// can connect.
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	adapter, err := NewAdapter()
	require.NoError(t, err)

	ok := adapter.HealthCheckTCP("127.0.0.1", port)
	assert.True(t, ok)
}

func TestIntegration_HealthCheckTCP_ClosedPort(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	adapter, err := NewAdapter()
	require.NoError(t, err)

	// Port 19 (chargen) is almost certainly not listening on
	// localhost.
	ok := adapter.HealthCheckTCP("127.0.0.1", 19)
	assert.False(t, ok)
}

func TestIntegration_ComposeStatus_NonExistentFile(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}
	skipIfNoRuntime(t)

	cfg := &config.Config{}
	adapter, err := NewAdapterFromConfig(cfg)
	require.NoError(t, err)

	if adapter.Orchestrator() == nil {
		t.Skip("compose orchestrator not available")  // SKIP-OK: #legacy-untriaged
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 15*time.Second,
	)
	defer cancel()

	// Query compose status for a file that does not exist.
	// This should return an error.
	_, err = adapter.ComposeStatus(
		ctx,
		"/tmp/nonexistent-compose-"+
			fmt.Sprintf("%d", time.Now().UnixNano())+
			".yml",
	)
	assert.Error(t, err)
}

func TestIntegration_Shutdown_FreshAdapter(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	// A fresh adapter has no distributor, tunnel manager, or
	// volume manager -- Shutdown should be a no-op and return
	// nil.
	adapter, err := NewAdapter()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(
		context.Background(), 5*time.Second,
	)
	defer cancel()

	err = adapter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestIntegration_RemoteEnabled_NoRemoteConfig(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	adapter, err := NewAdapter()
	require.NoError(t, err)

	// Without any remote configuration, RemoteEnabled must
	// return false.
	assert.False(t, adapter.RemoteEnabled())
}

func TestIntegration_ListHosts_NoHostsConfigured(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	adapter, err := NewAdapter()
	require.NoError(t, err)

	// With no host manager, ListHosts returns nil.
	hosts := adapter.ListHosts()
	assert.Nil(t, hosts)
}

func TestIntegration_DistributionStatus_NoDistributor(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	adapter, err := NewAdapter()
	require.NoError(t, err)

	// When no distributor is set, DistributionStatus returns
	// nil.
	status := adapter.DistributionStatus(context.Background())
	assert.Nil(t, status)
}
