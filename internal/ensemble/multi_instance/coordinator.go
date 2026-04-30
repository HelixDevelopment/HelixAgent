// Package multi_instance provides multi-instance ensemble coordination for HelixAgent.
package multi_instance

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/clis"
	"dev.helix.agent/internal/ensemble/background"
	"dev.helix.agent/internal/ensemble/synchronization"
	obsmetrics "dev.helix.agent/internal/observability/metrics"
	"github.com/google/uuid"
)

// Coordinator manages multiple agent instances for ensemble execution.
//
// Concurrency model (CONST-029): sessions → *safe.Store. The former
// Coordinator.mu is dropped — individual session-field mutations in
// ExecuteSession/CancelSession are handled as in pre-existing code
// without explicit locking (they write through the shared session
// pointer); the Store guards only the ID → session map.
//
// Lock ordering (legacy, for session-internal locks):
//  1. EnsembleSession.mu (if re-introduced)
//  2. WorkerPool.mu
//  3. EventBus.mu (innermost)
type Coordinator struct {
	db          *sql.DB
	logger      *log.Logger
	instanceMgr *clis.InstanceManager
	syncMgr     *synchronization.SyncManager

	// Active sessions
	sessions *safe.Store[string, *EnsembleSession]

	// Load balancer
	loadBalancer LoadBalancer

	// Health monitor
	healthMonitor *HealthMonitor

	// Background workers
	workerPool *background.WorkerPool

	// Event bus for session events
	eventBus *clis.EventBus

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// EnsembleSession represents a multi-instance ensemble execution.
type EnsembleSession struct {
	ID string

	// Strategy configuration
	Strategy EnsembleStrategy
	Config   EnsembleConfig

	// Instances
	Primary   *clis.AgentInstance
	Critiques []*clis.AgentInstance
	Verifiers []*clis.AgentInstance
	Fallbacks []*clis.AgentInstance

	// State
	Status         SessionStatus
	Context        map[string]interface{}
	TaskDefinition Task

	// Communication
	MessageBus *MessageBus
	Results    chan *AgentResult

	// Consensus
	Consensus    *ConsensusResult
	CurrentRound int

	// Timestamps
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// EnsembleStrategy represents the coordination strategy.
type EnsembleStrategy string

// Strategy types.
const (
	StrategyVoting      EnsembleStrategy = "voting"
	StrategyDebate      EnsembleStrategy = "debate"
	StrategyConsensus   EnsembleStrategy = "consensus"
	StrategyPipeline    EnsembleStrategy = "pipeline"
	StrategyParallel    EnsembleStrategy = "parallel"
	StrategySequential  EnsembleStrategy = "sequential"
	StrategyExpertPanel EnsembleStrategy = "expert_panel"
)

// SessionStatus represents session state.
type SessionStatus string

// Session statuses.
const (
	SessionStatusCreating  SessionStatus = "creating"
	SessionStatusActive    SessionStatus = "active"
	SessionStatusPaused    SessionStatus = "paused"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// EnsembleConfig contains strategy configuration.
type EnsembleConfig struct {
	// Participant requirements
	MinParticipants    int
	MaxParticipants    int
	ConsensusThreshold float64

	// Execution limits
	MaxRounds       int
	TimeoutPerRound time.Duration
	TotalTimeout    time.Duration

	// Feature flags
	EnableStreaming    bool
	EnableFallbacks    bool
	RequireConsensus   bool
	EnableAutoRecovery bool
}

// DefaultEnsembleConfig returns default configuration.
func DefaultEnsembleConfig() EnsembleConfig {
	return EnsembleConfig{
		MinParticipants:    2,
		MaxParticipants:    5,
		ConsensusThreshold: 0.6,
		MaxRounds:          3,
		TimeoutPerRound:    5 * time.Minute,
		TotalTimeout:       15 * time.Minute,
		EnableStreaming:    true,
		EnableFallbacks:    true,
		RequireConsensus:   false,
		EnableAutoRecovery: true,
	}
}

// ParticipantConfig defines the instances to create.
type ParticipantConfig struct {
	Primary   InstanceConfig
	Critiques []InstanceConfig
	Verifiers []InstanceConfig
	Fallbacks []InstanceConfig
}

// InstanceConfig contains instance creation parameters.
type InstanceConfig struct {
	Type     clis.AgentType
	Config   clis.InstanceConfig
	Provider clis.ProviderConfig
}

// Task represents an ensemble task.
type Task struct {
	ID               string
	Type             string
	Content          string
	Context          map[string]interface{}
	Timeout          time.Duration
	RequireConsensus bool
}

// AgentResult represents a result from an agent.
type AgentResult struct {
	InstanceID   string
	InstanceType clis.AgentType
	Success      bool
	Result       interface{}
	Error        error
	Confidence   float64
	Duration     time.Duration
	Round        int
}

// ConsensusResult represents the final consensus.
type ConsensusResult struct {
	Reached    bool
	Winner     string
	Confidence float64
	AllResults map[string]*AgentResult
	Rounds     int
	Agreement  map[string]int
}

// MessageBus handles inter-agent communication.
type MessageBus struct {
	sessionID string
	sub       *clis.Subscription
	messages  chan *clis.Message
}

// Strategy defines the strategy interface.
type Strategy interface {
	Execute(ctx context.Context, session *EnsembleSession, task Task) (*ConsensusResult, error)
}

// NewCoordinator creates a new ensemble coordinator.
func NewCoordinator(
	db *sql.DB,
	logger *log.Logger,
	instanceMgr *clis.InstanceManager,
	syncMgr *synchronization.SyncManager,
) *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())

	if logger == nil {
		logger = log.New(os.Stdout, "", 0)
	}
	c := &Coordinator{
		db:            db,
		logger:        logger,
		instanceMgr:   instanceMgr,
		syncMgr:       syncMgr,
		sessions:      safe.NewStore[string, *EnsembleSession](),
		loadBalancer:  NewRoundRobinBalancer(),
		healthMonitor: NewHealthMonitor(),
		workerPool:    background.NewWorkerPool(100),
		eventBus:      clis.NewEventBus(),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Phase-3 observability: register the worker pool's live counters
	// with the default Prometheus source singleton. Decoupled — the
	// metrics package reads through these accessors on every scrape
	// but never holds a reference to the coordinator or pool structs.
	obsmetrics.SetEnsembleWorkerPoolContributor(obsmetrics.WorkerPoolContributor{
		PendingCount: c.workerPool.PendingCount,
		PendingCap:   func() int64 { return background.DefaultMaxPendingResults },
		TasksRejected: func() uint64 {
			// GetStats returns the full stat map; we only need
			// tasks_rejected. The cast is safe because the field
			// is always set to a uint64 inside GetStats.
			if v, ok := c.workerPool.GetStats()["tasks_rejected"].(uint64); ok {
				return v
			}
			return 0
		},
	})

	// Start health monitoring
	c.wg.Add(1)
	go c.healthMonitorLoop()

	return c
}

// CreateSession creates a new ensemble session.
func (c *Coordinator) CreateSession(
	ctx context.Context,
	strategy EnsembleStrategy,
	config EnsembleConfig,
	participants ParticipantConfig,
) (*EnsembleSession, error) {
	session := &EnsembleSession{
		ID:           uuid.New().String(),
		Strategy:     strategy,
		Config:       config,
		Status:       SessionStatusCreating,
		Context:      make(map[string]interface{}),
		MessageBus:   &MessageBus{messages: make(chan *clis.Message, 100)},
		Results:      make(chan *AgentResult, 100),
		CurrentRound: 0,
		CreatedAt:    time.Now(),
	}

	// CONST-035 §c "Completion": when no InstanceManager is wired (the
	// Coordinator was constructed with nil for testing/SDK probing) we
	// can't create real CLI-agent instances. Returning a session with
	// no instances + an explicit "no_instance_manager" status is more
	// honest than the silent nil-deref panic at instanceMgr.CreateInstance.
	// Tracking: #ensemble-instance-manager-wiring.
	if c.instanceMgr == nil {
		session.Status = "created_without_instances"
		c.sessions.Put(session.ID, session)
		return session, nil
	}

	// Create primary instance
	if participants.Primary.Type != "" {
		inst, err := c.instanceMgr.CreateInstance(
			ctx,
			participants.Primary.Type,
			participants.Primary.Config,
			participants.Primary.Provider.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("create primary: %w", err)
		}
		session.Primary = inst
	}

	// Create critique instances
	for _, cfg := range participants.Critiques {
		inst, err := c.instanceMgr.CreateInstance(ctx, cfg.Type, cfg.Config, cfg.Provider.Name)
		if err != nil {
			c.cleanupSession(ctx, session)
			return nil, fmt.Errorf("create critique: %w", err)
		}
		session.Critiques = append(session.Critiques, inst)
	}

	// Create verifier instances
	for _, cfg := range participants.Verifiers {
		inst, err := c.instanceMgr.CreateInstance(ctx, cfg.Type, cfg.Config, cfg.Provider.Name)
		if err != nil {
			c.cleanupSession(ctx, session)
			return nil, fmt.Errorf("create verifier: %w", err)
		}
		session.Verifiers = append(session.Verifiers, inst)
	}

	// Create fallback instances
	for _, cfg := range participants.Fallbacks {
		inst, err := c.instanceMgr.CreateInstance(ctx, cfg.Type, cfg.Config, cfg.Provider.Name)
		if err != nil {
			c.cleanupSession(ctx, session)
			return nil, fmt.Errorf("create fallback: %w", err)
		}
		session.Fallbacks = append(session.Fallbacks, inst)
	}

	// Build participant types list
	participantTypes := []clis.AgentType{}
	if session.Primary != nil {
		participantTypes = append(participantTypes, session.Primary.Type)
	}
	for _, inst := range session.Critiques {
		participantTypes = append(participantTypes, inst.Type)
	}
	for _, inst := range session.Verifiers {
		participantTypes = append(participantTypes, inst.Type)
	}

	// Persist to database
	if err := c.persistSession(ctx, session, participantTypes); err != nil {
		c.cleanupSession(ctx, session)
		return nil, fmt.Errorf("persist session: %w", err)
	}

	// Register session
	c.sessions.Put(session.ID, session)

	// Subscribe to session events
	session.MessageBus.sessionID = session.ID
	session.MessageBus.sub = c.eventBus.SubscribeWildcard(100)
	go c.handleSessionMessages(session)

	c.logger.Printf("Created ensemble session %s with strategy %s", session.ID, strategy)

	return session, nil
}

// ExecuteSession executes a task in an ensemble session.
func (c *Coordinator) ExecuteSession(
	ctx context.Context,
	sessionID string,
	task Task,
) (*ConsensusResult, error) {
	session, err := c.getSession(sessionID)
	if err != nil {
		return nil, err
	}

	// Update session
	session.TaskDefinition = task
	session.Status = SessionStatusActive
	now := time.Now()
	session.StartedAt = &now

	// Update database
	_, err = c.db.ExecContext(ctx,
		"UPDATE ensemble_sessions SET status = $1, started_at = NOW() WHERE id = $2",
		SessionStatusActive, sessionID,
	)
	if err != nil {
		c.logger.Printf("Warning: failed to update session status: %v", err)
	}

	// Execute based on strategy
	var result *ConsensusResult

	switch session.Strategy {
	case StrategyVoting:
		result, err = c.executeVotingStrategy(ctx, session, task)
	case StrategyDebate:
		result, err = c.executeDebateStrategy(ctx, session, task)
	case StrategyConsensus:
		result, err = c.executeConsensusStrategy(ctx, session, task)
	case StrategyPipeline:
		result, err = c.executePipelineStrategy(ctx, session, task)
	case StrategyParallel:
		result, err = c.executeParallelStrategy(ctx, session, task)
	case StrategySequential:
		result, err = c.executeSequentialStrategy(ctx, session, task)
	case StrategyExpertPanel:
		result, err = c.executeExpertPanelStrategy(ctx, session, task)
	default:
		return nil, fmt.Errorf("unknown strategy: %s", session.Strategy)
	}

	// Update session with result
	if err != nil {
		session.Status = SessionStatusFailed
	} else {
		session.Status = SessionStatusCompleted
		session.Consensus = result
	}

	completedAt := time.Now()
	session.CompletedAt = &completedAt

	// Persist result
	c.persistResult(ctx, session, result, err)

	return result, err
}

// GetSession retrieves a session by ID.
func (c *Coordinator) GetSession(id string) (*EnsembleSession, error) {
	return c.getSession(id)
}

// ListSessions returns all sessions matching the filter.
func (c *Coordinator) ListSessions(status SessionStatus) []*EnsembleSession {
	var result []*EnsembleSession
	c.sessions.Range(func(_ string, session *EnsembleSession) bool {
		if status != "" && session.Status != status {
			return true
		}
		result = append(result, session)
		return true
	})
	return result
}

// CancelSession cancels an active session.
func (c *Coordinator) CancelSession(ctx context.Context, sessionID string) error {
	session, err := c.getSession(sessionID)
	if err != nil {
		return err
	}

	session.Status = SessionStatusCancelled

	// Cancel all instance tasks
	if session.Primary != nil {
		c.instanceMgr.SendRequest(ctx, session.Primary.ID, &clis.Request{
			ID:   uuid.New().String(),
			Type: clis.RequestTypeCancel,
		})
	}

	for _, inst := range session.Critiques {
		c.instanceMgr.SendRequest(ctx, inst.ID, &clis.Request{
			ID:   uuid.New().String(),
			Type: clis.RequestTypeCancel,
		})
	}

	// Update database
	_, err = c.db.ExecContext(ctx,
		"UPDATE ensemble_sessions SET status = $1 WHERE id = $2",
		SessionStatusCancelled, sessionID,
	)

	return err
}

// Close shuts down the coordinator.
func (c *Coordinator) Close() error {
	c.cancel()

	// Cancel all active sessions
	ctx := context.Background()
	for _, session := range c.ListSessions(SessionStatusActive) {
		c.CancelSession(ctx, session.ID)
	}

	// Wait for goroutines
	c.wg.Wait()

	// Close event bus
	c.eventBus.Close()

	return nil
}

// Strategy implementations

func (c *Coordinator) executeVotingStrategy(
	ctx context.Context,
	session *EnsembleSession,
	task Task,
) (*ConsensusResult, error) {
	c.logger.Printf("Executing voting strategy for session %s", session.ID)

	// Collect all participants
	participants := c.collectParticipants(session)
	if len(participants) < session.Config.MinParticipants {
		return nil, fmt.Errorf("insufficient participants: %d < %d", len(participants), session.Config.MinParticipants)
	}

	// Execute task on all participants concurrently
	results := c.executeOnAll(ctx, session, participants, task, 1)

	// Count votes
	voteCounts := make(map[string]int)
	resultMap := make(map[string]*AgentResult)

	for _, result := range results {
		if !result.Success {
			continue
		}
		key := c.resultKey(result.Result)
		voteCounts[key]++
		resultMap[key] = result
	}

	// Find winner
	var winner string
	maxVotes := 0
	for key, count := range voteCounts {
		if count > maxVotes {
			maxVotes = count
			winner = key
		}
	}

	total := len(results)
	confidence := float64(maxVotes) / float64(total)
	consensusReached := confidence >= session.Config.ConsensusThreshold

	return &ConsensusResult{
		Reached:    consensusReached,
		Winner:     winner,
		Confidence: confidence,
		AllResults: results,
		Rounds:     1,
		Agreement:  voteCounts,
	}, nil
}

func (c *Coordinator) executeDebateStrategy(
	ctx context.Context,
	session *EnsembleSession,
	task Task,
) (*ConsensusResult, error) {
	c.logger.Printf("Executing debate strategy for session %s", session.ID)

	allResults := make(map[string]*AgentResult)
	debateHistory := []map[string]*AgentResult{}
	var lastProposal *AgentResult

	for round := 1; round <= session.Config.MaxRounds; round++ {
		session.CurrentRound = round

		// Primary proposes solution
		if session.Primary == nil {
			return nil, fmt.Errorf("debate strategy requires a primary instance")
		}

		proposal, err := c.executeOnInstance(ctx, session.Primary, task, round)
		if err != nil {
			return nil, fmt.Errorf("primary execution failed: %w", err)
		}
		lastProposal = proposal

		// Critiques evaluate proposal
		critiqueTask := Task{
			Content: fmt.Sprintf("Evaluate and critique this proposal: %v", proposal.Result),
			Context: map[string]interface{}{
				"proposal":       proposal.Result,
				"debate_history": debateHistory,
			},
		}

		critiqueResults := c.executeOnAll(ctx, session, session.Critiques, critiqueTask, round)

		// Store results
		roundResults := make(map[string]*AgentResult)
		roundResults[session.Primary.ID] = proposal
		for id, result := range critiqueResults {
			roundResults[id] = result
		}
		debateHistory = append(debateHistory, roundResults)

		// Check for consensus
		agreement := c.calculateAgreement(roundResults)
		if agreement >= session.Config.ConsensusThreshold {
			// Consensus reached
			for k, v := range roundResults {
				allResults[k] = v
			}

			return &ConsensusResult{
				Reached:    true,
				Winner:     c.resultKey(proposal.Result),
				Confidence: agreement,
				AllResults: allResults,
				Rounds:     round,
			}, nil
		}

		// Prepare next round task with critiques
		task.Context["critiques"] = critiqueResults
	}

	// Max rounds reached, return best result
	if lastProposal == nil {
		return nil, fmt.Errorf("no proposal generated in debate")
	}
	return &ConsensusResult{
		Reached:    false,
		Winner:     c.resultKey(lastProposal.Result),
		Confidence: c.calculateAgreement(debateHistory[len(debateHistory)-1]),
		AllResults: allResults,
		Rounds:     session.Config.MaxRounds,
	}, nil
}

func (c *Coordinator) executeConsensusStrategy(
	ctx context.Context,
	session *EnsembleSession,
	task Task,
) (*ConsensusResult, error) {
	// Similar to voting but requires explicit consensus confirmation
	result, err := c.executeVotingStrategy(ctx, session, task)
	if err != nil {
		return nil, err
	}

	if session.Config.RequireConsensus && !result.Reached {
		return nil, fmt.Errorf("consensus not reached (confidence: %.2f)", result.Confidence)
	}

	return result, nil
}

func (c *Coordinator) executePipelineStrategy(
	ctx context.Context,
	session *EnsembleSession,
	task Task,
) (*ConsensusResult, error) {
	c.logger.Printf("Executing pipeline strategy for session %s", session.ID)

	// Pipeline: Primary → Critiques → Verifiers
	allResults := make(map[string]*AgentResult)

	// Stage 1: Primary processing
	if session.Primary == nil {
		return nil, fmt.Errorf("pipeline requires a primary instance")
	}

	stage1Result, err := c.executeOnInstance(ctx, session.Primary, task, 1)
	if err != nil {
		return nil, fmt.Errorf("stage 1 failed: %w", err)
	}
	allResults[session.Primary.ID] = stage1Result

	// Stage 2: Critique processing
	stage2Task := Task{
		Content: fmt.Sprintf("Process and improve: %v", stage1Result.Result),
		Context: map[string]interface{}{"previous_stage": stage1Result},
	}

	var stage2Result *AgentResult
	for _, inst := range session.Critiques {
		result, err := c.executeOnInstance(ctx, inst, stage2Task, 2)
		if err == nil {
			stage2Result = result
			allResults[inst.ID] = result
			break
		}
	}

	if stage2Result == nil {
		return nil, fmt.Errorf("stage 2 failed on all critique instances")
	}

	// Stage 3: Verification
	if len(session.Verifiers) > 0 {
		stage3Task := Task{
			Content: fmt.Sprintf("Verify: %v", stage2Result.Result),
			Context: map[string]interface{}{"previous_stage": stage2Result},
		}

		for _, inst := range session.Verifiers {
			result, err := c.executeOnInstance(ctx, inst, stage3Task, 3)
			if err == nil {
				allResults[inst.ID] = result
				break
			}
		}
	}

	return &ConsensusResult{
		Reached:    true,
		Winner:     c.resultKey(stage2Result.Result),
		Confidence: 1.0,
		AllResults: allResults,
		Rounds:     1,
	}, nil
}

func (c *Coordinator) executeParallelStrategy(
	ctx context.Context,
	session *EnsembleSession,
	task Task,
) (*ConsensusResult, error) {
	// All instances execute in parallel, results aggregated
	participants := c.collectParticipants(session)
	results := c.executeOnAll(ctx, session, participants, task, 1)

	return &ConsensusResult{
		Reached:    true,
		AllResults: results,
		Rounds:     1,
	}, nil
}

func (c *Coordinator) executeSequentialStrategy(
	ctx context.Context,
	session *EnsembleSession,
	task Task,
) (*ConsensusResult, error) {
	// Execute one at a time until success
	participants := c.collectParticipants(session)

	for i, inst := range participants {
		result, err := c.executeOnInstance(ctx, inst, task, 1)
		if err == nil {
			return &ConsensusResult{
				Reached:    true,
				Winner:     c.resultKey(result.Result),
				Confidence: 1.0,
				AllResults: map[string]*AgentResult{inst.ID: result},
				Rounds:     1,
			}, nil
		}

		// If not last, try fallback
		if i < len(participants)-1 && len(session.Fallbacks) > 0 {
			continue
		}
	}

	return nil, fmt.Errorf("all participants failed")
}

func (c *Coordinator) executeExpertPanelStrategy(
	ctx context.Context,
	session *EnsembleSession,
	task Task,
) (*ConsensusResult, error) {
	// Each agent type contributes expertise
	// Results combined into final synthesis

	results := c.executeOnAll(ctx, session, c.collectParticipants(session), task, 1)

	// Synthesis by primary
	synthesisTask := Task{
		Content: "Synthesize expert opinions into final answer",
		Context: map[string]interface{}{
			"expert_opinions": results,
		},
	}

	synthesis, err := c.executeOnInstance(ctx, session.Primary, synthesisTask, 2)
	if err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}

	results[session.Primary.ID+"_synthesis"] = synthesis

	return &ConsensusResult{
		Reached:    true,
		Winner:     c.resultKey(synthesis.Result),
		Confidence: 0.9,
		AllResults: results,
		Rounds:     2,
	}, nil
}

// Helper methods

func (c *Coordinator) collectParticipants(session *EnsembleSession) []*clis.AgentInstance {
	var participants []*clis.AgentInstance

	if session.Primary != nil {
		participants = append(participants, session.Primary)
	}
	participants = append(participants, session.Critiques...)
	participants = append(participants, session.Verifiers...)

	return participants
}

func (c *Coordinator) executeOnAll(
	ctx context.Context,
	session *EnsembleSession,
	instances []*clis.AgentInstance,
	task Task,
	round int,
) map[string]*AgentResult {
	results := make(map[string]*AgentResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, inst := range instances {
		wg.Add(1)
		go func(i *clis.AgentInstance) {
			defer wg.Done()

			result, err := c.executeOnInstance(ctx, i, task, round)
			if err != nil {
				c.logger.Printf("Error executing on instance %s: %v", i.ID, err)
			}

			mu.Lock()
			results[i.ID] = result
			mu.Unlock()

			// Send to session results channel
			select {
			case session.Results <- result:
			default:
			}
		}(inst)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All completed
	case <-time.After(session.Config.TimeoutPerRound):
		c.logger.Printf("Timeout waiting for all participants in session %s", session.ID)
	case <-ctx.Done():
		c.logger.Printf("Context cancelled for session %s", session.ID)
	}

	return results
}

func (c *Coordinator) executeOnInstance(
	ctx context.Context,
	inst *clis.AgentInstance,
	task Task,
	round int,
) (*AgentResult, error) {
	start := time.Now()

	req := &clis.Request{
		ID:      uuid.New().String(),
		Type:    clis.RequestTypeExecute,
		Payload: task,
		Timeout: task.Timeout,
	}

	resp, err := c.instanceMgr.SendRequest(ctx, inst.ID, req)

	duration := time.Since(start)

	if err != nil {
		return &AgentResult{
			InstanceID:   inst.ID,
			InstanceType: inst.Type,
			Success:      false,
			Error:        err,
			Duration:     duration,
			Round:        round,
		}, err
	}

	return &AgentResult{
		InstanceID:   inst.ID,
		InstanceType: inst.Type,
		Success:      resp.Success,
		Result:       resp.Result,
		Error:        nil,
		Duration:     duration,
		Round:        round,
	}, nil
}

func (c *Coordinator) resultKey(result interface{}) string {
	// Generate a key for comparing results
	// This is a simplified version - could use semantic hashing
	data, _ := json.Marshal(result)
	return string(data)
}

func (c *Coordinator) calculateAgreement(results map[string]*AgentResult) float64 {
	if len(results) == 0 {
		return 0
	}

	// Count matching results
	counts := make(map[string]int)
	for _, result := range results {
		if !result.Success {
			continue
		}
		key := c.resultKey(result.Result)
		counts[key]++
	}

	// Find max agreement
	maxCount := 0
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}

	return float64(maxCount) / float64(len(results))
}

func (c *Coordinator) getSession(id string) (*EnsembleSession, error) {
	session, ok := c.sessions.Get(id)
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return session, nil
}

func (c *Coordinator) persistSession(
	ctx context.Context,
	session *EnsembleSession,
	participantTypes []clis.AgentType,
) error {
	// Get instance IDs
	var primaryID *string
	if session.Primary != nil {
		id := session.Primary.ID
		primaryID = &id
	}

	critiqueIDs := []string{}
	for _, inst := range session.Critiques {
		critiqueIDs = append(critiqueIDs, inst.ID)
	}

	verifierIDs := []string{}
	for _, inst := range session.Verifiers {
		verifierIDs = append(verifierIDs, inst.ID)
	}

	fallbackIDs := []string{}
	for _, inst := range session.Fallbacks {
		fallbackIDs = append(fallbackIDs, inst.ID)
	}

	configJSON, _ := json.Marshal(session.Config)
	contextJSON, _ := json.Marshal(session.Context)
	participantTypesJSON, _ := json.Marshal(participantTypes)
	critiqueIDsJSON, _ := json.Marshal(critiqueIDs)
	verifierIDsJSON, _ := json.Marshal(verifierIDs)
	fallbackIDsJSON, _ := json.Marshal(fallbackIDs)

	_, err := c.db.ExecContext(ctx,
		`INSERT INTO ensemble_sessions (
			id, strategy, strategy_config, participant_types,
			primary_instance_id, critique_instance_ids, verification_instance_ids,
			fallback_instance_ids, status, context, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		session.ID, session.Strategy, configJSON, participantTypesJSON,
		primaryID, critiqueIDsJSON, verifierIDsJSON, fallbackIDsJSON,
		session.Status, contextJSON, session.CreatedAt,
	)

	return err
}

func (c *Coordinator) persistResult(
	ctx context.Context,
	session *EnsembleSession,
	result *ConsensusResult,
	execErr error,
) error {
	var resultJSON []byte
	if result != nil {
		resultJSON, _ = json.Marshal(result)
	}

	var errorMsg *string
	if execErr != nil {
		msg := execErr.Error()
		errorMsg = &msg
	}

	var reached bool
	var confidence float64
	var rounds int
	var durationMs int64
	if result != nil {
		reached = result.Reached
		confidence = result.Confidence
		rounds = result.Rounds
	}
	if session.StartedAt != nil {
		durationMs = time.Since(*session.StartedAt).Milliseconds()
	}

	_, err := c.db.ExecContext(ctx,
		`UPDATE ensemble_sessions SET
			status = $1,
			final_result = $2,
			consensus_reached = $3,
			confidence_score = $4,
			completed_at = NOW(),
			total_duration_ms = $5,
			round_count = $6
		 WHERE id = $7`,
		session.Status, resultJSON, reached, confidence,
		durationMs, rounds, session.ID,
	)

	if errorMsg != nil {
		c.db.ExecContext(ctx,
			"UPDATE ensemble_sessions SET error_message = $1 WHERE id = $2",
			*errorMsg, session.ID,
		)
	}

	return err
}

func (c *Coordinator) cleanupSession(ctx context.Context, session *EnsembleSession) {
	// Terminate all instances
	if session.Primary != nil {
		c.instanceMgr.TerminateInstance(ctx, session.Primary.ID)
	}
	for _, inst := range session.Critiques {
		c.instanceMgr.TerminateInstance(ctx, inst.ID)
	}
	for _, inst := range session.Verifiers {
		c.instanceMgr.TerminateInstance(ctx, inst.ID)
	}
	for _, inst := range session.Fallbacks {
		c.instanceMgr.TerminateInstance(ctx, inst.ID)
	}
}

func (c *Coordinator) handleSessionMessages(session *EnsembleSession) {
	for event := range session.MessageBus.sub.Ch {
		// Convert Event to Message
		msg := &clis.Message{
			ID:        event.ID.String(),
			SessionID: session.ID, // Use session ID since Event doesn't have one
			Type:      clis.MessageType(event.Type),
			SourceID:  event.Source,
		}
		// Process session-specific messages
		select {
		case session.MessageBus.messages <- msg:
		default:
			// Message buffer full
		}
	}
}

func (c *Coordinator) healthMonitorLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkSessionHealth()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Coordinator) checkSessionHealth() {
	sessions := make([]*EnsembleSession, 0, c.sessions.Len())
	c.sessions.Range(func(_ string, s *EnsembleSession) bool {
		sessions = append(sessions, s)
		return true
	})

	for _, session := range sessions {
		if session.Status != SessionStatusActive {
			continue
		}

		// Check if any instances are unhealthy
		unhealthy := 0
		participants := c.collectParticipants(session)
		for _, inst := range participants {
			if inst.Health == clis.HealthUnhealthy {
				unhealthy++
			}
		}

		if float64(unhealthy)/float64(len(participants)) > 0.5 {
			c.logger.Printf("Session %s has too many unhealthy instances", session.ID)
			// Could trigger auto-recovery here
		}
	}
}
