package background

import (
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/models"
)

// MockResourceMonitor provides a mock implementation for testing.
// This should only be used in tests, not in production code.
//
// Concurrency model (CONST-029): systemResources → atomic.Pointer
// (whole struct swap); processResources → *safe.Store.
type MockResourceMonitor struct {
	systemResources  atomic.Pointer[SystemResources]
	processResources *safe.Store[int, *models.ResourceSnapshot]
}

// NewMockResourceMonitor creates a new mock resource monitor with default values
func NewMockResourceMonitor() *MockResourceMonitor {
	m := &MockResourceMonitor{
		processResources: safe.NewStore[int, *models.ResourceSnapshot](),
	}
	m.systemResources.Store(&SystemResources{
		TotalCPUCores:     8,
		AvailableCPUCores: 6,
		TotalMemoryMB:     16384,
		AvailableMemoryMB: 8192,
		CPULoadPercent:    25,
		MemoryUsedPercent: 50,
	})
	return m
}

// GetSystemResources returns the mock system resources
func (m *MockResourceMonitor) GetSystemResources() (*SystemResources, error) {
	cur := m.systemResources.Load()
	if cur == nil {
		return &SystemResources{}, nil
	}
	resources := *cur
	return &resources, nil
}

// GetProcessResources returns mock resources for a process
func (m *MockResourceMonitor) GetProcessResources(pid int) (*models.ResourceSnapshot, error) {
	if snapshot, exists := m.processResources.Get(pid); exists {
		return snapshot, nil
	}

	return &models.ResourceSnapshot{
		CPUPercent:     10,
		MemoryRSSBytes: 100 * 1024 * 1024,
		MemoryPercent:  5,
		SampledAt:      time.Now(),
	}, nil
}

// StartMonitoring is a no-op for mock
func (m *MockResourceMonitor) StartMonitoring(taskID string, pid int, interval time.Duration) error {
	return nil
}

// StopMonitoring is a no-op for mock
func (m *MockResourceMonitor) StopMonitoring(taskID string) error {
	return nil
}

// GetLatestSnapshot returns a mock snapshot for the task
func (m *MockResourceMonitor) GetLatestSnapshot(taskID string) (*models.ResourceSnapshot, error) {
	return &models.ResourceSnapshot{
		TaskID:         taskID,
		CPUPercent:     10,
		MemoryRSSBytes: 100 * 1024 * 1024,
		SampledAt:      time.Now(),
	}, nil
}

// IsResourceAvailable always returns true for mock
func (m *MockResourceMonitor) IsResourceAvailable(requirements ResourceRequirements) bool {
	return true
}

// SetSystemResources sets the mock system resources for testing
func (m *MockResourceMonitor) SetSystemResources(resources *SystemResources) {
	m.systemResources.Store(resources)
}

// SetProcessResources sets mock resources for a process for testing
func (m *MockResourceMonitor) SetProcessResources(pid int, snapshot *models.ResourceSnapshot) {
	m.processResources.Put(pid, snapshot)
}

// SetResourceAvailable allows configuring whether resources are available (for testing edge cases)
func (m *MockResourceMonitor) SetResourceAvailable(available bool) {
	// This is a placeholder for more sophisticated mock behavior
	// The current implementation always returns true from IsResourceAvailable
}
