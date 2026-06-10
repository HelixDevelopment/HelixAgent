// Package promptfoo provides Promptfoo agent integration.
// Promptfoo: LLM testing and evaluation framework.
package promptfoo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Promptfoo provides Promptfoo integration
type Promptfoo struct {
	*base.BaseIntegration
	config *Config
	suites []TestSuite
}

// Config holds Promptfoo configuration
type Config struct {
	base.BaseConfig
	OutputFormat   string
	MaxConcurrency int
}

// TestSuite represents a test suite
type TestSuite struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Prompts     []Prompt `json:"prompts"`
	Tests       []Test   `json:"tests"`
	Status      string   `json:"status"`
}

// Prompt represents a prompt
type Prompt struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Label   string `json:"label"`
}

// Test represents a test case
type Test struct {
	ID       string                 `json:"id"`
	Vars     map[string]interface{} `json:"vars"`
	Assert   []Assertion            `json:"assert"`
	Expected string                 `json:"expected"`
}

// Assertion represents a test assertion
type Assertion struct {
	Type   string  `json:"type"`
	Value  string  `json:"value"`
	Weight float64 `json:"weight"`
}

// New creates a new Promptfoo integration
func New() *Promptfoo {
	info := agents.AgentInfo{
		Type:        agents.TypePromptfoo,
		Name:        "Promptfoo",
		Description: "LLM testing and evaluation framework",
		Vendor:      "Promptfoo",
		Version:     "1.0.0",
		Capabilities: []string{
			"llm_testing",
			"prompt_evaluation",
			"regression_testing",
			"red_teaming",
			"multi_provider",
			"benchmarking",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Promptfoo{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			OutputFormat:   "json",
			MaxConcurrency: 4,
		},
		suites: make([]TestSuite, 0),
	}
}

// Initialize initializes Promptfoo
func (p *Promptfoo) Initialize(ctx context.Context, config interface{}) error {
	if err := p.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		p.config = cfg
	}

	return p.loadSuites()
}

// loadSuites loads test suites
func (p *Promptfoo) loadSuites() error {
	suitesPath := filepath.Join(p.GetWorkDir(), "suites.json")

	if _, err := os.Stat(suitesPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(suitesPath)
	if err != nil {
		return fmt.Errorf("read suites: %w", err)
	}

	// Handle empty file gracefully
	if len(data) == 0 {
		return nil
	}

	return json.Unmarshal(data, &p.suites)
}

// saveSuites saves test suites
func (p *Promptfoo) saveSuites() error {
	suitesPath := filepath.Join(p.GetWorkDir(), "suites.json")
	data, err := json.MarshalIndent(p.suites, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal suites: %w", err)
	}
	return os.WriteFile(suitesPath, data, 0644)
}

// Execute executes a command
func (p *Promptfoo) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !p.IsStarted() {
		if err := p.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "init":
		return p.init(ctx, params)
	case "eval":
		return p.eval(ctx, params)
	case "create_suite":
		return p.createSuite(ctx, params)
	case "add_test":
		return p.addTest(ctx, params)
	case "run_suite":
		return p.runSuite(ctx, params)
	case "list_suites":
		return p.listSuites(ctx)
	case "view":
		return p.view(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// init initializes a project
func (p *Promptfoo) init(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		name = "promptfoo-project"
	}

	return map[string]interface{}{
		"name": name,
		"files": []string{
			"promptfooconfig.yaml",
			"prompts/",
			"tests/",
		},
		"status": "initialized",
	}, nil
}

// eval returns an HONEST error: real evaluation requires invoking the promptfoo
// CLI (`promptfoo eval -c <config>`) against real LLM providers, which this
// integration does not do. The previous implementation returned a hardcoded
// scorecard (tests_run:10, passed:8, score:0.8) for an evaluation that never
// ran — a BLUFF-001 / CONST-035 false-success. Refusing to fabricate eval
// results. See https://www.promptfoo.dev/docs/usage/command-line/ .
func (p *Promptfoo) eval(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	configPath, _ := params["config"].(string)
	if configPath == "" {
		configPath = "promptfooconfig.yaml"
	}
	return nil, fmt.Errorf("promptfoo eval not run: real evaluation requires the promptfoo CLI against real providers (`promptfoo eval -c %s`), not performed here — refusing to fabricate a scorecard (BLUFF-001)", configPath)
}

// createSuite creates a test suite
func (p *Promptfoo) createSuite(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	description, _ := params["description"].(string)

	suite := TestSuite{
		ID:          fmt.Sprintf("suite-%d", len(p.suites)+1),
		Name:        name,
		Description: description,
		Prompts:     []Prompt{},
		Tests:       []Test{},
		Status:      "created",
	}

	p.suites = append(p.suites, suite)

	if err := p.saveSuites(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"suite":  suite,
		"status": "created",
	}, nil
}

// addTest adds a test case
func (p *Promptfoo) addTest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	suiteID, _ := params["suite_id"].(string)
	if suiteID == "" {
		return nil, fmt.Errorf("suite_id required")
	}

	var suite *TestSuite
	for i := range p.suites {
		if p.suites[i].ID == suiteID {
			suite = &p.suites[i]
			break
		}
	}

	if suite == nil {
		return nil, fmt.Errorf("suite not found: %s", suiteID)
	}

	vars, _ := params["vars"].(map[string]interface{})
	expected, _ := params["expected"].(string)

	test := Test{
		ID:       fmt.Sprintf("test-%d", len(suite.Tests)+1),
		Vars:     vars,
		Expected: expected,
		Assert:   []Assertion{},
	}

	suite.Tests = append(suite.Tests, test)

	if err := p.saveSuites(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"test":   test,
		"suite":  suite,
		"status": "added",
	}, nil
}

// runSuite runs a test suite
func (p *Promptfoo) runSuite(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	suiteID, _ := params["suite_id"].(string)
	if suiteID == "" {
		return nil, fmt.Errorf("suite_id required")
	}

	var suite *TestSuite
	for i := range p.suites {
		if p.suites[i].ID == suiteID {
			suite = &p.suites[i]
			break
		}
	}

	if suite == nil {
		return nil, fmt.Errorf("suite not found: %s", suiteID)
	}

	// HONEST error: actually running the suite requires the promptfoo CLI to call
	// real LLM providers and grade their responses. The previous implementation
	// returned passed:true / score:1.0 for EVERY test without running anything —
	// a BLUFF-001 / CONST-035 false-success (the strongest bluff: an always-green
	// scorecard). Refusing to fabricate per-test results.
	return nil, fmt.Errorf("promptfoo suite %q not run: real grading requires the promptfoo CLI against real providers, not performed here — refusing to fabricate all-pass results (BLUFF-001)", suiteID)
}

// listSuites lists test suites
func (p *Promptfoo) listSuites(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"suites": p.suites,
		"count":  len(p.suites),
	}, nil
}

// view views results
func (p *Promptfoo) view(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"viewer": "Promptfoo Viewer",
		"url":    "http://localhost:15500",
	}, nil
}

// IsAvailable checks availability
func (p *Promptfoo) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Promptfoo)(nil)
