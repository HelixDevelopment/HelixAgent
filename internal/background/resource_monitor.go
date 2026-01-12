package background

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/models"
)

// ProcessResourceMonitor implements ResourceMonitor using gopsutil
type ProcessResourceMonitor struct {
	repository TaskRepository
	logger     *logrus.Logger

	monitors map[string]*processMonitor
	mu       sync.RWMutex

	// Cache for system resources
	cachedResources     *SystemResources
	cacheTime           time.Time
	cacheTTL            time.Duration
	systemResourceMu    sync.RWMutex
}

type processMonitor struct {
	taskID        string
	pid           int
	interval      time.Duration
	stopChan      chan struct{}
	lastSnapshot  *models.ResourceSnapshot
	snapshotMu    sync.RWMutex
}

// NewProcessResourceMonitor creates a new process resource monitor
func NewProcessResourceMonitor(repository TaskRepository, logger *logrus.Logger) *ProcessResourceMonitor {
	return &ProcessResourceMonitor{
		repository: repository,
		logger:     logger,
		monitors:   make(map[string]*processMonitor),
		cacheTTL:   2 * time.Second,
	}
}

// GetSystemResources returns current system resource usage
func (m *ProcessResourceMonitor) GetSystemResources() (*SystemResources, error) {
	// Check cache
	m.systemResourceMu.RLock()
	if m.cachedResources != nil && time.Since(m.cacheTime) < m.cacheTTL {
		resources := *m.cachedResources
		m.systemResourceMu.RUnlock()
		return &resources, nil
	}
	m.systemResourceMu.RUnlock()

	// Get CPU info
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU percent: %w", err)
	}

	cpuCores, err := cpu.Counts(true)
	if err != nil {
		cpuCores = runtime.NumCPU()
	}

	// Get memory info
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	// Get disk info (root partition)
	diskInfo, err := disk.Usage("/")
	if err != nil {
		diskInfo = &disk.UsageStat{}
	}

	// Get load average
	loadAvg, err := load.Avg()
	if err != nil {
		loadAvg = &load.AvgStat{}
	}

	cpuLoad := float64(0)
	if len(cpuPercent) > 0 {
		cpuLoad = cpuPercent[0]
	}

	resources := &SystemResources{
		TotalCPUCores:     cpuCores,
		AvailableCPUCores: float64(cpuCores) * (100 - cpuLoad) / 100,
		TotalMemoryMB:     int64(memInfo.Total / 1024 / 1024),
		AvailableMemoryMB: int64(memInfo.Available / 1024 / 1024),
		CPULoadPercent:    cpuLoad,
		MemoryUsedPercent: memInfo.UsedPercent,
		DiskUsedPercent:   diskInfo.UsedPercent,
		LoadAvg1:          loadAvg.Load1,
		LoadAvg5:          loadAvg.Load5,
		LoadAvg15:         loadAvg.Load15,
	}

	// Update cache
	m.systemResourceMu.Lock()
	m.cachedResources = resources
	m.cacheTime = time.Now()
	m.systemResourceMu.Unlock()

	return resources, nil
}

// GetProcessResources returns resource usage for a specific process
func (m *ProcessResourceMonitor) GetProcessResources(pid int) (*models.ResourceSnapshot, error) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, fmt.Errorf("process not found: %w", err)
	}

	snapshot := &models.ResourceSnapshot{
		SampledAt: time.Now(),
	}

	// CPU percent
	cpuPercent, err := proc.CPUPercent()
	if err == nil {
		snapshot.CPUPercent = cpuPercent
	}

	// CPU times
	cpuTimes, err := proc.Times()
	if err == nil && cpuTimes != nil {
		snapshot.CPUUserTime = cpuTimes.User
		snapshot.CPUSystemTime = cpuTimes.System
	}

	// Memory info
	memInfo, err := proc.MemoryInfo()
	if err == nil && memInfo != nil {
		snapshot.MemoryRSSBytes = int64(memInfo.RSS)
		snapshot.MemoryVMSBytes = int64(memInfo.VMS)
	}

	// Memory percent
	memPercent, err := proc.MemoryPercent()
	if err == nil {
		snapshot.MemoryPercent = float64(memPercent)
	}

	// I/O counters
	ioCounters, err := proc.IOCounters()
	if err == nil && ioCounters != nil {
		snapshot.IOReadBytes = int64(ioCounters.ReadBytes)
		snapshot.IOWriteBytes = int64(ioCounters.WriteBytes)
		snapshot.IOReadCount = int64(ioCounters.ReadCount)
		snapshot.IOWriteCount = int64(ioCounters.WriteCount)
	}

	// Network connections
	connections, err := proc.Connections()
	if err == nil {
		snapshot.NetConnections = len(connections)
	}

	// Get network I/O (system-wide, as process-level is harder to get)
	netStats, err := net.IOCounters(false)
	if err == nil && len(netStats) > 0 {
		snapshot.NetBytesSent = int64(netStats[0].BytesSent)
		snapshot.NetBytesRecv = int64(netStats[0].BytesRecv)
	}

	// Open files
	openFiles, err := proc.OpenFiles()
	if err == nil {
		snapshot.OpenFiles = len(openFiles)
	}

	// File descriptors
	fds, err := proc.NumFDs()
	if err == nil {
		snapshot.OpenFDs = int(fds)
	}

	// Thread count
	threads, err := proc.NumThreads()
	if err == nil {
		snapshot.ThreadCount = int(threads)
	}

	// Process status
	status, err := proc.Status()
	if err == nil && len(status) > 0 {
		snapshot.ProcessState = status[0]
	}

	return snapshot, nil
}

// StartMonitoring begins periodic monitoring of a process
func (m *ProcessResourceMonitor) StartMonitoring(taskID string, pid int, interval time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.monitors[taskID]; exists {
		return fmt.Errorf("monitoring already started for task %s", taskID)
	}

	pm := &processMonitor{
		taskID:   taskID,
		pid:      pid,
		interval: interval,
		stopChan: make(chan struct{}),
	}

	m.monitors[taskID] = pm
	go m.monitorLoop(pm)

	m.logger.WithFields(logrus.Fields{
		"task_id":  taskID,
		"pid":      pid,
		"interval": interval,
	}).Debug("Started process monitoring")

	return nil
}

// StopMonitoring stops monitoring a process
func (m *ProcessResourceMonitor) StopMonitoring(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pm, exists := m.monitors[taskID]
	if !exists {
		return nil
	}

	close(pm.stopChan)
	delete(m.monitors, taskID)

	m.logger.WithField("task_id", taskID).Debug("Stopped process monitoring")

	return nil
}

// GetLatestSnapshot returns the most recent snapshot for a task
func (m *ProcessResourceMonitor) GetLatestSnapshot(taskID string) (*models.ResourceSnapshot, error) {
	m.mu.RLock()
	pm, exists := m.monitors[taskID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no monitoring active for task %s", taskID)
	}

	pm.snapshotMu.RLock()
	defer pm.snapshotMu.RUnlock()

	if pm.lastSnapshot == nil {
		return nil, fmt.Errorf("no snapshot available for task %s", taskID)
	}

	snapshot := *pm.lastSnapshot
	return &snapshot, nil
}

// IsResourceAvailable checks if system has enough resources
func (m *ProcessResourceMonitor) IsResourceAvailable(requirements ResourceRequirements) bool {
	resources, err := m.GetSystemResources()
	if err != nil {
		return false
	}

	// Check CPU availability
	if requirements.CPUCores > 0 && int(resources.AvailableCPUCores) < requirements.CPUCores {
		return false
	}

	// Check memory availability
	if requirements.MemoryMB > 0 && resources.AvailableMemoryMB < int64(requirements.MemoryMB) {
		return false
	}

	// Check overall system load
	if resources.CPULoadPercent > 90 {
		return false
	}

	if resources.MemoryUsedPercent > 90 {
		return false
	}

	return true
}

// monitorLoop periodically samples process resources
func (m *ProcessResourceMonitor) monitorLoop(pm *processMonitor) {
	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopChan:
			return
		case <-ticker.C:
			snapshot, err := m.GetProcessResources(pm.pid)
			if err != nil {
				m.logger.WithError(err).WithField("task_id", pm.taskID).Debug("Failed to get process resources")
				continue
			}

			snapshot.TaskID = pm.taskID

			// Update last snapshot
			pm.snapshotMu.Lock()
			pm.lastSnapshot = snapshot
			pm.snapshotMu.Unlock()

			// Save to repository
			if m.repository != nil {
				if err := m.repository.SaveResourceSnapshot(nil, snapshot); err != nil {
					m.logger.WithError(err).WithField("task_id", pm.taskID).Debug("Failed to save resource snapshot")
				}
			}
		}
	}
}

// MockResourceMonitor provides a mock implementation for testing
type MockResourceMonitor struct {
	systemResources *SystemResources
	processResources map[int]*models.ResourceSnapshot
	mu              sync.RWMutex
}

// NewMockResourceMonitor creates a new mock resource monitor
func NewMockResourceMonitor() *MockResourceMonitor {
	return &MockResourceMonitor{
		systemResources: &SystemResources{
			TotalCPUCores:     8,
			AvailableCPUCores: 6,
			TotalMemoryMB:     16384,
			AvailableMemoryMB: 8192,
			CPULoadPercent:    25,
			MemoryUsedPercent: 50,
		},
		processResources: make(map[int]*models.ResourceSnapshot),
	}
}

func (m *MockResourceMonitor) GetSystemResources() (*SystemResources, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	resources := *m.systemResources
	return &resources, nil
}

func (m *MockResourceMonitor) GetProcessResources(pid int) (*models.ResourceSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if snapshot, exists := m.processResources[pid]; exists {
		return snapshot, nil
	}

	return &models.ResourceSnapshot{
		CPUPercent:     10,
		MemoryRSSBytes: 100 * 1024 * 1024,
		MemoryPercent:  5,
		SampledAt:      time.Now(),
	}, nil
}

func (m *MockResourceMonitor) StartMonitoring(taskID string, pid int, interval time.Duration) error {
	return nil
}

func (m *MockResourceMonitor) StopMonitoring(taskID string) error {
	return nil
}

func (m *MockResourceMonitor) GetLatestSnapshot(taskID string) (*models.ResourceSnapshot, error) {
	return &models.ResourceSnapshot{
		TaskID:         taskID,
		CPUPercent:     10,
		MemoryRSSBytes: 100 * 1024 * 1024,
		SampledAt:      time.Now(),
	}, nil
}

func (m *MockResourceMonitor) IsResourceAvailable(requirements ResourceRequirements) bool {
	return true
}

// SetSystemResources sets the mock system resources
func (m *MockResourceMonitor) SetSystemResources(resources *SystemResources) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systemResources = resources
}

// SetProcessResources sets mock resources for a process
func (m *MockResourceMonitor) SetProcessResources(pid int, snapshot *models.ResourceSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processResources[pid] = snapshot
}
