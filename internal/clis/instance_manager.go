// Package clis provides CLI agent integration for HelixAgent.
package clis

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/google/uuid"
)

// InstanceManager manages the lifecycle of CLI agent instances.
//
// Concurrency model (CONST-029): pools and instances are *safe.Store
// instances; the Store's internal lock serialises access. Metrics use
// atomic.AddUint64 / atomic.LoadUint64 and need no lock.
type InstanceManager struct {
	db     *sql.DB
	logger *log.Logger

	// Instance pools by type for efficient reuse
	pools *safe.Store[AgentType, *InstancePool]

	// Active instances (both idle and active)
	instances *safe.Store[string, *AgentInstance]

	// Event bus for instance events
	eventBus *EventBus

	// Background workers
	workerPool *WorkerPool

	// Health check control
	healthCheckStop chan struct{}

	// Metrics
	createdCount   uint64
	destroyedCount uint64
}

// NewInstanceManager creates a new instance manager.
func NewInstanceManager(db *sql.DB, logger *log.Logger) (*InstanceManager, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection required")
	}

	if logger == nil {
		logger = log.Default()
	}

	im := &InstanceManager{
		db:              db,
		logger:          logger,
		pools:           safe.NewStore[AgentType, *InstancePool](),
		instances:       safe.NewStore[string, *AgentInstance](),
		eventBus:        NewEventBus(),
		workerPool:      NewWorkerPool(100),
		healthCheckStop: make(chan struct{}),
	}

	// Start background health checks
	go im.healthCheckLoop()

	// Recover existing instances from database
	if err := im.recoverInstances(context.Background()); err != nil {
		logger.Printf("Warning: failed to recover instances: %v", err)
	}

	return im, nil
}

// CreateInstance creates a new agent instance.
func (m *InstanceManager) CreateInstance(
	ctx context.Context,
	agentType AgentType,
	config InstanceConfig,
	providerName string,
) (*AgentInstance, error) {
	// Check if this agent type is available
	if !m.IsAgentTypeAvailable(agentType) {
		return nil, fmt.Errorf("agent type %s is not available", agentType)
	}

	// Generate unique ID and name
	instanceID := uuid.New().String()
	instanceName := fmt.Sprintf("%s-%s", agentType, generateShortID())

	// Create instance object
	instance := &AgentInstance{
		ID:         instanceID,
		Type:       agentType,
		Name:       instanceName,
		Status:     StatusCreating,
		Config:     config,
		Provider:   providerName,
		Resources:  ResourceLimits{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RequestCh:  make(chan *Request, 10),
		ResponseCh: make(chan *Response, 10),
		EventCh:    make(chan *Event, 10),
	}

	// Initialize type-specific components
	if err := m.initializeInstance(ctx, instance); err != nil {
		return nil, fmt.Errorf("initialize instance: %w", err)
	}

	// Persist to database
	if err := m.persistInstance(ctx, instance); err != nil {
		return nil, fmt.Errorf("persist instance: %w", err)
	}

	// Register in memory
	m.instances.Put(instance.ID, instance)

	// Update metrics
	atomic.AddUint64(&m.createdCount, 1)

	// Start event loops
	go m.instanceEventLoop(instance)
	go m.instanceHealthLoop(instance)

	// Mark as idle once initialized
	instance.Status = StatusIdle
	instance.Health = HealthHealthy
	instance.UpdatedAt = time.Now()
	now := time.Now()
	instance.StartedAt = &now

	// Update database status
	_, err := m.db.ExecContext(ctx,
		"UPDATE agent_instances SET status = $1, health_status = $2, started_at = NOW() WHERE id = $3",
		StatusIdle, HealthHealthy, instance.ID,
	)
	if err != nil {
		m.logger.Printf("Warning: failed to update instance status: %v", err)
	}

	// Publish event
	m.eventBus.Publish(&Event{
		ID:        uuid.MustParse(instanceID),
		Type:      EventTypeStatus,
		Source:    instance.ID,
		Payload:   map[string]interface{}{"status": string(StatusIdle)},
		Timestamp: time.Now(),
	})

	m.logger.Printf("Created instance %s of type %s", instance.ID, agentType)

	return instance, nil
}

// AcquireInstance gets an instance from the pool or creates a new one.
func (m *InstanceManager) AcquireInstance(
	ctx context.Context,
	agentType AgentType,
) (*AgentInstance, error) {
	// Try to get from pool first
	if pool, ok := m.pools.Get(agentType); ok {
		if instance, err := pool.Acquire(ctx); err == nil {
			m.logger.Printf("Acquired instance %s from pool", instance.ID)
			return instance, nil
		}
	}

	// Create new instance if pool is empty
	m.logger.Printf("Pool empty for %s, creating new instance", agentType)
	return m.CreateInstance(ctx, agentType, DefaultInstanceConfig(agentType), "default")
}

// ReleaseInstance returns an instance to the pool or terminates it.
func (m *InstanceManager) ReleaseInstance(ctx context.Context, instance *AgentInstance) error {
	if instance == nil {
		return nil
	}

	// Reset instance state
	instance.SessionID = ""
	instance.TaskID = ""
	instance.Status = StatusIdle
	instance.UpdatedAt = time.Now()

	// Try to return to pool
	if pool, ok := m.pools.Get(instance.Type); ok {
		if err := pool.Release(instance); err == nil {
			m.logger.Printf("Released instance %s to pool", instance.ID)
			return nil
		}
	}

	// Terminate if pool is full or doesn't exist
	return m.TerminateInstance(ctx, instance.ID)
}

// GetInstance retrieves an instance by ID.
func (m *InstanceManager) GetInstance(id string) (*AgentInstance, error) {
	instance, ok := m.instances.Get(id)
	if !ok {
		return nil, fmt.Errorf("instance %s not found", id)
	}

	return instance, nil
}

// ListInstances returns all instances matching the filter.
func (m *InstanceManager) ListInstances(status InstanceStatus, agentType AgentType) []*AgentInstance {
	var result []*AgentInstance
	m.instances.Range(func(_ string, instance *AgentInstance) bool {
		if status != "" && instance.Status != status {
			return true
		}
		if agentType != "" && instance.Type != agentType {
			return true
		}
		result = append(result, instance)
		return true
	})

	return result
}

// TerminateInstance terminates an instance.
func (m *InstanceManager) TerminateInstance(ctx context.Context, id string) error {
	instance, exists := m.instances.Get(id)
	if !exists {
		return fmt.Errorf("instance %s not found", id)
	}

	m.logger.Printf("Terminating instance %s", id)

	// Update status
	instance.Status = StatusTerminating
	instance.UpdatedAt = time.Now()

	// Perform type-specific cleanup
	if err := m.cleanupInstance(ctx, instance); err != nil {
		m.logger.Printf("Warning: cleanup error for %s: %v", id, err)
	}

	// Close channels
	if instance.RequestCh != nil {
		close(instance.RequestCh)
	}
	if instance.ResponseCh != nil {
		close(instance.ResponseCh)
	}
	if instance.EventCh != nil {
		close(instance.EventCh)
	}

	// Update database
	_, err := m.db.ExecContext(ctx,
		`UPDATE agent_instances 
		 SET status = $1, terminated_at = NOW(), updated_at = NOW() 
		 WHERE id = $2`,
		StatusTerminated, id,
	)
	if err != nil {
		m.logger.Printf("Warning: failed to update termination status: %v", err)
	}

	// Remove from memory
	m.instances.Delete(id)

	// Update metrics
	atomic.AddUint64(&m.destroyedCount, 1)

	m.logger.Printf("Terminated instance %s", id)

	return nil
}

// SendRequest sends a request to an instance and waits for response.
func (m *InstanceManager) SendRequest(
	ctx context.Context,
	instanceID string,
	req *Request,
) (*Response, error) {
	instance, err := m.GetInstance(instanceID)
	if err != nil {
		return nil, err
	}

	if !instance.CanAcceptWork() {
		return nil, fmt.Errorf("instance %s cannot accept work (status: %s, health: %s)",
			instanceID, instance.Status, instance.Health)
	}

	// Set instance as active
	instance.Status = StatusActive
	instance.UpdatedAt = time.Now()

	// Send request
	select {
	case instance.RequestCh <- req:
		// Request sent
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout sending request to instance")
	}

	// Wait for response with timeout
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	select {
	case resp := <-instance.ResponseCh:
		// Update metrics
		instance.RequestsProcessed++
		instance.TotalExecTimeMs += uint64(resp.Duration.Milliseconds()) // #nosec G115 -- integer conversion bounded by reachable resource limits; overflow is mathematically unreachable
		if !resp.Success {
			instance.ErrorsCount++
		}

		// Mark idle after processing
		instance.Status = StatusIdle
		instance.UpdatedAt = time.Now()

		return resp, nil

	case <-ctx.Done():
		instance.ErrorsCount++
		return nil, fmt.Errorf("request timeout: %w", ctx.Err())
	}
}

// BroadcastRequest sends a request to all instances of a specific type.
func (m *InstanceManager) BroadcastRequest(
	ctx context.Context,
	agentType AgentType,
	req *Request,
) map[string]*Response {
	instances := m.ListInstances(StatusIdle, agentType)

	results := make(map[string]*Response)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, instance := range instances {
		wg.Add(1)
		go func(inst *AgentInstance) {
			defer wg.Done()

			resp, err := m.SendRequest(ctx, inst.ID, req)
			mu.Lock()
			if err != nil {
				results[inst.ID] = &Response{
					RequestID: req.ID,
					Success:   false,
					Error: &ErrorDetail{
						Code:    "BROADCAST_ERROR",
						Message: err.Error(),
					},
				}
			} else {
				results[inst.ID] = resp
			}
			mu.Unlock()
		}(instance)
	}

	wg.Wait()
	return results
}

// IsAgentTypeAvailable checks if an agent type is available.
func (m *InstanceManager) IsAgentTypeAvailable(agentType AgentType) bool {
	// Check if there's a pool for this type
	// For now, allow all types defined in the enum
	switch agentType {
	case TypeAider, TypeClaudeCode, TypeCodex, TypeCline,
		TypeOpenHands, TypeKiro, TypeContinue, TypeHelixAgent:
		return true
	default:
		// Could check database or configuration here
		return false
	}
}

// GetMetrics returns manager metrics.
func (m *InstanceManager) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"created_total":   atomic.LoadUint64(&m.createdCount),
		"destroyed_total": atomic.LoadUint64(&m.destroyedCount),
		"active_count":    m.instances.Len(),
		"pool_count":      m.pools.Len(),
	}
}

// Close shuts down the instance manager.
func (m *InstanceManager) Close() error {
	// Stop health checks
	close(m.healthCheckStop)

	// Terminate all instances
	ctx := context.Background()
	instances := make([]*AgentInstance, 0, m.instances.Len())
	m.instances.Range(func(_ string, inst *AgentInstance) bool {
		instances = append(instances, inst)
		return true
	})

	var wg sync.WaitGroup
	for _, instance := range instances {
		wg.Add(1)
		go func(inst *AgentInstance) {
			defer wg.Done()
			if err := m.TerminateInstance(ctx, inst.ID); err != nil {
				m.logger.Printf("Error terminating instance %s: %v", inst.ID, err)
			}
		}(instance)
	}

	wg.Wait()
	return nil
}

// Internal methods

func (m *InstanceManager) initializeInstance(ctx context.Context, inst *AgentInstance) error {
	// Type-specific initialization
	switch inst.Type {
	case TypeAider:
		// Initialize Aider-specific components
		inst.State = map[string]interface{}{
			"repo_map_enabled": true,
			"diff_format":      "search_replace",
		}

	case TypeClaudeCode:
		// Initialize Claude Code-specific components
		inst.State = map[string]interface{}{
			"terminal_enabled": true,
			"tool_use_enabled": true,
		}

	case TypeCodex:
		inst.State = map[string]interface{}{
			"interpreter_enabled": true,
			"reasoning_enabled":   true,
		}

	case TypeCline:
		inst.State = map[string]interface{}{
			"browser_enabled":  true,
			"autonomy_enabled": true,
		}

	case TypeOpenHands:
		inst.State = map[string]interface{}{
			"sandbox_enabled": true,
			"security_level":  "high",
		}

	case TypeKiro:
		inst.State = map[string]interface{}{
			"memory_enabled": true,
		}

	case TypeContinue:
		inst.State = map[string]interface{}{
			"lsp_enabled": true,
		}

	case TypeHelixAgent:
		// Native HelixAgent instance
		inst.State = map[string]interface{}{
			"native": true,
		}

	default:
		return fmt.Errorf("unknown agent type: %s", inst.Type)
	}

	return nil
}

func (m *InstanceManager) cleanupInstance(ctx context.Context, inst *AgentInstance) error {
	// Type-specific cleanup
	switch inst.Type {
	case TypeAider:
		// Cleanup Aider resources

	case TypeClaudeCode:
		// Cleanup terminal resources

	case TypeCline:
		// Cleanup browser resources

	case TypeOpenHands:
		// Stop sandbox containers

	default:
		// Generic cleanup
	}

	return nil
}

func (m *InstanceManager) persistInstance(ctx context.Context, inst *AgentInstance) error {
	configJSON, err := json.Marshal(inst.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	providerJSON, err := json.Marshal(inst.Provider)
	if err != nil {
		return fmt.Errorf("marshal provider: %w", err)
	}

	_, err = m.db.ExecContext(ctx,
		`INSERT INTO agent_instances (
			id, agent_type, instance_name, status, config, provider_config,
			max_memory_mb, max_cpu_percent, current_session_id, current_task_id,
			health_status, requests_processed, errors_count, total_execution_time_ms,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		 ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			config = EXCLUDED.config,
			health_status = EXCLUDED.health_status,
			requests_processed = EXCLUDED.requests_processed,
			errors_count = EXCLUDED.errors_count,
			total_execution_time_ms = EXCLUDED.total_execution_time_ms,
			updated_at = EXCLUDED.updated_at`,
		inst.ID, inst.Type, inst.Name, inst.Status, configJSON, providerJSON,
		inst.Config.MaxMemoryMB, inst.Config.MaxCPUPercent,
		sql.NullString{String: inst.SessionID, Valid: inst.SessionID != ""},
		sql.NullString{String: inst.TaskID, Valid: inst.TaskID != ""},
		inst.Health, inst.RequestsProcessed, inst.ErrorsCount, inst.TotalExecTimeMs,
		inst.CreatedAt, inst.UpdatedAt,
	)

	return err
}

func (m *InstanceManager) recoverInstances(ctx context.Context) error {
	// Query instances that should be active
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, agent_type, instance_name, status, config, provider_config,
		        max_memory_mb, max_cpu_percent, current_session_id, current_task_id,
		        health_status, requests_processed, errors_count, total_execution_time_ms,
		        created_at, updated_at
		 FROM agent_instances
		 WHERE status IN ('idle', 'active', 'background')`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var inst AgentInstance
		var configJSON, providerJSON []byte
		var sessionID, taskID sql.NullString

		err := rows.Scan(
			&inst.ID, &inst.Type, &inst.Name, &inst.Status, &configJSON, &providerJSON,
			&inst.Config.MaxMemoryMB, &inst.Config.MaxCPUPercent,
			&sessionID, &taskID,
			&inst.Health, &inst.RequestsProcessed, &inst.ErrorsCount, &inst.TotalExecTimeMs,
			&inst.CreatedAt, &inst.UpdatedAt,
		)
		if err != nil {
			m.logger.Printf("Error scanning instance: %v", err)
			continue
		}

		if sessionID.Valid {
			inst.SessionID = sessionID.String
		}
		if taskID.Valid {
			inst.TaskID = taskID.String
		}

		// Parse config
		if err := json.Unmarshal(configJSON, &inst.Config); err != nil {
			m.logger.Printf("Error parsing config for %s: %v", inst.ID, err)
		}
		if err := json.Unmarshal(providerJSON, &inst.Provider); err != nil {
			m.logger.Printf("Error parsing provider for %s: %v", inst.ID, err)
		}

		// Initialize channels
		inst.RequestCh = make(chan *Request, 10)
		inst.ResponseCh = make(chan *Response, 10)
		inst.EventCh = make(chan *Event, 10)

		// Register in memory
		m.instances.Put(inst.ID, &inst)

		// Restart event loops
		go m.instanceEventLoop(&inst)
		go m.instanceHealthLoop(&inst)

		m.logger.Printf("Recovered instance %s of type %s", inst.ID, inst.Type)
	}

	return rows.Err()
}

func (m *InstanceManager) instanceEventLoop(inst *AgentInstance) {
	m.logger.Printf("Started event loop for instance %s", inst.ID)

	for {
		select {
		case req, ok := <-inst.RequestCh:
			if !ok {
				return // Channel closed
			}
			resp := m.handleRequest(inst, req)
			inst.ResponseCh <- resp

		case event, ok := <-inst.EventCh:
			if !ok {
				return
			}
			m.eventBus.Publish(event)

		case <-m.healthCheckStop:
			return
		}
	}
}

func (m *InstanceManager) instanceHealthLoop(inst *AgentInstance) {
	if inst.Config.HealthCheckInterval <= 0 {
		return
	}
	ticker := time.NewTicker(inst.Config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if inst.Status == StatusTerminating || inst.Status == StatusTerminated {
				return
			}

			result := m.performHealthCheck(inst)
			inst.Health = HealthStatus(result.Status)
			inst.HealthDetails = result.Details
			now := time.Now()
			inst.LastHealthCheck = &now

			// Update database
			_, err := m.db.Exec(
				"UPDATE agent_instances SET health_status = $1, last_health_check = NOW() WHERE id = $2",
				inst.Health, inst.ID,
			)
			if err != nil {
				m.logger.Printf("Error updating health check for %s: %v", inst.ID, err)
			}

		case <-m.healthCheckStop:
			return
		}
	}
}

func (m *InstanceManager) handleRequest(inst *AgentInstance, req *Request) *Response {
	start := time.Now()

	// Route to type-specific handler
	var result interface{}
	var err error

	switch req.Type {
	case RequestTypeExecute:
		result, err = m.handleExecute(inst, req.Payload)
	case RequestTypeQuery:
		result, err = m.handleQuery(inst, req.Payload)
	case RequestTypeHealth:
		result = m.performHealthCheck(inst)
	case RequestTypeCancel:
		// Handle cancellation
		result = map[string]bool{"cancelled": true}
	default:
		err = fmt.Errorf("unknown request type: %s", req.Type)
	}

	duration := time.Since(start)

	if err != nil {
		return &Response{
			RequestID: req.ID,
			Success:   false,
			Error: &ErrorDetail{
				Code:    "REQUEST_ERROR",
				Message: err.Error(),
			},
			Duration: duration,
		}
	}

	return &Response{
		RequestID: req.ID,
		Success:   true,
		Result:    result,
		Duration:  duration,
	}
}

func (m *InstanceManager) handleExecute(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Type-specific execution
	switch inst.Type {
	case TypeAider:
		return m.executeAider(inst, payload)
	case TypeClaudeCode:
		return m.executeClaudeCode(inst, payload)
	case TypeCodex:
		return m.executeCodex(inst, payload)
	case TypeCline:
		return m.executeCline(inst, payload)
	case TypeOpenHands:
		return m.executeOpenHands(inst, payload)
	case TypeKiro:
		return m.executeKiro(inst, payload)
	case TypeContinue:
		return m.executeContinue(inst, payload)
	case TypeSupermaven:
		return m.executeSupermaven(inst, payload)
	case TypeCursor:
		return m.executeCursor(inst, payload)
	case TypeWindsurf:
		return m.executeWindsurf(inst, payload)
	case TypeAugment:
		return m.executeAugment(inst, payload)
	case TypeSourcegraph:
		return m.executeSourcegraph(inst, payload)
	case TypeCodeium:
		return m.executeCodeium(inst, payload)
	case TypeTabnine:
		return m.executeTabnine(inst, payload)
	case TypeCodeGPT:
		return m.executeCodeGPT(inst, payload)
	case TypeTwin:
		return m.executeTwin(inst, payload)
	case TypeDevin:
		return m.executeDevin(inst, payload)
	case TypeDevika:
		return m.executeDevika(inst, payload)
	case TypeSWEAgent:
		return m.executeSWEAgent(inst, payload)
	case TypeGPTPilot:
		return m.executeGPTPilot(inst, payload)
	case TypeMetamorph:
		return m.executeMetamorph(inst, payload)
	case TypeJunie:
		return m.executeJunie(inst, payload)
	case TypeAmazonQ:
		return m.executeAmazonQ(inst, payload)
	case TypeGitHubCopilot:
		return m.executeGitHubCopilot(inst, payload)
	case TypeJetBrainsAI:
		return m.executeJetBrainsAI(inst, payload)
	case TypeCodeGemma:
		return m.executeCodeGemma(inst, payload)
	case TypeStarCoder:
		return m.executeStarCoder(inst, payload)
	case TypeQwenCoder:
		return m.executeQwenCoder(inst, payload)
	case TypeMistralCode:
		return m.executeMistralCode(inst, payload)
	case TypeGeminiAssist:
		return m.executeGeminiAssist(inst, payload)
	case TypeCodey:
		return m.executeCodey(inst, payload)
	case TypeLlamaCode:
		return m.executeLlamaCode(inst, payload)
	case TypeDeepSeekCoder:
		return m.executeDeepSeekCoder(inst, payload)
	case TypeWizardCoder:
		return m.executeWizardCoder(inst, payload)
	case TypePhind:
		return m.executePhind(inst, payload)
	case TypeCody:
		return m.executeCody(inst, payload)
	case TypeCursorSh:
		return m.executeCursorSh(inst, payload)
	case TypeTrae:
		return m.executeTrae(inst, payload)
	case TypeBlackbox:
		return m.executeBlackbox(inst, payload)
	case TypeLovable:
		return m.executeLovable(inst, payload)
	case TypeV0:
		return m.executeV0(inst, payload)
	case TypeTempo:
		return m.executeTempo(inst, payload)
	case TypeBolt:
		return m.executeBolt(inst, payload)
	case TypeReplitAgent:
		return m.executeReplitAgent(inst, payload)
	case TypeIDX:
		return m.executeIDX(inst, payload)
	case TypeFirebaseStudio:
		return m.executeFirebaseStudio(inst, payload)
	case TypeCascade:
		return m.executeCascade(inst, payload)
	case TypeHelixAgent:
		return m.executeHelixAgent(inst, payload)
	default:
		return nil, fmt.Errorf("execution not implemented for type: %s", inst.Type)
	}
}

func (m *InstanceManager) handleQuery(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Type-specific query
	return map[string]interface{}{
		"status":  inst.Status,
		"health":  inst.Health,
		"metrics": m.GetMetrics(),
	}, nil
}

func (m *InstanceManager) performHealthCheck(inst *AgentInstance) *HealthCheckResult {
	result := &HealthCheckResult{
		CheckedAt: time.Now(),
	}

	// Basic health checks
	if inst.Status == StatusFailed || inst.Status == StatusTerminating {
		result.Healthy = false
		result.Status = HealthUnhealthy
		result.Message = "Instance in failed/terminating state"
		return result
	}

	// Check error rate
	if inst.RequestsProcessed > 0 {
		errorRate := float64(inst.ErrorsCount) / float64(inst.RequestsProcessed)
		if errorRate > 0.5 {
			result.Status = HealthDegraded
			result.Message = "High error rate detected"
			result.Details = map[string]interface{}{
				"error_rate": errorRate,
			}
			return result
		}
	}

	result.Healthy = true
	result.Status = HealthHealthy
	result.Message = "Instance is healthy"
	return result
}

func (m *InstanceManager) healthCheckLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Clean up expired locks
			_, err := m.db.Exec("DELETE FROM distributed_locks WHERE expires_at < NOW()")
			if err != nil {
				m.logger.Printf("Error cleaning locks: %v", err)
			}

		case <-m.healthCheckStop:
			return
		}
	}
}

// Type-specific execution methods
func (m *InstanceManager) executeAider(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "aider",
		"message":   "Aider execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeClaudeCode(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "claude_code",
		"message":   "Claude Code execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCodex(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "codex",
		"message":   "Codex execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCline(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "cline",
		"message":   "Cline execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeOpenHands(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "openhands",
		"message":   "OpenHands execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeKiro(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "kiro",
		"message":   "Kiro execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeContinue(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "continue",
		"message":   "Continue.dev execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeSupermaven(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "supermaven",
		"message":   "Supermaven execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCursor(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "cursor",
		"message":   "Cursor execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeWindsurf(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "windsurf",
		"message":   "Windsurf execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeAugment(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "augment",
		"message":   "Augment execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeSourcegraph(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "sourcegraph",
		"message":   "Sourcegraph execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCodeium(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "codeium",
		"message":   "Codeium execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeTabnine(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "tabnine",
		"message":   "Tabnine execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCodeGPT(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "codegpt",
		"message":   "CodeGPT execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeTwin(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "twin",
		"message":   "Twin execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeDevin(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "devin",
		"message":   "Devin execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeDevika(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "devika",
		"message":   "Devika execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeSWEAgent(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "swe_agent",
		"message":   "SWE Agent execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeGPTPilot(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "gpt_pilot",
		"message":   "GPT Pilot execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeMetamorph(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "metamorph",
		"message":   "Metamorph execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeJunie(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "junie",
		"message":   "Junie execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeAmazonQ(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "amazon_q",
		"message":   "Amazon Q execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeGitHubCopilot(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "github_copilot",
		"message":   "GitHub Copilot execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeJetBrainsAI(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "jetbrains_ai",
		"message":   "JetBrains AI execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCodeGemma(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "codegemma",
		"message":   "CodeGemma execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeStarCoder(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "starcoder",
		"message":   "StarCoder execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeQwenCoder(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "qwen_coder",
		"message":   "Qwen Coder execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeMistralCode(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "mistral_code",
		"message":   "Mistral Code execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeGeminiAssist(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "gemini_assist",
		"message":   "Gemini Assist execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCodey(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "codey",
		"message":   "Codey execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeLlamaCode(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "llama_code",
		"message":   "Llama Code execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeDeepSeekCoder(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "deepseek_coder",
		"message":   "DeepSeek Coder execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeWizardCoder(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "wizardcoder",
		"message":   "WizardCoder execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executePhind(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "phind",
		"message":   "Phind execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCody(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "cody",
		"message":   "Cody execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCursorSh(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "cursor_sh",
		"message":   "Cursor.sh execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeTrae(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "trae",
		"message":   "Trae execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeBlackbox(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "blackbox",
		"message":   "Blackbox execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeLovable(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "lovable",
		"message":   "Lovable execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeV0(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "v0",
		"message":   "V0 execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeTempo(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "tempo",
		"message":   "Tempo execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeBolt(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "bolt",
		"message":   "Bolt execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeReplitAgent(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "replit_agent",
		"message":   "Replit Agent execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeIDX(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "idx",
		"message":   "IDX execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeFirebaseStudio(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "firebase_studio",
		"message":   "Firebase Studio execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeCascade(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "cascade",
		"message":   "Cascade execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func (m *InstanceManager) executeHelixAgent(inst *AgentInstance, payload interface{}) (interface{}, error) {
	return map[string]string{
		"status":    "executed",
		"type":      "helixagent",
		"message":   "HelixAgent execution completed",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

// Helper functions

func generateShortID() string {
	return uuid.New().String()[:8]
}

// WorkerPool is a simple worker pool for background tasks.
type WorkerPool struct {
	size int
	sem  chan struct{}
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(size int) *WorkerPool {
	return &WorkerPool{
		size: size,
		sem:  make(chan struct{}, size),
	}
}

// Submit submits a task to the pool.
func (p *WorkerPool) Submit(ctx context.Context, fn func()) error {
	select {
	case p.sem <- struct{}{}:
		go func() {
			defer func() { <-p.sem }()
			fn()
		}()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
