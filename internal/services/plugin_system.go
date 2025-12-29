package services

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HighAvailabilityManager provides high availability features with load balancing and failover
type HighAvailabilityManager struct {
	mu             sync.RWMutex
	instances      map[string]*ServiceInstance
	loadBalancer   LoadBalancer
	failoverManager *FailoverManager
	healthChecker  *HealthChecker
	logger         *logrus.Logger
	stopChan       chan struct{}
}

// ServiceInstance represents a service instance in the HA cluster
type ServiceInstance struct {
	ID          string
	Address     string
	Port        int
	Protocol    string
	Status      InstanceStatus
	LastHealth  time.Time
	LoadScore   int // 0-100, higher means more loaded
	Metadata    map[string]interface{}
}

// InstanceStatus represents the status of a service instance
type InstanceStatus int

const (
	StatusStarting InstanceStatus = iota
	StatusHealthy
	StatusDegraded
	StatusUnhealthy
	StatusDown
)

// LoadBalancer handles load distribution across instances
type LoadBalancer interface {
	SelectInstance(protocol string, instances []*ServiceInstance) *ServiceInstance
	UpdateLoad(instanceID string, loadScore int)
}

// RoundRobinLoadBalancer implements round-robin load balancing
type RoundRobinLoadBalancer struct {
	mu       sync.Mutex
	lastUsed map[string]int // protocol -> last used index
}

// LeastLoadedLoadBalancer implements least-loaded load balancing
type LeastLoadedLoadBalancer struct {
	mu sync.RWMutex
}

// FailoverManager handles automatic failover
type FailoverManager struct {
	mu                sync.RWMutex
	failoverGroups    map[string][]*ServiceInstance
	activeInstances   map[string]*ServiceInstance
	failoverThreshold time.Duration
	logger            *logrus.Logger
}

// HealthChecker performs health checks on service instances
type HealthChecker struct {
	mu              sync.RWMutex
	checkInterval   time.Duration
	timeout         time.Duration
	unhealthyThreshold int
	healthChecks    map[string]*HealthStatus
	logger          *logrus.Logger
}

// HealthStatus represents the health status of an instance
type HealthStatus struct {
	InstanceID     string
	LastCheck      time.Time
	ConsecutiveFailures int
	IsHealthy      bool
	ResponseTime   time.Duration
	Error          string
}

// NewHighAvailabilityManager creates a new HA manager
func NewHighAvailabilityManager(logger *logrus.Logger) *HighAvailabilityManager {
	return &HighAvailabilityManager{
		instances:       make(map[string]*ServiceInstance),
		loadBalancer:    &LeastLoadedLoadBalancer{},
		failoverManager: NewFailoverManager(logger),
		healthChecker:   NewHealthChecker(logger),
		logger:          logger,
		stopChan:        make(chan struct{}),
	}
}

// RegisterInstance registers a new service instance
func (ham *HighAvailabilityManager) RegisterInstance(instance *ServiceInstance) error {
	ham.mu.Lock()
	defer ham.mu.Unlock()

	if _, exists := ham.instances[instance.ID]; exists {
		return fmt.Errorf("instance %s already registered", instance.ID)
	}

	instance.Status = StatusStarting
	instance.LastHealth = time.Now()
	ham.instances[instance.ID] = instance

	// Register with failover manager
	ham.failoverManager.RegisterInstance(instance)

	// Register with health checker
	ham.healthChecker.RegisterInstance(instance.ID, instance.Address, instance.Port)

	ham.logger.WithFields(logrus.Fields{
		"instanceId": instance.ID,
		"protocol":   instance.Protocol,
		"address":    instance.Address,
		"port":       instance.Port,
	}).Info("Service instance registered")

	return nil
}

// UnregisterInstance removes a service instance
func (ham *HighAvailabilityManager) UnregisterInstance(instanceID string) error {
	ham.mu.Lock()
	defer ham.mu.Unlock()

	if _, exists := ham.instances[instanceID]; !exists {
		return fmt.Errorf("instance %s not registered", instanceID)
	}

	delete(ham.instances, instanceID)

	// Unregister from failover manager
	ham.failoverManager.UnregisterInstance(instanceID)

	// Unregister from health checker
	ham.healthChecker.UnregisterInstance(instanceID)

	ham.logger.WithField("instanceId", instanceID).Info("Service instance unregistered")
	return nil
}

// GetInstance selects an available instance for a protocol
func (ham *HighAvailabilityManager) GetInstance(protocol string) (*ServiceInstance, error) {
	ham.mu.RLock()
	var instances []*ServiceInstance
	for _, instance := range ham.instances {
		if instance.Protocol == protocol && instance.Status == StatusHealthy {
			instances = append(instances, instance)
		}
	}
	ham.mu.RUnlock()

	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instances available for protocol %s", protocol)
	}

	selected := ham.loadBalancer.SelectInstance(protocol, instances)

	ham.logger.WithFields(logrus.Fields{
		"protocol":   protocol,
		"instanceId": selected.ID,
		"address":    selected.Address,
		"port":       selected.Port,
	}).Debug("Instance selected by load balancer")

	return selected, nil
}

// UpdateInstanceLoad updates the load score for an instance
func (ham *HighAvailabilityManager) UpdateInstanceLoad(instanceID string, loadScore int) error {
	ham.mu.Lock()
	defer ham.mu.Unlock()

	instance, exists := ham.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	instance.LoadScore = loadScore
	ham.loadBalancer.UpdateLoad(instanceID, loadScore)

	return nil
}

// GetAllInstances returns all registered instances
func (ham *HighAvailabilityManager) GetAllInstances() []*ServiceInstance {
	ham.mu.RLock()
	defer ham.mu.RUnlock()

	instances := make([]*ServiceInstance, 0, len(ham.instances))
	for _, instance := range ham.instances {
		instances = append(instances, instance)
	}

	return instances
}

// GetInstancesByProtocol returns instances for a specific protocol
func (ham *HighAvailabilityManager) GetInstancesByProtocol(protocol string) []*ServiceInstance {
	ham.mu.RLock()
	defer ham.mu.RUnlock()

	var instances []*ServiceInstance
	for _, instance := range ham.instances {
		if instance.Protocol == protocol {
			instances = append(instances, instance)
		}
	}

	return instances
}

// Start begins the HA management processes
func (ham *HighAvailabilityManager) Start(ctx context.Context) error {
	ham.logger.Info("Starting High Availability Manager")

	// Start health checker
	go ham.healthChecker.Start(ctx, ham.handleHealthUpdate)

	// Start failover manager
	go ham.failoverManager.Start(ctx)

	return nil
}

// Stop stops the HA management processes
func (ham *HighAvailabilityManager) Stop() {
	ham.logger.Info("Stopping High Availability Manager")

	close(ham.stopChan)
	ham.healthChecker.Stop()
	ham.failoverManager.Stop()
}

// Private methods

func (ham *HighAvailabilityManager) handleHealthUpdate(instanceID string, healthy bool) {
	ham.mu.Lock()
	defer ham.mu.Unlock()

	instance, exists := ham.instances[instanceID]
	if !exists {
		return
	}

	oldStatus := instance.Status

	if healthy {
		if instance.Status != StatusHealthy {
			instance.Status = StatusHealthy
			ham.logger.WithField("instanceId", instanceID).Info("Instance became healthy")
		}
	} else {
		if instance.Status == StatusHealthy {
			instance.Status = StatusUnhealthy
			ham.logger.WithField("instanceId", instanceID).Warn("Instance became unhealthy")

			// Trigger failover
			go ham.failoverManager.HandleInstanceFailure(instance)
		}
	}

	instance.LastHealth = time.Now()

	if oldStatus != instance.Status {
		ham.logger.WithFields(logrus.Fields{
			"instanceId": instanceID,
			"oldStatus":  oldStatus,
			"newStatus":  instance.Status,
		}).Info("Instance status changed")
	}
}

// LoadBalancer implementations

// SelectInstance selects an instance using round-robin
func (rr *RoundRobinLoadBalancer) SelectInstance(protocol string, instances []*ServiceInstance) *ServiceInstance {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if len(instances) == 0 {
		return nil
	}

	if rr.lastUsed == nil {
		rr.lastUsed = make(map[string]int)
	}

	lastIndex := rr.lastUsed[protocol]
	nextIndex := (lastIndex + 1) % len(instances)
	rr.lastUsed[protocol] = nextIndex

	return instances[nextIndex]
}

// UpdateLoad updates load information (no-op for round-robin)
func (rr *RoundRobinLoadBalancer) UpdateLoad(instanceID string, loadScore int) {
	// Round-robin doesn't use load scores
}

// SelectInstance selects the least loaded instance
func (ll *LeastLoadedLoadBalancer) SelectInstance(protocol string, instances []*ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// Find instance with lowest load score
	var selected *ServiceInstance
	minLoad := 101 // Higher than max possible load score

	for _, instance := range instances {
		if instance.LoadScore < minLoad {
			minLoad = instance.LoadScore
			selected = instance
		}
	}

	return selected
}

// UpdateLoad updates load information
func (ll *LeastLoadedLoadBalancer) UpdateLoad(instanceID string, loadScore int) {
	// Load scores are stored in the instances themselves
}

// FailoverManager implementation

// NewFailoverManager creates a new failover manager
func NewFailoverManager(logger *logrus.Logger) *FailoverManager {
	return &FailoverManager{
		failoverGroups:    make(map[string][]*ServiceInstance),
		activeInstances:   make(map[string]*ServiceInstance),
		failoverThreshold: 30 * time.Second,
		logger:            logger,
	}
}

// RegisterInstance registers an instance with the failover manager
func (fm *FailoverManager) RegisterInstance(instance *ServiceInstance) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	protocol := instance.Protocol
	fm.failoverGroups[protocol] = append(fm.failoverGroups[protocol], instance)

	// If this is the first instance or current active is unhealthy, make it active
	if _, exists := fm.activeInstances[protocol]; !exists {
		fm.activeInstances[protocol] = instance
		fm.logger.WithFields(logrus.Fields{
			"protocol":   protocol,
			"instanceId": instance.ID,
		}).Info("Instance set as active for protocol")
	}
}

// UnregisterInstance removes an instance from failover management
func (fm *FailoverManager) UnregisterInstance(instanceID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Remove from all failover groups
	for protocol, instances := range fm.failoverGroups {
		for i, instance := range instances {
			if instance.ID == instanceID {
				fm.failoverGroups[protocol] = append(instances[:i], instances[i+1:]...)

				// If this was the active instance, promote another
				if active, exists := fm.activeInstances[protocol]; exists && active.ID == instanceID {
					fm.promoteNewActive(protocol)
				}
				break
			}
		}
	}
}

// HandleInstanceFailure handles failure of an instance
func (fm *FailoverManager) HandleInstanceFailure(instance *ServiceInstance) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	protocol := instance.Protocol

	// If this was the active instance, promote a backup
	if active, exists := fm.activeInstances[protocol]; exists && active.ID == instance.ID {
		fm.logger.WithFields(logrus.Fields{
			"protocol":   protocol,
			"instanceId": instance.ID,
		}).Warn("Active instance failed, promoting backup")

		fm.promoteNewActive(protocol)
	}
}

// Start begins failover monitoring
func (fm *FailoverManager) Start(ctx context.Context) {
	// Periodic check for failed instances
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fm.checkFailoverStatus()
			}
		}
	}()
}

// Stop stops failover monitoring
func (fm *FailoverManager) Stop() {
	// Cleanup handled by context cancellation
}

func (fm *FailoverManager) promoteNewActive(protocol string) {
	instances := fm.failoverGroups[protocol]

	// Find a healthy backup instance
	for _, instance := range instances {
		if instance.Status == StatusHealthy {
			fm.activeInstances[protocol] = instance
			fm.logger.WithFields(logrus.Fields{
				"protocol":   protocol,
				"instanceId": instance.ID,
			}).Info("New active instance promoted")
			return
		}
	}

	fm.logger.WithField("protocol", protocol).Error("No healthy backup instances available")
}

func (fm *FailoverManager) checkFailoverStatus() {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	for protocol, active := range fm.activeInstances {
		if active.Status != StatusHealthy {
			// Active instance is not healthy, should have been handled by failure detection
			fm.logger.WithFields(logrus.Fields{
				"protocol": protocol,
				"activeInstance": active.ID,
				"status": active.Status,
			}).Warn("Active instance is not healthy")
		}
	}
}

// HealthChecker implementation

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger *logrus.Logger) *HealthChecker {
	return &HealthChecker{
		checkInterval:      30 * time.Second,
		timeout:            5 * time.Second,
		unhealthyThreshold: 3,
		healthChecks:       make(map[string]*HealthStatus),
		logger:             logger,
	}
}

// RegisterInstance registers an instance for health checking
func (hc *HealthChecker) RegisterInstance(instanceID, address string, port int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.healthChecks[instanceID] = &HealthStatus{
		InstanceID: instanceID,
		LastCheck:  time.Now(),
		IsHealthy:  true, // Assume healthy initially
	}
}

// UnregisterInstance removes an instance from health checking
func (hc *HealthChecker) UnregisterInstance(instanceID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	delete(hc.healthChecks, instanceID)
}

// Start begins health checking
func (hc *HealthChecker) Start(ctx context.Context, healthUpdateFunc func(string, bool)) {
	go func() {
		ticker := time.NewTicker(hc.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hc.performHealthChecks(healthUpdateFunc)
			}
		}
	}()
}

// Stop stops health checking
func (hc *HealthChecker) Stop() {
	// Cleanup handled by context cancellation
}

func (hc *HealthChecker) performHealthChecks(healthUpdateFunc func(string, bool)) {
	hc.mu.Lock()
	instances := make(map[string]*HealthStatus)
	for k, v := range hc.healthChecks {
		instances[k] = v
	}
	hc.mu.Unlock()

	for instanceID, status := range instances {
		healthy := hc.checkInstanceHealth(instanceID)
		oldHealthy := status.IsHealthy

		if healthy {
			status.ConsecutiveFailures = 0
			status.IsHealthy = true
		} else {
			status.ConsecutiveFailures++
			if status.ConsecutiveFailures >= hc.unhealthyThreshold {
				status.IsHealthy = false
			}
		}

		status.LastCheck = time.Now()

		// Notify if health status changed
		if oldHealthy != status.IsHealthy {
			healthUpdateFunc(instanceID, status.IsHealthy)
		}
	}
}

func (hc *HealthChecker) checkInstanceHealth(instanceID string) bool {
	// Simplified health check - in real implementation, this would:
	// 1. Make HTTP request to /health endpoint
	// 2. Check TCP connectivity
	// 3. Perform protocol-specific health checks

	// For demo, randomly succeed/fail
	return rand.Intn(10) > 1 // 80% success rate
}

// Circuit Breaker for fault tolerance

type CircuitBreaker struct {
	mu                sync.Mutex
	state             CircuitState
	failureThreshold  int
	successThreshold  int
	timeout           time.Duration
	consecutiveFailures int
	consecutiveSuccesses int
	lastFailure       time.Time
}

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen {
		if time.Since(cb.lastFailure) < cb.timeout {
			return fmt.Errorf("circuit breaker is open")
		}
		cb.state = StateHalfOpen
	}

	err := fn()

	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

func (cb *CircuitBreaker) onFailure() {
	cb.consecutiveFailures++
	cb.lastFailure = time.Now()

	if cb.consecutiveFailures >= cb.failureThreshold {
		cb.state = StateOpen
		cb.consecutiveSuccesses = 0
	}
}

func (cb *CircuitBreaker) onSuccess() {
	cb.consecutiveSuccesses++

	if cb.state == StateHalfOpen && cb.consecutiveSuccesses >= cb.successThreshold {
		cb.state = StateClosed
		cb.consecutiveFailures = 0
		cb.consecutiveSuccesses = 0
	}
}

// Service Registry for service discovery

type ServiceRegistry struct {
	mu        sync.RWMutex
	services  map[string][]*ServiceEndpoint
	logger    *logrus.Logger
}

type ServiceEndpoint struct {
	ID       string
	Address  string
	Port     int
	Protocol string
	Metadata map[string]interface{}
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(logger *logrus.Logger) *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string][]*ServiceEndpoint),
		logger:   logger,
	}
}

// RegisterService registers a service endpoint
func (sr *ServiceRegistry) RegisterService(serviceType string, endpoint *ServiceEndpoint) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.services[serviceType] = append(sr.services[serviceType], endpoint)

	sr.logger.WithFields(logrus.Fields{
		"serviceType": serviceType,
		"endpointId":  endpoint.ID,
		"address":     endpoint.Address,
		"port":        endpoint.Port,
	}).Info("Service endpoint registered")
}

// UnregisterService removes a service endpoint
func (sr *ServiceRegistry) UnregisterService(serviceType, endpointID string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	endpoints := sr.services[serviceType]
	for i, endpoint := range endpoints {
		if endpoint.ID == endpointID {
			sr.services[serviceType] = append(endpoints[:i], endpoints[i+1:]...)
			break
		}
	}
}

// DiscoverServices discovers service endpoints
func (sr *ServiceRegistry) DiscoverServices(serviceType string) []*ServiceEndpoint {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	endpoints := sr.services[serviceType]
	result := make([]*ServiceEndpoint, len(endpoints))
	copy(result, endpoints)

	return result
}

// Load Balancer Strategies

// WeightedRoundRobinLoadBalancer implements weighted round-robin
type WeightedRoundRobinLoadBalancer struct {
	mu       sync.Mutex
	current  map[string]int
	weights  map[string]int
}

// RandomLoadBalancer implements random load balancing
type RandomLoadBalancer struct{}

// SelectInstance selects a random instance
func (rl *RandomLoadBalancer) SelectInstance(protocol string, instances []*ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	return instances[rand.Intn(len(instances))]
}

// UpdateLoad updates load information (no-op for random)
func (rl *RandomLoadBalancer) UpdateLoad(instanceID string, loadScore int) {
	// Random load balancer doesn't use load scores
}
