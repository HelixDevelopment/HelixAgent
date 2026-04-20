package plugins

import (
	"context"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/utils"
)

// HealthMonitor manages plugin health checks and circuit breaking.
//
// Concurrency model (CONST-029): healthStatus → *safe.Store. mu
// dropped — the per-plugin compound "read → mutate → write" in
// checkPlugin is expressed as a single Store.Update closure that
// runs under the Store's internal write lock.
type HealthMonitor struct {
	registry      *Registry
	checkInterval time.Duration
	timeout       time.Duration
	healthStatus  *safe.Store[string, PluginHealth]
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
		healthStatus:  safe.NewStore[string, PluginHealth](),
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

	var circuitOpened bool
	var failures int
	h.healthStatus.Update(name, func(cur PluginHealth, _ bool) (PluginHealth, bool) {
		cur.Name = name
		cur.LastCheck = time.Now()
		cur.ResponseTime = responseTime

		if err != nil {
			cur.ErrorCount++
			cur.ConsecutiveFailures++
			cur.Status = "unhealthy"

			if cur.ConsecutiveFailures >= 3 {
				cur.CircuitOpen = true
				circuitOpened = true
				failures = cur.ConsecutiveFailures
			}
		} else {
			cur.ConsecutiveFailures = 0
			cur.CircuitOpen = false

			if responseTime > 5*time.Second {
				cur.Status = "degraded"
			} else {
				cur.Status = "healthy"
			}
		}
		return cur, true
	})

	if circuitOpened {
		utils.GetLogger().Warnf("Plugin %s circuit breaker opened after %d failures", name, failures)
	}
}

func (h *HealthMonitor) GetHealth(name string) (PluginHealth, bool) {
	return h.healthStatus.Get(name)
}

func (h *HealthMonitor) IsHealthy(name string) bool {
	health, exists := h.GetHealth(name)
	return exists && health.Status == "healthy" && !health.CircuitOpen
}
