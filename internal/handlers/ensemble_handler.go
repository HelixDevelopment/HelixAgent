// Package handlers provides HTTP handlers for HelixAgent.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/clis"
	"dev.helix.agent/internal/ensemble/multi_instance"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// AgentType represents the type of agent in a team
type AgentType string

const (
	AgentTypePrimary    AgentType = "primary"
	AgentTypeCritic     AgentType = "critic"
	AgentTypeVerifier   AgentType = "verifier"
	AgentTypeResearcher AgentType = "researcher"
	AgentTypeCoder      AgentType = "coder"
	AgentTypeTester     AgentType = "tester"
)

// AgentDefinition defines an agent in a team
type AgentDefinition struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     AgentType              `json:"type"`
	Config   map[string]interface{} `json:"config"`
	Provider ProviderConfig         `json:"provider"`
}

// ProviderConfig defines LLM provider configuration
type ProviderConfig struct {
	Name   string                 `json:"name"`
	Model  string                 `json:"model"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// Team defines a team of agents
type Team struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Agents      []AgentDefinition `json:"agents"`
	Config      TeamConfig        `json:"config"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TeamConfig configures team behavior
type TeamConfig struct {
	MaxParallel        int     `json:"max_parallel"`
	ConsensusThreshold float64 `json:"consensus_threshold"` // 0.0 - 1.0
	Timeout            int     `json:"timeout_seconds"`
	EnableVoting       bool    `json:"enable_voting"`
}

// TeamExecutionRequest represents a team execution request
type TeamExecutionRequest struct {
	Task             string                 `json:"task" binding:"required"`
	Context          map[string]interface{} `json:"context,omitempty"`
	Timeout          int                    `json:"timeout_seconds,omitempty"`
	RequireConsensus bool                   `json:"require_consensus,omitempty"`
}

// TeamExecutionResult represents the result of team execution
type TeamExecutionResult struct {
	TeamID           string            `json:"team_id"`
	Task             string            `json:"task"`
	Consensus        string            `json:"consensus"`
	Confidence       float64           `json:"confidence"`
	IndividualVotes  map[string]string `json:"individual_votes"`
	AgentResults     []AgentResult     `json:"agent_results"`
	ExecutionTime    int64             `json:"execution_time_ms"`
	ConsensusReached bool              `json:"consensus_reached"`
}

// AgentResult represents an individual agent's result
type AgentResult struct {
	AgentID       string    `json:"agent_id"`
	AgentName     string    `json:"agent_name"`
	AgentType     AgentType `json:"agent_type"`
	Output        string    `json:"output"`
	Confidence    float64   `json:"confidence"`
	ExecutionTime int64     `json:"execution_time_ms"`
}

// EnsembleHandler handles ensemble-related endpoints.
//
// Concurrency model (CONST-029): teams is a *safe.Store. Multi-field
// mutations (name/description/agents updates, agent append/remove)
// use safe.Store.Update with a copy-on-write closure so readers via
// Get see either the pre- or post-update Team pointer but never
// partial writes. No survivor mutex because all compound operations
// fit in a single Update closure.
type EnsembleHandler struct {
	coordinator *multi_instance.Coordinator
	logger      *logrus.Logger

	// Team management
	teams *safe.Store[string, *Team]
}

// NewEnsembleHandler creates a new ensemble handler.
func NewEnsembleHandler(coordinator *multi_instance.Coordinator, logger *logrus.Logger) *EnsembleHandler {
	return &EnsembleHandler{
		coordinator: coordinator,
		logger:      logger,
		teams:       safe.NewStore[string, *Team](),
	}
}

// RegisterRoutes registers the ensemble routes.
func (h *EnsembleHandler) RegisterRoutes(r *gin.RouterGroup) {
	// Existing ensemble session routes
	sessions := r.Group("/ensemble/sessions")
	{
		sessions.POST("", h.CreateSession)
		sessions.GET("", h.ListSessions)
		sessions.GET("/:id", h.GetSession)
		sessions.POST("/:id/execute", h.ExecuteSession)
		sessions.POST("/:id/cancel", h.CancelSession)
	}

	// New team management routes
	teams := r.Group("/ensemble/teams")
	{
		teams.POST("", h.CreateTeam)
		teams.GET("", h.ListTeams)
		teams.GET("/:id", h.GetTeam)
		teams.PUT("/:id", h.UpdateTeam)
		teams.DELETE("/:id", h.DeleteTeam)
		teams.POST("/:id/agents", h.AddAgentToTeam)
		teams.DELETE("/:id/agents/:agentId", h.RemoveAgentFromTeam)
		teams.POST("/:id/execute", h.ExecuteTeam)
	}
}

// CreateSessionRequest represents a create session request.
type CreateSessionRequest struct {
	Strategy     string             `json:"strategy" binding:"required"`
	Participants ParticipantRequest `json:"participants" binding:"required"`
}

// ParticipantRequest represents participant configuration.
type ParticipantRequest struct {
	Primary   *InstanceRequest  `json:"primary,omitempty"`
	Critiques []InstanceRequest `json:"critiques,omitempty"`
	Verifiers []InstanceRequest `json:"verifiers,omitempty"`
	Fallbacks []InstanceRequest `json:"fallbacks,omitempty"`
}

// InstanceRequest represents instance configuration.
type InstanceRequest struct {
	Type     string                 `json:"type" binding:"required"`
	Config   map[string]interface{} `json:"config,omitempty"`
	Provider map[string]interface{} `json:"provider,omitempty"`
}

// CreateSession creates a new ensemble session.
func (h *EnsembleHandler) CreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert request to coordinator format
	participants := multi_instance.ParticipantConfig{}

	if req.Participants.Primary != nil {
		participants.Primary = multi_instance.InstanceConfig{
			Type: clis.AgentType(req.Participants.Primary.Type),
		}
	}

	for _, ic := range req.Participants.Critiques {
		participants.Critiques = append(participants.Critiques, multi_instance.InstanceConfig{
			Type: clis.AgentType(ic.Type),
		})
	}

	for _, ic := range req.Participants.Verifiers {
		participants.Verifiers = append(participants.Verifiers, multi_instance.InstanceConfig{
			Type: clis.AgentType(ic.Type),
		})
	}

	strategy := multi_instance.EnsembleStrategy(req.Strategy)
	config := multi_instance.DefaultEnsembleConfig()

	session, err := h.coordinator.CreateSession(c.Request.Context(), strategy, config, participants)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       session.ID,
		"strategy": session.Strategy,
		"status":   session.Status,
	})
}

// ListSessions lists all ensemble sessions.
func (h *EnsembleHandler) ListSessions(c *gin.Context) {
	status := c.Query("status")
	sessions := h.coordinator.ListSessions(multi_instance.SessionStatus(status))

	var response []gin.H
	for _, s := range sessions {
		response = append(response, gin.H{
			"id":         s.ID,
			"strategy":   s.Strategy,
			"status":     s.Status,
			"created_at": s.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// GetSession gets a session by ID.
func (h *EnsembleHandler) GetSession(c *gin.Context) {
	id := c.Param("id")

	session, err := h.coordinator.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         session.ID,
		"strategy":   session.Strategy,
		"status":     session.Status,
		"created_at": session.CreatedAt,
		"started_at": session.StartedAt,
	})
}

// ExecuteRequest represents an execute request.
type ExecuteRequest struct {
	Content string `json:"content" binding:"required"`
	Timeout int    `json:"timeout,omitempty"`
}

// ExecuteSession executes a task in a session.
func (h *EnsembleHandler) ExecuteSession(c *gin.Context) {
	id := c.Param("id")

	var req ExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task := multi_instance.Task{
		Type:    "completion",
		Content: req.Content,
	}

	if req.Timeout > 0 {
		task.Timeout = time.Duration(req.Timeout) * time.Second
	}

	result, err := h.coordinator.ExecuteSession(c.Request.Context(), id, task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"consensus_reached": result.Reached,
		"confidence":        result.Confidence,
		"rounds":            result.Rounds,
		"results":           result.AllResults,
	})
}

// CancelSession cancels a session.
func (h *EnsembleHandler) CancelSession(c *gin.Context) {
	id := c.Param("id")

	if err := h.coordinator.CancelSession(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// ============================================
// TEAM MANAGEMENT METHODS
// ============================================

// CreateTeamRequest represents a create team request
type CreateTeamRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Agents      []AgentDefinition `json:"agents,omitempty"`
	Config      *TeamConfig       `json:"config,omitempty"`
}

// CreateTeam creates a new team
func (h *EnsembleHandler) CreateTeam(c *gin.Context) {
	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team := &Team{
		ID:          generateTeamID(),
		Name:        req.Name,
		Description: req.Description,
		Agents:      req.Agents,
		Config:      DefaultTeamConfig(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if req.Config != nil {
		team.Config = *req.Config
	}

	// Assign IDs to agents without them
	for i := range team.Agents {
		if team.Agents[i].ID == "" {
			team.Agents[i].ID = fmt.Sprintf("agent_%d_%d", time.Now().Unix(), i)
		}
	}

	h.teams.Put(team.ID, team)

	h.logger.Infof("Created team: %s (%s)", team.Name, team.ID)

	c.JSON(http.StatusCreated, gin.H{
		"id":          team.ID,
		"name":        team.Name,
		"description": team.Description,
		"agents":      len(team.Agents),
		"created_at":  team.CreatedAt,
	})
}

// ListTeams lists all teams
func (h *EnsembleHandler) ListTeams(c *gin.Context) {
	var teams []gin.H
	h.teams.Range(func(_ string, team *Team) bool {
		teams = append(teams, gin.H{
			"id":          team.ID,
			"name":        team.Name,
			"description": team.Description,
			"agent_count": len(team.Agents),
			"created_at":  team.CreatedAt,
		})
		return true
	})

	c.JSON(http.StatusOK, teams)
}

// GetTeam gets a team by ID
func (h *EnsembleHandler) GetTeam(c *gin.Context) {
	id := c.Param("id")

	team, exists := h.teams.Get(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	c.JSON(http.StatusOK, team)
}

// UpdateTeamRequest represents an update team request
type UpdateTeamRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Agents      []AgentDefinition `json:"agents,omitempty"`
	Config      *TeamConfig       `json:"config,omitempty"`
}

// UpdateTeam updates a team
func (h *EnsembleHandler) UpdateTeam(c *gin.Context) {
	id := c.Param("id")

	var req UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var updated *Team
	var found bool
	h.teams.Update(id, func(cur *Team, present bool) (*Team, bool) {
		if !present {
			return nil, false
		}
		cp := *cur
		if req.Name != "" {
			cp.Name = req.Name
		}
		if req.Description != "" {
			cp.Description = req.Description
		}
		if req.Agents != nil {
			cp.Agents = req.Agents
		}
		if req.Config != nil {
			cp.Config = *req.Config
		}
		cp.UpdatedAt = time.Now()
		updated = &cp
		found = true
		return &cp, true
	})

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// DeleteTeam deletes a team
func (h *EnsembleHandler) DeleteTeam(c *gin.Context) {
	id := c.Param("id")

	if _, ok := h.teams.Delete(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	h.logger.Infof("Deleted team: %s", id)

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// AddAgentToTeam adds an agent to a team
func (h *EnsembleHandler) AddAgentToTeam(c *gin.Context) {
	teamID := c.Param("id")

	var agent AgentDefinition
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if agent.ID == "" {
		// D-20: uuid suffix makes the agent ID unique-by-construction; the bare
		// UnixNano suffix collided for agents created within one coarse clock tick.
		agent.ID = fmt.Sprintf("agent_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
	}

	var found bool
	h.teams.Update(teamID, func(cur *Team, present bool) (*Team, bool) {
		if !present {
			return nil, false
		}
		cp := *cur
		cp.Agents = append([]AgentDefinition(nil), cur.Agents...)
		cp.Agents = append(cp.Agents, agent)
		cp.UpdatedAt = time.Now()
		found = true
		return &cp, true
	})

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	h.logger.Infof("Added agent %s to team %s", agent.ID, teamID)

	c.JSON(http.StatusCreated, agent)
}

// RemoveAgentFromTeam removes an agent from a team
func (h *EnsembleHandler) RemoveAgentFromTeam(c *gin.Context) {
	teamID := c.Param("id")
	agentID := c.Param("agentId")

	var teamFound, agentFound bool
	h.teams.Update(teamID, func(cur *Team, present bool) (*Team, bool) {
		if !present {
			return nil, false
		}
		teamFound = true
		cp := *cur
		// Copy Agents so we don't mutate the shared pre-update slice.
		agents := make([]AgentDefinition, 0, len(cur.Agents))
		for _, a := range cur.Agents {
			if a.ID == agentID {
				agentFound = true
				continue
			}
			agents = append(agents, a)
		}
		if !agentFound {
			// Keep the existing pointer unchanged.
			return cur, true
		}
		cp.Agents = agents
		cp.UpdatedAt = time.Now()
		return &cp, true
	})

	if !teamFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}
	if !agentFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	h.logger.Infof("Removed agent %s from team %s", agentID, teamID)

	c.JSON(http.StatusOK, gin.H{"status": "removed"})
}

// ExecuteTeam executes a task with a team
func (h *EnsembleHandler) ExecuteTeam(c *gin.Context) {
	teamID := c.Param("id")

	var req TeamExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team, exists := h.teams.Get(teamID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	if len(team.Agents) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team has no agents"})
		return
	}

	// Execute with team
	startTime := time.Now()
	result, err := h.executeTeamTask(c.Request.Context(), team, req)
	executionTime := time.Since(startTime).Milliseconds()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result.ExecutionTime = executionTime

	c.JSON(http.StatusOK, result)
}

// executeTeamTask executes a task with parallel agent coordination
func (h *EnsembleHandler) executeTeamTask(ctx context.Context, team *Team, req TeamExecutionRequest) (*TeamExecutionResult, error) {
	result := &TeamExecutionResult{
		TeamID:          team.ID,
		Task:            req.Task,
		IndividualVotes: make(map[string]string),
		AgentResults:    make([]AgentResult, 0, len(team.Agents)),
	}

	// Determine timeout
	timeout := time.Duration(team.Config.Timeout) * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute agents in parallel with semaphore for max parallel
	semaphore := make(chan struct{}, team.Config.MaxParallel)
	if team.Config.MaxParallel <= 0 {
		semaphore = make(chan struct{}, len(team.Agents))
	}

	var wg sync.WaitGroup
	resultsChan := make(chan AgentResult, len(team.Agents))

	for _, agent := range team.Agents {
		wg.Add(1)
		go func(agent AgentDefinition) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			agentResult := h.executeAgent(execCtx, agent, req)
			resultsChan <- agentResult
		}(agent)
	}

	// Close results channel when all agents complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for agentResult := range resultsChan {
		result.AgentResults = append(result.AgentResults, agentResult)
		result.IndividualVotes[agentResult.AgentID] = agentResult.Output
	}

	// Calculate consensus if voting is enabled
	if team.Config.EnableVoting {
		result.Consensus, result.Confidence, result.ConsensusReached = h.calculateConsensus(result.AgentResults, team.Config.ConsensusThreshold)
	} else {
		// Use primary agent result as consensus
		for _, ar := range result.AgentResults {
			if ar.AgentType == AgentTypePrimary {
				result.Consensus = ar.Output
				result.Confidence = ar.Confidence
				result.ConsensusReached = true
				break
			}
		}
		if !result.ConsensusReached && len(result.AgentResults) > 0 {
			result.Consensus = result.AgentResults[0].Output
			result.Confidence = result.AgentResults[0].Confidence
			result.ConsensusReached = true
		}
	}

	return result, nil
}

// executeAgent executes a single agent's task
func (h *EnsembleHandler) executeAgent(ctx context.Context, agent AgentDefinition, req TeamExecutionRequest) AgentResult {
	startTime := time.Now()

	result := AgentResult{
		AgentID:   agent.ID,
		AgentName: agent.Name,
		AgentType: agent.Type,
	}

	// This would integrate with the LLM provider system
	// For now, simulate execution
	select {
	case <-ctx.Done():
		result.Output = "Execution cancelled or timed out"
		result.Confidence = 0.0
	case <-time.After(time.Duration(100+len(req.Task)*10) * time.Millisecond):
		// Simulate different agent behaviors
		switch agent.Type {
		case AgentTypePrimary:
			result.Output = fmt.Sprintf("Primary analysis: Task '%s' requires careful consideration", req.Task)
			result.Confidence = 0.85
		case AgentTypeCritic:
			result.Output = fmt.Sprintf("Critical review: Approach to '%s' could be improved", req.Task)
			result.Confidence = 0.75
		case AgentTypeVerifier:
			result.Output = fmt.Sprintf("Verification: Task '%s' specifications verified", req.Task)
			result.Confidence = 0.90
		case AgentTypeResearcher:
			result.Output = fmt.Sprintf("Research findings: Related to '%s' found", req.Task)
			result.Confidence = 0.80
		case AgentTypeCoder:
			result.Output = fmt.Sprintf("Implementation: Code for '%s' generated", req.Task)
			result.Confidence = 0.88
		case AgentTypeTester:
			result.Output = fmt.Sprintf("Testing: Test cases for '%s' created", req.Task)
			result.Confidence = 0.82
		default:
			result.Output = fmt.Sprintf("Agent processed task: %s", req.Task)
			result.Confidence = 0.70
		}
	}

	result.ExecutionTime = time.Since(startTime).Milliseconds()
	return result
}

// calculateConsensus calculates consensus from agent results
func (h *EnsembleHandler) calculateConsensus(results []AgentResult, threshold float64) (string, float64, bool) {
	if len(results) == 0 {
		return "", 0.0, false
	}

	if threshold <= 0 {
		threshold = 0.5
	}

	// Group similar outputs (simplified - exact match)
	votes := make(map[string]int)
	confidenceSum := make(map[string]float64)

	for _, result := range results {
		votes[result.Output]++
		confidenceSum[result.Output] += result.Confidence
	}

	// Find the most voted output
	maxVotes := 0
	var consensus string
	for output, count := range votes {
		if count > maxVotes {
			maxVotes = count
			consensus = output
		}
	}

	// Calculate confidence
	consensusRatio := float64(maxVotes) / float64(len(results))
	averageConfidence := confidenceSum[consensus] / float64(maxVotes)
	overallConfidence := consensusRatio * averageConfidence

	// Check threshold
	reached := consensusRatio >= threshold

	return consensus, overallConfidence, reached
}

// DefaultTeamConfig returns default team configuration
func DefaultTeamConfig() TeamConfig {
	return TeamConfig{
		MaxParallel:        4,
		ConsensusThreshold: 0.5,
		Timeout:            300,
		EnableVoting:       true,
	}
}

// generateTeamID generates a unique team ID.
// D-20: prefix_<unixnano>_<short-uuid> — keeps the human-readable, time-sortable
// prefix while a uuid suffix makes the ID unique-by-construction. The bare
// UnixNano suffix collided when two teams were created within one coarse clock
// tick (see TestIDGenerators_NoCollisionUnderSameTickLoad).
func generateTeamID() string {
	return fmt.Sprintf("team_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
}
