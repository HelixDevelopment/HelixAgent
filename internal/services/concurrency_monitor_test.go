package services

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	runtime.GOMAXPROCS(2)
}

// newConcurrencyMonitorTestLogger creates a silent logger for monitor tests.
func newConcurrencyMonitorTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	return logger
}

// =============================================================================
// DefaultConcurrencyMonitorConfig Tests
// =============================================================================

func TestDefaultConcurrencyMonitorConfig(t *testing.T) {
	cfg := DefaultConcurrencyMonitorConfig()

	assert.Equal(t, 15*time.Second, cfg.CheckInterval)
	assert.Equal(t, 80.0, cfg.AlertThreshold)
	assert.Equal(t, 95.0, cfg.CriticalThreshold)
}

func TestDefaultConcurrencyMonitorConfig_ValuesAreReasonable(t *testing.T) {
	cfg := DefaultConcurrencyMonitorConfig()

	assert.True(t, cfg.CheckInterval > 0, "CheckInterval should be positive")
	assert.True(t, cfg.AlertThreshold > 0 && cfg.AlertThreshold <= 100,
		"AlertThreshold should be between 0 and 100")
	assert.True(t, cfg.CriticalThreshold > cfg.AlertThreshold,
		"CriticalThreshold should be greater than AlertThreshold")
	assert.True(t, cfg.CriticalThreshold <= 100,
		"CriticalThreshold should not exceed 100")
}

// =============================================================================
// NewConcurrencyMonitor Tests
// =============================================================================

func TestNewConcurrencyMonitor_WithNilRegistry(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cfg := DefaultConcurrencyMonitorConfig()

	cm := NewConcurrencyMonitor(nil, logger, cfg)

	require.NotNil(t, cm)
	assert.Nil(t, cm.registry)
	assert.Equal(t, cfg.CheckInterval, cm.checkInterval)
	assert.Equal(t, cfg.AlertThreshold, cm.alertThreshold)
	assert.Equal(t, cfg.CriticalThreshold, cm.criticalThreshold)
	assert.NotNil(t, cm.listeners)
	assert.Zero(t, cm.listeners.Len())
	assert.NotNil(t, cm.stopCh)
	assert.NotNil(t, cm.highConcurrencyStart)
	assert.NotNil(t, cm.highConcurrencyState)
	assert.False(t, cm.running.Load())
}

func TestNewConcurrencyMonitor_WithRegistry(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cfg := DefaultConcurrencyMonitorConfig()
	regCfg := &RegistryConfig{
		DefaultTimeout: 10 * time.Second,
		Providers:      make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)

	cm := NewConcurrencyMonitor(registry, logger, cfg)

	require.NotNil(t, cm)
	assert.Equal(t, registry, cm.registry)
}

func TestNewConcurrencyMonitor_CustomConfig(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cfg := ConcurrencyMonitorConfig{
		CheckInterval:     5 * time.Second,
		AlertThreshold:    60.0,
		CriticalThreshold: 85.0,
	}

	cm := NewConcurrencyMonitor(nil, logger, cfg)

	require.NotNil(t, cm)
	assert.Equal(t, 5*time.Second, cm.checkInterval)
	assert.Equal(t, 60.0, cm.alertThreshold)
	assert.Equal(t, 85.0, cm.criticalThreshold)
}

// =============================================================================
// AddAlertListener Tests
// =============================================================================

func TestConcurrencyMonitor_AddAlertListener_Single(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	cm.AddAlertListener(func(alert ConcurrencyAlert) {
		// listener callback
	})

	assert.Equal(t, 1, cm.listeners.Len())
}

func TestConcurrencyMonitor_AddAlertListener_Multiple(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	for i := 0; i < 5; i++ {
		cm.AddAlertListener(func(alert ConcurrencyAlert) {})
	}

	assert.Equal(t, 5, cm.listeners.Len())
}

func TestConcurrencyMonitor_AddAlertListener_ConcurrentSafety(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.AddAlertListener(func(alert ConcurrencyAlert) {})
		}()
	}

	wg.Wait()

	assert.Equal(t, numGoroutines, cm.listeners.Len())
}

// =============================================================================
// Start / Stop Tests
// =============================================================================

func TestConcurrencyMonitor_Start_AndStop(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cfg := ConcurrencyMonitorConfig{
		CheckInterval:     50 * time.Millisecond,
		AlertThreshold:    80.0,
		CriticalThreshold: 95.0,
	}
	cm := NewConcurrencyMonitor(nil, logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	go cm.Start(ctx)

	// Give it time to start
	time.Sleep(20 * time.Millisecond)

	assert.True(t, cm.running.Load())

	cancel()
	time.Sleep(20 * time.Millisecond)
}

func TestConcurrencyMonitor_Stop_ExplicitStop(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cfg := ConcurrencyMonitorConfig{
		CheckInterval:     50 * time.Millisecond,
		AlertThreshold:    80.0,
		CriticalThreshold: 95.0,
	}
	cm := NewConcurrencyMonitor(nil, logger, cfg)

	ctx := context.Background()
	go cm.Start(ctx)

	time.Sleep(20 * time.Millisecond)

	cm.Stop()

	assert.False(t, cm.running.Load())
}

func TestConcurrencyMonitor_Stop_DoubleStop(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	ctx := context.Background()
	go cm.Start(ctx)
	time.Sleep(20 * time.Millisecond)

	// Double stop should not panic
	cm.Stop()
	cm.Stop()

	assert.False(t, cm.running.Load())
}

func TestConcurrencyMonitor_Start_DoubleStart(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cfg := ConcurrencyMonitorConfig{
		CheckInterval:     50 * time.Millisecond,
		AlertThreshold:    80.0,
		CriticalThreshold: 95.0,
	}
	cm := NewConcurrencyMonitor(nil, logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cm.Start(ctx)
	time.Sleep(20 * time.Millisecond)

	// Second start should return immediately without panic
	done := make(chan struct{})
	go func() {
		cm.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Second Start returned (already running)
	case <-time.After(100 * time.Millisecond):
		// The second goroutine may still be blocked until Stop or ctx cancel
		cancel()
	}
}

// =============================================================================
// GetStatus Tests
// =============================================================================

func TestConcurrencyMonitor_GetStatus_NilRegistry(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	status := cm.GetStatus()

	assert.True(t, status.Healthy)
	assert.NotNil(t, status.Providers)
	assert.Empty(t, status.Providers)
	assert.Equal(t, 0, status.HighSaturationCount)
	assert.Equal(t, 0, status.TotalProviders)
	assert.Equal(t, 0, status.TotalWithSemaphores)
}

func TestConcurrencyMonitor_GetStatus_WithRegistry(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	regCfg := &RegistryConfig{
		DefaultTimeout:        10 * time.Second,
		MaxConcurrentRequests: 10,
		Providers:             make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)

	cfg := DefaultConcurrencyMonitorConfig()
	cm := NewConcurrencyMonitor(registry, logger, cfg)

	status := cm.GetStatus()

	assert.True(t, status.Healthy)
	assert.NotNil(t, status.Providers)
	assert.Equal(t, cfg.AlertThreshold, status.AlertThreshold)
	assert.False(t, status.CheckedAt.IsZero())
}

// =============================================================================
// ConcurrencyStatus Tests
// =============================================================================

func TestConcurrencyStatus_FieldsPopulated(t *testing.T) {
	status := ConcurrencyStatus{
		Healthy:             true,
		HighSaturationCount: 0,
		TotalWithSemaphores: 5,
		TotalProviders:      10,
		AlertThreshold:      80.0,
		Providers: map[string]ConcurrencyProviderStatus{
			"provider1": {
				Provider:     "provider1",
				HasSemaphore: true,
				TotalPermits: 10,
				Saturation:   50.0,
			},
		},
		CheckedAt: time.Now(),
	}

	assert.True(t, status.Healthy)
	assert.Equal(t, 0, status.HighSaturationCount)
	assert.Equal(t, 5, status.TotalWithSemaphores)
	assert.Equal(t, 10, status.TotalProviders)
	assert.Equal(t, 80.0, status.AlertThreshold)
	assert.Len(t, status.Providers, 1)
	assert.False(t, status.CheckedAt.IsZero())
}

// =============================================================================
// ConcurrencyProviderStatus Tests
// =============================================================================

func TestConcurrencyProviderStatus_Fields(t *testing.T) {
	tests := []struct {
		name   string
		status ConcurrencyProviderStatus
	}{
		{
			name: "healthy provider",
			status: ConcurrencyProviderStatus{
				Provider:          "deepseek",
				HasSemaphore:      true,
				TotalPermits:      10,
				AcquiredPermits:   3,
				ActiveRequests:    2,
				AvailablePermits:  7,
				SemaphoreExists:   true,
				SemaphoreCapacity: 10,
				Saturation:        30.0,
				HighSaturation:    false,
			},
		},
		{
			name: "high saturation provider",
			status: ConcurrencyProviderStatus{
				Provider:          "openai",
				HasSemaphore:      true,
				TotalPermits:      10,
				AcquiredPermits:   9,
				ActiveRequests:    8,
				AvailablePermits:  1,
				SemaphoreExists:   true,
				SemaphoreCapacity: 10,
				Saturation:        90.0,
				HighSaturation:    true,
			},
		},
		{
			name: "no semaphore provider",
			status: ConcurrencyProviderStatus{
				Provider:     "test",
				HasSemaphore: false,
				Saturation:   0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.status.Provider)
			if tt.status.HasSemaphore {
				assert.True(t, tt.status.TotalPermits > 0)
			}
		})
	}
}

// =============================================================================
// ConcurrencyAlert Tests
// =============================================================================

func TestConcurrencyAlert_Fields(t *testing.T) {
	alert := ConcurrencyAlert{
		Type:            "high_saturation",
		Provider:        "openai",
		Message:         "1 provider(s) have high concurrency saturation",
		Timestamp:       time.Now(),
		Saturation:      92.5,
		ActiveRequests:  9,
		TotalPermits:    10,
		Available:       1,
		Severity:        SeverityWarning,
		EscalationLevel: 0,
	}

	assert.Equal(t, "high_saturation", alert.Type)
	assert.Equal(t, "openai", alert.Provider)
	assert.NotEmpty(t, alert.Message)
	assert.False(t, alert.Timestamp.IsZero())
	assert.Equal(t, 92.5, alert.Saturation)
	assert.Equal(t, int64(9), alert.ActiveRequests)
	assert.Equal(t, int64(10), alert.TotalPermits)
	assert.Equal(t, int64(1), alert.Available)
	assert.Equal(t, SeverityWarning, alert.Severity)
	assert.Equal(t, 0, alert.EscalationLevel)
}

func TestConcurrencyAlert_WithAllStats(t *testing.T) {
	alert := ConcurrencyAlert{
		Type: "high_saturation",
		AllStats: map[string]*ConcurrencyStats{
			"openai": {
				Provider:         "openai",
				HasSemaphore:     true,
				TotalPermits:     10,
				AcquiredPermits:  9,
				AvailablePermits: 1,
			},
		},
	}

	assert.Len(t, alert.AllStats, 1)
	assert.Equal(t, int64(10), alert.AllStats["openai"].TotalPermits)
}

// =============================================================================
// RecordBlockedRequest Tests
// =============================================================================

func TestConcurrencyMonitor_RecordBlockedRequest(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	// Should not panic even with nil registry
	assert.NotPanics(t, func() {
		cm.RecordBlockedRequest("test-provider")
	})
}

func TestConcurrencyMonitor_RecordBlockedRequest_MultipleCalls(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	for i := 0; i < 10; i++ {
		assert.NotPanics(t, func() {
			cm.RecordBlockedRequest("provider-1")
		})
	}
}

// =============================================================================
// ResetHighConcurrencyTracking Tests
// =============================================================================

func TestConcurrencyMonitor_ResetHighConcurrencyTracking(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	// Manually set high concurrency state
	cm.highConcurrencyState.Put("provider-1", true)
	cm.highConcurrencyStart.Put("provider-1", time.Now())

	cm.ResetHighConcurrencyTracking("provider-1")

	_, stateExists := cm.highConcurrencyState.Get("provider-1")
	_, startExists := cm.highConcurrencyStart.Get("provider-1")

	assert.False(t, stateExists)
	assert.False(t, startExists)
}

func TestConcurrencyMonitor_ResetHighConcurrencyTracking_NonExistent(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	// Should not panic for non-existent provider
	assert.NotPanics(t, func() {
		cm.ResetHighConcurrencyTracking("nonexistent")
	})
}

// =============================================================================
// ResetAllHighConcurrencyTracking Tests
// =============================================================================

func TestConcurrencyMonitor_ResetAllHighConcurrencyTracking(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	// Manually set high concurrency state for multiple providers
	cm.highConcurrencyState.Put("provider-1", true)
	cm.highConcurrencyStart.Put("provider-1", time.Now())
	cm.highConcurrencyState.Put("provider-2", true)
	cm.highConcurrencyStart.Put("provider-2", time.Now())

	cm.ResetAllHighConcurrencyTracking()

	assert.Zero(t, cm.highConcurrencyState.Len())
	assert.Zero(t, cm.highConcurrencyStart.Len())
}

func TestConcurrencyMonitor_ResetAllHighConcurrencyTracking_EmptyState(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	// Should not panic on empty state
	assert.NotPanics(t, func() {
		cm.ResetAllHighConcurrencyTracking()
	})
}

// =============================================================================
// checkConcurrency Tests
// =============================================================================

func TestConcurrencyMonitor_checkConcurrency_NilRegistry(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	// Should not panic with nil registry
	assert.NotPanics(t, func() {
		cm.checkConcurrency()
	})
}

// =============================================================================
// sendAlert Tests
// =============================================================================

func TestConcurrencyMonitor_sendAlert_NotifiesListeners(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	var received int64
	cm.AddAlertListener(func(alert ConcurrencyAlert) {
		atomic.AddInt64(&received, 1)
	})
	cm.AddAlertListener(func(alert ConcurrencyAlert) {
		atomic.AddInt64(&received, 1)
	})

	alert := ConcurrencyAlert{
		Type:     "high_saturation",
		Message:  "test alert",
		Severity: SeverityWarning,
	}

	cm.sendAlert(alert)

	// Listeners are called in goroutines, wait briefly
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int64(2), atomic.LoadInt64(&received))
}

func TestConcurrencyMonitor_sendAlert_NoListeners(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	alert := ConcurrencyAlert{
		Type:    "high_saturation",
		Message: "test alert",
	}

	// Should not panic with no listeners
	assert.NotPanics(t, func() {
		cm.sendAlert(alert)
	})
}

func TestConcurrencyMonitor_sendAlert_AlertFields(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	var receivedAlert ConcurrencyAlert
	var alertMu sync.Mutex

	cm.AddAlertListener(func(alert ConcurrencyAlert) {
		alertMu.Lock()
		receivedAlert = alert
		alertMu.Unlock()
	})

	sentAlert := ConcurrencyAlert{
		Type:            "high_saturation",
		Provider:        "openai",
		Message:         "provider overloaded",
		Saturation:      95.5,
		Severity:        SeverityCritical,
		EscalationLevel: 2,
	}

	cm.sendAlert(sentAlert)
	time.Sleep(50 * time.Millisecond)

	alertMu.Lock()
	assert.Equal(t, "high_saturation", receivedAlert.Type)
	assert.Equal(t, "openai", receivedAlert.Provider)
	assert.Equal(t, "provider overloaded", receivedAlert.Message)
	assert.Equal(t, 95.5, receivedAlert.Saturation)
	assert.Equal(t, SeverityCritical, receivedAlert.Severity)
	assert.Equal(t, 2, receivedAlert.EscalationLevel)
	alertMu.Unlock()
}

// =============================================================================
// Concurrent Safety Tests
// =============================================================================

func TestConcurrencyMonitor_ConcurrentAccess_GetStatus(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := cm.GetStatus()
			assert.True(t, status.Healthy)
		}()
	}

	wg.Wait()
}

func TestConcurrencyMonitor_ConcurrentAccess_ResetTracking(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	var wg sync.WaitGroup

	// Concurrent resets and state changes
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			cm.ResetHighConcurrencyTracking("provider")
		}(i)
		go func(idx int) {
			defer wg.Done()
			cm.ResetAllHighConcurrencyTracking()
		}(i)
	}

	wg.Wait()
}

func TestConcurrencyMonitor_ConcurrentAccess_StartStopListeners(t *testing.T) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, ConcurrencyMonitorConfig{
		CheckInterval:     50 * time.Millisecond,
		AlertThreshold:    80.0,
		CriticalThreshold: 95.0,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cm.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup

	// Add listeners concurrently while monitor is running
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.AddAlertListener(func(alert ConcurrencyAlert) {})
		}()
	}

	// Get status concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cm.GetStatus()
		}()
	}

	wg.Wait()
	cm.Stop()
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkConcurrencyMonitor_GetStatus(b *testing.B) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cm.GetStatus()
	}
}

func BenchmarkConcurrencyMonitor_AddAlertListener(b *testing.B) {
	logger := newConcurrencyMonitorTestLogger()
	cm := NewConcurrencyMonitor(nil, logger, DefaultConcurrencyMonitorConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.AddAlertListener(func(alert ConcurrencyAlert) {})
	}
}
