// Package codexskills provides Codex Skills agent integration.
// Codex Skills: Skill-based code generation system.
package codexskills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// CodexSkills provides Codex Skills integration
type CodexSkills struct {
	*base.BaseIntegration
	config *Config
	skills []Skill
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// Skill represents a skill
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
}

// New creates a new Codex Skills integration
func New() *CodexSkills {
	info := agents.AgentInfo{
		Type:        agents.TypeCodexSkills,
		Name:        "Codex Skills",
		Description: "Skill-based code generation",
		Vendor:      "OpenAI",
		Version:     "1.0.0",
		Capabilities: []string{
			"skill_system",
			"reusable_prompts",
			"code_generation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &CodexSkills{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "gpt-4",
		},
		skills: make([]Skill, 0),
	}
}

// Initialize initializes Codex Skills
func (c *CodexSkills) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return c.loadSkills()
}

// loadSkills loads skills
func (c *CodexSkills) loadSkills() error {
	skillsPath := filepath.Join(c.GetWorkDir(), "skills.json")

	if _, err := os.Stat(skillsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(skillsPath)
	if err != nil {
		return fmt.Errorf("read skills: %w", err)
	}

	return json.Unmarshal(data, &c.skills)
}

// saveSkills saves skills
func (c *CodexSkills) saveSkills() error {
	skillsPath := filepath.Join(c.GetWorkDir(), "skills.json")
	data, err := json.MarshalIndent(c.skills, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skills: %w", err)
	}
	return os.WriteFile(skillsPath, data, 0644)
}

// Execute executes a command
func (c *CodexSkills) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "create":
		return c.create(ctx, params)
	case "use":
		return c.use(ctx, params)
	case "list":
		return c.list(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// create creates a skill
func (c *CodexSkills) create(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	prompt, _ := params["prompt"].(string)

	if name == "" || prompt == "" {
		return nil, fmt.Errorf("name and prompt required")
	}

	skill := Skill{
		ID:     fmt.Sprintf("skill-%d", len(c.skills)+1),
		Name:   name,
		Prompt: prompt,
	}

	c.skills = append(c.skills, skill)

	if err := c.saveSkills(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"skill":  skill,
		"status": "created",
	}, nil
}

// use uses a skill
func (c *CodexSkills) use(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	skillID, _ := params["skill_id"].(string)
	if skillID == "" {
		return nil, fmt.Errorf("skill_id required")
	}

	for _, skill := range c.skills {
		if skill.ID == skillID {
			return map[string]interface{}{
				"skill":  skill,
				"result": fmt.Sprintf("Used skill: %s", skill.Name),
			}, nil
		}
	}

	return nil, fmt.Errorf("skill not found: %s", skillID)
}

// list lists skills
func (c *CodexSkills) list(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"skills": c.skills,
		"count":  len(c.skills),
	}, nil
}

// IsAvailable checks availability
func (c *CodexSkills) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*CodexSkills)(nil)
