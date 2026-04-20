package handlers

import (
	"context"
	"net/http"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/agentic"
)

// AgenticHandler provides HTTP endpoints for graph-based agentic workflows.
// Endpoints:
//
//	POST /v1/agentic/workflows     - create and execute a workflow
//	GET  /v1/agentic/workflows/:id - get workflow status
type AgenticHandler struct {
	logger    *logrus.Logger
	workflows *safe.Store[string, *workflowRecord]
}

// workflowRecord tracks a workflow execution and its result.
type workflowRecord struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      agentic.WorkflowStatus `json:"status"`
	State       *agentic.WorkflowState `json:"state,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// NewAgenticHandler creates a new agentic handler.
func NewAgenticHandler(logger *logrus.Logger) *AgenticHandler {
	if logger == nil {
		logger = logrus.New()
	}
	return &AgenticHandler{
		logger:    logger,
		workflows: safe.NewStore[string, *workflowRecord](),
	}
}

// CreateWorkflowRequest represents a request to create and execute a workflow.
type CreateWorkflowRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Nodes       []WorkflowNodeRequest  `json:"nodes" binding:"required"`
	Edges       []WorkflowEdgeRequest  `json:"edges"`
	EntryPoint  string                 `json:"entry_point" binding:"required"`
	EndNodes    []string               `json:"end_nodes"`
	Config      *WorkflowConfigRequest `json:"config,omitempty"`
	Input       *WorkflowInputRequest  `json:"input,omitempty"`
}

// WorkflowNodeRequest represents a node in the workflow request.
type WorkflowNodeRequest struct {
	ID   string `json:"id" binding:"required"`
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
}

// WorkflowEdgeRequest represents an edge in the workflow request.
type WorkflowEdgeRequest struct {
	From  string `json:"from" binding:"required"`
	To    string `json:"to" binding:"required"`
	Label string `json:"label,omitempty"`
}

// WorkflowConfigRequest represents optional workflow configuration.
type WorkflowConfigRequest struct {
	MaxIterations        int   `json:"max_iterations,omitempty"`
	TimeoutSeconds       int   `json:"timeout_seconds,omitempty"`
	EnableCheckpoints    *bool `json:"enable_checkpoints,omitempty"`
	EnableSelfCorrection *bool `json:"enable_self_correction,omitempty"`
	MaxRetries           int   `json:"max_retries,omitempty"`
}

// WorkflowInputRequest represents optional input for workflow execution.
type WorkflowInputRequest struct {
	Query   string                 `json:"query,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// CreateWorkflowResponse represents the response after creating a workflow.
type CreateWorkflowResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Status      string                 `json:"status"`
	NodesCount  int                    `json:"nodes_count"`
	EdgesCount  int                    `json:"edges_count"`
	EntryPoint  string                 `json:"entry_point"`
	EndNodes    []string               `json:"end_nodes"`
	History     []NodeExecutionSummary `json:"history,omitempty"`
	Error       string                 `json:"error,omitempty"`
	DurationMs  int64                  `json:"duration_ms"`
	CreatedAt   string                 `json:"created_at"`
	CompletedAt string                 `json:"completed_at,omitempty"`
}

// NodeExecutionSummary summarises a single node execution.
type NodeExecutionSummary struct {
	NodeID     string `json:"node_id"`
	NodeName   string `json:"node_name"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// GetWorkflowResponse represents the response for a workflow status query.
type GetWorkflowResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	History     []NodeExecutionSummary `json:"history,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   string                 `json:"created_at"`
	CompletedAt string                 `json:"completed_at,omitempty"`
}

// CreateWorkflow godoc
// @Summary Create and execute an agentic workflow
// @Description Creates a graph-based workflow from the provided definition and executes it
// @Tags agentic
// @Accept json
// @Produce json
// @Param request body CreateWorkflowRequest true "Workflow definition"
// @Success 200 {object} CreateWorkflowResponse
// @Failure 400 {object} VerifierErrorResponse
// @Failure 500 {object} VerifierErrorResponse
// @Router /api/v1/agentic/workflows [post]
func (h *AgenticHandler) CreateWorkflow(c *gin.Context) {
	var req CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	// Build config
	cfg := agentic.DefaultWorkflowConfig()
	if req.Config != nil {
		if req.Config.MaxIterations > 0 {
			cfg.MaxIterations = req.Config.MaxIterations
		}
		if req.Config.TimeoutSeconds > 0 {
			cfg.Timeout = time.Duration(req.Config.TimeoutSeconds) * time.Second
		}
		if req.Config.EnableCheckpoints != nil {
			cfg.EnableCheckpoints = *req.Config.EnableCheckpoints
		}
		if req.Config.EnableSelfCorrection != nil {
			cfg.EnableSelfCorrection = *req.Config.EnableSelfCorrection
		}
		if req.Config.MaxRetries > 0 {
			cfg.MaxRetries = req.Config.MaxRetries
		}
	}

	wf := agentic.NewWorkflow(req.Name, req.Description, cfg, h.logger)

	// Add nodes
	for _, n := range req.Nodes {
		node := &agentic.Node{
			ID:   n.ID,
			Name: n.Name,
			Type: agentic.NodeType(n.Type),
		}
		if err := wf.AddNode(node); err != nil {
			c.JSON(http.StatusBadRequest, VerifierErrorResponse{
				Error: "failed to add node " + n.ID + ": " + err.Error(),
			})
			return
		}
	}

	// Set entry point
	if err := wf.SetEntryPoint(req.EntryPoint); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{
			Error: "invalid entry_point: " + err.Error(),
		})
		return
	}

	// Add end nodes
	for _, endID := range req.EndNodes {
		if err := wf.AddEndNode(endID); err != nil {
			c.JSON(http.StatusBadRequest, VerifierErrorResponse{
				Error: "invalid end_node " + endID + ": " + err.Error(),
			})
			return
		}
	}

	// Add edges
	for _, e := range req.Edges {
		if err := wf.AddEdge(e.From, e.To, nil, e.Label); err != nil {
			c.JSON(http.StatusBadRequest, VerifierErrorResponse{
				Error: "failed to add edge: " + err.Error(),
			})
			return
		}
	}

	// Build input
	var input *agentic.NodeInput
	if req.Input != nil {
		input = &agentic.NodeInput{
			Query:   req.Input.Query,
			Context: req.Input.Context,
		}
	}

	// Execute workflow
	createdAt := time.Now()
	state, execErr := wf.Execute(context.Background(), input)

	record := &workflowRecord{
		ID:          wf.ID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   createdAt,
	}

	resp := CreateWorkflowResponse{
		ID:         wf.ID,
		Name:       req.Name,
		NodesCount: len(req.Nodes),
		EdgesCount: len(req.Edges),
		EntryPoint: req.EntryPoint,
		EndNodes:   req.EndNodes,
		CreatedAt:  createdAt.Format(time.RFC3339),
	}

	if state != nil {
		resp.Status = string(state.Status)
		record.Status = state.Status
		record.State = state

		if state.EndTime != nil {
			resp.DurationMs = state.EndTime.Sub(state.StartTime).Milliseconds()
			formatted := state.EndTime.Format(time.RFC3339)
			resp.CompletedAt = formatted
			record.CompletedAt = state.EndTime
		}

		// Build history summaries
		for _, exec := range state.History {
			summary := NodeExecutionSummary{
				NodeID:     exec.NodeID,
				NodeName:   exec.NodeName,
				DurationMs: exec.EndTime.Sub(exec.StartTime).Milliseconds(),
			}
			if exec.Error != nil {
				summary.Error = exec.Error.Error()
			}
			resp.History = append(resp.History, summary)
		}
	}

	if execErr != nil {
		resp.Error = execErr.Error()
		record.Error = execErr.Error()
		if state == nil {
			resp.Status = string(agentic.StatusFailed)
			record.Status = agentic.StatusFailed
		}
	}

	// Store record
	h.workflows.Put(wf.ID, record)

	h.logger.WithFields(logrus.Fields{
		"workflow_id": wf.ID,
		"name":        req.Name,
		"status":      resp.Status,
	}).Info("Workflow executed")

	c.JSON(http.StatusOK, resp)
}

// GetWorkflow godoc
// @Summary Get workflow status
// @Description Get the status and execution history of a workflow by ID
// @Tags agentic
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} GetWorkflowResponse
// @Failure 404 {object} VerifierErrorResponse
// @Router /api/v1/agentic/workflows/{id} [get]
func (h *AgenticHandler) GetWorkflow(c *gin.Context) {
	id := c.Param("id")

	record, exists := h.workflows.Get(id)
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{
			Error: "workflow not found: " + id,
		})
		return
	}

	resp := GetWorkflowResponse{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
		Status:      string(record.Status),
		Error:       record.Error,
		CreatedAt:   record.CreatedAt.Format(time.RFC3339),
	}

	if record.CompletedAt != nil {
		resp.CompletedAt = record.CompletedAt.Format(time.RFC3339)
	}

	if record.State != nil {
		for _, exec := range record.State.History {
			summary := NodeExecutionSummary{
				NodeID:     exec.NodeID,
				NodeName:   exec.NodeName,
				DurationMs: exec.EndTime.Sub(exec.StartTime).Milliseconds(),
			}
			if exec.Error != nil {
				summary.Error = exec.Error.Error()
			}
			resp.History = append(resp.History, summary)
		}
	}

	c.JSON(http.StatusOK, resp)
}

// RegisterAgenticRoutes registers agentic workflow routes.
func RegisterAgenticRoutes(r *gin.RouterGroup, h *AgenticHandler) {
	ag := r.Group("/agentic")
	{
		ag.POST("/workflows", h.CreateWorkflow)
		ag.GET("/workflows/:id", h.GetWorkflow)
	}
}
