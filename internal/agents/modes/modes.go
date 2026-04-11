// Package modes provides agent mode functionality
// Inspired by Roo Code's agent modes
package modes

import (
	"context"
	"fmt"
	"strings"
)

// AgentMode represents an agent operating mode
type AgentMode string

const (
	// ModeCode - focused on writing and modifying code
	ModeCode AgentMode = "code"

	// ModeArchitect - focused on system design and architecture
	ModeArchitect AgentMode = "architect"

	// ModeAsk - focused on answering questions
	ModeAsk AgentMode = "ask"

	// ModeDebug - focused on debugging issues
	ModeDebug AgentMode = "debug"

	// ModeTest - focused on writing tests
	ModeTest AgentMode = "test"

	// ModeReview - focused on code review
	ModeReview AgentMode = "review"
)

// ModeConfig defines configuration for an agent mode
type ModeConfig struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Prompt      string          `json:"prompt"`
	Tools       []string        `json:"tools"`
	Permissions map[string]bool `json:"permissions"`
	AutoApprove []string        `json:"auto_approve"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

// Registry manages agent modes
type Registry struct {
	modes map[AgentMode]*ModeConfig
}

// NewRegistry creates a new mode registry
func NewRegistry() *Registry {
	r := &Registry{
		modes: make(map[AgentMode]*ModeConfig),
	}
	r.registerDefaults()
	return r
}

// registerDefaults registers the default modes
func (r *Registry) registerDefaults() {
	r.Register(&ModeConfig{
		Name:        "Code",
		Description: "Write and modify code",
		Prompt: `You are in CODE mode. Focus on:
- Writing clean, efficient code
- Following best practices and conventions
- Making minimal, targeted changes
- Writing tests when appropriate`,
		Tools:       []string{"edit", "read", "search", "terminal"},
		Permissions: map[string]bool{"write": true, "execute": false},
		MaxTokens:   4096,
		Temperature: 0.1,
	})

	r.Register(&ModeConfig{
		Name:        "Architect",
		Description: "Design and plan system architecture",
		Prompt: `You are in ARCHITECT mode. Focus on:
- System design and architecture
- API design and interfaces
- Technology selection and trade-offs
- Documentation and diagrams
- Long-term maintainability`,
		Tools:       []string{"read", "search", "browser"},
		Permissions: map[string]bool{"write": false, "execute": false},
		MaxTokens:   8192,
		Temperature: 0.3,
	})

	r.Register(&ModeConfig{
		Name:        "Ask",
		Description: "Answer questions and explain code",
		Prompt: `You are in ASK mode. Focus on:
- Explaining code and concepts clearly
- Answering questions accurately
- Providing examples and documentation
- Being helpful and educational`,
		Tools:       []string{"read", "search"},
		Permissions: map[string]bool{"write": false, "execute": false},
		MaxTokens:   4096,
		Temperature: 0.3,
	})

	r.Register(&ModeConfig{
		Name:        "Debug",
		Description: "Debug issues and find bugs",
		Prompt: `You are in DEBUG mode. Focus on:
- Identifying root causes
- Analyzing error messages and logs
- Reproducing issues
- Suggesting fixes with explanations`,
		Tools:       []string{"read", "search", "terminal", "browser"},
		Permissions: map[string]bool{"write": true, "execute": true},
		MaxTokens:   4096,
		Temperature: 0.1,
	})

	r.Register(&ModeConfig{
		Name:        "Test",
		Description: "Write and run tests",
		Prompt: `You are in TEST mode. Focus on:
- Writing comprehensive tests
- Test coverage analysis
- Edge case identification
- Mock and fixture creation`,
		Tools:       []string{"edit", "read", "search", "terminal"},
		Permissions: map[string]bool{"write": true, "execute": true},
		MaxTokens:   4096,
		Temperature: 0.2,
	})

	r.Register(&ModeConfig{
		Name:        "Review",
		Description: "Review code for quality and issues",
		Prompt: `You are in REVIEW mode. Focus on:
- Code quality assessment
- Security vulnerability identification
- Performance optimization opportunities
- Style and convention compliance
- Constructive feedback`,
		Tools:       []string{"read", "search"},
		Permissions: map[string]bool{"write": false, "execute": false},
		MaxTokens:   4096,
		Temperature: 0.2,
	})
}

// Register registers a mode
func (r *Registry) Register(config *ModeConfig) {
	mode := AgentMode(strings.ToLower(config.Name))
	r.modes[mode] = config
}

// Get retrieves a mode configuration
func (r *Registry) Get(mode AgentMode) (*ModeConfig, bool) {
	config, ok := r.modes[mode]
	return config, ok
}

// List returns all available modes
func (r *Registry) List() []AgentMode {
	modes := make([]AgentMode, 0, len(r.modes))
	for mode := range r.modes {
		modes = append(modes, mode)
	}
	return modes
}

// ListConfigs returns all mode configurations
func (r *Registry) ListConfigs() []*ModeConfig {
	configs := make([]*ModeConfig, 0, len(r.modes))
	for _, config := range r.modes {
		configs = append(configs, config)
	}
	return configs
}

// HasMode checks if a mode exists
func (r *Registry) HasMode(mode AgentMode) bool {
	_, ok := r.modes[mode]
	return ok
}

// Agent represents an agent with a mode
type Agent struct {
	mode     AgentMode
	registry *Registry
	state    map[string]interface{}
}

// NewAgent creates a new agent with a mode
func NewAgent(registry *Registry, mode AgentMode) (*Agent, error) {
	if !registry.HasMode(mode) {
		return nil, fmt.Errorf("unknown mode: %s", mode)
	}

	return &Agent{
		mode:     mode,
		registry: registry,
		state:    make(map[string]interface{}),
	}, nil
}

// GetMode returns the agent's current mode
func (a *Agent) GetMode() AgentMode {
	return a.mode
}

// SetMode changes the agent's mode
func (a *Agent) SetMode(mode AgentMode) error {
	if !a.registry.HasMode(mode) {
		return fmt.Errorf("unknown mode: %s", mode)
	}
	a.mode = mode
	return nil
}

// GetConfig returns the current mode configuration
func (a *Agent) GetConfig() *ModeConfig {
	config, _ := a.registry.Get(a.mode)
	return config
}

// CanUseTool checks if the current mode allows a tool
func (a *Agent) CanUseTool(tool string) bool {
	config := a.GetConfig()
	if config == nil {
		return false
	}

	for _, t := range config.Tools {
		if t == tool {
			return true
		}
	}
	return false
}

// CanExecute checks if the current mode allows execution
func (a *Agent) CanExecute(permission string) bool {
	config := a.GetConfig()
	if config == nil {
		return false
	}

	allowed, ok := config.Permissions[permission]
	return ok && allowed
}

// GetPrompt returns the mode prompt
func (a *Agent) GetPrompt() string {
	config := a.GetConfig()
	if config == nil {
		return ""
	}
	return config.Prompt
}

// GetTemperature returns the mode temperature
func (a *Agent) GetTemperature() float64 {
	config := a.GetConfig()
	if config == nil {
		return 0.3
	}
	return config.Temperature
}

// GetMaxTokens returns the mode max tokens
func (a *Agent) GetMaxTokens() int {
	config := a.GetConfig()
	if config == nil {
		return 4096
	}
	return config.MaxTokens
}

// ModeContext wraps context with mode information
type ModeContext struct {
	context.Context
	Mode   AgentMode
	Config *ModeConfig
}

// WithMode adds mode to context
func WithMode(ctx context.Context, agent *Agent) context.Context {
	return &ModeContext{
		Context: ctx,
		Mode:    agent.mode,
		Config:  agent.GetConfig(),
	}
}

// GetModeFromContext retrieves mode from context
func GetModeFromContext(ctx context.Context) (AgentMode, bool) {
	if mc, ok := ctx.(*ModeContext); ok {
		return mc.Mode, true
	}
	return "", false
}

// SwitchMode switches agent mode and returns a new context
func (a *Agent) SwitchMode(ctx context.Context, newMode AgentMode) (context.Context, error) {
	if err := a.SetMode(newMode); err != nil {
		return ctx, err
	}
	return WithMode(ctx, a), nil
}

// String returns string representation of mode
func (m AgentMode) String() string {
	return string(m)
}

// IsValid checks if mode is valid
func (m AgentMode) IsValid() bool {
	switch m {
	case ModeCode, ModeArchitect, ModeAsk, ModeDebug, ModeTest, ModeReview:
		return true
	}
	return false
}
