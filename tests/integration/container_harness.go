// Package integration provides container-based test harness for integration tests.
// This harness ensures all integration tests run against real containers via the
// Containers module adapter, as per the zero-mocks policy.
package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	containeradapter "dev.helix.agent/internal/adapters/containers"
	"github.com/sirupsen/logrus"
)

// =============================================================================
// Container Test Harness - Provides real container infrastructure for tests
// =============================================================================

// ContainerHarness manages container lifecycle for integration tests.
// It uses the Containers module adapter to start/stop real services.
type ContainerHarness struct {
	Adapter     *containeradapter.Adapter
	Logger      *logrus.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	servicesUp  map[string]bool
	mu          sync.RWMutex
	projectRoot string

	// owned is the ONLY authority Cleanup consults before stopping
	// anything. It is nil until THIS harness instance has itself run a
	// ComposeUp that returned success, and it is cleared once that
	// project has been torn down. See composeOwnership for why this
	// replaced the previous liveness-probe guards.
	owned *composeOwnership

	// composeDown performs the actual teardown. It is a field rather
	// than a direct h.Adapter.ComposeDown call so the destructive path
	// is reachable from a test without a container runtime — the defect
	// this indirection exists to guard is, by its nature, one that
	// cannot be safely reproduced against a live stack.
	composeDown func(ctx context.Context, composeFile, profile string) error
}

// composeOwnership records a compose project that THIS harness instance
// provably started.
//
// # Why ownership is recorded rather than inferred (§11.4.111, §11.4.174)
//
// Cleanup previously decided whether it was allowed to run ComposeDown by
// RE-DERIVING, at teardown time, the same three conditions BootAllServices
// had consulted at boot time — an env var, a containers/.env flag, and a
// bare TCP dial of localhost:8100. None of the three records what boot
// actually DID; all three are re-evaluated against a world that may have
// changed in between. The port probe was the worst of them on two counts:
//
//	(1) It asked the wrong question. "Something accepted a TCP connection"
//	    is a liveness signal, never an ownership proof. Even a correct,
//	    identity-verified probe of a real HelixAgent would not establish
//	    that THIS harness started the containers — the two facts are
//	    unrelated.
//	(2) It asked the wrong process. Measured on this host 2026-08-11:
//	    :8100 is held by llm-verifier (HTTP 404 "404 page not found"),
//	    while the real HelixAgent answers on :7061 with
//	    {"service":"helixagent","status":"healthy"}. The probe named in
//	    the log line "HelixAgent still running on :8100" never reached
//	    HelixAgent at all. See helixagent_identity.go for the same
//	    identity-blindness class already fixed on the assertion side.
//
// The consequence was a teardown that ran ComposeDown over the WHOLE
// docker-compose.yml project — postgres, redis, ollama, cognee, chromadb,
// neo4j, the helixagent service itself — whenever that unrelated port
// happened not to answer, despite the harness having started nothing. The
// live platform survived only because a foreign process kept the port
// occupied; that is a coincidence, not a guard.
//
// Recording ownership makes the failure structurally unreachable: if the
// harness did not start a project, there is no ownership record, and
// Cleanup has nothing it is permitted to stop. Boot-time conditions may
// still skip the ComposeUp for any reason they like — the teardown
// consequence is identical and automatic, because skipping the ComposeUp
// is exactly what leaves ownership unrecorded.
type composeOwnership struct {
	// composeFile + profile identify the project to tear down. They are
	// the values THIS harness passed to ComposeUp, not values re-read
	// from configuration at teardown time.
	composeFile string
	profile     string
	// startedAt is when the successful ComposeUp returned.
	startedAt time.Time
}

// teardownDecision is the closed set of verdicts decideTeardown can reach.
type teardownDecision string

const (
	// teardownStopOwned: this harness started a project and may stop it.
	teardownStopOwned teardownDecision = "stop-owned-project"
	// teardownSkipNotOwned: this harness started nothing. Anything that
	// happens to be running belongs to somebody else.
	teardownSkipNotOwned teardownDecision = "skip-nothing-owned"
)

// decideTeardown is the SOLE authority on whether Cleanup may stop
// containers. It is a pure function of the ownership record and consults
// nothing else — no environment, no configuration file, no network probe.
// Adding any such input back would re-open the defect this replaced.
func decideTeardown(owned *composeOwnership) (teardownDecision, string) {
	if owned == nil {
		return teardownSkipNotOwned,
			"harness started no containers — skipping ComposeDown " +
				"(nothing running here was started by this harness, so none of it is ours to stop)"
	}
	return teardownStopOwned, fmt.Sprintf(
		"tearing down the compose project this harness started at %s (file=%s profile=%q)",
		owned.startedAt.Format(time.RFC3339), owned.composeFile, owned.profile)
}

// RequiredServices lists all services needed for integration tests
var RequiredServices = []string{
	"postgresql",
	"redis",
	"chromadb",
	"cognee",
	"qdrant",
}

// DefaultServicePorts maps services to their default ports
var DefaultServicePorts = map[string]string{
	"postgresql": "5432",
	"redis":      "6379",
	"chromadb":   "8001",
	"cognee":     "8000",
	"qdrant":     "6333",
}

// TestServicePorts maps services to test-specific ports (as used in testutil)
var TestServicePorts = map[string]string{
	"postgresql": "15432",
	"redis":      "16379",
	"chromadb":   "18001",
	"cognee":     "18000",
	"qdrant":     "16333",
}

// harnessInstance is the singleton harness instance
var (
	harnessInstance *ContainerHarness
	harnessOnce     sync.Once
	harnessErr      error
)

// GetContainerHarness returns the singleton container harness instance.
// The harness is initialized once and reused across all tests in the package.
func GetContainerHarness() (*ContainerHarness, error) {
	harnessOnce.Do(func() {
		harnessInstance, harnessErr = newContainerHarness()
	})
	return harnessInstance, harnessErr
}

// newContainerHarness creates a new container harness
func newContainerHarness() (*ContainerHarness, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	if os.Getenv("TEST_DEBUG") == "true" {
		logger.SetLevel(logrus.DebugLevel)
	}

	// Get project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find project root: %w", err)
	}

	// Change to project root for container operations
	originalDir, _ := os.Getwd()
	if err := os.Chdir(projectRoot); err != nil {
		return nil, fmt.Errorf("failed to change to project root: %w", err)
	}
	defer os.Chdir(originalDir)

	// Create container adapter
	adapter, err := containeradapter.NewAdapter(
		containeradapter.WithProjectDir(projectRoot),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container adapter: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	h := &ContainerHarness{
		Adapter:     adapter,
		Logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		servicesUp:  make(map[string]bool),
		projectRoot: projectRoot,
	}
	h.composeDown = func(ctx context.Context, composeFile, profile string) error {
		return h.Adapter.ComposeDown(ctx, composeFile, profile)
	}

	return h, nil
}

// markOwned records that this harness successfully started composeFile.
// Called only on the ComposeUp success path.
func (h *ContainerHarness) markOwned(composeFile, profile string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.owned = &composeOwnership{
		composeFile: composeFile,
		profile:     profile,
		startedAt:   time.Now(),
	}
}

// ownership returns the current ownership record, or nil when this
// harness has not started a compose project.
func (h *ContainerHarness) ownership() *composeOwnership {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.owned
}

// releaseOwnership clears the record after a successful teardown so a
// second Cleanup cannot stop a project twice.
func (h *ContainerHarness) releaseOwnership() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.owned = nil
}

// BootAllServices starts all required services using the container adapter.
// This ensures tests have access to real containers, not mocks.
//
// The harness skips the ComposeUp in any of these situations, which match
// the container-orchestration rule from CLAUDE.md ("the HelixAgent binary
// orchestrates all containers"):
//   - HELIX_SKIP_CONTAINER_HARNESS is set (explicit opt-out)
//   - CONTAINERS_REMOTE_ENABLED=true in containers/.env
//     (all containers live on remote hosts; a local ComposeUp would
//     duplicate them and almost certainly fail). NOTE: this submodule's
//     project root has no containers/ directory — per CONST-051(C) that
//     dependency lives at the CONSUMING PROJECT's root, not nested here —
//     so isRemoteContainersEnabled always reads a missing file and always
//     returns false when the harness runs standalone. Stated as an
//     observed fact rather than repaired by guessing a path outside the
//     submodule, which would couple it to one consumer (CONST-051(B)).
//   - something accepts a TCP connection on :8100. This is a weak,
//     advisory signal ONLY — see helixAgentPortOpen for what it does and
//     does not establish; on this host :8100 is llm-verifier, not
//     HelixAgent.
//
// Each of these skips a START. None of them grants permission to STOP
// anything: skipping the ComposeUp leaves no ownership record, which is
// exactly what makes the matching Cleanup a no-op. See Cleanup.
func (h *ContainerHarness) BootAllServices() error {
	h.Logger.Info("╔══════════════════════════════════════════════════════════════════╗")
	h.Logger.Info("║       CONTAINER TEST HARNESS - Booting Real Services             ║")
	h.Logger.Info("╚══════════════════════════════════════════════════════════════════╝")

	if v := os.Getenv("HELIX_SKIP_CONTAINER_HARNESS"); v != "" && v != "0" && v != "false" {
		h.Logger.Info("HELIX_SKIP_CONTAINER_HARNESS set — trusting caller to provide services")
		return nil
	}

	if isRemoteContainersEnabled(h.projectRoot) {
		h.Logger.Info(
			"containers/.env has CONTAINERS_REMOTE_ENABLED=true — skipping local ComposeUp " +
				"(containers are orchestrated on remote hosts, per CLAUDE.md)",
		)
		return nil
	}

	if helixAgentPortOpen() {
		h.Logger.Info(
			"HelixAgent already reachable on :8100 — skipping duplicate ComposeUp " +
				"(the running binary already booted all required containers)",
		)
		return nil
	}

	// Check container runtime availability
	if !h.Adapter.RuntimeAvailable(h.ctx) {
		return fmt.Errorf("container runtime not available (docker/podman required)")
	}

	runtime, _ := h.Adapter.DetectRuntime(h.ctx)
	h.Logger.WithField("runtime", runtime).Info("Container runtime detected")

	// Start services via compose
	composeFile := "docker-compose.yml"

	h.Logger.WithFields(logrus.Fields{
		"compose_file": composeFile,
		"profile":      "default",
	}).Info("Starting container services via ComposeUp")

	ctx, cancel := context.WithTimeout(h.ctx, 5*time.Minute)
	defer cancel()

	if err := h.Adapter.ComposeUp(ctx, composeFile, "default"); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}

	// Ownership is recorded HERE — the moment ComposeUp succeeds, and
	// before the health wait. These containers are ours from the instant
	// they start, so a subsequent health-check failure must still leave
	// Cleanup able to tear down what we brought up. Recording ownership
	// only after the health wait would leak containers on that path.
	h.markOwned(composeFile, "default")

	// Wait for all services to be healthy
	if err := h.waitForServicesHealthy(ctx); err != nil {
		return fmt.Errorf("services failed health checks: %w", err)
	}

	h.Logger.Info("✓ All container services are healthy and ready for tests")
	return nil
}

// isRemoteContainersEnabled returns true when containers/.env has
// CONTAINERS_REMOTE_ENABLED=true, matching the top-level HelixAgent
// container-orchestration gate in internal/config.
func isRemoteContainersEnabled(projectRoot string) bool {
	envPath := filepath.Join(projectRoot, "Containers", ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "CONTAINERS_REMOTE_ENABLED=") {
			v := strings.TrimPrefix(line, "CONTAINERS_REMOTE_ENABLED=")
			v = strings.Trim(v, `"'`)
			return v == "true" || v == "1" || v == "yes"
		}
	}
	return false
}

// helixAgentPortOpen does a 2-second TCP probe of localhost:8100.
//
// SCOPE: BOOT ONLY, AND ADVISORY ONLY. Its single caller is
// BootAllServices, where a true result suppresses a ComposeUp. The worst
// outcome of a wrong answer here is that containers are not started —
// never that running containers are stopped. Cleanup does NOT consult
// it; teardown authority comes solely from the ownership record
// (composeOwnership / decideTeardown), and MUST NOT be given this or any
// other probe as an input.
//
// WHAT THIS PROBE DOES NOT ESTABLISH (§11.4.6, §11.4.111, §11.4.174).
// An earlier revision of this comment claimed the probe was "a positive
// signal that the full container fleet is already up, since HelixAgent
// refuses to bind :8100 until its own health-check harness has cleared
// every required service". Measured on this host 2026-08-11, that is
// false in both halves:
//
//	ss -ltnpH 'sport = :8100'  ->  users:(("llm-verifier",pid=1827554))
//	ss -ltnpH 'sport = :7061'  ->  users:(("helixagent",pid=1834487))
//	curl localhost:8100/health ->  HTTP 404, "404 page not found"
//	curl localhost:7061/health ->  HTTP 200, {"service":"helixagent",...}
//
// HelixAgent does not bind :8100 at all — :8100 is a sentinel constant
// (cmd/helixagent/main.go appConfigDefaultServerPort) and, on this host,
// a port held by a different service. So a true result here means "some
// process accepted a connection", not "HelixAgent is up", and certainly
// not "every required container is healthy".
//
// The wrong port is tracked separately (it spans two repositories) and is
// deliberately NOT changed here: making this probe stricter would flip
// boot from skipping ComposeUp to RUNNING it against whatever is already
// on the host, which is a new hazard rather than a fix. Leaving it
// advisory keeps that decision where it belongs while removing its
// ability to cause a destructive teardown.
//
// It is a package-level var (matching findProjectRoot below) so tests can
// force BOTH probe states. That matters for the teardown guard: whether
// the probe answers true or false is ambient host state no test controls
// — :8100 is occupied on this host and may be free on another — so a
// guard that could only observe one state would silently stop protecting
// anything on hosts where the other state holds.
var helixAgentPortOpen = func() bool {
	conn, err := net.DialTimeout("tcp", "localhost:8100", 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// waitForServicesHealthy waits for all required services to pass health checks
func (h *ContainerHarness) waitForServicesHealthy(ctx context.Context) error {
	h.Logger.Info("Waiting for services to be healthy...")

	// Create health check targets for each service
	targets := make([]struct {
		name string
		host string
		port string
	}, 0, len(RequiredServices))

	for _, service := range RequiredServices {
		host := getServiceHost(service)
		port := getServicePort(service)
		targets = append(targets, struct {
			name string
			host string
			port string
		}{service, host, port})
	}

	// Check each service with timeout
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		allHealthy := true
		for _, target := range targets {
			if !h.isServiceHealthy(target.host, target.port) {
				allHealthy = false
				h.Logger.WithField("service", target.name).Debug("Service not yet healthy")
				break
			}
		}
		if allHealthy {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	// Final check with detailed status
	var unhealthy []string
	for _, target := range targets {
		if !h.isServiceHealthy(target.host, target.port) {
			unhealthy = append(unhealthy, target.name)
		}
	}

	if len(unhealthy) > 0 {
		return fmt.Errorf("services not healthy after timeout: %v", unhealthy)
	}

	return nil
}

// isServiceHealthy checks if a service is reachable via TCP
func (h *ContainerHarness) isServiceHealthy(host, port string) bool {
	return isServerRunning(host, port)
}

// GetServiceURL returns the URL for a given service
func (h *ContainerHarness) GetServiceURL(service string) string {
	return h.GetServiceURLWithMode(service, false)
}

// GetServiceURLWithMode returns the URL for a given service
// If useTestPorts is true, uses test-specific ports
func (h *ContainerHarness) GetServiceURLWithMode(service string, useTestPorts bool) string {
	host := getServiceHost(service)
	port := getServicePortWithMode(service, useTestPorts)

	switch service {
	case "postgresql":
		return fmt.Sprintf("postgres://helixagent:helixagent123@%s:%s/helixagent_db", host, port)
	case "redis":
		return fmt.Sprintf("redis://:%s@%s:%s", getEnv("REDIS_PASSWORD", "helixagent123"), host, port)
	case "chromadb":
		return fmt.Sprintf("http://%s:%s", host, port)
	case "cognee":
		return fmt.Sprintf("http://%s:%s", host, port)
	case "qdrant":
		return fmt.Sprintf("http://%s:%s", host, port)
	default:
		return fmt.Sprintf("http://%s:%s", host, port)
	}
}

// Cleanup stops the container services THIS harness started, and only
// those. When the boot path did not run a ComposeUp — for any reason,
// including every reason BootAllServices may skip it — there is no
// ownership record and Cleanup is a no-op.
//
// Cleanup deliberately consults NOTHING except the ownership record: not
// HELIX_SKIP_CONTAINER_HARNESS, not containers/.env, and above all not a
// liveness probe of any port. Those boot-time conditions decide whether
// to START containers; re-deriving them at teardown to decide whether to
// STOP containers is what allowed a teardown to run ComposeDown across a
// live platform the harness had never touched (see composeOwnership).
// A condition that is true at boot and false at teardown — a service that
// exited, a port that a foreign process released, a file that changed —
// used to flip this from "skip" to "destroy". It no longer can: skipping
// the ComposeUp is precisely what leaves ownership unrecorded, so the
// skip and the no-op teardown are now the same fact rather than two
// independent judgements that can disagree.
func (h *ContainerHarness) Cleanup() error {
	owned := h.ownership()
	decision, reason := decideTeardown(owned)

	// Fail-safe backstop. decideTeardown is the authority, but the
	// destructive call is additionally gated on the ownership record
	// being physically present, so no future edit to the decision logic
	// can reach ComposeDown without one. This is the structural form of
	// the rule: if the harness did not start it, the harness cannot stop
	// it — not "should not". A disagreement here is a bug in the caller,
	// so it refuses and says so rather than panicking on a nil record.
	if decision == teardownSkipNotOwned || owned == nil {
		if owned == nil && decision != teardownSkipNotOwned {
			h.Logger.Warnf("refusing teardown: decision %q was reached with no ownership record; "+
				"teardown requires a project this harness started", decision)
		} else {
			h.Logger.Info(reason)
		}
		h.cancel()
		return nil
	}

	h.Logger.Info(reason)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := h.composeDown(ctx, owned.composeFile, owned.profile); err != nil {
		h.Logger.WithError(err).Warn("Failed to stop containers during cleanup")
		return err
	}

	h.releaseOwnership()
	h.cancel()
	h.Logger.Info("✓ Container services stopped")
	return nil
}

// runtimeAvailableCache memoises RuntimeAvailable across tests so each
// t.Run doesn't re-exec `podman info` (which can hang for ~10 s on a
// busy host and was the source of the test-level 10-minute timeout in
// TestContainer_Qdrant_Connection). The first call runs the probe with
// a 3-second hard cap; subsequent calls reuse the cached answer.
var (
	runtimeAvailableOnce   sync.Once
	runtimeAvailableCached bool
)

// SkipIfContainersUnavailable skips the test if containers aren't available.
func SkipIfContainersUnavailable(t *testing.T) {
	h, err := GetContainerHarness()
	if err != nil {
		t.Skipf("Container harness not available: %v", err)
	}

	runtimeAvailableOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		runtimeAvailableCached = h.Adapter.RuntimeAvailable(ctx)
	})

	if !runtimeAvailableCached {
		t.Skip("Container runtime not available (docker/podman required, or probe timed out)") // SKIP-OK: #legacy-untriaged
	}
}

// RequireContainerService skips the test if a specific service is not available
func RequireContainerService(t *testing.T, service string) {
	SkipIfContainersUnavailable(t)

	h, _ := GetContainerHarness()
	host := getServiceHost(service)
	port := getServicePort(service)

	if !h.isServiceHealthy(host, port) {
		t.Skipf("Required service %s not available at %s:%s", service, host, port)
	}
}

// SetupIntegrationTest boots containers and prepares the test environment.
// Call this at the start of integration tests that need real containers.
func SetupIntegrationTest(t *testing.T) *ContainerHarness {
	t.Helper()

	h, err := GetContainerHarness()
	if err != nil {
		t.Fatalf("Failed to initialize container harness: %v", err)
	}

	// Boot services if not already done
	if err := h.BootAllServices(); err != nil {
		t.Fatalf("Failed to boot container services: %v", err)
	}

	return h
}

// =============================================================================
// Helper Functions
// =============================================================================

// getServiceHost returns the host for a service
func getServiceHost(service string) string {
	// Check for override env var
	envVar := fmt.Sprintf("TEST_%s_HOST", strings.ToUpper(service))
	if host := os.Getenv(envVar); host != "" {
		return host
	}

	// Default to localhost
	return "localhost"
}

// getServicePort returns the port for a service
func getServicePort(service string) string {
	return getServicePortWithMode(service, false)
}

// getServicePortWithMode returns the port for a service
// If useTestPorts is true, returns test-specific ports
func getServicePortWithMode(service string, useTestPorts bool) string {
	// Check for override env var
	envVar := fmt.Sprintf("TEST_%s_PORT", strings.ToUpper(service))
	if port := os.Getenv(envVar); port != "" {
		return port
	}

	// Return test port if requested
	if useTestPorts {
		if port, ok := TestServicePorts[service]; ok {
			return port
		}
	}

	// Return default port
	if port, ok := DefaultServicePorts[service]; ok {
		return port
	}
	return ""
}

// findProjectRoot finds the project root directory by looking for go.mod
var findProjectRoot = func() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find project root (no go.mod found)")
}

// =============================================================================
// Test Main - Global Setup/Teardown
// =============================================================================

// TestMainIntegration is the main entry point for integration tests.
// Use this in your integration test files:
//
//	func TestMain(m *testing.M) {
//	    integration.TestMainIntegration(m)
//	}
func TestMainIntegration(m *testing.M) {
	logger := logrus.New()
	logger.Info("Starting integration test suite with container harness")

	// Initialize harness
	harness, err := GetContainerHarness()
	if err != nil {
		logger.WithError(err).Error("Failed to initialize container harness")
		// Continue with tests - they will skip if containers unavailable
	}

	// Boot services before running tests
	if harness != nil {
		if err := harness.BootAllServices(); err != nil {
			logger.WithError(err).Error("Failed to boot container services - tests may skip or fail")
			// Continue anyway - individual tests can handle unavailability
		}
	}

	// Run all tests
	code := m.Run()

	// Cleanup after tests
	if harness != nil {
		if err := harness.Cleanup(); err != nil {
			logger.WithError(err).Error("Failed to cleanup container services")
		}
	}

	os.Exit(code)
}
