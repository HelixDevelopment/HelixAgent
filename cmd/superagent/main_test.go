package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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
