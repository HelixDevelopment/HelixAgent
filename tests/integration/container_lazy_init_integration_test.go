//go:build integration
// +build integration

// Package integration provides lazy container orchestrator integration tests.
// These tests verify that LazyOrchestrator correctly registers services,
// lists them, and reports status without requiring live Docker.
package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/containers"
)

// TestLazyOrchestrator_RegisterAndList verifies that services can be
// registered and retrieved via ListServices.
func TestLazyOrchestrator_RegisterAndList(t *testing.T) {
	lo, err := containers.NewLazyOrchestrator(t.TempDir(), nil)
	require.NoError(t, err, "NewLazyOrchestrator should succeed")

	err = lo.RegisterService(&containers.ServiceDefinition{
		Name:         "test-redis",
		ComposeFile:  "docker-compose.test.yml",
		Required:     false,
		Description:  "Test Redis instance",
		Category:     "cache",
		CostModel:    "free",
		StartTimeout: 10 * time.Second,
	})
	require.NoError(t, err, "RegisterService should succeed")

	services := lo.ListServices()
	assert.NotEmpty(t, services, "should have at least one registered service")

	found := false
	for _, s := range services {
		if s.Name == "test-redis" {
			found = true
			assert.Equal(t, "cache", s.Category)
			assert.Equal(t, "free", s.CostModel)
		}
	}
	assert.True(t, found, "test-redis should appear in service list")
}

// TestLazyOrchestrator_DuplicateRegistration verifies that registering
// the same service name twice returns an error.
func TestLazyOrchestrator_DuplicateRegistration(t *testing.T) {
	lo, err := containers.NewLazyOrchestrator(t.TempDir(), nil)
	require.NoError(t, err)

	svc := &containers.ServiceDefinition{
		Name:        "dup-svc",
		ComposeFile: "docker-compose.test.yml",
		Category:    "test",
	}

	err = lo.RegisterService(svc)
	require.NoError(t, err, "first registration should succeed")

	err = lo.RegisterService(svc)
	assert.Error(t, err, "duplicate registration should fail")
}

// TestLazyOrchestrator_GetServiceStatus_NotStarted verifies that a
// registered but not-yet-started service reports Started=false.
func TestLazyOrchestrator_GetServiceStatus_NotStarted(t *testing.T) {
	lo, err := containers.NewLazyOrchestrator(t.TempDir(), nil)
	require.NoError(t, err)

	err = lo.RegisterService(&containers.ServiceDefinition{
		Name:        "status-check",
		ComposeFile: "docker-compose.test.yml",
	})
	require.NoError(t, err)

	status, err := lo.GetServiceStatus("status-check")
	require.NoError(t, err)
	assert.False(t, status.Started, "service should not be started yet")
}

// TestLazyOrchestrator_GetServiceStatus_Unknown verifies that querying
// an unregistered service name returns an error.
func TestLazyOrchestrator_GetServiceStatus_Unknown(t *testing.T) {
	lo, err := containers.NewLazyOrchestrator(t.TempDir(), nil)
	require.NoError(t, err)

	_, err = lo.GetServiceStatus("nonexistent-service")
	assert.Error(t, err, "unknown service should return an error")
}
