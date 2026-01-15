package background

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

// ============================================================================
// MockResourceMonitor Tests
// ============================================================================

func TestNewMockResourceMonitor(t *testing.T) {
	mock := NewMockResourceMonitor()
	require.NotNil(t, mock)
	assert.NotNil(t, mock.systemResources)
	assert.NotNil(t, mock.processResources)

	// Verify default values
	assert.Equal(t, 8, mock.systemResources.TotalCPUCores)
	assert.Equal(t, float64(6), mock.systemResources.AvailableCPUCores)
	assert.Equal(t, int64(16384), mock.systemResources.TotalMemoryMB)
	assert.Equal(t, int64(8192), mock.systemResources.AvailableMemoryMB)
}

func TestMockResourceMonitor_GetSystemResources(t *testing.T) {
	mock := NewMockResourceMonitor()

	resources, err := mock.GetSystemResources()
	require.NoError(t, err)
	require.NotNil(t, resources)

	assert.Equal(t, 8, resources.TotalCPUCores)
	assert.Equal(t, float64(6), resources.AvailableCPUCores)
	assert.Equal(t, int64(16384), resources.TotalMemoryMB)
	assert.Equal(t, int64(8192), resources.AvailableMemoryMB)
	assert.Equal(t, float64(25), resources.CPULoadPercent)
	assert.Equal(t, float64(50), resources.MemoryUsedPercent)
}

func TestMockResourceMonitor_GetProcessResources_Default(t *testing.T) {
	mock := NewMockResourceMonitor()

	// Get resources for a PID that wasn't set explicitly
	snapshot, err := mock.GetProcessResources(12345)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	assert.Equal(t, float64(10), snapshot.CPUPercent)
	assert.Equal(t, int64(100*1024*1024), snapshot.MemoryRSSBytes)
	assert.Equal(t, float64(5), snapshot.MemoryPercent)
	assert.False(t, snapshot.SampledAt.IsZero())
}

func TestMockResourceMonitor_GetProcessResources_Custom(t *testing.T) {
	mock := NewMockResourceMonitor()

	// Set custom resources for a specific PID
	customSnapshot := &models.ResourceSnapshot{
		CPUPercent:     75.5,
		MemoryRSSBytes: 500 * 1024 * 1024,
		MemoryPercent:  30.0,
		OpenFDs:        150,
		ThreadCount:    20,
	}
	mock.SetProcessResources(9999, customSnapshot)

	// Get resources for the custom PID
	snapshot, err := mock.GetProcessResources(9999)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	assert.Equal(t, float64(75.5), snapshot.CPUPercent)
	assert.Equal(t, int64(500*1024*1024), snapshot.MemoryRSSBytes)
	assert.Equal(t, float64(30.0), snapshot.MemoryPercent)
	assert.Equal(t, 150, snapshot.OpenFDs)
	assert.Equal(t, 20, snapshot.ThreadCount)
}

func TestMockResourceMonitor_StartMonitoring(t *testing.T) {
	mock := NewMockResourceMonitor()

	err := mock.StartMonitoring("task-1", 12345, time.Second)
	assert.NoError(t, err)
}

func TestMockResourceMonitor_StopMonitoring(t *testing.T) {
	mock := NewMockResourceMonitor()

	err := mock.StopMonitoring("task-1")
	assert.NoError(t, err)
}

func TestMockResourceMonitor_GetLatestSnapshot(t *testing.T) {
	mock := NewMockResourceMonitor()

	snapshot, err := mock.GetLatestSnapshot("task-abc")
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	assert.Equal(t, "task-abc", snapshot.TaskID)
	assert.Equal(t, float64(10), snapshot.CPUPercent)
	assert.Equal(t, int64(100*1024*1024), snapshot.MemoryRSSBytes)
	assert.False(t, snapshot.SampledAt.IsZero())
}

func TestMockResourceMonitor_IsResourceAvailable(t *testing.T) {
	mock := NewMockResourceMonitor()

	// Always returns true for mock
	assert.True(t, mock.IsResourceAvailable(ResourceRequirements{
		CPUCores: 4,
		MemoryMB: 1024,
	}))

	assert.True(t, mock.IsResourceAvailable(ResourceRequirements{
		CPUCores: 100,
		MemoryMB: 999999,
	}))
}

func TestMockResourceMonitor_SetSystemResources(t *testing.T) {
	mock := NewMockResourceMonitor()

	newResources := &SystemResources{
		TotalCPUCores:     16,
		AvailableCPUCores: 12,
		TotalMemoryMB:     32768,
		AvailableMemoryMB: 24000,
		CPULoadPercent:    15.5,
		MemoryUsedPercent: 26.7,
	}
	mock.SetSystemResources(newResources)

	resources, err := mock.GetSystemResources()
	require.NoError(t, err)

	assert.Equal(t, 16, resources.TotalCPUCores)
	assert.Equal(t, float64(12), resources.AvailableCPUCores)
	assert.Equal(t, int64(32768), resources.TotalMemoryMB)
	assert.Equal(t, float64(15.5), resources.CPULoadPercent)
}

func TestMockResourceMonitor_SetProcessResources(t *testing.T) {
	mock := NewMockResourceMonitor()

	// Set resources for multiple PIDs
	mock.SetProcessResources(100, &models.ResourceSnapshot{CPUPercent: 10})
	mock.SetProcessResources(200, &models.ResourceSnapshot{CPUPercent: 20})
	mock.SetProcessResources(300, &models.ResourceSnapshot{CPUPercent: 30})

	// Verify each
	snap1, _ := mock.GetProcessResources(100)
	snap2, _ := mock.GetProcessResources(200)
	snap3, _ := mock.GetProcessResources(300)

	assert.Equal(t, float64(10), snap1.CPUPercent)
	assert.Equal(t, float64(20), snap2.CPUPercent)
	assert.Equal(t, float64(30), snap3.CPUPercent)
}

func TestMockResourceMonitor_ConcurrentAccess(t *testing.T) {
	mock := NewMockResourceMonitor()

	done := make(chan bool, 20)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = mock.GetSystemResources()
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(idx int) {
			mock.SetSystemResources(&SystemResources{TotalCPUCores: idx})
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

// ============================================================================
// ResourceMonitor Interface Tests
// ============================================================================

func TestResourceMonitorInterface_MockImplementsInterface(t *testing.T) {
	var _ ResourceMonitor = (*MockResourceMonitor)(nil)
}

// ============================================================================
// SystemResources Tests
// ============================================================================

func TestSystemResources_Fields(t *testing.T) {
	res := SystemResources{
		TotalCPUCores:     8,
		AvailableCPUCores: 6.5,
		TotalMemoryMB:     16384,
		AvailableMemoryMB: 12000,
		CPULoadPercent:    18.75,
		MemoryUsedPercent: 26.76,
		DiskUsedPercent:   45.0,
		LoadAvg1:          1.5,
		LoadAvg5:          2.0,
		LoadAvg15:         1.8,
	}

	assert.Equal(t, 8, res.TotalCPUCores)
	assert.Equal(t, 6.5, res.AvailableCPUCores)
	assert.Equal(t, int64(16384), res.TotalMemoryMB)
	assert.Equal(t, int64(12000), res.AvailableMemoryMB)
	assert.Equal(t, 18.75, res.CPULoadPercent)
	assert.Equal(t, 26.76, res.MemoryUsedPercent)
	assert.Equal(t, 45.0, res.DiskUsedPercent)
	assert.Equal(t, 1.5, res.LoadAvg1)
	assert.Equal(t, 2.0, res.LoadAvg5)
	assert.Equal(t, 1.8, res.LoadAvg15)
}

func TestSystemResources_ZeroValue(t *testing.T) {
	var res SystemResources

	assert.Equal(t, 0, res.TotalCPUCores)
	assert.Equal(t, float64(0), res.AvailableCPUCores)
	assert.Equal(t, int64(0), res.TotalMemoryMB)
	assert.Equal(t, float64(0), res.CPULoadPercent)
}
