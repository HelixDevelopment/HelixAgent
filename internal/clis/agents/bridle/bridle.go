// Package bridle provides Bridle CLI agent integration.
// Bridle: AI agent framework for structured task execution with guardrails.
package bridle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Bridle provides Bridle CLI integration
type Bridle struct {
	*base.BaseIntegration
	config     *Config
	workflows  []Workflow
	guardrails []Guardrail
}

// Config holds Bridle configuration
type Config struct {
	base.BaseConfig
	StrictMode   bool
	AutoApprove  bool
	MaxRetries   int
	Timeout      int
	WorkspaceDir string
}

// Workflow represents a Bridle workflow
type Workflow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Steps     []Step `json:"steps"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// Step represents a workflow step
type Step struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Action     string                 `json:"action"`
	Params     map[string]interface{} `json:"params"`
	Guardrails []string               `json:"guardrails"`
	Status     string                 `json:"status"`
}

// Guardrail represents a safety guardrail
type Guardrail struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"` // "safety", "quality", "compliance"
	Rule    string `json:"rule"`
	Action  string `json:"action"` // "block", "warn", "log"
	Enabled bool   `json:"enabled"`
}

// New creates a new Bridle integration
func New() *Bridle {
	info := agents.AgentInfo{
		Type:        agents.TypeBridle,
		Name:        "Bridle",
		Description: "AI agent framework with guardrails",
		Vendor:      "Bridle",
		Version:     "1.0.0",
		Capabilities: []string{
			"structured_workflows",
			"guardrails",
			"safety_checks",
			"quality_gates",
			"approval_workflows",
			"audit_logging",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Bridle{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			StrictMode:  true,
			AutoApprove: false,
			MaxRetries:  3,
			Timeout:     300,
		},
		workflows:  make([]Workflow, 0),
		guardrails: make([]Guardrail, 0),
	}
}

// Initialize initializes Bridle
func (b *Bridle) Initialize(ctx context.Context, config interface{}) error {
	if err := b.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		b.config = cfg
	}

	if b.config.WorkspaceDir == "" {
		b.config.WorkspaceDir = b.GetWorkDir()
	}

	return b.loadConfig()
}

// loadConfig loads configuration
func (b *Bridle) loadConfig() error {
	configPath := filepath.Join(b.config.WorkspaceDir, "bridle.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return b.createDefaultConfig()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var config struct {
		Workflows  []Workflow  `json:"workflows"`
		Guardrails []Guardrail `json:"guardrails"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	b.workflows = config.Workflows
	b.guardrails = config.Guardrails
	return nil
}

// createDefaultConfig creates default configuration
func (b *Bridle) createDefaultConfig() error {
	b.guardrails = []Guardrail{
		{ID: "safety-1", Name: "No Secrets", Type: "safety", Rule: "no_api_keys_in_output", Action: "block", Enabled: true},
		{ID: "quality-1", Name: "Code Quality", Type: "quality", Rule: "pass_linting", Action: "warn", Enabled: true},
		{ID: "compliance-1", Name: "License Check", Type: "compliance", Rule: "compatible_license", Action: "block", Enabled: true},
	}
	return b.saveConfig()
}

// saveConfig saves configuration
func (b *Bridle) saveConfig() error {
	configPath := filepath.Join(b.config.WorkspaceDir, "bridle.json")
	data, err := json.MarshalIndent(struct {
		Workflows  []Workflow  `json:"workflows"`
		Guardrails []Guardrail `json:"guardrails"`
	}{Workflows: b.workflows, Guardrails: b.guardrails}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

// Execute executes a command
func (b *Bridle) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !b.IsStarted() {
		if err := b.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "run_workflow":
		return b.runWorkflow(ctx, params)
	case "create_workflow":
		return b.createWorkflow(ctx, params)
	case "add_guardrail":
		return b.addGuardrail(ctx, params)
	case "check_compliance":
		return b.checkCompliance(ctx, params)
	case "list_guardrails":
		return b.listGuardrails(ctx)
	case "validate":
		return b.validate(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// runWorkflow executes a workflow
func (b *Bridle) runWorkflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	workflowID, _ := params["workflow_id"].(string)
	if workflowID == "" {
		return nil, fmt.Errorf("workflow_id required")
	}

	// Find workflow
	var workflow *Workflow
	for i := range b.workflows {
		if b.workflows[i].ID == workflowID {
			workflow = &b.workflows[i]
			break
		}
	}

	if workflow == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Evaluate workflow steps against guardrails.
	//
	// Anti-bluff (BLUFF-001, D-18): this method does NOT dispatch step.Action —
	// there is no action runner wired. It only EVALUATES each step's guardrails.
	// Reporting "completed" would falsely claim each step's action ran. The
	// honest per-step state is "evaluated" (guardrails checked, no action run),
	// or "blocked" when a guardrail violation trips strict mode. The overall
	// workflow status is "blocked" if any step blocked, else "evaluated".
	results := make([]map[string]interface{}, 0, len(workflow.Steps))
	anyBlocked := false

	for _, step := range workflow.Steps {
		// Check guardrails
		violations := b.checkGuardrails(step)

		result := map[string]interface{}{
			"step_id":    step.ID,
			"step_name":  step.Name,
			"status":     "evaluated",
			"violations": violations,
		}

		if len(violations) > 0 && b.config.StrictMode {
			result["status"] = "blocked"
			anyBlocked = true
		}

		results = append(results, result)
	}

	workflowStatus := "evaluated"
	if anyBlocked {
		workflowStatus = "blocked"
	}

	return map[string]interface{}{
		"workflow_id": workflowID,
		"results":     results,
		"status":      workflowStatus,
	}, nil
}

// checkGuardrails checks guardrails for a step
func (b *Bridle) checkGuardrails(step Step) []string {
	violations := make([]string, 0)

	for _, guardrailID := range step.Guardrails {
		for _, guardrail := range b.guardrails {
			if guardrail.ID == guardrailID && guardrail.Enabled {
				// Check if violated (simplified)
				if guardrail.Action == "block" {
					violations = append(violations, guardrail.Name)
				}
			}
		}
	}

	return violations
}

// createWorkflow creates a new workflow
func (b *Bridle) createWorkflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	stepsData, _ := params["steps"].([]interface{})
	steps := make([]Step, 0, len(stepsData))

	for i, stepData := range stepsData {
		if stepMap, ok := stepData.(map[string]interface{}); ok {
			step := Step{
				ID:     fmt.Sprintf("step-%d", i+1),
				Name:   stepMap["name"].(string),
				Action: stepMap["action"].(string),
				Params: stepMap["params"].(map[string]interface{}),
				Status: "pending",
			}
			steps = append(steps, step)
		}
	}

	workflow := Workflow{
		ID:     fmt.Sprintf("workflow-%d", len(b.workflows)+1),
		Name:   name,
		Steps:  steps,
		Status: "created",
	}

	b.workflows = append(b.workflows, workflow)

	if err := b.saveConfig(); err != nil {
		return nil, err
	}

	return map[string]interface{}{"workflow": workflow, "status": "created"}, nil
}

// addGuardrail adds a guardrail
func (b *Bridle) addGuardrail(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	guardrailType, _ := params["type"].(string)
	rule, _ := params["rule"].(string)
	action, _ := params["action"].(string)

	if name == "" || guardrailType == "" || rule == "" {
		return nil, fmt.Errorf("name, type, and rule required")
	}

	guardrail := Guardrail{
		ID:      fmt.Sprintf("guardrail-%d", len(b.guardrails)+1),
		Name:    name,
		Type:    guardrailType,
		Rule:    rule,
		Action:  action,
		Enabled: true,
	}

	b.guardrails = append(b.guardrails, guardrail)

	if err := b.saveConfig(); err != nil {
		return nil, err
	}

	return map[string]interface{}{"guardrail": guardrail, "status": "added"}, nil
}

// checkCompliance checks compliance
func (b *Bridle) checkCompliance(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	content, _ := params["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("content required")
	}

	violations := make([]string, 0)
	passed := make([]string, 0)

	for _, guardrail := range b.guardrails {
		if !guardrail.Enabled {
			continue
		}

		// Simplified compliance check
		if guardrail.Type == "safety" {
			violations = append(violations, guardrail.Name)
		} else {
			passed = append(passed, guardrail.Name)
		}
	}

	return map[string]interface{}{
		"content":    content,
		"violations": violations,
		"passed":     passed,
		"compliant":  len(violations) == 0,
	}, nil
}

// listGuardrails lists guardrails
func (b *Bridle) listGuardrails(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"guardrails": b.guardrails,
		"count":      len(b.guardrails),
	}, nil
}

// validate validates input against guardrails
func (b *Bridle) validate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	input, _ := params["input"].(string)
	if input == "" {
		return nil, fmt.Errorf("input required")
	}

	// Run validation
	isValid := true
	messages := make([]string, 0)

	for _, guardrail := range b.guardrails {
		if guardrail.Enabled && guardrail.Action == "block" {
			// Simplified validation
			isValid = false
			messages = append(messages, fmt.Sprintf("Failed: %s", guardrail.Name))
		}
	}

	return map[string]interface{}{
		"input":    input,
		"valid":    isValid,
		"messages": messages,
	}, nil
}

// IsAvailable checks availability
func (b *Bridle) IsAvailable() bool {
	_, err := exec.LookPath("bridle")
	return err == nil
}

var _ agents.AgentIntegration = (*Bridle)(nil)
