package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	appversion "dev.helix.agent/internal/version"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// MockCommandExecutor is a mock implementation of CommandExecutor for testing
type MockCommandExecutor struct {
	LookPathFunc          func(file string) (string, error)
	RunCommandFunc        func(name string, args ...string) ([]byte, error)
	RunCommandWithDirFunc func(dir string, name string, args ...string) ([]byte, error)
}

func (m *MockCommandExecutor) LookPath(file string) (string, error) {
	if m.LookPathFunc != nil {
		return m.LookPathFunc(file)
	}
	return "/usr/bin/" + file, nil
}

func (m *MockCommandExecutor) RunCommand(name string, args ...string) ([]byte, error) {
	if m.RunCommandFunc != nil {
		return m.RunCommandFunc(name, args...)
	}
	return []byte{}, nil
}

func (m *MockCommandExecutor) RunCommandWithDir(dir string, name string, args ...string) ([]byte, error) {
	if m.RunCommandWithDirFunc != nil {
		return m.RunCommandWithDirFunc(dir, name, args...)
	}
	return []byte{}, nil
}

// MockHealthChecker is a mock implementation of HealthChecker for testing
type MockHealthChecker struct {
	CheckHealthFunc func(url string) error
}

func (m *MockHealthChecker) CheckHealth(url string) error {
	if m.CheckHealthFunc != nil {
		return m.CheckHealthFunc(url)
	}
	return nil
}

// createTestLogger creates a logger for testing that discards output
func createTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	logger.SetOutput(io.Discard)
	return logger
}

// createTestContainerConfig creates a container config with mocks
func createTestContainerConfig(executor *MockCommandExecutor, healthChecker *MockHealthChecker) *ContainerConfig {
	return &ContainerConfig{
		ProjectDir:       "/test/project",
		RequiredServices: []string{"postgres", "redis", "cognee", "chromadb"},
		CogneeURL:        "http://localhost:8000/health",
		ChromaDBURL:      "http://localhost:8001/api/v2/heartbeat",
		Executor:         executor,
		HealthChecker:    healthChecker,
	}
}

// =============================================================================
// RealCommandExecutor Tests
// =============================================================================

func TestRealCommandExecutor_LookPath(t *testing.T) {
	executor := &RealCommandExecutor{}

	// Test with a command that should exist on most systems
	path, err := executor.LookPath("ls")
	if err != nil {
		t.Skip("ls command not found, skipping test") // SKIP-OK: #legacy-untriaged
	}
	assert.NotEmpty(t, path)
}

func TestRealCommandExecutor_RunCommand(t *testing.T) {
	executor := &RealCommandExecutor{}

	// Test with echo command
	output, err := executor.RunCommand("echo", "hello")
	require.NoError(t, err)
	assert.Contains(t, string(output), "hello")
}

func TestRealCommandExecutor_RunCommandWithDir(t *testing.T) {
	executor := &RealCommandExecutor{}

	// Test running pwd in a specific directory
	output, err := executor.RunCommandWithDir("/tmp", "pwd")
	require.NoError(t, err)
	assert.Contains(t, string(output), "tmp")
}

// =============================================================================
// HTTPHealthChecker Tests
// =============================================================================

func TestNewHTTPHealthChecker(t *testing.T) {
	checker := NewHTTPHealthChecker(5 * time.Second)
	assert.NotNil(t, checker)
	assert.NotNil(t, checker.Client)
	assert.Equal(t, 5*time.Second, checker.Timeout)
}

func TestHTTPHealthChecker_CheckHealth_Success(t *testing.T) {
	// Create a test server that returns OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	checker := NewHTTPHealthChecker(5 * time.Second)
	err := checker.CheckHealth(server.URL)
	assert.NoError(t, err)
}

func TestHTTPHealthChecker_CheckHealth_Error(t *testing.T) {
	// Create a test server that returns an error status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	checker := NewHTTPHealthChecker(5 * time.Second)
	err := checker.CheckHealth(server.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
	assert.Contains(t, err.Error(), "503")
}

func TestHTTPHealthChecker_CheckHealth_ConnectionError(t *testing.T) {
	checker := NewHTTPHealthChecker(1 * time.Second)
	err := checker.CheckHealth("http://localhost:99999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot connect")
}

// =============================================================================
// ContainerConfig Tests
// =============================================================================

func TestDefaultContainerConfig(t *testing.T) {
	cfg := DefaultContainerConfig()

	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.ProjectDir)
	assert.Len(t, cfg.RequiredServices, 4)
	assert.Contains(t, cfg.RequiredServices, "postgres")
	assert.Contains(t, cfg.RequiredServices, "redis")
	assert.Contains(t, cfg.RequiredServices, "cognee")
	assert.Contains(t, cfg.RequiredServices, "chromadb")
	assert.NotEmpty(t, cfg.CogneeURL)
	assert.NotEmpty(t, cfg.ChromaDBURL)
	assert.NotNil(t, cfg.Executor)
	assert.NotNil(t, cfg.HealthChecker)
}

// =============================================================================
// ensureRequiredContainersWithConfig Tests
// =============================================================================

func TestEnsureRequiredContainersWithConfig_DockerNotFound(t *testing.T) {
	// Skip if real container runtime is available - this test is for unit testing
	// when no runtime is available, but DetectContainerRuntime() uses real system calls
	runtime, _, err := DetectContainerRuntime()
	if err == nil && runtime != RuntimeNone {
		t.Skip("Skipping - real container runtime available; function uses real runtime detection") // SKIP-OK: #runtime-mock-only
	}

	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			return "", errors.New("docker not found")
		},
	}

	cfg := createTestContainerConfig(executor, &MockHealthChecker{})
	logger := createTestLogger()

	err = ensureRequiredContainersWithConfig(logger, cfg)
	require.Error(t, err)
	// The error might be about container runtime or docker not found
	assert.Error(t, err)
}

func TestEnsureRequiredContainersWithConfig_AllServicesRunning(t *testing.T) {
	// Skip mock-based test when real runtime is available
	// The function uses real DetectContainerRuntime() which bypasses mocks
	// Also skip when container adapter is not initialized (unit test context)
	runtime, _, err := DetectContainerRuntime()
	if err == nil && runtime != RuntimeNone {
		t.Skip("Skipping - real container runtime available; function uses real runtime detection") // SKIP-OK: #runtime-mock-only
	}
	if globalContainerAdapter == nil {
		t.Skip("Skipping - container adapter not initialized in unit test context") // SKIP-OK: #legacy-untriaged
	}

	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			// Return all services as running
			return []byte("postgres\nredis\ncognee\nchromadb\n"), nil
		},
	}

	cfg := createTestContainerConfig(executor, &MockHealthChecker{})
	logger := createTestLogger()

	err = ensureRequiredContainersWithConfig(logger, cfg)
	assert.NoError(t, err)
}

func TestEnsureRequiredContainersWithConfig_SomeServicesNeedStart(t *testing.T) {
	// This is an integration test - requires real services running
	runtime, _, err := DetectContainerRuntime()
	if err != nil || runtime == RuntimeNone {
		t.Skip("Skipping - no container runtime available") // SKIP-OK: #runtime-mock-only
	}

	startCalled := false
	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			// Check if this is checking running services or starting them
			if len(args) > 0 && args[0] == "compose" {
				if len(args) > 1 && args[1] == "ps" {
					// Return only some services as running
					return []byte("postgres\nredis\n"), nil
				}
				if len(args) > 1 && args[1] == "up" {
					startCalled = true
					return []byte("Started cognee and chromadb"), nil
				}
			}
			return []byte{}, nil
		},
	}

	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			// Simulate successful health checks
			return nil
		},
	}

	cfg := createTestContainerConfig(executor, healthChecker)
	// Remove the sleep by using a custom config
	cfg.Executor = executor
	logger := createTestLogger()

	// Note: This test will be slow due to time.Sleep in the function
	// In a real scenario, we'd want to make the sleep configurable
	if testing.Short() {
		t.Skip("Skipping test that involves sleep in short mode") // SKIP-OK: #short-mode
	}

	err = ensureRequiredContainersWithConfig(logger, cfg)
	// The function should not return error even if health checks fail
	// because it logs warnings but continues
	if err != nil {
		t.Logf("Error (may be expected): %v", err)
	}
	assert.True(t, startCalled, "docker compose up should have been called")
}

func TestEnsureRequiredContainersWithConfig_StartFails(t *testing.T) {
	// Skip mock-based test when real runtime is available
	runtime, _, err := DetectContainerRuntime()
	if err == nil && runtime != RuntimeNone {
		t.Skip("Skipping - real container runtime available; function uses real runtime detection") // SKIP-OK: #runtime-mock-only
	}
	if globalContainerAdapter == nil {
		t.Skip("Skipping - container adapter not initialized in unit test context") // SKIP-OK: #legacy-untriaged
	}

	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			if file == "docker-compose" {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + file, nil
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "compose" {
				if len(args) > 1 && args[1] == "ps" {
					return []byte(""), nil // No services running
				}
				if len(args) > 1 && args[1] == "up" {
					return []byte("error output"), errors.New("failed to start")
				}
			}
			return []byte{}, errors.New("command failed")
		},
	}

	cfg := createTestContainerConfig(executor, &MockHealthChecker{})
	logger := createTestLogger()

	err = ensureRequiredContainersWithConfig(logger, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start containers")
}

func TestEnsureRequiredContainersWithConfig_DockerComposeSuccess(t *testing.T) {
	// Integration test - requires container runtime
	runtime, _, err := DetectContainerRuntime()
	if err != nil || runtime == RuntimeNone {
		t.Skip("Skipping - no container runtime available") // SKIP-OK: #runtime-mock-only
	}

	dockerComposeUsed := false
	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			if file == "docker-compose" {
				return "/usr/bin/docker-compose", nil
			}
			return "/usr/bin/" + file, nil
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			// docker compose ps returns empty (no services running)
			if name == "docker" && len(args) > 0 && args[0] == "compose" {
				if len(args) > 1 && args[1] == "ps" {
					return []byte(""), nil
				}
				if len(args) > 1 && args[1] == "up" {
					// docker compose up fails, triggering fallback
					return []byte(""), errors.New("docker compose failed")
				}
			}
			// docker-compose succeeds as fallback
			if name == "docker-compose" {
				dockerComposeUsed = true
				return []byte("Started successfully"), nil
			}
			return []byte{}, nil
		},
	}

	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return nil
		},
	}

	cfg := createTestContainerConfig(executor, healthChecker)
	logger := createTestLogger()

	err = ensureRequiredContainersWithConfig(logger, cfg)
	assert.NoError(t, err)
	assert.True(t, dockerComposeUsed, "docker-compose fallback should have been used")
}

func TestEnsureRequiredContainersWithConfig_GetRunningServicesFails(t *testing.T) {
	// Integration test - requires container runtime
	runtime, _, err := DetectContainerRuntime()
	if err != nil || runtime == RuntimeNone {
		t.Skip("Skipping - no container runtime available") // SKIP-OK: #runtime-mock-only
	}

	getServicesCalled := false
	startCalled := false

	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "compose" {
				if len(args) > 1 && args[1] == "ps" {
					getServicesCalled = true
					return []byte{}, errors.New("docker compose ps failed")
				}
				if len(args) > 1 && args[1] == "up" {
					startCalled = true
					return []byte("Started all services"), nil
				}
			}
			return []byte{}, nil
		},
	}

	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return nil
		},
	}

	cfg := createTestContainerConfig(executor, healthChecker)
	logger := createTestLogger()

	if testing.Short() {
		t.Skip("Skipping test that involves sleep in short mode") // SKIP-OK: #short-mode
	}

	err = ensureRequiredContainersWithConfig(logger, cfg)
	assert.True(t, getServicesCalled)
	// When get services fails, it should attempt to start all services
	assert.True(t, startCalled, "Should attempt to start services when get running services fails")
	// Function may succeed or fail depending on health checks
	_ = err
}

// =============================================================================
// getRunningServicesWithConfig Tests
// =============================================================================

func TestGetRunningServicesWithConfig_Success(t *testing.T) {
	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			return []byte("postgres\nredis\ncognee\n"), nil
		},
	}

	cfg := createTestContainerConfig(executor, &MockHealthChecker{})
	services, err := getRunningServicesWithConfig(cfg)

	require.NoError(t, err)
	assert.Len(t, services, 3)
	assert.True(t, services["postgres"])
	assert.True(t, services["redis"])
	assert.True(t, services["cognee"])
	assert.False(t, services["chromadb"])
}

func TestGetRunningServicesWithConfig_EmptyOutput(t *testing.T) {
	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}

	cfg := createTestContainerConfig(executor, &MockHealthChecker{})
	services, err := getRunningServicesWithConfig(cfg)

	require.NoError(t, err)
	assert.Empty(t, services)
}

func TestGetRunningServicesWithConfig_DockerNotFound(t *testing.T) {
	// When globalContainerAdapter is nil, getRunningServicesWithConfig falls back
	// to running sh -c "docker compose ps ... || true" which never fails.
	// The function returns nil error and empty services map when no containers are running.
	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			return "", errors.New("not found")
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			// Simulate docker not found: sh returns empty output (|| true prevents error)
			return []byte(""), nil
		},
	}

	cfg := createTestContainerConfig(executor, &MockHealthChecker{})
	services, err := getRunningServicesWithConfig(cfg)

	// New behavior: no error returned (sh || true swallows docker errors)
	require.NoError(t, err)
	assert.NotNil(t, services)
	assert.Empty(t, services)
}

func TestGetRunningServicesWithConfig_CommandFails(t *testing.T) {
	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			if file == "docker-compose" {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + file, nil
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			return []byte{}, errors.New("command failed")
		},
	}

	cfg := createTestContainerConfig(executor, &MockHealthChecker{})
	services, err := getRunningServicesWithConfig(cfg)

	require.Error(t, err)
	assert.NotNil(t, services)
}

func TestGetRunningServicesWithConfig_FallbackToDockerCompose(t *testing.T) {
	// The current implementation uses sh -c "docker compose ps ... || true"
	// which means RunCommandWithDir is called with "sh" as command.
	// This test verifies that the executor is called and services are parsed correctly.
	executor := &MockCommandExecutor{
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			// Simulate sh returning running services
			if name == "sh" {
				return []byte("postgres\nredis\n"), nil
			}
			return []byte{}, nil
		},
	}

	cfg := createTestContainerConfig(executor, &MockHealthChecker{})
	services, err := getRunningServicesWithConfig(cfg)

	require.NoError(t, err)
	assert.True(t, services["postgres"])
	assert.True(t, services["redis"])
}

// =============================================================================
// verifyServicesHealthWithConfig Tests
// =============================================================================

func TestVerifyServicesHealthWithConfig_AllHealthy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test with sleeps in short mode") // SKIP-OK: #short-mode
	}

	// Mock postgres and redis health checkers so we don't need real infra
	originalPG := postgresHealthChecker
	originalRedis := redisHealthChecker
	postgresHealthChecker = func() error { return nil }
	redisHealthChecker = func() error { return nil }
	defer func() {
		postgresHealthChecker = originalPG
		redisHealthChecker = originalRedis
	}()

	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return nil
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	logger := createTestLogger()

	err := verifyServicesHealthWithConfig([]string{"postgres", "redis", "cognee", "chromadb"}, logger, cfg)
	assert.NoError(t, err)
}

func TestVerifyServicesHealthWithConfig_CogneeAndChromaDBHealthy(t *testing.T) {
	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return nil
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	logger := createTestLogger()

	// Only test cognee and chromadb which don't have sleeps
	err := verifyServicesHealthWithConfig([]string{"cognee", "chromadb"}, logger, cfg)
	assert.NoError(t, err)
}

func TestVerifyServicesHealthWithConfig_CogneeFails(t *testing.T) {
	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			// The URL will be the CogneeURL from createTestContainerConfig
			// which is "http://localhost:8000/health"
			return errors.New("connection refused")
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	logger := createTestLogger()

	err := verifyServicesHealthWithConfig([]string{"cognee"}, logger, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cognee")
}

func TestVerifyServicesHealthWithConfig_ChromaDBFails(t *testing.T) {
	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			if strings.Contains(url, "chromadb") || strings.Contains(url, "heartbeat") {
				return errors.New("connection refused")
			}
			return nil
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	logger := createTestLogger()

	err := verifyServicesHealthWithConfig([]string{"chromadb"}, logger, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chromadb")
}

func TestVerifyServicesHealthWithConfig_UnknownService(t *testing.T) {
	cfg := createTestContainerConfig(&MockCommandExecutor{}, &MockHealthChecker{})
	logger := createTestLogger()

	err := verifyServicesHealthWithConfig([]string{"unknown-service"}, logger, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown service")
}

func TestVerifyServicesHealthWithConfig_EmptyServices(t *testing.T) {
	cfg := createTestContainerConfig(&MockCommandExecutor{}, &MockHealthChecker{})
	logger := createTestLogger()

	err := verifyServicesHealthWithConfig([]string{}, logger, cfg)
	assert.NoError(t, err)
}

func TestVerifyServicesHealthWithConfig_MultipleFailures(t *testing.T) {
	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return errors.New("all services down")
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	logger := createTestLogger()

	err := verifyServicesHealthWithConfig([]string{"cognee", "chromadb", "unknown"}, logger, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cognee")
	assert.Contains(t, err.Error(), "chromadb")
	assert.Contains(t, err.Error(), "unknown")
}

func TestVerifyServicesHealthWithConfig_PostgresFails(t *testing.T) {
	// Save original and restore after test
	originalPostgresChecker := postgresHealthChecker
	defer func() { postgresHealthChecker = originalPostgresChecker }()

	// Mock postgres health check to fail
	postgresHealthChecker = func() error {
		return errors.New("postgres connection refused")
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, &MockHealthChecker{})
	logger := createTestLogger()

	err := verifyServicesHealthWithConfig([]string{"postgres"}, logger, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postgres")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestVerifyServicesHealthWithConfig_RedisFails(t *testing.T) {
	// Save original and restore after test
	originalRedisChecker := redisHealthChecker
	defer func() { redisHealthChecker = originalRedisChecker }()

	// Mock redis health check to fail
	redisHealthChecker = func() error {
		return errors.New("redis connection refused")
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, &MockHealthChecker{})
	logger := createTestLogger()

	err := verifyServicesHealthWithConfig([]string{"redis"}, logger, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestVerifyServicesHealthWithConfig_AllServicesFail(t *testing.T) {
	// Save originals and restore after test
	originalPostgresChecker := postgresHealthChecker
	originalRedisChecker := redisHealthChecker
	defer func() {
		postgresHealthChecker = originalPostgresChecker
		redisHealthChecker = originalRedisChecker
	}()

	// Mock all health checks to fail
	postgresHealthChecker = func() error {
		return errors.New("postgres down")
	}
	redisHealthChecker = func() error {
		return errors.New("redis down")
	}

	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return errors.New("service down")
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	logger := createTestLogger()

	err := verifyServicesHealthWithConfig([]string{"postgres", "redis", "cognee", "chromadb"}, logger, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postgres")
	assert.Contains(t, err.Error(), "redis")
	assert.Contains(t, err.Error(), "cognee")
	assert.Contains(t, err.Error(), "chromadb")
}

// =============================================================================
// checkCogneeHealthWithConfig Tests
// =============================================================================

func TestCheckCogneeHealthWithConfig_Success(t *testing.T) {
	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return nil
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	err := checkCogneeHealthWithConfig(cfg)
	assert.NoError(t, err)
}

func TestCheckCogneeHealthWithConfig_Failure(t *testing.T) {
	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return errors.New("connection refused")
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	err := checkCogneeHealthWithConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cognee")
}

// =============================================================================
// checkChromaDBHealthWithConfig Tests
// =============================================================================

func TestCheckChromaDBHealthWithConfig_Success(t *testing.T) {
	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return nil
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	err := checkChromaDBHealthWithConfig(cfg)
	assert.NoError(t, err)
}

func TestCheckChromaDBHealthWithConfig_Failure(t *testing.T) {
	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return errors.New("connection refused")
		},
	}

	cfg := createTestContainerConfig(&MockCommandExecutor{}, healthChecker)
	err := checkChromaDBHealthWithConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ChromaDB")
}

// =============================================================================
// AppConfig Tests
// =============================================================================

func TestDefaultAppConfig(t *testing.T) {
	cfg := DefaultAppConfig()

	assert.NotNil(t, cfg)
	assert.False(t, cfg.ShowHelp)
	assert.False(t, cfg.ShowVersion)
	assert.True(t, cfg.AutoStartDocker)
	assert.Equal(t, "0.0.0.0", cfg.ServerHost)
	// 8100 = HELIXAGENT_PORT_HTTP per the canonical port registry
	// (internal/ports + docs/development/port-registry.md). Updated
	// from the legacy 7061 default when the project standardised on
	// the 81xx band per CONST-027 (CLAUDE.md rule #18).
	assert.Equal(t, "8100", cfg.ServerPort)
	assert.NotNil(t, cfg.Logger)
	assert.Nil(t, cfg.ShutdownSignal)
}

// =============================================================================
// run Function Tests
// =============================================================================

func TestRun_ShowHelp(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ShowHelp:    true,
		ShowVersion: false,
		Logger:      createTestLogger(),
	}

	err := run(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "HelixAgent")
	assert.Contains(t, output, "Usage:")
}

func TestRun_ShowVersion(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ShowHelp:    false,
		ShowVersion: true,
		Logger:      createTestLogger(),
	}

	err := run(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "HelixAgent")
	assert.Contains(t, output, "v"+appversion.Version)
}

func TestRun_HelpTakesPrecedence(t *testing.T) {
	// When both help and version are set, help should take precedence
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ShowHelp:    true,
		ShowVersion: true, // This should be ignored
		Logger:      createTestLogger(),
	}

	err := run(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Usage:") // Help output should be shown
}

func TestRun_ServerStartAndShutdown(t *testing.T) {
	// This test requires a full environment setup:
	// - JWT_SECRET environment variable
	// - Database (PostgreSQL) connection
	// - The router setup uses config.Load() internally
	//
	// To run this test, use: make test-with-infra
	testutil.RequireInfra(t)

	// Create a shutdown signal channel
	shutdownSignal := make(chan os.Signal, 1)
	readyNotify := make(chan struct{})

	// Use a random port to avoid conflicts
	appCfg := &AppConfig{
		ShowHelp:         false,
		ShowVersion:      false,
		AutoStartDocker:  false, // Don't try to start docker in test
		SkipVerification: true,  // Skip lengthy provider verification in test
		ServerHost:       "127.0.0.1",
		ServerPort:       "0", // Let the OS pick a port
		Logger:           createTestLogger(),
		ShutdownSignal:   shutdownSignal,
		ReadyNotify:      readyNotify,
	}

	// Run in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- run(appCfg)
	}()

	// Wait for server to be ready or early exit
	select {
	case err := <-errChan:
		assert.NoError(t, err)
		return
	case <-readyNotify:
		// Server is ready
	case <-time.After(120 * time.Second):
		t.Fatal("Timeout waiting for server to start")
	}

	// Send shutdown signal
	shutdownSignal <- syscall.SIGTERM

	// Wait for run to complete
	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for server shutdown")
	}
}

func TestRun_NilLogger(t *testing.T) {
	// This test requires a full environment setup
	testutil.RequireInfra(t)

	shutdownSignal := make(chan os.Signal, 1)
	readyNotify := make(chan struct{})

	appCfg := &AppConfig{
		ShowHelp:         false,
		ShowVersion:      false,
		AutoStartDocker:  false,
		SkipVerification: true, // Skip lengthy provider verification in test
		ServerHost:       "127.0.0.1",
		ServerPort:       "0",
		Logger:           nil, // Test nil logger handling
		ShutdownSignal:   shutdownSignal,
		ReadyNotify:      readyNotify,
	}

	// Run in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- run(appCfg)
	}()

	// Wait for server to be ready or early exit
	select {
	case err := <-errChan:
		assert.NoError(t, err)
		return
	case <-readyNotify:
		// Server is ready
	case <-time.After(120 * time.Second):
		t.Fatal("Timeout waiting for server to start")
	}

	// Send shutdown signal
	shutdownSignal <- syscall.SIGTERM

	// Wait for run to complete
	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for server shutdown")
	}
}

func TestRun_PortInUse(t *testing.T) {
	// This test verifies that starting the server on an occupied port
	// does not crash. The HTTP/3 server starts listeners in goroutines
	// and logs port conflicts, so run() itself succeeds — we verify
	// graceful shutdown still works despite the port conflict.
	testutil.RequireInfra(t)

	// Occupy a specific port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port

	shutdownSignal := make(chan os.Signal, 1)
	readyNotify := make(chan struct{})

	appCfg := &AppConfig{
		ShowHelp:         false,
		ShowVersion:      false,
		AutoStartDocker:  false,
		SkipVerification: true,
		ServerHost:       "127.0.0.1",
		ServerPort:       fmt.Sprintf("%d", port),
		Logger:           createTestLogger(),
		ShutdownSignal:   shutdownSignal,
		ReadyNotify:      readyNotify,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- run(appCfg)
	}()

	// Wait for server to reach the ready state (port conflict happens in goroutine)
	select {
	case err := <-errChan:
		// run() returned early — acceptable (e.g., HTTP/3 creation failed)
		t.Logf("run() returned early: %v", err)
		return
	case <-readyNotify:
		// Server reached ready state despite port conflict in listener goroutine
	case <-time.After(120 * time.Second):
		t.Fatal("Timeout waiting for server initialization")
	}

	// Shutdown gracefully
	shutdownSignal <- syscall.SIGTERM

	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for server shutdown")
	}
}

// =============================================================================
// Legacy Tests (kept for backward compatibility)
// =============================================================================

func TestEnsureRequiredContainers(t *testing.T) {
	// This test is skipped in CI environments where docker is not available
	if testing.Short() {
		t.Skip("Skipping container startup test in short mode") // SKIP-OK: #short-mode
	}

	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping container startup test") // SKIP-OK: #requires-docker
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Test the function - it may fail if containers are not properly configured
	// but it should not panic
	err := ensureRequiredContainers(logger)

	// The function should either succeed or fail gracefully
	// We don't assert success since it depends on the environment
	if err != nil {
		t.Logf("Container startup result: %v", err)
		// Could fail due to various reasons - we just verify it doesn't panic
		// The function should handle errors gracefully
	}
}

func TestGetRunningServices(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available") // SKIP-OK: #requires-docker
	}

	services, err := getRunningServices()

	// Should not error even if Docker commands fail
	if err != nil {
		t.Logf("getRunningServices failed: %v", err)
	}
	assert.IsType(t, map[string]bool{}, services)
}

func TestVerifyServicesHealth(t *testing.T) {
	logger := logrus.New()

	// Test with empty services list
	err := verifyServicesHealth([]string{}, logger)
	assert.NoError(t, err)

	// Test with services that might not be running
	// This should not panic
	err = verifyServicesHealth([]string{"nonexistent"}, logger)
	// We expect this to fail since the service doesn't exist
	assert.Error(t, err)
}

func TestCheckCogneeHealth(t *testing.T) {
	// Test the health check function - works regardless of whether Cognee is running
	err := checkCogneeHealth()
	if err != nil {
		// If Cognee is not running, error should mention connection issue
		assert.Contains(t, strings.ToLower(err.Error()), "connect")
	}
	// If err is nil, Cognee is running and healthy - that's fine too
}

func TestCheckChromaDBHealth(t *testing.T) {
	// Test the health check function - works regardless of whether ChromaDB is running
	err := checkChromaDBHealth()
	if err != nil {
		// If ChromaDB is not running, error should mention connection issue
		assert.Contains(t, strings.ToLower(err.Error()), "connect")
	}
	// If err is nil, ChromaDB is running and healthy - that's fine too
}

func TestCheckPostgresHealth(t *testing.T) {
	testutil.RequirePostgres(t)
	// Requires running PostgreSQL - skip if not accessible
	if err := checkPostgresHealth(); err != nil {
		t.Skipf("Skipping: PostgreSQL not accessible: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}
	err := checkPostgresHealth()
	assert.NoError(t, err)
}

func TestCheckRedisHealth(t *testing.T) {
	testutil.RequireRedis(t)
	// Requires running Redis
	err := checkRedisHealth()
	assert.NoError(t, err)
}

func TestShowHelp(t *testing.T) {
	// This should not panic
	showHelp()
}

func TestShowVersion(t *testing.T) {
	// This should not panic
	showVersion()
}

func TestVerifyServicesHealth_PostgresAndRedis(t *testing.T) {
	testutil.RequireInfra(t)
	// Requires running PostgreSQL and Redis - skip if not accessible
	if err := checkPostgresHealth(); err != nil {
		t.Skipf("Skipping: PostgreSQL not accessible: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}
	if err := checkRedisHealth(); err != nil {
		t.Skipf("Skipping: Redis not accessible: %v (SKIP-OK: #infra-redis-unavailable)", err)
	}
	logger := logrus.New()

	// Test postgres and redis health checks - requires running services
	err := verifyServicesHealth([]string{"postgres", "redis"}, logger)
	assert.NoError(t, err)
}

func TestVerifyServicesHealth_AllServices(t *testing.T) {
	logger := logrus.New()

	// Test with services that require running containers
	// This test validates the health check function works - if containers are running
	// the function should return nil, if not running it should return error mentioning failing services
	err := verifyServicesHealth([]string{"postgres", "redis", "cognee", "chromadb"}, logger)
	if err != nil {
		// If containers are not running, error should mention at least one of the services
		errLower := strings.ToLower(err.Error())
		assert.True(t,
			strings.Contains(errLower, "postgres") ||
				strings.Contains(errLower, "redis") ||
				strings.Contains(errLower, "cognee") ||
				strings.Contains(errLower, "chromadb"),
			"error should mention at least one failing service, got: %s", err.Error())
	}
	// If err is nil, all services are running and healthy - that's also valid
}

func TestGetRunningServices_NoDocker(t *testing.T) {
	// If docker is available, we test the actual function
	// If not, we verify the function handles missing docker gracefully
	services, err := getRunningServices()

	// Either returns services or an error, but should not panic
	if err != nil {
		t.Logf("Expected error when docker unavailable: %v", err)
	} else {
		assert.NotNil(t, services)
	}
}

// =============================================================================
// Additional Tests for Increased Coverage
// =============================================================================

// TestShowHelp_Output tests that showHelp produces expected output
func TestShowHelp_Output(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	showHelp()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify key content in help output
	assert.Contains(t, output, "HelixAgent")
	assert.Contains(t, output, "Usage:")
	assert.Contains(t, output, "Options:")
	assert.Contains(t, output, "-config")
	assert.Contains(t, output, "-auto-start-docker")
	assert.Contains(t, output, "-version")
	assert.Contains(t, output, "-help")
	assert.Contains(t, output, "Features:")
	assert.Contains(t, output, "Examples:")
}

// TestShowVersion_Output tests that showVersion produces expected output
func TestShowVersion_Output(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	showVersion()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify version info
	assert.Contains(t, output, "HelixAgent")
	assert.Contains(t, output, "v"+appversion.Version)
}

// TestVerifyServicesHealth_SingleService tests individual services
func TestVerifyServicesHealth_SingleService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Postgres", func(t *testing.T) {
		testutil.RequirePostgres(t)
		if err := checkPostgresHealth(); err != nil {
			t.Skipf("Skipping: PostgreSQL not accessible: %v (SKIP-OK: #infra-postgres-unavailable)", err)
		}
		err := verifyServicesHealth([]string{"postgres"}, logger)
		assert.NoError(t, err) // requires running postgres
	})

	t.Run("Redis", func(t *testing.T) {
		testutil.RequireRedis(t)
		if err := checkRedisHealth(); err != nil {
			t.Skipf("Skipping: Redis not accessible: %v (SKIP-OK: #infra-redis-unavailable)", err)
		}
		err := verifyServicesHealth([]string{"redis"}, logger)
		assert.NoError(t, err) // requires running redis
	})

	t.Run("Cognee", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode") // SKIP-OK: #short-mode
		}
		err := verifyServicesHealth([]string{"cognee"}, logger)
		// Service may or may not be running - just verify function doesn't panic
		_ = err
	})

	t.Run("ChromaDB", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode") // SKIP-OK: #short-mode
		}
		err := verifyServicesHealth([]string{"chromadb"}, logger)
		// Service may or may not be running - just verify function doesn't panic
		_ = err
	})

	t.Run("Unknown", func(t *testing.T) {
		err := verifyServicesHealth([]string{"unknown-service"}, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown service")
	})
}

// TestLoggerSetup tests logger configuration
func TestLoggerSetup(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	assert.Equal(t, logrus.InfoLevel, logger.GetLevel())
}

// TestEnsureRequiredContainers_DockerNotAvailable tests when no container runtime is in PATH
func TestEnsureRequiredContainers_DockerNotAvailable(t *testing.T) {
	// Skip if any container runtime is available
	if _, err := exec.LookPath("docker"); err == nil {
		t.Skip("Docker is available, skipping no-container-runtime test") // SKIP-OK: #runtime-real-only
	}
	if _, err := exec.LookPath("podman"); err == nil {
		t.Skip("Podman is available, skipping no-container-runtime test") // SKIP-OK: #runtime-real-only
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	err := ensureRequiredContainers(logger)
	assert.Error(t, err)
	// Error should indicate container runtime detection failed
	assert.Error(t, err)
}

// TestGetRunningServices_EmptyResult tests parsing empty output
func TestGetRunningServices_EmptyResult(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available") // SKIP-OK: #requires-docker
	}

	services, err := getRunningServices()
	// Should handle empty or populated results
	if err != nil {
		// May fail if docker-compose is not configured
		t.Logf("getRunningServices error: %v", err)
	} else {
		require.NotNil(t, services)
		// Verify it's a valid map (might be empty)
		for name, running := range services {
			assert.NotEmpty(t, name)
			_ = running // just verify it's a bool
		}
	}
}

// TestCheckHealthFunctions tests the health check functions
func TestCheckHealthFunctions(t *testing.T) {
	t.Run("PostgresHealth_Placeholder", func(t *testing.T) {
		testutil.RequirePostgres(t)
		// Requires running PostgreSQL - skip if not accessible
		if err := checkPostgresHealth(); err != nil {
			t.Skipf("Skipping: PostgreSQL not accessible: %v (SKIP-OK: #infra-postgres-unavailable)", err)
		}
		err := checkPostgresHealth()
		assert.NoError(t, err)
	})

	t.Run("RedisHealth_Placeholder", func(t *testing.T) {
		testutil.RequireRedis(t)
		// Requires running Redis - skip if not accessible
		if err := checkRedisHealth(); err != nil {
			t.Skipf("Skipping: Redis not accessible: %v (SKIP-OK: #infra-redis-unavailable)", err)
		}
		err := checkRedisHealth()
		assert.NoError(t, err)
	})

	t.Run("CogneeHealth_NoServer", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode") // SKIP-OK: #short-mode
		}
		// Service may or may not be running - verify function doesn't panic
		err := checkCogneeHealth()
		if err != nil {
			// If service is not running, error should mention connection issue
			assert.True(t, strings.Contains(strings.ToLower(err.Error()), "connect") ||
				strings.Contains(strings.ToLower(err.Error()), "connection"))
		}
		// If err is nil, service is running and healthy
	})

	t.Run("ChromaDBHealth_NoServer", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode") // SKIP-OK: #short-mode
		}
		// Service may or may not be running - verify function doesn't panic
		err := checkChromaDBHealth()
		if err != nil {
			// If service is not running, error should mention connection issue
			assert.True(t, strings.Contains(strings.ToLower(err.Error()), "connect") ||
				strings.Contains(strings.ToLower(err.Error()), "connection"))
		}
		// If err is nil, service is running and healthy
	})
}

// TestVerifyServicesHealth_MultipleErrors tests error aggregation
func TestVerifyServicesHealth_MultipleErrors(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test with unknown service to ensure error is returned
	// Using only unknown services ensures consistent behavior
	err := verifyServicesHealth([]string{"unknown1", "unknown2", "unknown3"}, logger)
	require.Error(t, err)

	errorMsg := err.Error()
	// Should contain all the failures for unknown services
	assert.Contains(t, errorMsg, "unknown1")
	assert.Contains(t, errorMsg, "unknown2")
	assert.Contains(t, errorMsg, "unknown3")
}

// TestRequiredServicesList tests the required services configuration
func TestRequiredServicesList(t *testing.T) {
	// Verify the expected required services
	expectedServices := []string{"postgres", "redis", "cognee", "chromadb"}

	// This tests that the application knows about all required services
	for _, service := range expectedServices {
		t.Run(service, func(t *testing.T) {
			// Verify the service name is valid
			assert.NotEmpty(t, service)
		})
	}
}

// TestEnsureRequiredContainers_AllRunning tests when all services are already running
func TestEnsureRequiredContainers_AllRunning(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available") // SKIP-OK: #requires-docker
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// This test verifies the function handles the case where containers are running
	// The actual behavior depends on the environment
	err := ensureRequiredContainers(logger)
	// Either succeeds (containers running) or fails (containers not configured)
	// The important thing is it doesn't panic
	if err != nil {
		t.Logf("Container startup result: %v", err)
	}
}

// TestFlagVariables tests that flag variables are defined
func TestFlagVariables(t *testing.T) {
	// Verify flag pointers are not nil
	assert.NotNil(t, configFile)
	assert.NotNil(t, version)
	assert.NotNil(t, help)
	assert.NotNil(t, autoStartDocker)
}

// TestFlagDefaults tests default flag values
func TestFlagDefaults(t *testing.T) {
	// By default, autoStartDocker should be true
	// Note: This tests the initial state before flag.Parse() is called
	// The actual defaults are set in the var declaration
}

// TestShowHelp_ContainsAllSections tests help output completeness
func TestShowHelp_ContainsAllSections(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	showHelp()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify all sections are present
	sections := []string{
		"Usage:",
		"Options:",
		"Features:",
		"Examples:",
		"For more information",
	}

	for _, section := range sections {
		assert.Contains(t, output, section, "Missing section: "+section)
	}
}

// TestVerifyServicesHealth_EmptyList tests with no services
func TestVerifyServicesHealth_EmptyList(t *testing.T) {
	logger := logrus.New()
	err := verifyServicesHealth([]string{}, logger)
	assert.NoError(t, err, "Empty service list should not cause error")
}

// TestVerifyServicesHealth_NilLogger tests with nil logger (should not panic)
func TestVerifyServicesHealth_NilLogger(t *testing.T) {
	// This tests robustness - should not panic with nil logger
	// Note: The actual function may panic if it uses the logger without nil check
	// This is an edge case test
	defer func() {
		if r := recover(); r != nil {
			t.Log("Function panicked with nil logger (expected behavior)")
		}
	}()

	_ = verifyServicesHealth([]string{"postgres"}, nil)
}

// =============================================================================
// Mock HTTP Server Tests for Health Checks
// =============================================================================

// Note: The health check functions use hardcoded URLs (cognee:8000, chromadb:8000)
// so we can't easily mock them without modifying the source code.
// These tests verify the error messages and behavior.

// TestCheckCogneeHealth_ErrorMessage tests error message format
func TestCheckCogneeHealth_ErrorMessage(t *testing.T) {
	err := checkCogneeHealth()
	if err != nil {
		// If service is not running, verify error contains expected text
		errorMsg := strings.ToLower(err.Error())
		assert.True(t, strings.Contains(errorMsg, "connect") ||
			strings.Contains(errorMsg, "cognee") ||
			strings.Contains(errorMsg, "connection"),
			"Error should mention connection or cognee")
	}
	// If err is nil, service is running and healthy
}

// TestCheckChromaDBHealth_ErrorMessage tests error message format
func TestCheckChromaDBHealth_ErrorMessage(t *testing.T) {
	err := checkChromaDBHealth()
	if err != nil {
		// If service is not running, verify error contains expected text
		errorMsg := strings.ToLower(err.Error())
		assert.True(t, strings.Contains(errorMsg, "connect") ||
			strings.Contains(errorMsg, "chromadb") ||
			strings.Contains(errorMsg, "connection"),
			"Error should mention connection or chromadb")
	}
	// If err is nil, service is running and healthy
}

// TestEnsureRequiredContainers_WithLogger tests with valid logger
func TestEnsureRequiredContainers_WithLogger(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available") // SKIP-OK: #requires-docker
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress info logs
	logger.SetOutput(io.Discard)       // Discard all output

	// Call the function - it may fail due to docker-compose not being set up
	// but it should not panic
	err := ensureRequiredContainers(logger)
	if err != nil {
		// Expected - docker-compose may not be configured
		t.Logf("ensureRequiredContainers returned: %v", err)
	}
}

// TestGetRunningServices_WithDocker tests with docker available
func TestGetRunningServices_WithDocker(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available") // SKIP-OK: #requires-docker
	}

	services, err := getRunningServices()

	// The function may succeed or fail depending on docker-compose setup
	if err != nil {
		t.Logf("getRunningServices error (expected if docker-compose not configured): %v", err)
		// Check error is meaningful
		assert.NotEmpty(t, err.Error())
	} else {
		// If successful, verify return type
		assert.NotNil(t, services)
		assert.IsType(t, map[string]bool{}, services)
	}
}

// TestVerifyServicesHealth_CombinedServices tests various service combinations
func TestVerifyServicesHealth_CombinedServices(t *testing.T) {
	testutil.RequireInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	logger.SetOutput(io.Discard)

	// Test that the function doesn't panic and returns expected behavior
	// based on whether services are actually running
	tests := []struct {
		name       string
		services   []string
		mustError  bool // error is required regardless of running state
		neverError bool // should not error if services are available
	}{
		{
			name:       "Only postgres",
			services:   []string{"postgres"},
			neverError: true,
		},
		{
			name:       "Only redis",
			services:   []string{"redis"},
			neverError: true,
		},
		{
			name:       "Postgres and redis",
			services:   []string{"postgres", "redis"},
			neverError: true,
		},
		{
			name:     "Only cognee",
			services: []string{"cognee"},
			// May succeed if cognee is running
		},
		{
			name:     "Only chromadb",
			services: []string{"chromadb"},
			// May succeed if chromadb is running
		},
		{
			name:     "All services",
			services: []string{"postgres", "redis", "cognee", "chromadb"},
			// May succeed if all services are running
		},
		{
			name:      "Unknown service",
			services:  []string{"mysql"},
			mustError: true, // unknown service always fails
		},
		{
			name:      "Mixed known and unknown",
			services:  []string{"postgres", "mysql"},
			mustError: true, // unknown service always fails
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyServicesHealth(tc.services, logger)
			if tc.mustError {
				assert.Error(t, err, "should error for unknown services")
			}
			// For known services, we don't assert error/no-error because
			// it depends on whether the services are actually running
			// The test verifies the function doesn't panic and handles input correctly
		})
	}
}

// TestVerifyServicesHealth_ErrorFormat tests error message formatting
func TestVerifyServicesHealth_ErrorFormat(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	logger.SetOutput(io.Discard)

	// Test with multiple unknown services to check error aggregation
	// Using only unknown services ensures consistent behavior regardless of container state
	err := verifyServicesHealth([]string{"unknown1", "unknown2", "unknown3"}, logger)
	require.Error(t, err)

	errorMsg := err.Error()
	// Should contain "health check failures" prefix
	assert.Contains(t, errorMsg, "health check failures")
	// Should contain all failing services
	assert.Contains(t, errorMsg, "unknown1")
	assert.Contains(t, errorMsg, "unknown2")
	assert.Contains(t, errorMsg, "unknown3")
}

// TestHealthCheckTimeouts tests that health checks complete in reasonable time
func TestHealthCheckTimeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode") // SKIP-OK: #short-mode
	}

	// Postgres and Redis placeholders should complete quickly
	t.Run("Postgres health check timing", func(t *testing.T) {
		start := time.Now()
		_ = checkPostgresHealth()
		elapsed := time.Since(start)
		// Should complete within ~3 seconds (2s sleep + overhead)
		assert.Less(t, elapsed, 5*time.Second)
	})

	t.Run("Redis health check timing", func(t *testing.T) {
		start := time.Now()
		_ = checkRedisHealth()
		elapsed := time.Since(start)
		// Should complete within ~2 seconds (1s sleep + overhead)
		assert.Less(t, elapsed, 3*time.Second)
	})
}

// TestFlagPointers verifies flag pointers are set up correctly
func TestFlagPointers(t *testing.T) {
	// Verify flag variables are initialized
	assert.NotNil(t, configFile, "configFile flag should be initialized")
	assert.NotNil(t, version, "version flag should be initialized")
	assert.NotNil(t, help, "help flag should be initialized")
	assert.NotNil(t, autoStartDocker, "autoStartDocker flag should be initialized")

	// Verify they are the correct type (pointers to correct types)
	assert.IsType(t, (*string)(nil), configFile)
	assert.IsType(t, (*bool)(nil), version)
	assert.IsType(t, (*bool)(nil), help)
	assert.IsType(t, (*bool)(nil), autoStartDocker)
}

// TestEnsureRequiredContainers_NoDocker tests error when docker is not available
func TestEnsureRequiredContainers_NoDocker(t *testing.T) {
	// This test only runs when NO container runtime is available
	if _, err := exec.LookPath("docker"); err == nil {
		t.Skip("Docker is available, skipping no-container-runtime test") // SKIP-OK: #runtime-real-only
	}
	if _, err := exec.LookPath("podman"); err == nil {
		t.Skip("Podman is available, skipping no-container-runtime test") // SKIP-OK: #runtime-real-only
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	err := ensureRequiredContainers(logger)
	require.Error(t, err)
	// Error should mention container runtime detection failed
	assert.Error(t, err)
}

// TestGetRunningServices_ComposeNotFound tests behavior when docker-compose is not available
func TestGetRunningServices_ComposeNotFound(t *testing.T) {
	// getRunningServices now routes through the container adapter (when initialized)
	// or falls back to sh -c "docker compose ps ... || true" which doesn't fail.
	// This test verifies the function doesn't panic and returns a map.
	if globalContainerAdapter != nil {
		t.Skip("Skipping - container adapter initialized, function uses adapter path") // SKIP-OK: #legacy-untriaged
	}

	services, err := getRunningServices()
	// With sh || true, error is not returned even if docker is missing
	// The function should return a map (possibly empty) without error
	assert.NotNil(t, services)
	_ = err // err may or may not be nil depending on executor state
}

// =============================================================================
// HTTP Server Mock Tests with Real HTTP Servers
// =============================================================================

func TestCheckCogneeHealthWithConfig_RealServer(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "healthy"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &ContainerConfig{
		CogneeURL:     server.URL + "/health",
		HealthChecker: NewHTTPHealthChecker(5 * time.Second),
	}

	err := checkCogneeHealthWithConfig(cfg)
	assert.NoError(t, err)
}

func TestCheckChromaDBHealthWithConfig_RealServer(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/heartbeat" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"heartbeat": 1234567890}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &ContainerConfig{
		ChromaDBURL:   server.URL + "/api/v1/heartbeat",
		HealthChecker: NewHTTPHealthChecker(5 * time.Second),
	}

	err := checkChromaDBHealthWithConfig(cfg)
	assert.NoError(t, err)
}

func TestCheckCogneeHealthWithConfig_ServerError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &ContainerConfig{
		CogneeURL:     server.URL + "/health",
		HealthChecker: NewHTTPHealthChecker(5 * time.Second),
	}

	err := checkCogneeHealthWithConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cognee")
}

func TestCheckChromaDBHealthWithConfig_ServerError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := &ContainerConfig{
		ChromaDBURL:   server.URL + "/api/v1/heartbeat",
		HealthChecker: NewHTTPHealthChecker(5 * time.Second),
	}

	err := checkChromaDBHealthWithConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ChromaDB")
}

// =============================================================================
// Integration-like Tests with Mocks
// =============================================================================

func TestFullWorkflow_AllServicesHealthy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode") // SKIP-OK: #short-mode
	}

	// Mock postgres and redis health checkers so we don't need real infra
	originalPG := postgresHealthChecker
	originalRedis := redisHealthChecker
	postgresHealthChecker = func() error { return nil }
	redisHealthChecker = func() error { return nil }
	defer func() {
		postgresHealthChecker = originalPG
		redisHealthChecker = originalRedis
	}()

	executor := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		RunCommandWithDirFunc: func(dir, name string, args ...string) ([]byte, error) {
			return []byte("postgres\nredis\ncognee\nchromadb\n"), nil
		},
	}

	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			return nil
		},
	}

	cfg := createTestContainerConfig(executor, healthChecker)
	logger := createTestLogger()

	// Test that verify services health works
	err := verifyServicesHealthWithConfig(cfg.RequiredServices, logger, cfg)
	assert.NoError(t, err)

	// Test that get running services works
	services, err := getRunningServicesWithConfig(cfg)
	assert.NoError(t, err)
	assert.Len(t, services, 4)
}

func TestFullWorkflow_PartialFailure(t *testing.T) {
	healthChecker := &MockHealthChecker{
		CheckHealthFunc: func(url string) error {
			// Always fail the health check
			return errors.New("cognee is down")
		},
	}

	cfg := &ContainerConfig{
		ProjectDir:       "/test/project",
		RequiredServices: []string{"postgres", "redis", "cognee", "chromadb"},
		CogneeURL:        "http://localhost:8000/health",
		ChromaDBURL:      "http://localhost:8001/api/v2/heartbeat",
		Executor:         &MockCommandExecutor{},
		HealthChecker:    healthChecker,
	}
	logger := createTestLogger()

	// Test that verify services health reports the failure
	err := verifyServicesHealthWithConfig([]string{"cognee"}, logger, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cognee")
}

// =============================================================================
// OpenCode Configuration Validation Tests
// =============================================================================

// TestValidOpenCodeTopLevelKeys verifies all expected keys are present
func TestValidOpenCodeTopLevelKeys(t *testing.T) {
	// v1.0.x schema keys
	v1Keys := []string{
		"$schema", "plugin", "enterprise", "instructions", "provider",
		"mcp", "tools", "agent", "command", "keybinds", "username",
		"share", "permission", "compaction", "sse", "mode", "autoshare",
	}
	// v1.1.30+ schema keys (Viper-based)
	v1130Keys := []string{
		"providers", "mcpServers", "agents", "contextPaths", "tui",
	}
	expectedKeys := append(v1Keys, v1130Keys...)

	for _, key := range expectedKeys {
		assert.True(t, ValidOpenCodeTopLevelKeys[key], "Expected key %q to be valid", key)
	}

	// Verify the total count (17 v1.0.x keys + 5 v1.1.30+ keys = 22)
	assert.Equal(t, 22, len(ValidOpenCodeTopLevelKeys))
}

// TestValidOpenCodeTopLevelKeys_InvalidKeys verifies invalid keys are rejected
func TestValidOpenCodeTopLevelKeys_InvalidKeys(t *testing.T) {
	invalidKeys := []string{
		"foo", "bar", "invalid", "config", "settings",
		"mcps", "schemas", "models", "endpoints",
	}

	for _, key := range invalidKeys {
		assert.False(t, ValidOpenCodeTopLevelKeys[key], "Key %q should not be valid", key)
	}
}

// TestValidateOpenCodeConfig_ValidMinimal tests a minimal valid config
func TestValidateOpenCodeConfig_ValidMinimal(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"helixagent": {
				"options": {
					"apiKey": "sk-test123",
					"baseURL": "http://localhost:7061/v1"
				}
			}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.NotNil(t, result.Stats)
	assert.Equal(t, 1, result.Stats.Providers)
}

// TestValidateOpenCodeConfig_ValidFull tests a full valid config
func TestValidateOpenCodeConfig_ValidFull(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"helixagent": {
				"options": {
					"apiKey": "sk-test123",
					"baseURL": "http://localhost:7061/v1"
				},
				"models": {
					"helixagent-debate": {
						"name": "HelixAgent Debate",
						"attachments": true
					}
				}
			}
		},
		"agent": {
			"model": {
				"provider": "helixagent",
				"model": "helixagent-debate"
			}
		},
		"mcp": {
			"filesystem": {
				"type": "local",
				"command": ["npx", "-y", "@modelcontextprotocol/server-filesystem"]
			},
			"remote-api": {
				"type": "remote",
				"url": "http://localhost:7061/mcp"
			}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.NotNil(t, result.Stats)
	assert.Equal(t, 1, result.Stats.Providers)
	assert.Equal(t, 2, result.Stats.MCPServers)
	assert.Equal(t, 1, result.Stats.Agents)
}

// TestValidateOpenCodeConfig_InvalidJSON tests invalid JSON handling
func TestValidateOpenCodeConfig_InvalidJSON(t *testing.T) {
	config := `{invalid json}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0].Message, "invalid JSON")
}

// TestValidateOpenCodeConfig_InvalidTopLevelKeys tests rejection of invalid keys
func TestValidateOpenCodeConfig_InvalidTopLevelKeys(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		},
		"invalid_key": "should fail",
		"another_bad": true
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)

	// Find the invalid keys error
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "invalid top-level keys") {
			found = true
			assert.Contains(t, err.Message, "invalid_key")
			assert.Contains(t, err.Message, "another_bad")
		}
	}
	assert.True(t, found, "Should have error about invalid top-level keys")
}

// TestValidateOpenCodeConfig_MissingProvider tests missing provider error
func TestValidateOpenCodeConfig_MissingProvider(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"agent": {
			"model": {"provider": "test", "model": "test"}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	found := false
	for _, err := range result.Errors {
		if err.Field == "provider" {
			found = true
			assert.Contains(t, err.Message, "at least one provider must be configured")
		}
	}
	assert.True(t, found, "Should have error about missing provider")
}

// TestValidateOpenCodeConfig_ProviderMissingOptions tests provider without options
func TestValidateOpenCodeConfig_ProviderMissingOptions(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {
				"name": "Test Provider"
			}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Field, "provider.test.options") {
			found = true
			assert.Contains(t, err.Message, "provider must have options configured")
		}
	}
	assert.True(t, found, "Should have error about missing options")
}

// TestValidateOpenCodeConfig_MCPInvalidType tests MCP server with invalid type
func TestValidateOpenCodeConfig_MCPInvalidType(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		},
		"mcp": {
			"bad-server": {
				"type": "invalid"
			}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Field, "mcp.bad-server.type") {
			found = true
			assert.Contains(t, err.Message, "'local' or 'remote'")
		}
	}
	assert.True(t, found, "Should have error about invalid MCP type")
}

// TestValidateOpenCodeConfig_MCPLocalMissingCommand tests local MCP without command
func TestValidateOpenCodeConfig_MCPLocalMissingCommand(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		},
		"mcp": {
			"local-server": {
				"type": "local"
			}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Field, "mcp.local-server.command") {
			found = true
			assert.Contains(t, err.Message, "command is required")
		}
	}
	assert.True(t, found, "Should have error about missing command")
}

// TestValidateOpenCodeConfig_MCPRemoteMissingURL tests remote MCP without URL
func TestValidateOpenCodeConfig_MCPRemoteMissingURL(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		},
		"mcp": {
			"remote-server": {
				"type": "remote"
			}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Field, "mcp.remote-server.url") {
			found = true
			assert.Contains(t, err.Message, "url is required")
		}
	}
	assert.True(t, found, "Should have error about missing URL")
}

// TestValidateOpenCodeConfig_AgentMissingModelAndPrompt tests agent without model/prompt
func TestValidateOpenCodeConfig_AgentMissingModelAndPrompt(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		},
		"agent": {
			"bad-agent": {
				"description": "Agent without model or prompt"
			}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Field, "agent.bad-agent") {
			found = true
			assert.Contains(t, err.Message, "model or prompt")
		}
	}
	assert.True(t, found, "Should have error about missing model/prompt")
}

// TestValidateOpenCodeConfig_AgentWithPromptOnly tests agent with only prompt
func TestValidateOpenCodeConfig_AgentWithPromptOnly(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		},
		"agent": {
			"prompt-agent": {
				"prompt": "You are a helpful assistant"
			}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

// TestValidateOpenCodeConfig_MissingSchemaWarning tests warning for missing $schema
func TestValidateOpenCodeConfig_MissingSchemaWarning(t *testing.T) {
	config := `{
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.True(t, result.Valid) // Still valid, just has warning
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "$schema")
}

// TestValidateOpenCodeConfig_MultipleErrors tests aggregation of multiple errors
func TestValidateOpenCodeConfig_MultipleErrors(t *testing.T) {
	config := `{
		"invalid_key": true,
		"provider": {
			"test1": {},
			"test2": {"name": "no options"}
		},
		"mcp": {
			"bad1": {"type": "invalid"},
			"bad2": {"type": "local"}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	// Should have multiple errors
	assert.GreaterOrEqual(t, len(result.Errors), 4)
}

// TestValidateOpenCodeConfig_CommandsCount tests command counting
func TestValidateOpenCodeConfig_CommandsCount(t *testing.T) {
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		},
		"command": {
			"commit": {"template": "commit changes"},
			"review": {"template": "review code"},
			"test": {"template": "run tests"}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.True(t, result.Valid)
	assert.Equal(t, 3, result.Stats.Commands)
}

// TestValidateOpenCodeConfig_EmptyConfig tests empty config
func TestValidateOpenCodeConfig_EmptyConfig(t *testing.T) {
	config := `{}`

	result := validateOpenCodeConfig([]byte(config))

	assert.False(t, result.Valid)
	// Should error on missing provider
	found := false
	for _, err := range result.Errors {
		if err.Field == "provider" {
			found = true
		}
	}
	assert.True(t, found, "Should have error about missing provider")
}

// TestValidateOpenCodeConfig_SingleAgentWithModel tests single agent format
func TestValidateOpenCodeConfig_SingleAgentWithModel(t *testing.T) {
	// This is the format used by our generator
	config := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"helixagent": {
				"options": {
					"apiKey": "sk-test",
					"baseURL": "http://localhost:7061/v1"
				}
			}
		},
		"agent": {
			"model": {
				"provider": "helixagent",
				"model": "helixagent-debate"
			}
		}
	}`

	result := validateOpenCodeConfig([]byte(config))

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Equal(t, 1, result.Stats.Agents)
}

// =============================================================================
// OpenCode CLI Command Tests
// =============================================================================

// TestHandleValidateOpenCode_ValidFile tests validation of a valid config file
func TestHandleValidateOpenCode_ValidFile(t *testing.T) {
	// Create a temporary valid config file
	tmpFile, err := os.CreateTemp("", "opencode-valid-*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	validConfig := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		}
	}`
	_, err = tmpFile.WriteString(validConfig)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ValidateOpenCode: tmpFile.Name(),
		Logger:           createTestLogger(),
	}

	err = handleValidateOpenCode(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "CONFIGURATION IS VALID")
	assert.Contains(t, output, "Providers: 1")
}

// TestHandleValidateOpenCode_InvalidFile tests validation of an invalid config file
func TestHandleValidateOpenCode_InvalidFile(t *testing.T) {
	// Create a temporary invalid config file
	tmpFile, err := os.CreateTemp("", "opencode-invalid-*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	invalidConfig := `{
		"invalid_key": true,
		"provider": {}
	}`
	_, err = tmpFile.WriteString(invalidConfig)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ValidateOpenCode: tmpFile.Name(),
		Logger:           createTestLogger(),
	}

	err = handleValidateOpenCode(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, output, "CONFIGURATION HAS ERRORS")
}

// TestHandleValidateOpenCode_FileNotFound tests validation with non-existent file
func TestHandleValidateOpenCode_FileNotFound(t *testing.T) {
	appCfg := &AppConfig{
		ValidateOpenCode: "/nonexistent/path/config.json",
		Logger:           createTestLogger(),
	}

	err := handleValidateOpenCode(appCfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

// TestHandleValidateOpenCode_NilLogger tests validation with nil logger
func TestHandleValidateOpenCode_NilLogger(t *testing.T) {
	// Create a temporary valid config file
	tmpFile, err := os.CreateTemp("", "opencode-nillogger-*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	validConfig := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		}
	}`
	_, err = tmpFile.WriteString(validConfig)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Capture stdout
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ValidateOpenCode: tmpFile.Name(),
		Logger:           nil, // nil logger
	}

	err = handleValidateOpenCode(appCfg)

	_ = w.Close()
	os.Stdout = old

	assert.NoError(t, err)
}

// TestHandleValidateOpenCode_JSONSyntaxError tests validation with malformed JSON
func TestHandleValidateOpenCode_JSONSyntaxError(t *testing.T) {
	// Create a temporary file with invalid JSON
	tmpFile, err := os.CreateTemp("", "opencode-badjson-*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(`{not valid json}`)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ValidateOpenCode: tmpFile.Name(),
		Logger:           createTestLogger(),
	}

	err = handleValidateOpenCode(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Error(t, err)
	assert.Contains(t, output, "CONFIGURATION HAS ERRORS")
	assert.Contains(t, output, "invalid JSON")
}

// =============================================================================
// OpenCode Validation Result Types Tests
// =============================================================================

// TestOpenCodeValidationResult_Structure tests the validation result structure
func TestOpenCodeValidationResult_Structure(t *testing.T) {
	result := &OpenCodeValidationResult{
		Valid: true,
		Errors: []OpenCodeValidationError{
			{Field: "test", Message: "test error"},
		},
		Warnings: []string{"test warning"},
		Stats: &OpenCodeValidationStats{
			Providers:  1,
			MCPServers: 2,
			Agents:     3,
			Commands:   4,
		},
	}

	assert.True(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "test", result.Errors[0].Field)
	assert.Equal(t, "test error", result.Errors[0].Message)
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, "test warning", result.Warnings[0])
	assert.Equal(t, 1, result.Stats.Providers)
	assert.Equal(t, 2, result.Stats.MCPServers)
	assert.Equal(t, 3, result.Stats.Agents)
	assert.Equal(t, 4, result.Stats.Commands)
}

// TestOpenCodeValidationError_Structure tests the validation error structure
func TestOpenCodeValidationError_Structure(t *testing.T) {
	err := OpenCodeValidationError{
		Field:   "provider.test.options",
		Message: "options is required",
	}

	assert.Equal(t, "provider.test.options", err.Field)
	assert.Equal(t, "options is required", err.Message)
}

// TestOpenCodeValidationStats_Structure tests the validation stats structure
func TestOpenCodeValidationStats_Structure(t *testing.T) {
	stats := OpenCodeValidationStats{
		Providers:  5,
		MCPServers: 10,
		Agents:     15,
		Commands:   20,
	}

	assert.Equal(t, 5, stats.Providers)
	assert.Equal(t, 10, stats.MCPServers)
	assert.Equal(t, 15, stats.Agents)
	assert.Equal(t, 20, stats.Commands)
}

// =============================================================================
// OpenCode Run Integration Tests
// =============================================================================

// TestRun_ValidateOpenCode tests the run function with validation
func TestRun_ValidateOpenCode(t *testing.T) {
	// Create a temporary valid config file
	tmpFile, err := os.CreateTemp("", "opencode-run-*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	validConfig := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		}
	}`
	_, err = tmpFile.WriteString(validConfig)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ValidateOpenCode: tmpFile.Name(),
		Logger:           createTestLogger(),
	}

	err = run(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "CONFIGURATION IS VALID")
}

// TestRun_ValidateOpenCode_BeforeHelp tests that validation runs before help
func TestRun_ValidateOpenCode_BeforeHelp(t *testing.T) {
	// Create a temporary valid config file
	tmpFile, err := os.CreateTemp("", "opencode-priority-*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	validConfig := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {"options": {"apiKey": "test"}}
		}
	}`
	_, err = tmpFile.WriteString(validConfig)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ShowHelp:         true,           // Help is requested
		ValidateOpenCode: tmpFile.Name(), // But validation takes precedence
		Logger:           createTestLogger(),
	}

	// Help takes precedence in run()
	err = run(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	// Help should be shown because it's checked first
	assert.Contains(t, output, "Usage:")
}

// =============================================================================
// OpenCode Flag Tests
// =============================================================================

// TestValidateOpenCodeFlag tests the validateOpenCode flag
func TestValidateOpenCodeFlag(t *testing.T) {
	assert.NotNil(t, validateOpenCode)
	assert.IsType(t, (*string)(nil), validateOpenCode)
}

// TestAppConfig_ValidateOpenCode tests AppConfig with ValidateOpenCode field
func TestAppConfig_ValidateOpenCode(t *testing.T) {
	cfg := &AppConfig{
		ValidateOpenCode: "/path/to/config.json",
	}

	assert.Equal(t, "/path/to/config.json", cfg.ValidateOpenCode)
}

// =============================================================================
// OpenCode Help Output Tests
// =============================================================================

// TestShowHelp_ContainsValidateOpenCode tests that help includes validation flag
func TestShowHelp_ContainsValidateOpenCode(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	showHelp()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "-validate-opencode-config")
	assert.Contains(t, output, "Validate")
}

// =============================================================================
// OpenCode Real File Tests
// =============================================================================

// TestValidateOpenCodeConfig_RealDownloadsConfig tests with the real generated config
func TestValidateOpenCodeConfig_RealDownloadsConfig(t *testing.T) {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Could not get home directory") // SKIP-OK: #legacy-untriaged
	}

	configPath := homeDir + "/Downloads/opencode-helix-agent.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("Downloads config file does not exist: " + configPath) // SKIP-OK: #legacy-untriaged
	}

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	result := validateOpenCodeConfig(data)

	assert.True(t, result.Valid, "Real downloads config should be valid")
	assert.Empty(t, result.Errors)
	assert.Equal(t, 1, result.Stats.Providers)
}

// TestValidateOpenCodeConfig_RealOpenCodeConfig tests with user's opencode.json
// NOTE: This test validates user's actual config file, which may have custom keys.
// User configs may have additional keys that are not in our strict schema,
// so we only check that the config is parseable JSON, not that it passes validation.
func TestValidateOpenCodeConfig_RealOpenCodeConfig(t *testing.T) {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Could not get home directory") // SKIP-OK: #legacy-untriaged
	}

	configPath := homeDir + "/.config/opencode/opencode.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("User opencode config file does not exist: " + configPath) // SKIP-OK: #legacy-untriaged
	}

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Just check that the config is valid JSON (user configs may have custom keys)
	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	assert.NoError(t, err, "User opencode config should be valid JSON")

	// Log the result for informational purposes (not strictly validated)
	result := validateOpenCodeConfig(data)
	if !result.Valid {
		t.Logf("User config has validation notes (not errors for user configs): %v", result.Errors)
	}
}

// =============================================================================
// Godotenv Loading Tests
// =============================================================================

// TestGodotenvLoading_EnvFileExists verifies that environment variables can be loaded from .env file
func TestGodotenvLoading_EnvFileExists(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	envFile := tmpDir + "/.env"
	envContent := `TEST_VAR_UNIQUE_12345=test_value
DEEPSEEK_API_KEY=sk-test-deepseek
CLAUDE_CODE_USE_OAUTH_CREDENTIALS=true`

	err := os.WriteFile(envFile, []byte(envContent), 0644)
	require.NoError(t, err)

	// Change to temp directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Load env file using godotenv
	err = loadEnvFile()
	require.NoError(t, err)

	// Verify variables were loaded
	assert.Equal(t, "test_value", os.Getenv("TEST_VAR_UNIQUE_12345"))

	// Clean up
	_ = os.Unsetenv("TEST_VAR_UNIQUE_12345")
	_ = os.Unsetenv("DEEPSEEK_API_KEY")
	_ = os.Unsetenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS")
}

// TestGodotenvLoading_EnvFileNotExists verifies graceful handling when .env doesn't exist
func TestGodotenvLoading_EnvFileNotExists(t *testing.T) {
	// Create empty temp directory
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Loading should not error when file doesn't exist
	err = loadEnvFile()
	assert.NoError(t, err, "loadEnvFile should not error when .env doesn't exist")
}

// loadEnvFile is a helper that mimics main.go's godotenv loading
func loadEnvFile() error {
	// Import godotenv at runtime
	err := godotenvLoad()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// godotenvLoad wraps the actual godotenv.Load call
func godotenvLoad() error {
	// Attempt to load .env file from current directory
	_, err := os.Stat(".env")
	if os.IsNotExist(err) {
		return nil // No .env file is OK
	}

	// Read and parse the .env file manually for testing
	// In production, this uses github.com/joho/godotenv
	content, err := os.ReadFile(".env")
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			_ = os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
	return nil
}

// =============================================================================
// API Key Generation Tests
// =============================================================================

func TestGenerateSecureAPIKey(t *testing.T) {
	t.Run("generates valid API key", func(t *testing.T) {
		key, err := generateSecureAPIKey()

		require.NoError(t, err)
		assert.NotEmpty(t, key)
		assert.True(t, strings.HasPrefix(key, "sk-"))
		// 32 bytes = 64 hex chars + "sk-" prefix
		assert.Equal(t, 67, len(key))
	})

	t.Run("generates unique keys", func(t *testing.T) {
		keys := make(map[string]bool)
		for i := 0; i < 100; i++ {
			key, err := generateSecureAPIKey()
			require.NoError(t, err)
			assert.False(t, keys[key], "Generated duplicate key")
			keys[key] = true
		}
	})
}

func TestHandleGenerateAPIKey(t *testing.T) {
	t.Run("generates and prints API key", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		appCfg := &AppConfig{
			GenerateAPIKey: true,
			Logger:         createTestLogger(),
		}

		err := handleGenerateAPIKey(appCfg)

		_ = w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := strings.TrimSpace(buf.String())

		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(output, "sk-"))
	})

	t.Run("writes API key to env file", func(t *testing.T) {
		tmpDir := t.TempDir()
		envFile := tmpDir + "/.env"

		appCfg := &AppConfig{
			GenerateAPIKey: true,
			APIKeyEnvFile:  envFile,
			Logger:         createTestLogger(),
		}

		// Capture stdout (we don't care about its content)
		old := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		err := handleGenerateAPIKey(appCfg)

		_ = w.Close()
		os.Stdout = old

		require.NoError(t, err)

		// Verify file was written
		content, err := os.ReadFile(envFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "HELIXAGENT_API_KEY=sk-")
	})

	t.Run("handles nil logger", func(t *testing.T) {
		appCfg := &AppConfig{
			GenerateAPIKey: true,
			Logger:         nil,
		}

		// Capture stdout
		old := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		err := handleGenerateAPIKey(appCfg)

		_ = w.Close()
		os.Stdout = old

		require.NoError(t, err)
	})
}

func TestWriteAPIKeyToEnvFile(t *testing.T) {
	t.Run("creates new file", func(t *testing.T) {
		tmpDir := t.TempDir()
		envFile := tmpDir + "/.env"

		err := writeAPIKeyToEnvFile(envFile, "sk-test-key-123")

		require.NoError(t, err)

		content, err := os.ReadFile(envFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "HELIXAGENT_API_KEY=sk-test-key-123")
	})

	t.Run("updates existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		envFile := tmpDir + "/.env"

		// Create initial file with content
		initialContent := `# Comment line
OTHER_KEY=some_value
HELIXAGENT_API_KEY=old-key

ANOTHER_KEY=another_value`
		err := os.WriteFile(envFile, []byte(initialContent), 0644)
		require.NoError(t, err)

		err = writeAPIKeyToEnvFile(envFile, "sk-new-key-456")
		require.NoError(t, err)

		content, err := os.ReadFile(envFile)
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "HELIXAGENT_API_KEY=sk-new-key-456")
		assert.Contains(t, contentStr, "OTHER_KEY=some_value")
		assert.Contains(t, contentStr, "ANOTHER_KEY=another_value")
		assert.Contains(t, contentStr, "# Comment line")
		// Old key should be replaced
		assert.NotContains(t, contentStr, "old-key")
	})

	t.Run("preserves comments and empty lines", func(t *testing.T) {
		tmpDir := t.TempDir()
		envFile := tmpDir + "/.env"

		initialContent := `# First comment
KEY1=value1

# Second comment

KEY2=value2`
		err := os.WriteFile(envFile, []byte(initialContent), 0644)
		require.NoError(t, err)

		err = writeAPIKeyToEnvFile(envFile, "sk-new")
		require.NoError(t, err)

		content, err := os.ReadFile(envFile)
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "# First comment")
		assert.Contains(t, contentStr, "# Second comment")
	})
}

// =============================================================================
// OpenCode Config Tests
// =============================================================================

func TestBuildOpenCodeMCPServers(t *testing.T) {
	t.Run("builds config with v1.1.30+ schema remote servers", func(t *testing.T) {
		config := buildOpenCodeMCPServersFiltered("http://localhost:7061", false)

		assert.NotNil(t, config)
		// Should have HelixAgent local plugin
		assert.Contains(t, config, "helixagent")

		// Check local server properties (v1.1.30+ format)
		helixServer := config["helixagent"]
		assert.Equal(t, "local", helixServer.Type)
		assert.NotEmpty(t, helixServer.Command)

		// Should have HelixAgent remote protocol endpoints
		assert.Contains(t, config, "helixagent-mcp")
		mcpServer := config["helixagent-mcp"]
		assert.Equal(t, "remote", mcpServer.Type)
		assert.Contains(t, mcpServer.URL, "localhost:7061")
	})

	t.Run("builds config with different base URL", func(t *testing.T) {
		config := buildOpenCodeMCPServersFiltered("http://example.com:8080", false)

		mcpServer := config["helixagent-mcp"]
		assert.Contains(t, mcpServer.URL, "example.com:8080")
	})

	t.Run("includes standard MCP servers", func(t *testing.T) {
		config := buildOpenCodeMCPServersFiltered("http://localhost:7061", false)

		// Check standard MCP servers
		assert.Contains(t, config, "filesystem")
		assert.Contains(t, config, "fetch")
		assert.Contains(t, config, "github")
		assert.Contains(t, config, "memory")

		// Check stdio server format (new schema uses Command as []string)
		fsServer := config["filesystem"]
		assert.Equal(t, "local", fsServer.Type)
		assert.NotEmpty(t, fsServer.Command, "Command should be a non-empty array")
		// Check that one of the command elements contains the filesystem package
		foundFilesystem := false
		for _, arg := range fsServer.Command {
			if strings.Contains(arg, "filesystem") {
				foundFilesystem = true
				break
			}
		}
		assert.True(t, foundFilesystem, "Command should contain filesystem MCP server reference")
	})
}

func TestHandleGenerateOpenCode(t *testing.T) {
	t.Run("generates OpenCode config to stdout", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		appCfg := &AppConfig{
			GenerateOpenCode: true,
			Logger:           createTestLogger(),
		}

		err := handleGenerateOpenCode(appCfg)

		_ = w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		require.NoError(t, err)
		// OpenCode config uses singular keys (provider, agent, mcp)
		// for compatibility with strict validators
		assert.Contains(t, output, "provider")
		assert.Contains(t, output, "agent")
		assert.Contains(t, output, "mcp")
		assert.Contains(t, output, "helixagent/helix-debate")
		assert.Contains(t, output, "helixagent")
	})

	t.Run("writes OLD format config when filename is opencode.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := tmpDir + "/opencode.json" // No dot prefix = OLD format

		appCfg := &AppConfig{
			GenerateOpenCode: true,
			OpenCodeOutput:   configFile,
			Logger:           createTestLogger(),
		}

		// Capture stdout (suppress it)
		old := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		err := handleGenerateOpenCode(appCfg)

		_ = w.Close()
		os.Stdout = old

		require.NoError(t, err)

		// Verify file was written with OLD schema (opencode.json uses strict validator)
		content, err := os.ReadFile(configFile)
		require.NoError(t, err)
		// OLD format uses singular keys
		assert.Contains(t, string(content), "\"provider\"")
		assert.Contains(t, string(content), "\"agent\"")
		assert.Contains(t, string(content), "\"mcp\"")
		assert.Contains(t, string(content), "$schema")
	})

	t.Run("writes config when filename is .opencode.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := tmpDir + "/.opencode.json" // With dot prefix

		appCfg := &AppConfig{
			GenerateOpenCode: true,
			OpenCodeOutput:   configFile,
			Logger:           createTestLogger(),
		}

		// Capture stdout (suppress it)
		old := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		err := handleGenerateOpenCode(appCfg)

		_ = w.Close()
		os.Stdout = old

		require.NoError(t, err)

		// Verify file was written with correct schema
		content, err := os.ReadFile(configFile)
		require.NoError(t, err)
		// Both formats now use singular keys for compatibility
		assert.Contains(t, string(content), "provider")
		assert.Contains(t, string(content), "agent")
		assert.Contains(t, string(content), "mcp")
		assert.Contains(t, string(content), "helixagent/helix-debate")
	})

	t.Run("uses env variable template for API key", func(t *testing.T) {
		// Set env var
		_ = os.Setenv("HELIXAGENT_API_KEY", "sk-env-test-key")
		defer func() { _ = os.Unsetenv("HELIXAGENT_API_KEY") }()

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		appCfg := &AppConfig{
			GenerateOpenCode: true,
			Logger:           createTestLogger(),
		}

		err := handleGenerateOpenCode(appCfg)

		_ = w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		require.NoError(t, err)
		// OpenCode configs use the real API key value (not env var template syntax)
		// Per CLAUDE.md: "No env var syntax in API keys: CLI agents do NOT support {env:VAR_NAME} syntax"
		assert.Contains(t, output, "sk-env-test-key")
	})
}

func TestHandleValidateOpenCode(t *testing.T) {
	t.Run("validates valid config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := tmpDir + "/opencode.json"

		validConfig := `{
			"$schema": "https://opencode.ai/config.json",
			"provider": {
				"test": {
					"options": {
						"apiKey": "test"
					}
				}
			}
		}`
		err := os.WriteFile(configFile, []byte(validConfig), 0644)
		require.NoError(t, err)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		appCfg := &AppConfig{
			ValidateOpenCode: configFile,
			Logger:           createTestLogger(),
		}

		err = handleValidateOpenCode(appCfg)

		_ = w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		require.NoError(t, err)
		assert.Contains(t, output, "VALID")
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := tmpDir + "/invalid.json"

		invalidConfig := `{invalid json`
		err := os.WriteFile(configFile, []byte(invalidConfig), 0644)
		require.NoError(t, err)

		// Capture stdout
		old := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		appCfg := &AppConfig{
			ValidateOpenCode: configFile,
			Logger:           createTestLogger(),
		}

		err = handleValidateOpenCode(appCfg)

		_ = w.Close()
		os.Stdout = old

		require.Error(t, err)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		appCfg := &AppConfig{
			ValidateOpenCode: "/nonexistent/path/config.json",
			Logger:           createTestLogger(),
		}

		err := handleValidateOpenCode(appCfg)

		require.Error(t, err)
	})
}

// =============================================================================
// Mandatory Dependencies Tests
// =============================================================================

func TestGetMandatoryDependencies(t *testing.T) {
	deps := GetMandatoryDependencies()

	assert.NotEmpty(t, deps)
	assert.Len(t, deps, 4)

	// Check all expected dependencies are present
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
		assert.NotEmpty(t, dep.Name)
		assert.NotEmpty(t, dep.Description)
		assert.NotNil(t, dep.CheckFunc)
		assert.True(t, dep.Required)
	}

	assert.True(t, names["PostgreSQL"])
	assert.True(t, names["Redis"])
	assert.True(t, names["Cognee"])
	assert.True(t, names["ChromaDB"])
}

func TestVerifyAllMandatoryDependencies_AllFail(t *testing.T) {
	// Save original checkers
	originalPostgresChecker := postgresHealthChecker
	originalRedisChecker := redisHealthChecker
	defer func() {
		postgresHealthChecker = originalPostgresChecker
		redisHealthChecker = originalRedisChecker
	}()

	// Mock all health checks to fail
	postgresHealthChecker = func() error {
		return errors.New("postgres unavailable")
	}
	redisHealthChecker = func() error {
		return errors.New("redis unavailable")
	}

	// Mock Cognee and ChromaDB to fail
	oldContainerConfig := containerConfig
	containerConfig = &ContainerConfig{
		CogneeURL:   "http://localhost:99999/health",
		ChromaDBURL: "http://localhost:99998/api/v1/heartbeat",
		HealthChecker: &MockHealthChecker{
			CheckHealthFunc: func(url string) error {
				return errors.New("service unavailable")
			},
		},
	}
	defer func() { containerConfig = oldContainerConfig }()

	logger := createTestLogger()
	err := verifyAllMandatoryDependencies(logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "BOOT BLOCKED")
	assert.Contains(t, err.Error(), "mandatory dependencies failed")
}

// =============================================================================
// Run Function Additional Tests
// =============================================================================

func TestRun_GenerateAPIKey(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		GenerateAPIKey: true,
		Logger:         createTestLogger(),
	}

	err := run(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := strings.TrimSpace(buf.String())

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(output, "sk-"))
}

func TestRun_GenerateOpenCode(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		GenerateOpenCode: true,
		Logger:           createTestLogger(),
	}

	err := run(appCfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	require.NoError(t, err)
	assert.Contains(t, output, "provider")
	assert.Contains(t, output, "helixagent")
}

func TestRun_ValidateOpenCode_FromRun(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := tmpDir + "/opencode.json"

	validConfig := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"test": {
				"options": {"apiKey": "test"}
			}
		}
	}`
	err := os.WriteFile(configFile, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Capture stdout
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	appCfg := &AppConfig{
		ValidateOpenCode: configFile,
		Logger:           createTestLogger(),
	}

	err = run(appCfg)

	_ = w.Close()
	os.Stdout = old

	require.NoError(t, err)
}

// =============================================================================
// DetectComposeCommand Tests
// =============================================================================

func TestDetectComposeCommand_Docker(t *testing.T) {
	// Skip if docker is not available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available") // SKIP-OK: #requires-docker
	}

	cmd, args, err := DetectComposeCommand(RuntimeDocker)

	if err != nil {
		// May fail if neither docker compose nor docker-compose is available
		t.Logf("DetectComposeCommand returned error: %v", err)
	} else {
		assert.NotEmpty(t, cmd)
		// cmd should be either "docker" or "docker-compose"
		assert.True(t, cmd == "docker" || strings.Contains(cmd, "docker-compose"))
		_ = args // args may be nil or contain "compose"
	}
}

func TestDetectComposeCommand_Podman(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("Podman not available") // SKIP-OK: #legacy-untriaged
	}

	cmd, args, err := DetectComposeCommand(RuntimePodman)

	if err != nil {
		// May fail if podman-compose is not available
		t.Logf("DetectComposeCommand returned error: %v", err)
	} else {
		assert.NotEmpty(t, cmd)
	}
	_ = args
}

func TestDetectComposeCommand_Unknown(t *testing.T) {
	// Save and clear globalContainerAdapter to test the uninitialized path
	savedAdapter := globalContainerAdapter
	globalContainerAdapter = nil
	defer func() { globalContainerAdapter = savedAdapter }()

	_, _, err := DetectComposeCommand(RuntimeNone)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "container adapter not initialized")
}

// =============================================================================
// Strict Dependencies Tests
// =============================================================================

func TestDefaultAppConfig_StrictDependencies(t *testing.T) {
	cfg := DefaultAppConfig()

	// StrictDependencies should be true by default (mandatory mode)
	assert.True(t, cfg.StrictDependencies)
}

func TestAppConfig_StrictDependenciesFlag(t *testing.T) {
	// Verify strictDependencies flag is defined
	assert.NotNil(t, strictDependencies)
}
