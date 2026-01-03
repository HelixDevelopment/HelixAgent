package plugins

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsCollector(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	require.NotNil(t, collector)
	assert.Equal(t, registry, collector.registry)
	assert.Equal(t, health, collector.health)
	assert.False(t, collector.running)
	assert.NotNil(t, collector.stopCh)
}

func TestMetricsCollector_StartStopCollection(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	t.Run("start collection", func(t *testing.T) {
		collector.StartCollection()
		assert.True(t, collector.running)

		// Wait a bit for goroutine to start
		time.Sleep(10 * time.Millisecond)

		collector.StopCollection()
		assert.False(t, collector.running)
	})

	t.Run("start twice does nothing", func(t *testing.T) {
		// Create new collector since stopCh is closed
		collector2 := NewMetricsCollector(registry, health)

		collector2.StartCollection()
		assert.True(t, collector2.running)

		// Start again - should be no-op
		collector2.StartCollection()
		assert.True(t, collector2.running)

		collector2.StopCollection()
	})

	t.Run("stop not running does nothing", func(t *testing.T) {
		collector3 := NewMetricsCollector(registry, health)
		assert.False(t, collector3.running)
		collector3.StopCollection()
		assert.False(t, collector3.running)
	})
}

func TestMetricsCollector_CollectMetrics(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("metrics-test-plugin")

	err := registry.Register(plugin)
	require.NoError(t, err)

	// Manually set health status
	health.mu.Lock()
	health.healthStatus["metrics-test-plugin"] = PluginHealth{
		Name:        "metrics-test-plugin",
		Status:      "healthy",
		CircuitOpen: false,
	}
	health.mu.Unlock()

	// This should not panic
	collector.collectMetrics()
}

func TestMetricsCollector_RecordRequest(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	// This should increment the metrics without panicking
	collector.RecordRequest("test-plugin", "Complete", 100*time.Millisecond)
	collector.RecordRequest("test-plugin", "CompleteStream", 500*time.Millisecond)
}

func TestMetricsCollector_RecordLoadError(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	// This should increment the load error counter without panicking
	collector.RecordLoadError("failed-plugin")
	collector.RecordLoadError("failed-plugin")
}

func TestMetricsCollector_CollectMetricsWithMixedHealth(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	// Register multiple plugins
	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("healthy-plugin")
	err := registry.Register(plugin1)
	require.NoError(t, err)

	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("unhealthy-plugin")
	err = registry.Register(plugin2)
	require.NoError(t, err)

	// Set mixed health statuses
	health.mu.Lock()
	health.healthStatus["healthy-plugin"] = PluginHealth{
		Name:        "healthy-plugin",
		Status:      "healthy",
		CircuitOpen: false,
	}
	health.healthStatus["unhealthy-plugin"] = PluginHealth{
		Name:        "unhealthy-plugin",
		Status:      "unhealthy",
		CircuitOpen: true,
	}
	health.mu.Unlock()

	// Collect metrics - should handle both healthy and unhealthy
	collector.collectMetrics()
}

func TestMetricsCollector_PeriodicCollection(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)

	// Register a plugin
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("periodic-test")
	plugin.On("HealthCheck", mock.Anything).Return(nil).Maybe()
	err := registry.Register(plugin)
	require.NoError(t, err)

	// Set health
	health.mu.Lock()
	health.healthStatus["periodic-test"] = PluginHealth{
		Name:        "periodic-test",
		Status:      "healthy",
		CircuitOpen: false,
	}
	health.mu.Unlock()

	collector := NewMetricsCollector(registry, health)
	collector.StartCollection()

	// Let it run for a bit
	time.Sleep(50 * time.Millisecond)

	collector.StopCollection()

	// Should have run without errors
	assert.False(t, collector.running)
}

func TestMetricsVariables(t *testing.T) {
	// Test that metrics variables are properly initialized
	assert.NotNil(t, PluginRequestsTotal)
	assert.NotNil(t, PluginRequestDuration)
	assert.NotNil(t, PluginHealthStatus)
	assert.NotNil(t, PluginLoadErrors)
	assert.NotNil(t, PluginActiveCount)
}

// =====================================================
// ADDITIONAL METRICS TESTS FOR COVERAGE
// =====================================================

func TestMetricsCollector_StopTwice(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	collector.StartCollection()
	time.Sleep(10 * time.Millisecond)

	// Stop twice should be safe
	collector.StopCollection()
	collector.StopCollection()

	assert.False(t, collector.running)
}

func TestMetricsCollector_RecordMultipleRequests(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	// Record multiple requests with different methods and durations
	for i := 0; i < 10; i++ {
		collector.RecordRequest("plugin-a", "Complete", time.Duration(i*100)*time.Millisecond)
		collector.RecordRequest("plugin-b", "CompleteStream", time.Duration(i*200)*time.Millisecond)
	}

	// No assertions needed - just ensure no panic
}

func TestMetricsCollector_RecordLoadError_Multiple(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	// Record multiple load errors
	for i := 0; i < 5; i++ {
		collector.RecordLoadError("plugin-" + string(rune('a'+i)))
	}
}

func TestMetricsCollector_CollectMetrics_NoPlugins(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	// Collect metrics with empty registry
	collector.collectMetrics()
}

func TestMetricsCollector_CollectMetrics_MissingHealth(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	collector := NewMetricsCollector(registry, health)

	// Register plugin but don't set health status
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("no-health-plugin")
	registry.Register(plugin)

	// Collect metrics - should handle missing health gracefully
	collector.collectMetrics()
}

func TestMetricsCollector_PeriodicCollection_ShortInterval(t *testing.T) {
	registry := NewRegistry()
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)

	// Register several plugins
	for i := 0; i < 3; i++ {
		plugin := new(MockLLMPlugin)
		name := "periodic-plugin-" + string(rune('a'+i))
		plugin.On("Name").Return(name)
		plugin.On("HealthCheck", mock.Anything).Return(nil).Maybe()
		registry.Register(plugin)

		// Set health status
		health.mu.Lock()
		health.healthStatus[name] = PluginHealth{
			Name:        name,
			Status:      "healthy",
			CircuitOpen: false,
		}
		health.mu.Unlock()
	}

	collector := NewMetricsCollector(registry, health)
	collector.StartCollection()

	// Let it run for a few collection cycles
	time.Sleep(100 * time.Millisecond)

	collector.StopCollection()
}
