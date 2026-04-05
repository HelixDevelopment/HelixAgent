package containers

import (
	"testing"
	"time"

	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/lifecycle"
	"digital.vasic.containers/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestOrchestrator creates a LazyOrchestrator for testing.
// It bypasses NewLazyOrchestrator (which requires a container runtime)
// by constructing the struct directly with a NopLogger.
func newTestOrchestrator(t *testing.T) *LazyOrchestrator {
	t.Helper()
	return &LazyOrchestrator{
		services:      make(map[string]*ServiceDefinition),
		booters:       make(map[string]*lifecycle.LazyBooter),
		started:       make(map[string]bool),
		failed:        make(map[string]error),
		healthChecker: health.NewDefaultChecker(),
		logger:        logging.NopLogger{},
		workDir:       t.TempDir(),
	}
}

// --- NewLazyOrchestrator ---

func TestNewLazyOrchestrator_NilLogger(t *testing.T) {
	// NewLazyOrchestrator requires a container runtime (docker/podman).
	// If unavailable, it returns an error. We test that nil logger is
	// handled (falls back to NopLogger) regardless of outcome.
	orch, err := NewLazyOrchestrator("", nil)
	if err != nil {
		// No container runtime — expected in unit test environments
		assert.Contains(t, err.Error(), "compose")
		return
	}
	require.NotNil(t, orch)
	assert.NotNil(t, orch.logger)
}

func TestNewLazyOrchestrator_EmptyWorkDir(t *testing.T) {
	orch, err := NewLazyOrchestrator("", logging.NopLogger{})
	if err != nil {
		// No container runtime — expected in unit test environments
		assert.Contains(t, err.Error(), "compose")
		return
	}
	require.NotNil(t, orch)
	assert.NotEmpty(t, orch.workDir)
}

// --- RegisterService ---

func TestLazyOrchestrator_RegisterService(t *testing.T) {
	orch := newTestOrchestrator(t)

	svc := &ServiceDefinition{
		Name:        "test-svc",
		ComposeFile: "/tmp/docker-compose.test.yml",
		Description: "Test service",
		Category:    "test",
		CostModel:   "free",
	}

	err := orch.RegisterService(svc)
	require.NoError(t, err)

	// Verify service is registered
	assert.Contains(t, orch.services, "test-svc")
	assert.Equal(t, "test-svc", orch.services["test-svc"].Name)
	assert.Equal(t, "Test service", orch.services["test-svc"].Description)

	// Verify booter was created
	assert.Contains(t, orch.booters, "test-svc")
}

func TestLazyOrchestrator_RegisterService_Defaults(t *testing.T) {
	orch := newTestOrchestrator(t)

	svc := &ServiceDefinition{
		Name:        "defaults-svc",
		ComposeFile: "/tmp/docker-compose.yml",
	}

	err := orch.RegisterService(svc)
	require.NoError(t, err)

	registered := orch.services["defaults-svc"]
	assert.Equal(t, 5*time.Minute, registered.StartTimeout)
	assert.Equal(t, 30*time.Second, registered.StopTimeout)
	assert.Equal(t, "free", registered.CostModel)
}

func TestLazyOrchestrator_RegisterService_CustomTimeouts(t *testing.T) {
	orch := newTestOrchestrator(t)

	svc := &ServiceDefinition{
		Name:         "custom-svc",
		ComposeFile:  "/tmp/docker-compose.yml",
		StartTimeout: 10 * time.Minute,
		StopTimeout:  1 * time.Minute,
		CostModel:    "paid",
	}

	err := orch.RegisterService(svc)
	require.NoError(t, err)

	registered := orch.services["custom-svc"]
	assert.Equal(t, 10*time.Minute, registered.StartTimeout)
	assert.Equal(t, 1*time.Minute, registered.StopTimeout)
	assert.Equal(t, "paid", registered.CostModel)
}

func TestLazyOrchestrator_RegisterService_EmptyName(t *testing.T) {
	orch := newTestOrchestrator(t)

	svc := &ServiceDefinition{
		Name:        "",
		ComposeFile: "/tmp/docker-compose.yml",
	}

	err := orch.RegisterService(svc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service name is required")
}

func TestLazyOrchestrator_RegisterService_EmptyComposeFile(t *testing.T) {
	orch := newTestOrchestrator(t)

	svc := &ServiceDefinition{
		Name:        "no-compose",
		ComposeFile: "",
	}

	err := orch.RegisterService(svc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compose file is required")
}

func TestLazyOrchestrator_RegisterService_Duplicate(t *testing.T) {
	orch := newTestOrchestrator(t)

	svc1 := &ServiceDefinition{
		Name:        "dup-svc",
		ComposeFile: "/tmp/docker-compose.yml",
		Description: "first",
	}
	svc2 := &ServiceDefinition{
		Name:        "dup-svc",
		ComposeFile: "/tmp/docker-compose2.yml",
		Description: "second",
	}

	err := orch.RegisterService(svc1)
	require.NoError(t, err)

	// Registering a duplicate overwrites the previous one
	err = orch.RegisterService(svc2)
	require.NoError(t, err)

	assert.Equal(t, "second", orch.services["dup-svc"].Description)
}

func TestLazyOrchestrator_RegisterService_MultipleServices(t *testing.T) {
	orch := newTestOrchestrator(t)

	services := []struct {
		name     string
		category string
	}{
		{"svc-a", "infra"},
		{"svc-b", "rag"},
		{"svc-c", "monitoring"},
	}

	for _, s := range services {
		err := orch.RegisterService(&ServiceDefinition{
			Name:        s.name,
			ComposeFile: "/tmp/docker-compose.yml",
			Category:    s.category,
		})
		require.NoError(t, err)
	}

	assert.Len(t, orch.services, 3)
	assert.Len(t, orch.booters, 3)
}

// --- GetServiceStatus ---

func TestLazyOrchestrator_GetServiceStatus(t *testing.T) {
	orch := newTestOrchestrator(t)

	svc := &ServiceDefinition{
		Name:        "status-svc",
		ComposeFile: "/tmp/docker-compose.yml",
		Description: "Status test",
		Category:    "test",
		CostModel:   "freemium",
	}

	err := orch.RegisterService(svc)
	require.NoError(t, err)

	status, err := orch.GetServiceStatus("status-svc")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, "status-svc", status.Name)
	assert.Equal(t, "test", status.Category)
	assert.Equal(t, "freemium", status.CostModel)
	assert.Equal(t, "Status test", status.Description)
	assert.False(t, status.Started)
	assert.False(t, status.IsStarting)
	assert.Empty(t, status.LastError)
}

func TestLazyOrchestrator_GetServiceStatus_Unknown(t *testing.T) {
	orch := newTestOrchestrator(t)

	_, err := orch.GetServiceStatus("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service not found")
}

// --- ListServices ---

func TestLazyOrchestrator_ListServices_Empty(t *testing.T) {
	orch := newTestOrchestrator(t)

	services := orch.ListServices()
	assert.NotNil(t, services)
	assert.Len(t, services, 0)
}

func TestLazyOrchestrator_ListServices(t *testing.T) {
	orch := newTestOrchestrator(t)

	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		err := orch.RegisterService(&ServiceDefinition{
			Name:        name,
			ComposeFile: "/tmp/docker-compose.yml",
		})
		require.NoError(t, err)
	}

	services := orch.ListServices()
	assert.Len(t, services, 3)

	// Collect returned names
	returnedNames := make(map[string]bool)
	for _, svc := range services {
		returnedNames[svc.Name] = true
	}

	for _, name := range names {
		assert.True(t, returnedNames[name], "expected service %q in list", name)
	}
}

// --- ServiceDefinition struct ---

func TestServiceDefinition_Fields(t *testing.T) {
	svc := &ServiceDefinition{
		Name:         "full-svc",
		ComposeFile:  "/path/to/compose.yml",
		Profile:      "minimal",
		Required:     true,
		Dependencies: []string{"dep-a", "dep-b"},
		HealthTarget: &health.HealthTarget{
			Name: "full-svc",
			Host: "localhost",
			Port: "8080",
			Type: health.HealthTCP,
		},
		StartTimeout: 3 * time.Minute,
		StopTimeout:  15 * time.Second,
		Description:  "Full service definition",
		Category:     "infrastructure",
		CostModel:    "paid",
	}

	assert.Equal(t, "full-svc", svc.Name)
	assert.Equal(t, "/path/to/compose.yml", svc.ComposeFile)
	assert.Equal(t, "minimal", svc.Profile)
	assert.True(t, svc.Required)
	assert.Len(t, svc.Dependencies, 2)
	assert.NotNil(t, svc.HealthTarget)
	assert.Equal(t, 3*time.Minute, svc.StartTimeout)
	assert.Equal(t, 15*time.Second, svc.StopTimeout)
	assert.Equal(t, "Full service definition", svc.Description)
	assert.Equal(t, "infrastructure", svc.Category)
	assert.Equal(t, "paid", svc.CostModel)
}

// --- ServiceStatus struct ---

func TestServiceStatus_Fields(t *testing.T) {
	status := &ServiceStatus{
		Name:        "test",
		Category:    "infra",
		CostModel:   "free",
		Description: "A test service",
		Started:     true,
		IsStarting:  false,
		LastError:   "some error",
	}

	assert.Equal(t, "test", status.Name)
	assert.Equal(t, "infra", status.Category)
	assert.Equal(t, "free", status.CostModel)
	assert.Equal(t, "A test service", status.Description)
	assert.True(t, status.Started)
	assert.False(t, status.IsStarting)
	assert.Equal(t, "some error", status.LastError)
}

// --- StartService (unit-level: error cases only, no real Docker) ---

func TestLazyOrchestrator_StartService_UnknownService(t *testing.T) {
	orch := newTestOrchestrator(t)

	ctx := t.Context()
	err := orch.StartService(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service not found")
}
