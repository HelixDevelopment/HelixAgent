package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/google/uuid"
)

// Orchestrator coordinates multiple sub-agents for complex tasks.
//
// Concurrent-safe by construction (CONST-029): sessions is a safe.Store;
// Session field mutations route through Update callbacks (Pattern Beta).
type Orchestrator struct {
	manager  *Manager
	sessions *safe.Store[string, *Session]
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// Session represents an orchestrated multi-agent session
type Session struct {
	ID        string
	Status    SessionStatus
	Agents    []string // Agent IDs
	Tasks     []string // Task IDs
	Results   map[string]*SubAgentTaskResult
	CreatedAt time.Time
	UpdatedAt time.Time
	Context   map[string]interface{}
}

// SessionStatus represents the status of an orchestration session
type SessionStatus string

const (
	SessionStatusPending   SessionStatus = "pending"
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// OrchestrationPlan defines how agents should work together
type OrchestrationPlan struct {
	Name        string
	Description string
	Steps       []OrchestrationStep
}

// OrchestrationStep represents a single step in the orchestration
type OrchestrationStep struct {
	Name        string
	AgentType   SubAgentType
	Description string
	DependsOn   []string // Names of steps that must complete first
	Input       map[string]interface{}
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(manager *Manager) *Orchestrator {
	return &Orchestrator{
		manager:  manager,
		sessions: safe.NewStore[string, *Session](),
		shutdown: make(chan struct{}),
	}
}

// CreateSession creates a new orchestration session
func (o *Orchestrator) CreateSession(ctx context.Context) (*Session, error) {
	now := time.Now()
	session := &Session{
		ID:        uuid.New().String(),
		Status:    SessionStatusPending,
		Agents:    []string{},
		Tasks:     []string{},
		Results:   make(map[string]*SubAgentTaskResult),
		CreatedAt: now,
		UpdatedAt: now,
		Context:   make(map[string]interface{}),
	}

	o.sessions.Put(session.ID, session)
	return session, nil
}

// GetSession retrieves a session by ID
func (o *Orchestrator) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	session, exists := o.sessions.Get(sessionID)
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// updateSession atomically applies fn to the session's fields. fn must
// not retain pointers to mutable fields beyond the callback.
func (o *Orchestrator) updateSession(sessionID string, fn func(*Session)) {
	o.sessions.Update(sessionID, func(s *Session, ok bool) (*Session, bool) {
		if !ok {
			return nil, false
		}
		fn(s)
		return s, true
	})
}

// ExecutePlan executes an orchestration plan within a session
func (o *Orchestrator) ExecutePlan(ctx context.Context, sessionID string, plan OrchestrationPlan) error {
	session, err := o.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	o.updateSession(sessionID, func(s *Session) {
		s.Status = SessionStatusRunning
		s.UpdatedAt = time.Now()
	})

	// Build dependency graph
	completedSteps := make(map[string]bool)
	stepResults := make(map[string]*SubAgentTaskResult)

	// Execute steps respecting dependencies
	for len(completedSteps) < len(plan.Steps) {
		select {
		case <-ctx.Done():
			o.updateSession(sessionID, func(s *Session) {
				s.Status = SessionStatusCancelled
				s.UpdatedAt = time.Now()
			})
			return ctx.Err()
		case <-o.shutdown:
			o.updateSession(sessionID, func(s *Session) {
				s.Status = SessionStatusCancelled
				s.UpdatedAt = time.Now()
			})
			return fmt.Errorf("orchestrator shutting down")
		default:
		}

		// Find steps that are ready to execute
		for _, step := range plan.Steps {
			if completedSteps[step.Name] {
				continue
			}

			// Check if dependencies are met
			depsMet := true
			for _, dep := range step.DependsOn {
				if !completedSteps[dep] {
					depsMet = false
					break
				}
			}

			if !depsMet {
				continue
			}

			// Execute the step
			result, err := o.executeStep(ctx, session, step, stepResults)
			if err != nil {
				o.updateSession(sessionID, func(s *Session) {
					s.Status = SessionStatusFailed
					s.UpdatedAt = time.Now()
				})
				return fmt.Errorf("step %s failed: %w", step.Name, err)
			}

			completedSteps[step.Name] = true
			stepResults[step.Name] = result

			stepName := step.Name
			o.updateSession(sessionID, func(s *Session) {
				s.Results[stepName] = result
				s.UpdatedAt = time.Now()
			})
		}
	}

	o.updateSession(sessionID, func(s *Session) {
		s.Status = SessionStatusCompleted
		s.UpdatedAt = time.Now()
	})
	return nil
}

// ExecuteParallel executes multiple agents in parallel
func (o *Orchestrator) ExecuteParallel(ctx context.Context, sessionID string, prompts []string, agentType SubAgentType) ([]*SubAgentTaskResult, error) {
	if _, err := o.GetSession(ctx, sessionID); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	results := make(chan struct {
		index  int
		result *SubAgentTaskResult
		err    error
	}, len(prompts))

	for i, prompt := range prompts {
		wg.Add(1)
		go func(index int, p string) {
			defer wg.Done()

			// Create agent for this task
			profile := ProfileConfig{
				Name:        fmt.Sprintf("parallel-agent-%d", index),
				MaxTokens:   2000,
				Temperature: 0.7,
			}

			agent, err := o.manager.CreateAgent(ctx, string(agentType), profile)
			if err != nil {
				results <- struct {
					index  int
					result *SubAgentTaskResult
					err    error
				}{index, nil, err}
				return
			}

			task := Task{
				Description: p,
				MaxSteps:    5,
			}

			exploreResult, err := agent.Execute(ctx, task)
			if err != nil {
				results <- struct {
					index  int
					result *SubAgentTaskResult
					err    error
				}{index, nil, err}
				return
			}

			result := &SubAgentTaskResult{
				Content: exploreResult.Discoveries[0],
				Usage: &TokenUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
			}

			results <- struct {
				index  int
				result *SubAgentTaskResult
				err    error
			}{index, result, nil}
		}(i, prompt)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	resultList := make([]*SubAgentTaskResult, len(prompts))
	for res := range results {
		if res.err != nil {
			return nil, res.err
		}
		resultList[res.index] = res.result
	}

	o.updateSession(sessionID, func(s *Session) { s.UpdatedAt = time.Now() })

	return resultList, nil
}

// CancelSession cancels a running session
func (o *Orchestrator) CancelSession(ctx context.Context, sessionID string) error {
	var (
		notFound, notRunning bool
		taskIDs              []string
	)
	o.sessions.Update(sessionID, func(session *Session, ok bool) (*Session, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		if session.Status != SessionStatusRunning {
			notRunning = true
			return session, true
		}
		session.Status = SessionStatusCancelled
		session.UpdatedAt = time.Now()
		taskIDs = append(taskIDs, session.Tasks...)
		return session, true
	})
	if notFound {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if notRunning {
		return fmt.Errorf("session is not running: %s", sessionID)
	}
	for _, taskID := range taskIDs {
		_ = o.manager.CancelTask(ctx, taskID)
	}
	return nil
}

// ListSessions returns all sessions
func (o *Orchestrator) ListSessions(ctx context.Context) ([]*Session, error) {
	return o.sessions.Values(), nil
}

// Cleanup removes completed sessions older than the specified duration
func (o *Orchestrator) Cleanup(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	for _, id := range o.sessions.Keys() {
		o.sessions.Update(id, func(session *Session, ok bool) (*Session, bool) {
			if !ok {
				return nil, false
			}
			if session.Status == SessionStatusCompleted ||
				session.Status == SessionStatusFailed ||
				session.Status == SessionStatusCancelled {
				if session.UpdatedAt.Before(cutoff) {
					return nil, false // delete
				}
			}
			return session, true
		})
	}
	return nil
}

// Shutdown gracefully shuts down the orchestrator
func (o *Orchestrator) Shutdown(ctx context.Context) error {
	close(o.shutdown)

	// Cancel all running sessions
	sessions := o.sessions.Values()

	for _, session := range sessions {
		if session.Status == SessionStatusRunning {
			_ = o.CancelSession(ctx, session.ID)
		}
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		o.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// executeStep executes a single orchestration step
func (o *Orchestrator) executeStep(ctx context.Context, session *Session, step OrchestrationStep, previousResults map[string]*SubAgentTaskResult) (*SubAgentTaskResult, error) {
	// Create agent for this step
	profile := ProfileConfig{
		Name:        fmt.Sprintf("%s-agent", step.Name),
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	agent, err := o.manager.CreateAgent(ctx, string(step.AgentType), profile)
	if err != nil {
		return nil, err
	}

	task := Task{
		Description: step.Description,
		MaxSteps:    10,
	}

	exploreResult, err := agent.Execute(ctx, task)
	if err != nil {
		return nil, err
	}

	result := &SubAgentTaskResult{
		Content: exploreResult.Discoveries[0],
		Usage: &TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	return result, nil
}

// CreateDefaultPlan creates a default 3-step plan: Explore -> Plan -> Execute
func CreateDefaultPlan(objective string) OrchestrationPlan {
	return OrchestrationPlan{
		Name:        "default-3-step",
		Description: "Default 3-step orchestration: Explore, Plan, Execute",
		Steps: []OrchestrationStep{
			{
				Name:        "explore",
				AgentType:   ExploreAgent,
				Description: fmt.Sprintf("Explore and research: %s", objective),
				DependsOn:   []string{},
			},
			{
				Name:        "plan",
				AgentType:   PlanAgent,
				Description: fmt.Sprintf("Create implementation plan for: %s", objective),
				DependsOn:   []string{"explore"},
			},
			{
				Name:        "execute",
				AgentType:   GeneralAgent,
				Description: fmt.Sprintf("Execute the plan for: %s", objective),
				DependsOn:   []string{"plan"},
			},
		},
	}
}
