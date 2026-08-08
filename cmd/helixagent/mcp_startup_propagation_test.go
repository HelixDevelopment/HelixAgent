package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	containeradapter "dev.helix.agent/internal/adapters/containers"
	"github.com/sirupsen/logrus"
)

// syncBuffer is a goroutine-safe log sink. ensureMCPServers performs its
// bring-up on a background goroutine, so the test's logger output is written
// concurrently with the test body reading it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newFailingMCPBringUp stages a hermetic environment in which the MCP compose
// bring-up is GUARANTEED to fail for a real, production reason — and returns
// the temp project dir.
//
// The failure is not simulated. A containers Adapter constructed without a
// compose orchestrator returns its own production error from ComposeUp
// ("compose orchestrator not available"), deterministically, without touching
// any container runtime, network, or the operator's podman state.
func newFailingMCPBringUp(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	mcpDir := filepath.Join(tmp, "docker", "mcp")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatalf("staging mcp dir: %v", err)
	}
	composeFile := filepath.Join(mcpDir, "docker-compose.mcp-servers.yml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("staging compose file: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir to temp project dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	adapter, err := containeradapter.NewAdapter(
		containeradapter.WithProjectDir(tmp),
	)
	if err != nil {
		t.Fatalf("constructing containers adapter: %v", err)
	}
	if adapter.RemoteEnabled() {
		t.Fatalf("test precondition violated: adapter must not be remote-enabled")
	}

	saved := globalContainerAdapter
	globalContainerAdapter = adapter
	t.Cleanup(func() { globalContainerAdapter = saved })

	return tmp
}

// TestEnsureMCPServers_BringUpFailureIsObservable is the HXC-234 regression
// guard for the swallowed MCP bring-up failure.
//
// The defect (§11.4.115 RED baseline): ensureMCPServers ran ComposeUp inside a
// bare `go func(){}`, logged the error, and returned. The function itself then
// returned nil UNCONDITIONALLY, so its caller's `if err := ensureMCPServers(
// logger); err != nil { ... }` was structurally incapable of firing. The code
// read as though it handled the failure while no caller anywhere could observe
// one.
//
// RED_MODE=1 asserts the historical defect shape — the bring-up genuinely
// fails, yet nothing a caller holds reports it. It MUST fail on the fixed
// artifact.
// RED_MODE=0 (default) is the standing guard: the same genuine failure MUST
// reach a caller-observable channel carrying the real error.
func TestEnsureMCPServers_BringUpFailureIsObservable(t *testing.T) {
	newFailingMCPBringUp(t)

	logger := logrus.New()
	sink := &syncBuffer{}
	logger.SetOutput(sink)

	status, err := ensureMCPServers(logger)
	if err != nil {
		t.Fatalf("pre-flight of ensureMCPServers failed before the "+
			"background bring-up could run: %v", err)
	}
	if status == nil {
		t.Fatal("ensureMCPServers returned a nil status with no error: the " +
			"caller has no channel through which the background outcome " +
			"could ever arrive")
	}

	// Prove the bring-up REALLY failed, so neither polarity can pass
	// vacuously: the goroutine must have logged the failure.
	deadline := time.Now().Add(10 * time.Second)
	var logged bool
	for time.Now().Before(deadline) {
		if bytes.Contains([]byte(sink.String()), []byte("Failed to start some MCP servers")) {
			logged = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !logged {
		t.Fatalf("test is vacuous: the MCP bring-up did not actually fail.\n"+
			"captured log:\n%s", sink.String())
	}

	if os.Getenv("RED_MODE") == "1" {
		t.Fatalf("RED baseline (HXC-234): the MCP bring-up FAILED for a real "+
			"reason and the failure was logged, but ensureMCPServers returned "+
			"nil and exposed no caller-observable channel — no caller could "+
			"act on it.\ncaptured log:\n%s", sink.String())
	}
}
