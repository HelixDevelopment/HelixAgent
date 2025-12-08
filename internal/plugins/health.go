package plugins

import (
	"context"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/utils"
)

// HealthMonitor manages plugin health checks and circuit breaking
type HealthMonitor struct {
	registry      *Registry
	checkInterval time.Duration
	timeout       time.Duration
	mu            sync.RWMutex
	healthStatus  map[string]PluginHealth
}

type PluginHealth struct {
	Name                string
	Status              string // healthy, degraded, unhealthy
	LastCheck           time.Time
	ResponseTime        time.Duration
	ErrorCount          int
	ConsecutiveFailures int
	CircuitOpen         bool
}

func NewHealthMonitor(registry *Registry, checkInterval, timeout time.Duration) *HealthMonitor {
	return &HealthMonitor{
		registry:      registry,
		checkInterval: checkInterval,
		timeout:       timeout,
		healthStatus:  make(map[string]PluginHealth),
	}
}

func (h *HealthMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(h.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.checkAllPlugins(ctx)
		}
	}
}

func (h *HealthMonitor) checkAllPlugins(ctx context.Context) {
	plugins := h.registry.List()

	for _, name := range plugins {
		h.checkPlugin(ctx, name)
	}
}

func (h *HealthMonitor) checkPlugin(ctx context.Context, name string) {
	plugin, exists := h.registry.Get(name)
	if !exists {
		return
	}

	start := time.Now()
	checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	err := plugin.HealthCheck(checkCtx)
	responseTime := time.Since(start)

	h.mu.Lock()
	defer h.mu.Unlock()

	health := h.healthStatus[name]
	health.Name = name
	health.LastCheck = time.Now()
	health.ResponseTime = responseTime

	if err != nil {
		health.ErrorCount++
		health.ConsecutiveFailures++
		health.Status = "unhealthy"

		if health.ConsecutiveFailures >= 3 {
			health.CircuitOpen = true
			utils.GetLogger().Warnf("Plugin %s circuit breaker opened after %d failures", name, health.ConsecutiveFailures)
		}
	} else {
		health.ConsecutiveFailures = 0
		health.CircuitOpen = false

		if responseTime > 5*time.Second {
			health.Status = "degraded"
		} else {
			health.Status = "healthy"
		}
	}

	h.healthStatus[name] = health
}

func (h *HealthMonitor) GetHealth(name string) (PluginHealth, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	health, exists := h.healthStatus[name]
	return health, exists
}

func (h *HealthMonitor) IsHealthy(name string) bool {
	health, exists := h.GetHealth(name)
	return exists && health.Status == "healthy" && !health.CircuitOpen
}
