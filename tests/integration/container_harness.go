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

	return h, nil
}

// BootAllServices starts all required services using the container adapter.
// This ensures tests have access to real containers, not mocks.
//
// The harness is a no-op in any of these situations, which match the
// container-orchestration rule from CLAUDE.md ("the HelixAgent binary
// orchestrates all containers"):
//   - HELIX_SKIP_CONTAINER_HARNESS is set (explicit opt-out)
//   - CONTAINERS_REMOTE_ENABLED=true in Containers/.env
//     (all containers live on remote hosts; a local ComposeUp would
//     duplicate them and almost certainly fail)
//   - HelixAgent's own health endpoint is reachable on :7061
//     (the binary is already running, which means it has already
//     booted every required container via its own adapter)
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
			"Containers/.env has CONTAINERS_REMOTE_ENABLED=true — skipping local ComposeUp " +
				"(containers are orchestrated on remote hosts, per CLAUDE.md)",
		)
		return nil
	}

	if helixAgentPortOpen() {
		h.Logger.Info(
			"HelixAgent already reachable on :7061 — skipping duplicate ComposeUp " +
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

	// Wait for all services to be healthy
	if err := h.waitForServicesHealthy(ctx); err != nil {
		return fmt.Errorf("services failed health checks: %w", err)
	}

	h.Logger.Info("✓ All container services are healthy and ready for tests")
	return nil
}

// isRemoteContainersEnabled returns true when Containers/.env has
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

// helixAgentPortOpen does a 2-second TCP probe against HelixAgent's
// default port. Used as a positive signal that the full container
// fleet is already up (since HelixAgent refuses to bind :7061 until
// its own health-check harness has cleared every required service).
func helixAgentPortOpen() bool {
	conn, err := net.DialTimeout("tcp", "localhost:7061", 2*time.Second)
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

// Cleanup stops all container services. When the boot path was a
// no-op (caller already running HelixAgent / remote-enabled mode),
// cleanup is also a no-op — tearing down containers we did not
// start would kill the user's live environment.
func (h *ContainerHarness) Cleanup() error {
	if os.Getenv("HELIX_SKIP_CONTAINER_HARNESS") != "" {
		h.cancel()
		return nil
	}
	if isRemoteContainersEnabled(h.projectRoot) {
		h.Logger.Info("CONTAINERS_REMOTE_ENABLED=true — skipping local ComposeDown (remote containers owned by HelixAgent binary)")
		h.cancel()
		return nil
	}
	if helixAgentPortOpen() {
		h.Logger.Info("HelixAgent still running on :7061 — skipping ComposeDown (not our containers to stop)")
		h.cancel()
		return nil
	}

	h.Logger.Info("Cleaning up container services...")

	composeFile := "docker-compose.yml"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := h.Adapter.ComposeDown(ctx, composeFile, "default"); err != nil {
		h.Logger.WithError(err).Warn("Failed to stop containers during cleanup")
		return err
	}

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
		t.Skip("Container runtime not available (docker/podman required, or probe timed out)")  // SKIP-OK: #legacy-untriaged
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
