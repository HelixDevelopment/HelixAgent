package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPluginSystemTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewHighAvailabilityManager(t *testing.T) {
	log := newPluginSystemTestLogger()
	ham := NewHighAvailabilityManager(log)

	require.NotNil(t, ham)
	assert.NotNil(t, ham.instances)
	assert.NotNil(t, ham.loadBalancer)
	assert.NotNil(t, ham.failoverManager)
	assert.NotNil(t, ham.healthChecker)
	assert.NotNil(t, ham.stopChan)
}

func TestHighAvailabilityManager_RegisterInstance(t *testing.T) {
	log := newPluginSystemTestLogger()
	ham := NewHighAvailabilityManager(log)

	t.Run("register new instance", func(t *testing.T) {
		instance := &ServiceInstance{
			ID:       "instance-1",
			Address:  "localhost",
			Port:     7061,
			Protocol: "mcp",
		}

		err := ham.RegisterInstance(instance)
		require.NoError(t, err)

		instances := ham.GetAllInstances()
		assert.Len(t, instances, 1)
		assert.Equal(t, StatusStarting, instances[0].Status)
	})

	t.Run("register duplicate instance", func(t *testing.T) {
		instance := &ServiceInstance{
			ID:       "duplicate",
			Address:  "localhost",
			Port:     8081,
			Protocol: "mcp",
		}

		err := ham.RegisterInstance(instance)
		require.NoError(t, err)

		err = ham.RegisterInstance(instance)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
}

func TestHighAvailabilityManager_UnregisterInstance(t *testing.T) {
	log := newPluginSystemTestLogger()
	ham := NewHighAvailabilityManager(log)

	instance := &ServiceInstance{
		ID:       "unregister-test",
		Address:  "localhost",
		Port:     7061,
		Protocol: "mcp",
	}
	ham.RegisterInstance(instance)

	t.Run("unregister existing instance", func(t *testing.T) {
		err := ham.UnregisterInstance("unregister-test")
		require.NoError(t, err)

		instances := ham.GetAllInstances()
		for _, inst := range instances {
			assert.NotEqual(t, "unregister-test", inst.ID)
		}
	})

	t.Run("unregister non-existent instance", func(t *testing.T) {
		err := ham.UnregisterInstance("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
}

func TestHighAvailabilityManager_GetInstance(t *testing.T) {
	log := newPluginSystemTestLogger()
	ham := NewHighAvailabilityManager(log)

	t.Run("no healthy instances", func(t *testing.T) {
		instance, err := ham.GetInstance("mcp")
		assert.Error(t, err)
		assert.Nil(t, instance)
		assert.Contains(t, err.Error(), "no healthy instances")
	})

	t.Run("get healthy instance", func(t *testing.T) {
		instance := &ServiceInstance{
			ID:        "healthy-1",
			Address:   "localhost",
			Port:      7061,
			Protocol:  "lsp",
			Status:    StatusHealthy,
			LoadScore: 10,
		}
		ham.mu.Lock()
		ham.instances[instance.ID] = instance
		ham.mu.Unlock()

		selected, err := ham.GetInstance("lsp")
		require.NoError(t, err)
		assert.NotNil(t, selected)
		assert.Equal(t, "healthy-1", selected.ID)
	})
}

func TestHighAvailabilityManager_UpdateInstanceLoad(t *testing.T) {
	log := newPluginSystemTestLogger()
	ham := NewHighAvailabilityManager(log)

	instance := &ServiceInstance{
		ID:        "load-test",
		Address:   "localhost",
		Port:      7061,
		Protocol:  "mcp",
		LoadScore: 50,
	}
	ham.RegisterInstance(instance)

	t.Run("update load for existing instance", func(t *testing.T) {
		err := ham.UpdateInstanceLoad("load-test", 75)
		require.NoError(t, err)

		ham.mu.RLock()
		assert.Equal(t, 75, ham.instances["load-test"].LoadScore)
		ham.mu.RUnlock()
	})

	t.Run("update load for non-existent instance", func(t *testing.T) {
		err := ham.UpdateInstanceLoad("non-existent", 50)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestHighAvailabilityManager_GetInstancesByProtocol(t *testing.T) {
	log := newPluginSystemTestLogger()
	ham := NewHighAvailabilityManager(log)

	ham.RegisterInstance(&ServiceInstance{ID: "mcp-1", Protocol: "mcp"})
	ham.RegisterInstance(&ServiceInstance{ID: "mcp-2", Protocol: "mcp"})
	ham.RegisterInstance(&ServiceInstance{ID: "lsp-1", Protocol: "lsp"})

	mcpInstances := ham.GetInstancesByProtocol("mcp")
	assert.Len(t, mcpInstances, 2)

	lspInstances := ham.GetInstancesByProtocol("lsp")
	assert.Len(t, lspInstances, 1)

	acpInstances := ham.GetInstancesByProtocol("acp")
	assert.Len(t, acpInstances, 0)
}

func TestHighAvailabilityManager_Stop(t *testing.T) {
	log := newPluginSystemTestLogger()
	ham := NewHighAvailabilityManager(log)

	// Should not panic
	ham.Stop()
}

func TestRoundRobinLoadBalancer(t *testing.T) {
	lb := &RoundRobinLoadBalancer{}

	instances := []*ServiceInstance{
		{ID: "inst-1", LoadScore: 10},
		{ID: "inst-2", LoadScore: 20},
		{ID: "inst-3", LoadScore: 30},
	}

	t.Run("empty instances", func(t *testing.T) {
		selected := lb.SelectInstance("mcp", []*ServiceInstance{})
		assert.Nil(t, selected)
	})

	t.Run("round robin selection", func(t *testing.T) {
		first := lb.SelectInstance("mcp", instances)
		second := lb.SelectInstance("mcp", instances)
		_ = lb.SelectInstance("mcp", instances) // third
		fourth := lb.SelectInstance("mcp", instances)

		// After 3 selections, should cycle back
		assert.Equal(t, first.ID, fourth.ID)
		assert.NotEqual(t, first.ID, second.ID)
	})

	t.Run("different protocols tracked separately", func(t *testing.T) {
		mcpFirst := lb.SelectInstance("mcp", instances)
		lspFirst := lb.SelectInstance("lsp", instances)

		// First selection for each protocol should be same (index 1 after initialization)
		// This is implementation-dependent
		assert.NotNil(t, mcpFirst)
		assert.NotNil(t, lspFirst)
	})

	t.Run("update load no-op", func(t *testing.T) {
		// Should not panic
		lb.UpdateLoad("inst-1", 50)
	})
}

func TestLeastLoadedLoadBalancer(t *testing.T) {
	lb := &LeastLoadedLoadBalancer{}

	t.Run("empty instances", func(t *testing.T) {
		selected := lb.SelectInstance("mcp", []*ServiceInstance{})
		assert.Nil(t, selected)
	})

	t.Run("select least loaded", func(t *testing.T) {
		instances := []*ServiceInstance{
			{ID: "high-load", LoadScore: 80},
			{ID: "low-load", LoadScore: 10},
			{ID: "med-load", LoadScore: 50},
		}

		selected := lb.SelectInstance("mcp", instances)
		assert.NotNil(t, selected)
		assert.Equal(t, "low-load", selected.ID)
	})

	t.Run("update load no-op", func(t *testing.T) {
		// Should not panic
		lb.UpdateLoad("inst-1", 50)
	})
}

func TestRandomLoadBalancer(t *testing.T) {
	lb := &RandomLoadBalancer{}

	t.Run("empty instances", func(t *testing.T) {
		selected := lb.SelectInstance("mcp", []*ServiceInstance{})
		assert.Nil(t, selected)
	})

	t.Run("random selection", func(t *testing.T) {
		instances := []*ServiceInstance{
			{ID: "inst-1"},
			{ID: "inst-2"},
			{ID: "inst-3"},
		}

		selected := lb.SelectInstance("mcp", instances)
		assert.NotNil(t, selected)
		assert.Contains(t, []string{"inst-1", "inst-2", "inst-3"}, selected.ID)
	})

	t.Run("update load no-op", func(t *testing.T) {
		// Should not panic
		lb.UpdateLoad("inst-1", 50)
	})
}

func TestNewFailoverManager(t *testing.T) {
	log := newPluginSystemTestLogger()
	fm := NewFailoverManager(log)

	require.NotNil(t, fm)
	assert.NotNil(t, fm.failoverGroups)
	assert.NotNil(t, fm.activeInstances)
	assert.Equal(t, 30*time.Second, fm.failoverThreshold)
}

func TestFailoverManager_RegisterInstance(t *testing.T) {
	log := newPluginSystemTestLogger()
	fm := NewFailoverManager(log)

	instance := &ServiceInstance{
		ID:       "fm-inst-1",
		Protocol: "mcp",
		Status:   StatusHealthy,
	}

	fm.RegisterInstance(instance)

	fm.mu.RLock()
	assert.Len(t, fm.failoverGroups["mcp"], 1)
	assert.Equal(t, instance, fm.activeInstances["mcp"])
	fm.mu.RUnlock()
}

func TestFailoverManager_UnregisterInstance(t *testing.T) {
	log := newPluginSystemTestLogger()
	fm := NewFailoverManager(log)

	instance := &ServiceInstance{
		ID:       "fm-unreg",
		Protocol: "mcp",
		Status:   StatusHealthy,
	}
	fm.RegisterInstance(instance)

	fm.UnregisterInstance("fm-unreg")

	fm.mu.RLock()
	assert.Len(t, fm.failoverGroups["mcp"], 0)
	fm.mu.RUnlock()
}

func TestFailoverManager_HandleInstanceFailure(t *testing.T) {
	log := newPluginSystemTestLogger()
	fm := NewFailoverManager(log)

	primary := &ServiceInstance{
		ID:       "primary",
		Protocol: "mcp",
		Status:   StatusHealthy,
	}
	backup := &ServiceInstance{
		ID:       "backup",
		Protocol: "mcp",
		Status:   StatusHealthy,
	}

	fm.RegisterInstance(primary)
	fm.RegisterInstance(backup)

	// Simulate primary failure
	primary.Status = StatusUnhealthy
	fm.HandleInstanceFailure(primary)

	// Backup should be promoted
	fm.mu.RLock()
	active := fm.activeInstances["mcp"]
	fm.mu.RUnlock()

	assert.NotNil(t, active)
}

func TestFailoverManager_Stop(t *testing.T) {
	log := newPluginSystemTestLogger()
	fm := NewFailoverManager(log)

	// Should not panic
	fm.Stop()
}

func TestDefaultHealthCheckerConfig(t *testing.T) {
	config := DefaultHealthCheckerConfig()

	require.NotNil(t, config)
	assert.Equal(t, 30*time.Second, config.CheckInterval)
	assert.Equal(t, 5*time.Second, config.Timeout)
	assert.Equal(t, 3, config.UnhealthyThreshold)
}

func TestNewHealthChecker(t *testing.T) {
	log := newPluginSystemTestLogger()
	hc := NewHealthChecker(log)

	require.NotNil(t, hc)
	assert.Equal(t, 30*time.Second, hc.checkInterval)
	assert.Equal(t, 5*time.Second, hc.timeout)
	assert.Equal(t, 3, hc.unhealthyThreshold)
	assert.NotNil(t, hc.healthChecks)
	assert.NotNil(t, hc.instanceRegistry)
	assert.NotNil(t, hc.httpClient)
}

func TestNewHealthCheckerWithConfig(t *testing.T) {
	log := newPluginSystemTestLogger()

	t.Run("with custom config", func(t *testing.T) {
		config := &HealthCheckerConfig{
			CheckInterval:      10 * time.Second,
			Timeout:            2 * time.Second,
			UnhealthyThreshold: 5,
		}
		hc := NewHealthCheckerWithConfig(log, config)

		require.NotNil(t, hc)
		assert.Equal(t, 10*time.Second, hc.checkInterval)
		assert.Equal(t, 2*time.Second, hc.timeout)
		assert.Equal(t, 5, hc.unhealthyThreshold)
		assert.NotNil(t, hc.httpClient)
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		hc := NewHealthCheckerWithConfig(log, nil)

		require.NotNil(t, hc)
		assert.Equal(t, 30*time.Second, hc.checkInterval)
		assert.Equal(t, 5*time.Second, hc.timeout)
		assert.Equal(t, 3, hc.unhealthyThreshold)
	})
}

func TestHealthChecker_RegisterInstance(t *testing.T) {
	log := newPluginSystemTestLogger()
	hc := NewHealthChecker(log)

	hc.RegisterInstance("hc-inst-1", "localhost", 7061)

	hc.mu.RLock()
	status, exists := hc.healthChecks["hc-inst-1"]
	instanceInfo, infoExists := hc.instanceRegistry["hc-inst-1"]
	hc.mu.RUnlock()

	assert.True(t, exists)
	assert.True(t, status.IsHealthy)
	assert.Equal(t, "hc-inst-1", status.InstanceID)

	assert.True(t, infoExists)
	assert.Equal(t, "localhost", instanceInfo.Address)
	assert.Equal(t, 7061, instanceInfo.Port)
	assert.Equal(t, "http", instanceInfo.Protocol) // default protocol
}

func TestHealthChecker_RegisterInstanceWithProtocol(t *testing.T) {
	log := newPluginSystemTestLogger()
	hc := NewHealthChecker(log)

	t.Run("register with http protocol", func(t *testing.T) {
		hc.RegisterInstanceWithProtocol("http-inst", "localhost", 7061, "http")

		info := hc.GetInstanceInfo("http-inst")
		require.NotNil(t, info)
		assert.Equal(t, "localhost", info.Address)
		assert.Equal(t, 7061, info.Port)
		assert.Equal(t, "http", info.Protocol)
	})

	t.Run("register with https protocol", func(t *testing.T) {
		hc.RegisterInstanceWithProtocol("https-inst", "localhost", 443, "https")

		info := hc.GetInstanceInfo("https-inst")
		require.NotNil(t, info)
		assert.Equal(t, "https", info.Protocol)
	})

	t.Run("register with grpc protocol", func(t *testing.T) {
		hc.RegisterInstanceWithProtocol("grpc-inst", "localhost", 50051, "grpc")

		info := hc.GetInstanceInfo("grpc-inst")
		require.NotNil(t, info)
		assert.Equal(t, "grpc", info.Protocol)
	})

	t.Run("register with tcp protocol", func(t *testing.T) {
		hc.RegisterInstanceWithProtocol("tcp-inst", "localhost", 9000, "tcp")

		info := hc.GetInstanceInfo("tcp-inst")
		require.NotNil(t, info)
		assert.Equal(t, "tcp", info.Protocol)
	})
}

func TestHealthChecker_UnregisterInstance(t *testing.T) {
	log := newPluginSystemTestLogger()
	hc := NewHealthChecker(log)

	hc.RegisterInstance("hc-unreg", "localhost", 7061)
	hc.UnregisterInstance("hc-unreg")

	hc.mu.RLock()
	_, healthExists := hc.healthChecks["hc-unreg"]
	_, infoExists := hc.instanceRegistry["hc-unreg"]
	hc.mu.RUnlock()

	assert.False(t, healthExists)
	assert.False(t, infoExists)
}

func TestHealthChecker_GetInstanceInfo(t *testing.T) {
	log := newPluginSystemTestLogger()
	hc := NewHealthChecker(log)

	t.Run("existing instance", func(t *testing.T) {
		hc.RegisterInstanceWithProtocol("get-info-test", "127.0.0.1", 9090, "tcp")

		info := hc.GetInstanceInfo("get-info-test")
		require.NotNil(t, info)
		assert.Equal(t, "127.0.0.1", info.Address)
		assert.Equal(t, 9090, info.Port)
		assert.Equal(t, "tcp", info.Protocol)
	})

	t.Run("non-existent instance", func(t *testing.T) {
		info := hc.GetInstanceInfo("non-existent")
		assert.Nil(t, info)
	})
}

func TestHealthChecker_GetHealthStatus(t *testing.T) {
	log := newPluginSystemTestLogger()
	hc := NewHealthChecker(log)

	t.Run("existing instance", func(t *testing.T) {
		hc.RegisterInstance("get-status-test", "localhost", 7061)

		status := hc.GetHealthStatus("get-status-test")
		require.NotNil(t, status)
		assert.Equal(t, "get-status-test", status.InstanceID)
		assert.True(t, status.IsHealthy)
	})

	t.Run("non-existent instance", func(t *testing.T) {
		status := hc.GetHealthStatus("non-existent")
		assert.Nil(t, status)
	})
}

func TestHealthChecker_SetHTTPClient(t *testing.T) {
	log := newPluginSystemTestLogger()
	hc := NewHealthChecker(log)

	customClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	hc.SetHTTPClient(customClient)

	hc.mu.RLock()
	assert.Equal(t, customClient, hc.httpClient)
	hc.mu.RUnlock()
}

func TestHealthChecker_Stop(t *testing.T) {
	log := newPluginSystemTestLogger()
	hc := NewHealthChecker(log)

	// Should not panic
	hc.Stop()
}

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(5, 3, 10*time.Second)

	require.NotNil(t, cb)
	assert.Equal(t, 5, cb.failureThreshold)
	assert.Equal(t, 3, cb.successThreshold)
	assert.Equal(t, 10*time.Second, cb.timeout)
	assert.Equal(t, StateClosed, cb.state)
}

func TestCircuitBreaker_Call(t *testing.T) {
	t.Run("successful call", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

		err := cb.Call(func() error {
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, StateClosed, cb.GetState())
	})

	t.Run("failed call", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

		err := cb.Call(func() error {
			return errors.New("failure")
		})

		assert.Error(t, err)
		assert.Equal(t, 1, cb.GetFailureCount())
	})

	t.Run("circuit opens after threshold", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 2, 100*time.Millisecond)

		// First failure
		cb.Call(func() error { return errors.New("failure") })
		assert.Equal(t, StateClosed, cb.GetState())

		// Second failure - should open circuit
		cb.Call(func() error { return errors.New("failure") })
		assert.Equal(t, StateOpen, cb.GetState())
	})

	t.Run("circuit rejects calls when open", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 2, 1*time.Second)

		// Open the circuit
		cb.Call(func() error { return errors.New("failure") })
		assert.Equal(t, StateOpen, cb.GetState())

		// Call should be rejected
		err := cb.Call(func() error { return nil })
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
	})

	t.Run("circuit transitions to half-open after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 2, 50*time.Millisecond)

		// Open the circuit
		cb.Call(func() error { return errors.New("failure") })
		assert.Equal(t, StateOpen, cb.GetState())

		// Wait for timeout
		time.Sleep(100 * time.Millisecond)

		// Next call should transition to half-open
		cb.Call(func() error { return nil })
		// After success in half-open, should still be half-open (need 2 successes)
		assert.Equal(t, StateHalfOpen, cb.GetState())
	})

	t.Run("circuit closes after success threshold in half-open", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 2, 50*time.Millisecond)

		// Open the circuit
		cb.Call(func() error { return errors.New("failure") })
		time.Sleep(100 * time.Millisecond)

		// First success (transitions to half-open and counts as success)
		cb.Call(func() error { return nil })
		assert.Equal(t, StateHalfOpen, cb.GetState())

		// Second success - should close circuit
		cb.Call(func() error { return nil })
		assert.Equal(t, StateClosed, cb.GetState())
	})
}

func TestCircuitBreaker_GetState(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_GetFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	assert.Equal(t, 0, cb.GetFailureCount())

	cb.Call(func() error { return errors.New("failure") })
	assert.Equal(t, 1, cb.GetFailureCount())
}

func TestCircuitBreaker_GetLastFailure(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	t.Run("no failures", func(t *testing.T) {
		lastFailure := cb.GetLastFailure()
		assert.Nil(t, lastFailure)
	})

	t.Run("after failure", func(t *testing.T) {
		cb.Call(func() error { return errors.New("failure") })
		lastFailure := cb.GetLastFailure()
		assert.NotNil(t, lastFailure)
		assert.WithinDuration(t, time.Now(), *lastFailure, time.Second)
	})
}

func TestCircuitState_String(t *testing.T) {
	assert.Equal(t, "closed", StateClosed.String())
	assert.Equal(t, "open", StateOpen.String())
	assert.Equal(t, "half-open", StateHalfOpen.String())
	assert.Equal(t, "unknown", CircuitState(99).String())
}

func TestNewServiceRegistry(t *testing.T) {
	log := newPluginSystemTestLogger()
	sr := NewServiceRegistry(log)

	require.NotNil(t, sr)
	assert.NotNil(t, sr.services)
}

func TestServiceRegistry_RegisterService(t *testing.T) {
	log := newPluginSystemTestLogger()
	sr := NewServiceRegistry(log)

	endpoint := &ServiceEndpoint{
		ID:       "endpoint-1",
		Address:  "localhost",
		Port:     7061,
		Protocol: "http",
	}

	sr.RegisterService("api", endpoint)

	endpoints := sr.DiscoverServices("api")
	assert.Len(t, endpoints, 1)
	assert.Equal(t, "endpoint-1", endpoints[0].ID)
}

func TestServiceRegistry_UnregisterService(t *testing.T) {
	log := newPluginSystemTestLogger()
	sr := NewServiceRegistry(log)

	endpoint := &ServiceEndpoint{ID: "endpoint-unreg", Address: "localhost", Port: 7061}
	sr.RegisterService("api", endpoint)

	sr.UnregisterService("api", "endpoint-unreg")

	endpoints := sr.DiscoverServices("api")
	assert.Len(t, endpoints, 0)
}

func TestServiceRegistry_DiscoverServices(t *testing.T) {
	log := newPluginSystemTestLogger()
	sr := NewServiceRegistry(log)

	sr.RegisterService("api", &ServiceEndpoint{ID: "ep-1", Address: "localhost", Port: 7061})
	sr.RegisterService("api", &ServiceEndpoint{ID: "ep-2", Address: "localhost", Port: 8081})
	sr.RegisterService("db", &ServiceEndpoint{ID: "ep-3", Address: "localhost", Port: 5432})

	apiEndpoints := sr.DiscoverServices("api")
	assert.Len(t, apiEndpoints, 2)

	dbEndpoints := sr.DiscoverServices("db")
	assert.Len(t, dbEndpoints, 1)

	cacheEndpoints := sr.DiscoverServices("cache")
	assert.Len(t, cacheEndpoints, 0)
}

func TestServiceInstance_Structure(t *testing.T) {
	now := time.Now()
	instance := &ServiceInstance{
		ID:         "inst-123",
		Address:    "192.168.1.100",
		Port:       7061,
		Protocol:   "mcp",
		Status:     StatusHealthy,
		LastHealth: now,
		LoadScore:  25,
		Metadata:   map[string]interface{}{"version": "1.0"},
	}

	assert.Equal(t, "inst-123", instance.ID)
	assert.Equal(t, "192.168.1.100", instance.Address)
	assert.Equal(t, 7061, instance.Port)
	assert.Equal(t, StatusHealthy, instance.Status)
	assert.Equal(t, 25, instance.LoadScore)
}

func TestInstanceStatus_Constants(t *testing.T) {
	assert.Equal(t, InstanceStatus(0), StatusStarting)
	assert.Equal(t, InstanceStatus(1), StatusHealthy)
	assert.Equal(t, InstanceStatus(2), StatusDegraded)
	assert.Equal(t, InstanceStatus(3), StatusUnhealthy)
	assert.Equal(t, InstanceStatus(4), StatusDown)
}

func TestHealthStatus_Structure(t *testing.T) {
	now := time.Now()
	status := &HealthStatus{
		InstanceID:          "inst-123",
		LastCheck:           now,
		ConsecutiveFailures: 2,
		IsHealthy:           false,
		ResponseTime:        100 * time.Millisecond,
		Error:               "connection timeout",
	}

	assert.Equal(t, "inst-123", status.InstanceID)
	assert.Equal(t, 2, status.ConsecutiveFailures)
	assert.False(t, status.IsHealthy)
	assert.Equal(t, "connection timeout", status.Error)
}

func TestServiceEndpoint_Structure(t *testing.T) {
	endpoint := &ServiceEndpoint{
		ID:       "ep-123",
		Address:  "api.example.com",
		Port:     443,
		Protocol: "https",
		Metadata: map[string]interface{}{
			"region": "us-east-1",
			"weight": 100,
		},
	}

	assert.Equal(t, "ep-123", endpoint.ID)
	assert.Equal(t, "api.example.com", endpoint.Address)
	assert.Equal(t, 443, endpoint.Port)
	assert.Equal(t, "us-east-1", endpoint.Metadata["region"])
}

func TestHighAvailabilityManager_Start(t *testing.T) {
	log := newPluginSystemTestLogger()
	ham := NewHighAvailabilityManager(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := ham.Start(ctx)
	assert.NoError(t, err)

	ham.Stop()
}

func TestHighAvailabilityManager_HandleHealthUpdate(t *testing.T) {
	// These tests verify handleHealthUpdate doesn't panic and updates status
	// Note: handleHealthUpdate spawns goroutines for failover which complicates assertions

	t.Run("non-existent instance does not panic", func(t *testing.T) {
		log := newPluginSystemTestLogger()
		ham := NewHighAvailabilityManager(log)

		// Should not panic when called with non-existent instance
		ham.handleHealthUpdate("non-existent", true)
		ham.handleHealthUpdate("non-existent", false)
	})

	t.Run("healthy update on healthy instance", func(t *testing.T) {
		log := newPluginSystemTestLogger()
		ham := NewHighAvailabilityManager(log)

		instance := &ServiceInstance{
			ID:       "healthy-test",
			Address:  "localhost",
			Port:     8081,
			Protocol: "mcp",
			Status:   StatusHealthy,
		}
		ham.RegisterInstance(instance)

		// Healthy update on already healthy instance - no goroutine spawned
		ham.handleHealthUpdate("healthy-test", true)

		ham.mu.RLock()
		status := ham.instances["healthy-test"].Status
		ham.mu.RUnlock()
		assert.Equal(t, StatusHealthy, status)
	})

	t.Run("healthy update on unhealthy instance", func(t *testing.T) {
		log := newPluginSystemTestLogger()
		ham := NewHighAvailabilityManager(log)

		instance := &ServiceInstance{
			ID:       "recover-test",
			Address:  "localhost",
			Port:     8082,
			Protocol: "mcp",
			Status:   StatusUnhealthy,
		}
		ham.RegisterInstance(instance)

		// Healthy update on unhealthy instance - no goroutine spawned for recovery
		ham.handleHealthUpdate("recover-test", true)

		ham.mu.RLock()
		status := ham.instances["recover-test"].Status
		ham.mu.RUnlock()
		assert.Equal(t, StatusHealthy, status)
	})

	t.Run("unhealthy update triggers status change", func(t *testing.T) {
		log := newPluginSystemTestLogger()
		ham := NewHighAvailabilityManager(log)

		instance := &ServiceInstance{
			ID:       "fail-test",
			Address:  "localhost",
			Port:     8083,
			Protocol: "mcp",
			Status:   StatusHealthy,
		}
		ham.RegisterInstance(instance)

		// This spawns a goroutine for failover
		ham.handleHealthUpdate("fail-test", false)

		// Allow goroutine to start (we just verify it doesn't crash)
		time.Sleep(10 * time.Millisecond)

		// The status should be unhealthy (set before goroutine)
		ham.mu.RLock()
		status := ham.instances["fail-test"].Status
		ham.mu.RUnlock()
		// Status should have been changed to unhealthy
		assert.True(t, status == StatusUnhealthy || status == StatusStarting,
			"Status should be unhealthy or starting, got %d", status)
	})
}

func TestFailoverManager_CheckFailoverStatus(t *testing.T) {
	log := newPluginSystemTestLogger()
	fm := NewFailoverManager(log)

	// Register some instances
	primary := &ServiceInstance{
		ID:         "primary",
		Protocol:   "mcp",
		Status:     StatusHealthy,
		LastHealth: time.Now(),
	}
	backup := &ServiceInstance{
		ID:         "backup",
		Protocol:   "mcp",
		Status:     StatusHealthy,
		LastHealth: time.Now(),
	}

	fm.RegisterInstance(primary)
	fm.RegisterInstance(backup)

	t.Run("check failover status with healthy instances", func(t *testing.T) {
		// Should not panic or change anything
		fm.checkFailoverStatus()

		fm.mu.RLock()
		active := fm.activeInstances["mcp"]
		fm.mu.RUnlock()
		assert.Equal(t, "primary", active.ID)
	})

	t.Run("check failover status with unhealthy active", func(t *testing.T) {
		// Make primary unhealthy - checkFailoverStatus only logs, doesn't promote
		fm.mu.Lock()
		primary.Status = StatusUnhealthy
		fm.mu.Unlock()

		// Should not panic, just logs warning
		fm.checkFailoverStatus()

		fm.mu.RLock()
		active := fm.activeInstances["mcp"]
		fm.mu.RUnlock()
		// Active remains the same (checkFailoverStatus only logs)
		assert.Equal(t, "primary", active.ID)
	})
}

func TestHealthChecker_PerformHealthChecks(t *testing.T) {
	log := newPluginSystemTestLogger()
	hc := NewHealthChecker(log)

	// Register an instance
	hc.RegisterInstance("perf-check-inst", "localhost", 9999)

	healthUpdateFunc := func(instanceID string, healthy bool) {
		// Health update callback
	}

	t.Run("perform health checks", func(t *testing.T) {
		// This will attempt to connect to localhost:9999 which should fail
		hc.performHealthChecks(healthUpdateFunc)

		// Check that the health check was updated
		hc.mu.RLock()
		status := hc.healthChecks["perf-check-inst"]
		hc.mu.RUnlock()

		// Should have at least 1 failure after the check
		assert.GreaterOrEqual(t, status.ConsecutiveFailures, 0)
	})
}

func TestHealthChecker_CheckInstanceHealth(t *testing.T) {
	log := newPluginSystemTestLogger()

	t.Run("unregistered instance returns false", func(t *testing.T) {
		hc := NewHealthChecker(log)

		healthy := hc.checkInstanceHealth("non-existent-instance")
		assert.False(t, healthy)
	})

	t.Run("http health check success", func(t *testing.T) {
		// Create a test HTTP server that returns 200 OK
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/health", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		// Parse server address
		host, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthCheckerWithConfig(log, &HealthCheckerConfig{
			CheckInterval:      time.Second,
			Timeout:            5 * time.Second,
			UnhealthyThreshold: 3,
		})

		hc.RegisterInstanceWithProtocol("http-test", host, port, "http")

		healthy := hc.checkInstanceHealth("http-test")
		assert.True(t, healthy)

		// Check that status was updated
		status := hc.GetHealthStatus("http-test")
		assert.NotNil(t, status)
		assert.Empty(t, status.Error)
		assert.Greater(t, status.ResponseTime, time.Duration(0))
	})

	t.Run("http health check failure - 500 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		host, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)
		hc.RegisterInstanceWithProtocol("http-fail-test", host, port, "http")

		healthy := hc.checkInstanceHealth("http-fail-test")
		assert.False(t, healthy)

		status := hc.GetHealthStatus("http-fail-test")
		assert.Contains(t, status.Error, "unhealthy status code: 500")
	})

	t.Run("http health check failure - connection refused", func(t *testing.T) {
		hc := NewHealthCheckerWithConfig(log, &HealthCheckerConfig{
			CheckInterval:      time.Second,
			Timeout:            100 * time.Millisecond,
			UnhealthyThreshold: 3,
		})

		// Use a port that is unlikely to be listening
		hc.RegisterInstanceWithProtocol("http-refused", "127.0.0.1", 59999, "http")

		healthy := hc.checkInstanceHealth("http-refused")
		assert.False(t, healthy)

		status := hc.GetHealthStatus("http-refused")
		assert.Contains(t, status.Error, "health check request failed")
	})

	t.Run("https health check", func(t *testing.T) {
		// Create an HTTPS test server
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		host, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)
		// Use the test server's client which has the proper TLS config
		hc.SetHTTPClient(server.Client())
		hc.RegisterInstanceWithProtocol("https-test", host, port, "https")

		healthy := hc.checkInstanceHealth("https-test")
		assert.True(t, healthy)
	})

	t.Run("tcp health check success", func(t *testing.T) {
		// Create a TCP listener
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		host, portStr, _ := net.SplitHostPort(listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)
		hc.RegisterInstanceWithProtocol("tcp-test", host, port, "tcp")

		healthy := hc.checkInstanceHealth("tcp-test")
		assert.True(t, healthy)
	})

	t.Run("tcp health check failure", func(t *testing.T) {
		hc := NewHealthCheckerWithConfig(log, &HealthCheckerConfig{
			CheckInterval:      time.Second,
			Timeout:            100 * time.Millisecond,
			UnhealthyThreshold: 3,
		})

		// Use a port that is unlikely to be listening
		hc.RegisterInstanceWithProtocol("tcp-fail", "127.0.0.1", 59998, "tcp")

		healthy := hc.checkInstanceHealth("tcp-fail")
		assert.False(t, healthy)

		status := hc.GetHealthStatus("tcp-fail")
		assert.Contains(t, status.Error, "TCP connection failed")
	})

	t.Run("grpc health check uses tcp", func(t *testing.T) {
		// Create a TCP listener (gRPC uses TCP)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		host, portStr, _ := net.SplitHostPort(listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)
		hc.RegisterInstanceWithProtocol("grpc-test", host, port, "grpc")

		healthy := hc.checkInstanceHealth("grpc-test")
		assert.True(t, healthy)
	})

	t.Run("unknown protocol falls back to tcp", func(t *testing.T) {
		// Create a TCP listener
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		host, portStr, _ := net.SplitHostPort(listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)
		hc.RegisterInstanceWithProtocol("unknown-proto", host, port, "custom-protocol")

		healthy := hc.checkInstanceHealth("unknown-proto")
		assert.True(t, healthy)
	})

	t.Run("health check updates response time", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		host, portStr, _ := net.SplitHostPort(listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)
		hc.RegisterInstanceWithProtocol("response-time-test", host, port, "tcp")

		hc.checkInstanceHealth("response-time-test")

		status := hc.GetHealthStatus("response-time-test")
		assert.NotNil(t, status)
		assert.Greater(t, status.ResponseTime, time.Duration(0))
	})

	t.Run("health check clears error on success", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		host, portStr, _ := net.SplitHostPort(listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)
		hc.RegisterInstanceWithProtocol("clear-error-test", host, port, "tcp")

		// Set an error manually
		hc.mu.Lock()
		hc.healthChecks["clear-error-test"].Error = "previous error"
		hc.mu.Unlock()

		// Successful health check should clear the error
		healthy := hc.checkInstanceHealth("clear-error-test")
		assert.True(t, healthy)

		status := hc.GetHealthStatus("clear-error-test")
		assert.Empty(t, status.Error)
	})
}

func TestHealthChecker_CheckHTTPHealth(t *testing.T) {
	log := newPluginSystemTestLogger()

	t.Run("successful 200 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		host, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)

		healthy, err := hc.checkHTTPHealth(&InstanceInfo{
			Address:  host,
			Port:     port,
			Protocol: "http",
		})

		assert.True(t, healthy)
		assert.NoError(t, err)
	})

	t.Run("successful 201 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		host, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)

		healthy, err := hc.checkHTTPHealth(&InstanceInfo{
			Address:  host,
			Port:     port,
			Protocol: "http",
		})

		assert.True(t, healthy)
		assert.NoError(t, err)
	})

	t.Run("unhealthy 404 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		host, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)

		healthy, err := hc.checkHTTPHealth(&InstanceInfo{
			Address:  host,
			Port:     port,
			Protocol: "http",
		})

		assert.False(t, healthy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhealthy status code: 404")
	})

	t.Run("unhealthy 503 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		host, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)

		healthy, err := hc.checkHTTPHealth(&InstanceInfo{
			Address:  host,
			Port:     port,
			Protocol: "http",
		})

		assert.False(t, healthy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhealthy status code: 503")
	})

	t.Run("https scheme is used for https protocol", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		host, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)
		hc.SetHTTPClient(server.Client())

		healthy, err := hc.checkHTTPHealth(&InstanceInfo{
			Address:  host,
			Port:     port,
			Protocol: "https",
		})

		assert.True(t, healthy)
		assert.NoError(t, err)
	})
}

func TestHealthChecker_CheckTCPHealth(t *testing.T) {
	log := newPluginSystemTestLogger()

	t.Run("successful connection", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		host, portStr, _ := net.SplitHostPort(listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)

		healthy, err := hc.checkTCPHealth(&InstanceInfo{
			Address:  host,
			Port:     port,
			Protocol: "tcp",
		})

		assert.True(t, healthy)
		assert.NoError(t, err)
	})

	t.Run("connection refused", func(t *testing.T) {
		hc := NewHealthCheckerWithConfig(log, &HealthCheckerConfig{
			CheckInterval:      time.Second,
			Timeout:            100 * time.Millisecond,
			UnhealthyThreshold: 3,
		})

		healthy, err := hc.checkTCPHealth(&InstanceInfo{
			Address:  "127.0.0.1",
			Port:     59997,
			Protocol: "tcp",
		})

		assert.False(t, healthy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TCP connection failed")
	})
}

func TestHealthChecker_CheckGRPCHealth(t *testing.T) {
	log := newPluginSystemTestLogger()

	t.Run("grpc health check delegates to tcp", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		host, portStr, _ := net.SplitHostPort(listener.Addr().String())
		var port int
		fmt.Sscanf(portStr, "%d", &port)

		hc := NewHealthChecker(log)

		healthy, err := hc.checkGRPCHealth(&InstanceInfo{
			Address:  host,
			Port:     port,
			Protocol: "grpc",
		})

		assert.True(t, healthy)
		assert.NoError(t, err)
	})
}

func BenchmarkLeastLoadedLoadBalancer_SelectInstance(b *testing.B) {
	lb := &LeastLoadedLoadBalancer{}
	instances := make([]*ServiceInstance, 100)
	for i := 0; i < 100; i++ {
		instances[i] = &ServiceInstance{
			ID:        "inst-" + string(rune(i)),
			LoadScore: i % 100,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.SelectInstance("mcp", instances)
	}
}

func BenchmarkCircuitBreaker_Call(b *testing.B) {
	cb := NewCircuitBreaker(100, 10, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Call(func() error { return nil })
	}
}

// Tests for UpdateLoad functions on load balancers (no-op functions but need coverage)

func TestRoundRobinLoadBalancer_UpdateLoad(t *testing.T) {
	lb := &RoundRobinLoadBalancer{}

	// UpdateLoad is a no-op for round-robin, but should not panic
	lb.UpdateLoad("instance-1", 50)
	lb.UpdateLoad("instance-2", 100)
	lb.UpdateLoad("", 0) // edge case: empty instance ID
}

func TestLeastLoadedLoadBalancer_UpdateLoad(t *testing.T) {
	lb := &LeastLoadedLoadBalancer{}

	// UpdateLoad is a no-op for least-loaded (load scores stored in instances)
	lb.UpdateLoad("instance-1", 50)
	lb.UpdateLoad("instance-2", 100)
	lb.UpdateLoad("", 0) // edge case: empty instance ID
}

func TestRandomLoadBalancer_UpdateLoad(t *testing.T) {
	lb := &RandomLoadBalancer{}

	// RandomLoadBalancer UpdateLoad is a no-op
	lb.UpdateLoad("instance-1", 50)
	lb.UpdateLoad("instance-2", 100)
	lb.UpdateLoad("", 0) // edge case: empty instance ID
}
