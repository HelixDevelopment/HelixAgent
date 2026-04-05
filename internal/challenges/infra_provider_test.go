package challenges

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBooter struct {
	results []BootResult
	err     error
}

func (m *mockBooter) BootAll() []BootResult { return m.results }
func (m *mockBooter) ShutdownAll() error    { return m.err }

type mockChecker struct {
	err error
}

func (m *mockChecker) CheckWithRetry(
	ctx context.Context,
	name string,
) error {
	return m.err
}

func TestHelixInfraProvider_EnsureRunning_Success(t *testing.T) {
	t.Parallel()
	booter := &mockBooter{
		results: []BootResult{
			{Name: "postgresql", Status: "running"},
		},
	}
	p := NewHelixInfraProvider(booter, nil)

	err := p.EnsureRunning(context.Background(), "postgresql")
	require.NoError(t, err)
}

func TestHelixInfraProvider_EnsureRunning_Failure(t *testing.T) {
	t.Parallel()
	booter := &mockBooter{
		results: []BootResult{
			{
				Name:   "postgresql",
				Status: "failed",
				Error:  errors.New("connection refused"),
			},
		},
	}
	p := NewHelixInfraProvider(booter, nil)

	err := p.EnsureRunning(context.Background(), "postgresql")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestHelixInfraProvider_EnsureRunning_NilBooter(t *testing.T) {
	t.Parallel()
	p := NewHelixInfraProvider(nil, nil)

	err := p.EnsureRunning(context.Background(), "postgresql")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boot manager not configured")
}

func TestHelixInfraProvider_HealthCheck_Success(t *testing.T) {
	t.Parallel()
	checker := &mockChecker{err: nil}
	p := NewHelixInfraProvider(nil, checker)

	err := p.HealthCheck(context.Background(), "redis")
	require.NoError(t, err)
}

func TestHelixInfraProvider_HealthCheck_Failure(t *testing.T) {
	t.Parallel()
	checker := &mockChecker{err: errors.New("unhealthy")}
	p := NewHelixInfraProvider(nil, checker)

	err := p.HealthCheck(context.Background(), "redis")
	require.Error(t, err)
}

func TestHelixInfraProvider_HealthCheck_NilChecker(t *testing.T) {
	t.Parallel()
	p := NewHelixInfraProvider(nil, nil)

	err := p.HealthCheck(context.Background(), "redis")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health checker not configured")
}

func TestHelixInfraProvider_Release(t *testing.T) {
	t.Parallel()
	p := NewHelixInfraProvider(nil, nil)
	err := p.Release(context.Background(), "test")
	require.NoError(t, err)
}

func TestHelixInfraProvider_Shutdown(t *testing.T) {
	t.Parallel()
	booter := &mockBooter{err: nil}
	p := NewHelixInfraProvider(booter, nil)

	err := p.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestHelixInfraProvider_Shutdown_NilBooter(t *testing.T) {
	t.Parallel()
	p := NewHelixInfraProvider(nil, nil)
	err := p.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestHelixInfraProvider_Shutdown_Error(t *testing.T) {
	t.Parallel()
	booter := &mockBooter{err: errors.New("shutdown failed")}
	p := NewHelixInfraProvider(booter, nil)

	err := p.Shutdown(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown failed")
}
