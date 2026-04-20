package llm

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// HealthStatus represents the health state of a provider
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
	HealthStatusDegraded  HealthStatus = "degraded"
)

// ProviderHealth contains health information for a single provider
type ProviderHealth struct {
	ProviderID       string        `json:"provider_id"`
	Status           HealthStatus  `json:"status"`
	LastCheck        time.Time     `json:"last_check"`
	LastSuccess      time.Time     `json:"last_success,omitempty"`
	LastError        string        `json:"last_error,omitempty"`
	ConsecutiveFails int           `json:"consecutive_fails"`
	Latency          time.Duration `json:"latency,omitempty"`
	CheckCount       int64         `json:"check_count"`
	SuccessCount     int64         `json:"success_count"`
	FailureCount     int64         `json:"failure_count"`
}

// HealthMonitorConfig configures the health monitor
type HealthMonitorConfig struct {
	CheckInterval      time.Duration // How often to run health checks
	HealthyThreshold   int           // Consecutive successes to mark healthy
	UnhealthyThreshold int           // Consecutive failures to mark unhealthy
	Timeout            time.Duration // Timeout for individual health checks
	Enabled            bool          // Whether monitoring is enabled
}

// DefaultHealthMonitorConfig returns sensible defaults
func DefaultHealthMonitorConfig() HealthMonitorConfig {
	return HealthMonitorConfig{
		CheckInterval:      30 * time.Second,
		HealthyThreshold:   2,
		UnhealthyThreshold: 3,
		Timeout:            10 * time.Second,
		Enabled:            true,
	}
}

// HealthMonitor monitors the health of multiple LLM providers.
//
// Concurrency model (CONST-029): providers/health → 2× *safe.Store;
// listeners → *safe.Slice; running → atomic.Bool. healthMu
// (Pattern Zeta, renamed from mu) serialises per-*ProviderHealth
// field mutations in checkProvider/RecordSuccess/RecordFailure —
// those mutations go through the shared pointer returned by
// Store.Get and need a write-fence.
type HealthMonitor struct {
	providers *safe.Store[string, LLMProvider]
	health    *safe.Store[string, *ProviderHealth]
	healthMu  sync.Mutex
	config    HealthMonitorConfig
	ctx       context.Context
	cancel    context.CancelFunc
	running   atomic.Bool
	listeners *safe.Slice[HealthListener]
}

// HealthListener is called when provider health changes
type HealthListener func(providerID string, oldStatus, newStatus HealthStatus)

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(config HealthMonitorConfig) *HealthMonitor {
	return &HealthMonitor{
		providers: safe.NewStore[string, LLMProvider](),
		health:    safe.NewStore[string, *ProviderHealth](),
		config:    config,
		listeners: safe.NewSlice[HealthListener](),
	}
}

// NewDefaultHealthMonitor creates a health monitor with default config
func NewDefaultHealthMonitor() *HealthMonitor {
	return NewHealthMonitor(DefaultHealthMonitorConfig())
}

// RegisterProvider registers a provider for health monitoring
func (hm *HealthMonitor) RegisterProvider(providerID string, provider LLMProvider) {
	hm.providers.Put(providerID, provider)
	hm.health.Put(providerID, &ProviderHealth{
		ProviderID: providerID,
		Status:     HealthStatusUnknown,
	})
}

// UnregisterProvider removes a provider from health monitoring
func (hm *HealthMonitor) UnregisterProvider(providerID string) {
	hm.providers.Delete(providerID)
	hm.health.Delete(providerID)
}

// AddListener adds a listener for health status changes
func (hm *HealthMonitor) AddListener(listener HealthListener) {
	hm.listeners.Append(listener)
}

// Start begins the health monitoring loop
func (hm *HealthMonitor) Start() {
	if !hm.config.Enabled {
		return
	}
	if !hm.running.CompareAndSwap(false, true) {
		return
	}
	hm.ctx, hm.cancel = context.WithCancel(context.Background())

	// Run initial health check
	hm.checkAllProviders()

	// Start periodic health checks
	go hm.monitorLoop()
}

// Stop stops the health monitoring loop
func (hm *HealthMonitor) Stop() {
	if !hm.running.CompareAndSwap(true, false) {
		return
	}
	if hm.cancel != nil {
		hm.cancel()
	}
}

// IsRunning returns true if the monitor is running
func (hm *HealthMonitor) IsRunning() bool {
	return hm.running.Load()
}

// monitorLoop runs periodic health checks
func (hm *HealthMonitor) monitorLoop() {
	ticker := time.NewTicker(hm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hm.ctx.Done():
			return
		case <-ticker.C:
			hm.checkAllProviders()
		}
	}
}

// checkAllProviders checks health of all registered providers
func (hm *HealthMonitor) checkAllProviders() {
	providers := hm.providers.Snapshot()

	var wg sync.WaitGroup
	for id, provider := range providers {
		wg.Add(1)
		go func(providerID string, p LLMProvider) {
			defer wg.Done()
			hm.checkProvider(providerID, p)
		}(id, provider)
	}
	wg.Wait()
}

// checkProvider checks health of a single provider
func (hm *HealthMonitor) checkProvider(providerID string, provider LLMProvider) {
	ctx, cancel := context.WithTimeout(hm.ctx, hm.config.Timeout)
	defer cancel()

	start := time.Now()

	// Run health check in goroutine to respect timeout
	errChan := make(chan error, 1)
	go func() {
		errChan <- provider.HealthCheck()
	}()

	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errChan:
	}

	latency := time.Since(start)

	hm.healthMu.Lock()
	health, exists := hm.health.Get(providerID)
	if !exists {
		hm.healthMu.Unlock()
		return
	}

	oldStatus := health.Status
	health.LastCheck = time.Now()
	health.Latency = latency
	health.CheckCount++

	if err != nil {
		health.ConsecutiveFails++
		health.FailureCount++
		health.LastError = err.Error()

		if health.ConsecutiveFails >= hm.config.UnhealthyThreshold {
			health.Status = HealthStatusUnhealthy
		} else if health.Status == HealthStatusHealthy {
			health.Status = HealthStatusDegraded
		}
	} else {
		health.ConsecutiveFails = 0
		health.SuccessCount++
		health.LastSuccess = time.Now()
		health.LastError = ""

		if health.SuccessCount >= int64(hm.config.HealthyThreshold) || health.Status == HealthStatusUnknown {
			health.Status = HealthStatusHealthy
		}
	}
	newStatus := health.Status
	hm.healthMu.Unlock()

	// Notify listeners if status changed
	if oldStatus != newStatus {
		for _, listener := range hm.listeners.Snapshot() {
			go listener(providerID, oldStatus, newStatus)
		}
	}
}

// GetHealth returns the health status of a specific provider
func (hm *HealthMonitor) GetHealth(providerID string) (*ProviderHealth, bool) {
	health, exists := hm.health.Get(providerID)
	if !exists {
		return nil, false
	}

	// Return a copy to prevent mutation; take healthMu to fence against
	// concurrent writers in checkProvider.
	hm.healthMu.Lock()
	defer hm.healthMu.Unlock()
	copy := *health
	return &copy, true
}

// GetAllHealth returns health status for all providers
func (hm *HealthMonitor) GetAllHealth() map[string]*ProviderHealth {
	hm.healthMu.Lock()
	defer hm.healthMu.Unlock()

	snap := hm.health.Snapshot()
	result := make(map[string]*ProviderHealth, len(snap))
	for id, health := range snap {
		copy := *health
		result[id] = &copy
	}
	return result
}

// GetHealthyProviders returns IDs of all healthy providers
func (hm *HealthMonitor) GetHealthyProviders() []string {
	hm.healthMu.Lock()
	defer hm.healthMu.Unlock()

	var healthy []string
	hm.health.Range(func(id string, health *ProviderHealth) bool {
		if health.Status == HealthStatusHealthy {
			healthy = append(healthy, id)
		}
		return true
	})
	return healthy
}

// IsHealthy returns true if the specified provider is healthy
func (hm *HealthMonitor) IsHealthy(providerID string) bool {
	hm.healthMu.Lock()
	defer hm.healthMu.Unlock()

	health, exists := hm.health.Get(providerID)
	if !exists {
		return false
	}
	return health.Status == HealthStatusHealthy
}

// RecordSuccess manually records a successful operation for a provider
func (hm *HealthMonitor) RecordSuccess(providerID string) {
	hm.healthMu.Lock()
	health, exists := hm.health.Get(providerID)
	if !exists {
		hm.healthMu.Unlock()
		return
	}

	health.ConsecutiveFails = 0
	health.SuccessCount++
	health.LastSuccess = time.Now()

	var transitioned bool
	var oldStatus HealthStatus
	if health.Status != HealthStatusHealthy && health.SuccessCount >= int64(hm.config.HealthyThreshold) {
		oldStatus = health.Status
		health.Status = HealthStatusHealthy
		transitioned = true
	}
	hm.healthMu.Unlock()

	if transitioned {
		for _, listener := range hm.listeners.Snapshot() {
			go listener(providerID, oldStatus, HealthStatusHealthy)
		}
	}
}

// RecordFailure manually records a failed operation for a provider
func (hm *HealthMonitor) RecordFailure(providerID string, err error) {
	hm.healthMu.Lock()
	health, exists := hm.health.Get(providerID)
	if !exists {
		hm.healthMu.Unlock()
		return
	}

	health.ConsecutiveFails++
	health.FailureCount++
	if err != nil {
		health.LastError = err.Error()
	}

	var transitioned bool
	var oldStatus, newStatus HealthStatus
	if health.ConsecutiveFails >= hm.config.UnhealthyThreshold {
		if health.Status != HealthStatusUnhealthy {
			oldStatus = health.Status
			health.Status = HealthStatusUnhealthy
			newStatus = HealthStatusUnhealthy
			transitioned = true
		}
	} else if health.Status == HealthStatusHealthy {
		oldStatus = health.Status
		health.Status = HealthStatusDegraded
		newStatus = HealthStatusDegraded
		transitioned = true
	}
	hm.healthMu.Unlock()

	if transitioned {
		for _, listener := range hm.listeners.Snapshot() {
			go listener(providerID, oldStatus, newStatus)
		}
	}
}

// GetAggregateHealth returns overall system health summary
func (hm *HealthMonitor) GetAggregateHealth() AggregateHealth {
	hm.healthMu.Lock()
	defer hm.healthMu.Unlock()

	agg := AggregateHealth{
		TotalProviders:     hm.health.Len(),
		HealthyProviders:   0,
		DegradedProviders:  0,
		UnhealthyProviders: 0,
		UnknownProviders:   0,
		Providers:          make(map[string]HealthStatus),
	}

	hm.health.Range(func(id string, health *ProviderHealth) bool {
		agg.Providers[id] = health.Status
		switch health.Status {
		case HealthStatusHealthy:
			agg.HealthyProviders++
		case HealthStatusDegraded:
			agg.DegradedProviders++
		case HealthStatusUnhealthy:
			agg.UnhealthyProviders++
		case HealthStatusUnknown:
			agg.UnknownProviders++
		}
		return true
	})

	// Determine overall status
	if agg.HealthyProviders == agg.TotalProviders {
		agg.OverallStatus = HealthStatusHealthy
	} else if agg.UnhealthyProviders == agg.TotalProviders {
		agg.OverallStatus = HealthStatusUnhealthy
	} else if agg.HealthyProviders > 0 {
		agg.OverallStatus = HealthStatusDegraded
	} else {
		agg.OverallStatus = HealthStatusUnknown
	}

	return agg
}

// AggregateHealth contains overall health summary
type AggregateHealth struct {
	OverallStatus      HealthStatus            `json:"overall_status"`
	TotalProviders     int                     `json:"total_providers"`
	HealthyProviders   int                     `json:"healthy_providers"`
	DegradedProviders  int                     `json:"degraded_providers"`
	UnhealthyProviders int                     `json:"unhealthy_providers"`
	UnknownProviders   int                     `json:"unknown_providers"`
	Providers          map[string]HealthStatus `json:"providers"`
}

// ForceCheck forces an immediate health check for a specific provider
func (hm *HealthMonitor) ForceCheck(providerID string) error {
	provider, exists := hm.providers.Get(providerID)
	if !exists {
		return nil
	}

	hm.checkProvider(providerID, provider)
	return nil
}

// GetConfig returns the current monitor configuration
func (hm *HealthMonitor) GetConfig() HealthMonitorConfig {
	return hm.config
}
