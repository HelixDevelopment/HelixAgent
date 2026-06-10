// Package clis provides CLI agent integration for HelixAgent.
package clis

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
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
//
// CONST-035 §c: db may be nil for in-memory deployments. The
// persistInstance and recoverInstances methods both nil-check m.db
// and skip persistence/recovery when no DB is configured. The manager
// still tracks instances in memory via m.instances (safe.Store) and
// supports the full /v1/ensemble/sessions lifecycle without durability.
// Closes #ensemble-instance-manager-wiring tracking ticket.
func NewInstanceManager(db *sql.DB, logger *log.Logger) (*InstanceManager, error) {
	if logger == nil {
		logger = log.Default()
	}

	im := &InstanceManager{
		db:              db, // may be nil — see comment above
		logger:          logger,
		pools:           safe.NewStore[AgentType, *InstancePool](),
		instances:       safe.NewStore[string, *AgentInstance](),
		eventBus:        NewEventBus(),
		workerPool:      NewWorkerPool(100),
		healthCheckStop: make(chan struct{}),
	}

	// Start background health checks
	go im.healthCheckLoop()

	// Recover existing instances from database (no-op when db == nil)
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

	// Update database status (CONST-035 §c: nil-guard for in-memory mode)
	if m.db != nil {
		_, err := m.db.ExecContext(ctx,
			"UPDATE agent_instances SET status = $1, health_status = $2, started_at = NOW() WHERE id = $3",
			StatusIdle, HealthHealthy, instance.ID,
		)
		if err != nil {
			m.logger.Printf("Warning: failed to update instance status: %v", err)
		}
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

	// Update database (CONST-035 §c: nil-guard for in-memory mode)
	if m.db == nil {
		return nil
	}
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

// IsAgentTypeAvailable reports whether an agent type can actually run on THIS
// host right now. It performs a REAL per-type binary resolution (exec.LookPath,
// honoring the HELIX_AGENT_BIN_<TYPE> test override) driven by the shared
// agentDefaultCommand table — the single source of truth shared with the
// execute* dispatch (D-11). An agent whose table entry has an empty command
// (no standalone non-interactive CLI) is genuinely unavailable; this replaces
// the prior "For now, allow all types" hard-coded enum allowlist that falsely
// reported stub-only agents as available (BLUFF / CONST-035).
func (m *InstanceManager) IsAgentTypeAvailable(agentType AgentType) bool {
	command, ok := agentDefaultCommand[agentType]
	if !ok || command == "" {
		return false // no real CLI mapping → genuinely unavailable
	}
	_, err := resolveAgentBinary(agentType, command)
	return err == nil
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
	// CONST-035 §c: when no DB is wired (test/dev mode), skip persistence
	// rather than panicking on m.db.ExecContext nil deref. The instance is
	// still tracked in-memory by m.instances.Put() in CreateInstance, so
	// /v1/ensemble/sessions stays usable; durability is sacrificed when
	// no DB is configured. Tracking: #ensemble-db-wiring.
	if m.db == nil {
		return nil
	}

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
	// CONST-035 §c: skip recovery when no DB is wired (sibling of the
	// persistInstance nil-guard). No persisted state means nothing to
	// recover; constructor logs a warning rather than crashing.
	if m.db == nil {
		return nil
	}

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

			// Update database (CONST-035 §c: nil-guard for in-memory mode)
			if m.db != nil {
				_, err := m.db.Exec(
					"UPDATE agent_instances SET health_status = $1, last_health_check = NOW() WHERE id = $2",
					inst.Health, inst.ID,
				)
				if err != nil {
					m.logger.Printf("Error updating health check for %s: %v", inst.ID, err)
				}
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
			// Clean up expired locks (CONST-035 §c: nil-guard for in-memory mode)
			if m.db != nil {
				_, err := m.db.Exec("DELETE FROM distributed_locks WHERE expires_at < NOW()")
				if err != nil {
					m.logger.Printf("Error cleaning locks: %v", err)
				}
			}

		case <-m.healthCheckStop:
			return
		}
	}
}

// Type-specific execution methods
//
// Each method below resolves the agent's real CLI binary and exec's it with the
// agent's documented non-interactive flags (§11.4.99), returning the binary's
// actual stdout/stderr. When the binary is absent the method returns an honest
// error — NEVER a fabricated "<Agent> execution completed" template
// (BLUFF-003 / CONST-035).

// extractPrompt pulls the user prompt out of an execute payload.
func extractPrompt(payload interface{}) string {
	if m, ok := payload.(map[string]interface{}); ok {
		if p, ok := m["prompt"].(string); ok {
			return p
		}
		if p, ok := m["message"].(string); ok {
			return p
		}
	}
	if s, ok := payload.(string); ok {
		return s
	}
	return ""
}

// resolveAgentBinary locates an agent's CLI executable. Tests may inject a fake
// binary via the per-agent environment variable `HELIX_AGENT_BIN_<TYPE>`
// (uppercased CLIAgentType, non-alphanumeric → '_'); otherwise the named command
// is resolved on PATH. Returns an honest error when no binary is available.
func resolveAgentBinary(typ CLIAgentType, command string) (string, error) {
	envKey := "HELIX_AGENT_BIN_" + strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - 32
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, string(typ))
	if override := os.Getenv(envKey); override != "" {
		if _, err := exec.LookPath(override); err != nil {
			return "", fmt.Errorf("%s binary override %q not executable: %w", typ, override, err)
		}
		return override, nil
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("%s CLI %q not found on PATH: %w", typ, command, err)
	}
	return path, nil
}

// runCLIAgent exec's a CLI agent binary non-interactively and returns a result
// map carrying the binary's REAL combined output plus its exit code. The agent
// binary is resolved via resolveAgentBinary; when absent an honest error is
// returned. The work directory and timeout come from the instance config.
func (m *InstanceManager) runCLIAgent(inst *AgentInstance, typ CLIAgentType, command string, args []string, payload interface{}) (interface{}, error) {
	bin, err := resolveAgentBinary(typ, command)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if inst != nil && inst.Config.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, inst.Config.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	if inst != nil && inst.Config.WorkingDir != "" {
		cmd.Dir = inst.Config.WorkingDir
	}

	out, runErr := cmd.CombinedOutput()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	result := map[string]string{
		"status":    "executed",
		"type":      string(typ),
		"message":   strings.TrimSpace(string(out)),
		"exit_code": fmt.Sprintf("%d", exitCode),
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if runErr != nil {
		// A non-zero exit / spawn failure is surfaced as a real error with the
		// binary's actual output — never masked as success.
		return result, fmt.Errorf("%s execution failed (exit %d): %w (output: %s)", typ, exitCode, runErr, strings.TrimSpace(string(out)))
	}
	return result, nil
}

// agentDefaultCommand is the single source of truth mapping each CLIAgentType to
// the default name of its non-interactive CLI binary, shared by the execute*
// dispatch and IsAgentTypeAvailable (D-11). An EMPTY command means the agent has
// no standalone headless CLI (IDE extension / hosted-web agent / raw model name /
// a project-owned binary whose name is not yet verified) — such agents are never
// "available" and their dispatch returns an honest error rather than a fabricated
// success (§11.4.6 no-guessing; BLUFF-003). Commands for the confirmed entries are
// cross-referenced to docs/research/2026-06-10-sdk-cli-currency.md (§11.4.99).
var agentDefaultCommand = map[CLIAgentType]string{
	// Already-real (SP4) — real non-interactive CLIs.
	TypeAider:      "aider",
	TypeClaudeCode: "claude",
	TypeCodex:      "codex",
	TypeCline:      "cline",
	TypeOpenHands:  "openhands",
	// SP4-cont D-12 confirmed-binary conversions (§11.4.99).
	TypeQwenCoder:     "qwen",
	TypeGitHubCopilot: "copilot",
	TypeGeminiAssist:  "gemini",
	// No standalone headless CLI — honest-error class (empty command).
	TypeKiro:           "",
	TypeContinue:       "",
	TypeSupermaven:     "",
	TypeCursor:         "",
	TypeWindsurf:       "",
	TypeAugment:        "",
	TypeSourcegraph:    "",
	TypeCodeium:        "",
	TypeTabnine:        "",
	TypeCodeGPT:        "",
	TypeTwin:           "",
	TypeDevin:          "",
	TypeDevika:         "",
	TypeSWEAgent:       "",
	TypeGPTPilot:       "",
	TypeMetamorph:      "",
	TypeJunie:          "",
	TypeAmazonQ:        "",
	TypeJetBrainsAI:    "",
	TypeCodeGemma:      "",
	TypeStarCoder:      "",
	TypeMistralCode:    "",
	TypeCodey:          "",
	TypeLlamaCode:      "",
	TypeDeepSeekCoder:  "",
	TypeWizardCoder:    "",
	TypePhind:          "",
	TypeCody:           "",
	TypeCursorSh:       "",
	TypeTrae:           "",
	TypeBlackbox:       "",
	TypeLovable:        "",
	TypeV0:             "",
	TypeTempo:          "",
	TypeBolt:           "",
	TypeReplitAgent:    "",
	TypeIDX:            "",
	TypeFirebaseStudio: "",
	TypeCascade:        "",
	TypeHelixAgent:     "",
}

// noHeadlessCLIError returns the honest error emitted for agents in
// agentDefaultCommand whose command is empty: they have no standalone
// non-interactive CLI binary, so there is nothing to exec. Returning this error
// instead of a fabricated "<Agent> execution completed" success is the
// constitutionally-correct disposition (honest error > fake success; BLUFF-003 /
// CONST-035 / §11.4.6). The agent's IDE/extension/web/API surface — not a headless
// CLI — is the supported integration path.
func noHeadlessCLIError(typ CLIAgentType) error {
	return fmt.Errorf("%s has no non-interactive CLI binary on this host; "+
		"it is integrated via its IDE extension / hosted web service / API, not a headless command", typ)
}

func (m *InstanceManager) executeAider(inst *AgentInstance, payload interface{}) (interface{}, error) {
	prompt := extractPrompt(payload)
	// aider non-interactive: --message "<prompt>" --no-auto-commits --yes
	return m.runCLIAgent(inst, TypeAider, "aider", []string{"--message", prompt, "--no-auto-commits", "--yes"}, payload)
}

func (m *InstanceManager) executeClaudeCode(inst *AgentInstance, payload interface{}) (interface{}, error) {
	prompt := extractPrompt(payload)
	// claude non-interactive (§11.4.99): claude -p "<prompt>" --output-format json
	return m.runCLIAgent(inst, TypeClaudeCode, "claude", []string{"-p", prompt, "--output-format", "json"}, payload)
}

func (m *InstanceManager) executeCodex(inst *AgentInstance, payload interface{}) (interface{}, error) {
	prompt := extractPrompt(payload)
	// codex non-interactive (§11.4.99): codex exec --json "<prompt>"
	return m.runCLIAgent(inst, TypeCodex, "codex", []string{"exec", "--json", prompt}, payload)
}

func (m *InstanceManager) executeCline(inst *AgentInstance, payload interface{}) (interface{}, error) {
	prompt := extractPrompt(payload)
	// cline non-interactive: cline task "<prompt>"
	return m.runCLIAgent(inst, TypeCline, "cline", []string{"task", prompt}, payload)
}

func (m *InstanceManager) executeOpenHands(inst *AgentInstance, payload interface{}) (interface{}, error) {
	prompt := extractPrompt(payload)
	// openhands non-interactive: openhands -t "<prompt>"
	return m.runCLIAgent(inst, TypeOpenHands, "openhands", []string{"-t", prompt}, payload)
}

func (m *InstanceManager) executeKiro(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// AWS Kiro is an IDE — no standalone headless CLI. Honest error, never a
	// fabricated success (BLUFF-003 / §11.4.6).
	return nil, noHeadlessCLIError(TypeKiro)
}

func (m *InstanceManager) executeContinue(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Continue.dev is a VS Code / JetBrains extension — no headless CLI.
	return nil, noHeadlessCLIError(TypeContinue)
}

func (m *InstanceManager) executeSupermaven(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Supermaven is an editor extension — no headless CLI.
	return nil, noHeadlessCLIError(TypeSupermaven)
}

func (m *InstanceManager) executeCursor(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Cursor is an IDE — no standalone headless agent CLI.
	return nil, noHeadlessCLIError(TypeCursor)
}

func (m *InstanceManager) executeWindsurf(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Windsurf (Codeium) is an IDE — no headless CLI.
	return nil, noHeadlessCLIError(TypeWindsurf)
}

func (m *InstanceManager) executeAugment(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Augment is an IDE extension — no headless CLI.
	return nil, noHeadlessCLIError(TypeAugment)
}

func (m *InstanceManager) executeSourcegraph(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Sourcegraph's `src` CLI is code-search, not a verified agent runner —
	// research-needed before any exec (§11.4.99 / §11.4.6). Honest error for now.
	return nil, noHeadlessCLIError(TypeSourcegraph)
}

func (m *InstanceManager) executeCodeium(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Codeium is an IDE extension — no headless CLI.
	return nil, noHeadlessCLIError(TypeCodeium)
}

func (m *InstanceManager) executeTabnine(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Tabnine is an IDE extension — no headless CLI.
	return nil, noHeadlessCLIError(TypeTabnine)
}

func (m *InstanceManager) executeCodeGPT(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// CodeGPT is an IDE extension — no headless CLI.
	return nil, noHeadlessCLIError(TypeCodeGPT)
}

func (m *InstanceManager) executeTwin(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Twin is an IDE-integrated agent — no headless CLI.
	return nil, noHeadlessCLIError(TypeTwin)
}

func (m *InstanceManager) executeDevin(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Devin (Cognition) is a hosted web agent — no local headless CLI.
	return nil, noHeadlessCLIError(TypeDevin)
}

func (m *InstanceManager) executeDevika(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Devika is primarily a server + web UI; non-interactive CLI form unverified —
	// research-needed (§11.4.99 / §11.4.6). Honest error for now.
	return nil, noHeadlessCLIError(TypeDevika)
}

func (m *InstanceManager) executeSWEAgent(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// SWE-agent ships a `sweagent run` CLI, but exact current non-interactive flags
	// are unverified — research-needed (§11.4.99 / §11.4.6). Honest error for now.
	return nil, noHeadlessCLIError(TypeSWEAgent)
}

func (m *InstanceManager) executeGPTPilot(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// GPT-Pilot has a CLI entrypoint, but its non-interactive form is unverified —
	// research-needed (§11.4.99 / §11.4.6). Honest error for now.
	return nil, noHeadlessCLIError(TypeGPTPilot)
}

func (m *InstanceManager) executeMetamorph(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Metamorph is an IDE-integrated agent — no headless CLI.
	return nil, noHeadlessCLIError(TypeMetamorph)
}

func (m *InstanceManager) executeJunie(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Junie is a JetBrains-integrated agent — no headless CLI.
	return nil, noHeadlessCLIError(TypeJunie)
}

func (m *InstanceManager) executeAmazonQ(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Amazon Q Developer ships a `q` CLI, but its non-interactive prompt + output
	// flags are unverified — research-needed (§11.4.99 / §11.4.6). Honest error.
	return nil, noHeadlessCLIError(TypeAmazonQ)
}

func (m *InstanceManager) executeGitHubCopilot(inst *AgentInstance, payload interface{}) (interface{}, error) {
	prompt := extractPrompt(payload)
	// GitHub Copilot CLI non-interactive (§11.4.99): copilot -p "<prompt>" -s
	// (no JSON output format exists; -s emits clean stdout — research negative).
	return m.runCLIAgent(inst, TypeGitHubCopilot, agentDefaultCommand[TypeGitHubCopilot], []string{"-p", prompt, "-s"}, payload)
}

func (m *InstanceManager) executeJetBrainsAI(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// JetBrains AI Assistant is IDE-integrated — no headless CLI.
	return nil, noHeadlessCLIError(TypeJetBrainsAI)
}

func (m *InstanceManager) executeCodeGemma(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// CodeGemma is a MODEL, not a CLI agent — reachable only via an ollama/API
	// runner; the runner choice is an operator decision (§11.4.101). Honest error.
	return nil, noHeadlessCLIError(TypeCodeGemma)
}

func (m *InstanceManager) executeStarCoder(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// StarCoder is a MODEL, not a CLI agent — operator decision on the runner.
	return nil, noHeadlessCLIError(TypeStarCoder)
}

func (m *InstanceManager) executeQwenCoder(inst *AgentInstance, payload interface{}) (interface{}, error) {
	prompt := extractPrompt(payload)
	// qwen non-interactive (§11.4.99): qwen -p "<prompt>" --output-format json
	return m.runCLIAgent(inst, TypeQwenCoder, agentDefaultCommand[TypeQwenCoder], []string{"-p", prompt, "--output-format", "json"}, payload)
}

func (m *InstanceManager) executeMistralCode(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Mistral Code is a MODEL family, not a verified CLI agent — operator decision.
	return nil, noHeadlessCLIError(TypeMistralCode)
}

func (m *InstanceManager) executeGeminiAssist(inst *AgentInstance, payload interface{}) (interface{}, error) {
	prompt := extractPrompt(payload)
	// gemini non-interactive (§11.4.99): gemini -p "<prompt>" --output-format json
	return m.runCLIAgent(inst, TypeGeminiAssist, agentDefaultCommand[TypeGeminiAssist], []string{"-p", prompt, "--output-format", "json"}, payload)
}

func (m *InstanceManager) executeCodey(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Codey is a Google MODEL, not a CLI agent — no headless CLI.
	return nil, noHeadlessCLIError(TypeCodey)
}

func (m *InstanceManager) executeLlamaCode(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Code Llama is a MODEL — `ollama run codellama` is the realistic runner;
	// that is an operator decision (§11.4.101). Honest error for now.
	return nil, noHeadlessCLIError(TypeLlamaCode)
}

func (m *InstanceManager) executeDeepSeekCoder(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// DeepSeek-Coder is a MODEL, not a CLI agent — operator decision on the runner.
	return nil, noHeadlessCLIError(TypeDeepSeekCoder)
}

func (m *InstanceManager) executeWizardCoder(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// WizardCoder is a MODEL, not a CLI agent — operator decision on the runner.
	return nil, noHeadlessCLIError(TypeWizardCoder)
}

func (m *InstanceManager) executePhind(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Phind is a web/IDE product — no headless CLI.
	return nil, noHeadlessCLIError(TypePhind)
}

func (m *InstanceManager) executeCody(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Sourcegraph Cody's historical `cody` CLI / current headless form is
	// unverified — research-needed (§11.4.99 / §11.4.6). Honest error for now.
	return nil, noHeadlessCLIError(TypeCody)
}

func (m *InstanceManager) executeCursorSh(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Cursor.sh is a hosted web product — no local headless CLI.
	return nil, noHeadlessCLIError(TypeCursorSh)
}

func (m *InstanceManager) executeTrae(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Trae (ByteDance) is an IDE — no headless CLI.
	return nil, noHeadlessCLIError(TypeTrae)
}

func (m *InstanceManager) executeBlackbox(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Blackbox is an IDE extension / web product — no headless CLI.
	return nil, noHeadlessCLIError(TypeBlackbox)
}

func (m *InstanceManager) executeLovable(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Lovable is a hosted web product — no local headless CLI.
	return nil, noHeadlessCLIError(TypeLovable)
}

func (m *InstanceManager) executeV0(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// V0 (Vercel) is a hosted web product — no local headless CLI.
	return nil, noHeadlessCLIError(TypeV0)
}

func (m *InstanceManager) executeTempo(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Tempo is a hosted web product — no local headless CLI.
	return nil, noHeadlessCLIError(TypeTempo)
}

func (m *InstanceManager) executeBolt(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Bolt (StackBlitz) is a hosted web product — no local headless CLI.
	return nil, noHeadlessCLIError(TypeBolt)
}

func (m *InstanceManager) executeReplitAgent(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Replit Agent is a hosted web product — no local headless CLI.
	return nil, noHeadlessCLIError(TypeReplitAgent)
}

func (m *InstanceManager) executeIDX(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// IDX (Google web IDE) — no local headless CLI.
	return nil, noHeadlessCLIError(TypeIDX)
}

func (m *InstanceManager) executeFirebaseStudio(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Firebase Studio (Google web IDE) — no local headless CLI.
	return nil, noHeadlessCLIError(TypeFirebaseStudio)
}

func (m *InstanceManager) executeCascade(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// Cascade is Windsurf's IDE-integrated agent — no headless CLI.
	return nil, noHeadlessCLIError(TypeCascade)
}

func (m *InstanceManager) executeHelixAgent(inst *AgentInstance, payload interface{}) (interface{}, error) {
	// HelixAgent should exec the project's OWN binary, but its command name must
	// come from project config — NOT guessed (§11.4.6). Until that name is supplied
	// the table command is empty and this returns an honest error rather than a
	// fabricated success (research/operator-blocked, not a verified exec form).
	return nil, noHeadlessCLIError(TypeHelixAgent)
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
