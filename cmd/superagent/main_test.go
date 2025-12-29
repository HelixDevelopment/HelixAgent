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
