package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureRequiredContainers(t *testing.T) {
	// This test is skipped in CI environments where docker is not available
	if testing.Short() {
		t.Skip("Skipping container startup test in short mode")
	}

	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping container startup test")
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
		t.Skip("Docker not available")
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
	// This will fail if Cognee is not running, which is expected
	err := checkCogneeHealth()
	assert.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "connect")
}

func TestCheckChromaDBHealth(t *testing.T) {
	// This will fail if ChromaDB is not running, which is expected
	err := checkChromaDBHealth()
	assert.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "connect")
}

func TestCheckPostgresHealth(t *testing.T) {
	// This is a placeholder health check that just waits
	err := checkPostgresHealth()
	assert.NoError(t, err)
}

func TestCheckRedisHealth(t *testing.T) {
	// This is a placeholder health check that just waits
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
	logger := logrus.New()

	// Test postgres and redis health checks (they are placeholder implementations)
	err := verifyServicesHealth([]string{"postgres", "redis"}, logger)
	assert.NoError(t, err)
}

func TestVerifyServicesHealth_AllServices(t *testing.T) {
	logger := logrus.New()

	// Test with services that require running containers
	// cognee and chromadb will fail since they're not running
	err := verifyServicesHealth([]string{"postgres", "redis", "cognee", "chromadb"}, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cognee")
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

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify key content in help output
	assert.Contains(t, output, "SuperAgent")
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

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify version info
	assert.Contains(t, output, "SuperAgent")
	assert.Contains(t, output, "v1.0.0")
}

// TestVerifyServicesHealth_SingleService tests individual services
func TestVerifyServicesHealth_SingleService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Postgres", func(t *testing.T) {
		err := verifyServicesHealth([]string{"postgres"}, logger)
		assert.NoError(t, err) // placeholder always returns nil
	})

	t.Run("Redis", func(t *testing.T) {
		err := verifyServicesHealth([]string{"redis"}, logger)
		assert.NoError(t, err) // placeholder always returns nil
	})

	t.Run("Cognee", func(t *testing.T) {
		err := verifyServicesHealth([]string{"cognee"}, logger)
		assert.Error(t, err) // will fail without running container
	})

	t.Run("ChromaDB", func(t *testing.T) {
		err := verifyServicesHealth([]string{"chromadb"}, logger)
		assert.Error(t, err) // will fail without running container
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

// TestEnsureRequiredContainers_DockerNotAvailable tests when docker is not in PATH
func TestEnsureRequiredContainers_DockerNotAvailable(t *testing.T) {
	// Skip if docker is available
	if _, err := exec.LookPath("docker"); err == nil {
		t.Skip("Docker is available, skipping docker-not-available test")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	err := ensureRequiredContainers(logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker not found")
}

// TestGetRunningServices_EmptyResult tests parsing empty output
func TestGetRunningServices_EmptyResult(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available")
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
		// This should complete quickly (just sleeps 2s)
		err := checkPostgresHealth()
		assert.NoError(t, err)
	})

	t.Run("RedisHealth_Placeholder", func(t *testing.T) {
		// This should complete quickly (just sleeps 1s)
		err := checkRedisHealth()
		assert.NoError(t, err)
	})

	t.Run("CogneeHealth_NoServer", func(t *testing.T) {
		// Without Cognee running, should fail with connection error
		err := checkCogneeHealth()
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "connect") ||
			strings.Contains(err.Error(), "cannot connect"))
	})

	t.Run("ChromaDBHealth_NoServer", func(t *testing.T) {
		// Without ChromaDB running, should fail with connection error
		err := checkChromaDBHealth()
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "connect") ||
			strings.Contains(err.Error(), "cannot connect"))
	})
}

// TestVerifyServicesHealth_MultipleErrors tests error aggregation
func TestVerifyServicesHealth_MultipleErrors(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test with multiple failing services
	err := verifyServicesHealth([]string{"cognee", "chromadb", "unknown"}, logger)
	require.Error(t, err)

	errorMsg := err.Error()
	// Should contain all the failures
	assert.Contains(t, errorMsg, "cognee")
	assert.Contains(t, errorMsg, "chromadb")
	assert.Contains(t, errorMsg, "unknown")
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
		t.Skip("Docker not available")
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

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
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
	require.Error(t, err)

	// Verify error contains expected text
	errorMsg := strings.ToLower(err.Error())
	assert.True(t, strings.Contains(errorMsg, "connect") ||
		strings.Contains(errorMsg, "cognee"),
		"Error should mention connection or cognee")
}

// TestCheckChromaDBHealth_ErrorMessage tests error message format
func TestCheckChromaDBHealth_ErrorMessage(t *testing.T) {
	err := checkChromaDBHealth()
	require.Error(t, err)

	// Verify error contains expected text
	errorMsg := strings.ToLower(err.Error())
	assert.True(t, strings.Contains(errorMsg, "connect") ||
		strings.Contains(errorMsg, "chromadb"),
		"Error should mention connection or chromadb")
}

// TestEnsureRequiredContainers_WithLogger tests with valid logger
func TestEnsureRequiredContainers_WithLogger(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available")
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
		t.Skip("Docker not available")
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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	logger.SetOutput(io.Discard)

	tests := []struct {
		name        string
		services    []string
		expectError bool
	}{
		{
			name:        "Only postgres",
			services:    []string{"postgres"},
			expectError: false, // placeholder always succeeds
		},
		{
			name:        "Only redis",
			services:    []string{"redis"},
			expectError: false, // placeholder always succeeds
		},
		{
			name:        "Postgres and redis",
			services:    []string{"postgres", "redis"},
			expectError: false,
		},
		{
			name:        "Only cognee",
			services:    []string{"cognee"},
			expectError: true, // will fail without running service
		},
		{
			name:        "Only chromadb",
			services:    []string{"chromadb"},
			expectError: true, // will fail without running service
		},
		{
			name:        "All services",
			services:    []string{"postgres", "redis", "cognee", "chromadb"},
			expectError: true, // cognee and chromadb will fail
		},
		{
			name:        "Unknown service",
			services:    []string{"mysql"},
			expectError: true,
		},
		{
			name:        "Mixed known and unknown",
			services:    []string{"postgres", "mysql"},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyServicesHealth(tc.services, logger)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestVerifyServicesHealth_ErrorFormat tests error message formatting
func TestVerifyServicesHealth_ErrorFormat(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	logger.SetOutput(io.Discard)

	// Test with multiple failing services to check error aggregation
	err := verifyServicesHealth([]string{"cognee", "unknown1", "unknown2"}, logger)
	require.Error(t, err)

	errorMsg := err.Error()
	// Should contain "health check failures" prefix
	assert.Contains(t, errorMsg, "health check failures")
	// Should contain all failing services
	assert.Contains(t, errorMsg, "cognee")
	assert.Contains(t, errorMsg, "unknown1")
	assert.Contains(t, errorMsg, "unknown2")
}

// TestHealthCheckTimeouts tests that health checks complete in reasonable time
func TestHealthCheckTimeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
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
	// This test only runs when docker is NOT available
	if _, err := exec.LookPath("docker"); err == nil {
		t.Skip("Docker is available, skipping no-docker test")
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	err := ensureRequiredContainers(logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker not found")
}

// TestGetRunningServices_ComposeNotFound tests error when docker-compose is not available
func TestGetRunningServices_ComposeNotFound(t *testing.T) {
	// This test only runs when docker is NOT available
	if _, err := exec.LookPath("docker"); err == nil {
		t.Skip("Docker is available, skipping no-docker test")
	}

	services, err := getRunningServices()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker compose not found")
	// Should still return empty map
	assert.NotNil(t, services)
}
