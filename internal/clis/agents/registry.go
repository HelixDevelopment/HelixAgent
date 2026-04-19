// Package agents provides a unified registry for all CLI agent integrations.
package agents

import (
	"context"
	"fmt"
	"sync"

	"digital.vasic.concurrency/pkg/safe"
)

// AgentType represents the type of CLI agent
type AgentType string

// All CLI agent types
const (
	TypeAider            AgentType = "aider"
	TypeClaudeCode       AgentType = "claude_code"
	TypeCodex            AgentType = "codex"
	TypeOpenHands        AgentType = "openhands"
	TypeCline            AgentType = "cline"
	TypeGeminiCLI        AgentType = "gemini_cli"
	TypeAmazonQ          AgentType = "amazon_q"
	TypeKiro             AgentType = "kiro"
	TypeContinue         AgentType = "continue"
	TypeAgentDeck        AgentType = "agent_deck"
	TypeBridle           AgentType = "bridle"
	TypeClaudePlugins    AgentType = "claude_plugins"
	TypeClaudeSquad      AgentType = "claude_squad"
	TypeCodai            AgentType = "codai"
	TypeCodenameGoose    AgentType = "codename_goose"
	TypeCodexSkills      AgentType = "codex_skills"
	TypeConduit          AgentType = "conduit"
	TypeCopilotCLI       AgentType = "copilot_cli"
	TypeCrush            AgentType = "crush"
	TypeDeepseekCLI      AgentType = "deepseek_cli"
	TypeFauxpilot        AgentType = "fauxpilot"
	TypeForge            AgentType = "forge"
	TypeGetShitDone      AgentType = "get_shit_done"
	TypeGitMCP           AgentType = "git_mcp"
	TypeGptEngineer      AgentType = "gpt_engineer"
	TypeGptme            AgentType = "gptme"
	TypeJunie            AgentType = "junie"
	TypeKiloCode         AgentType = "kilo_code"
	TypeMistralCode      AgentType = "mistral_code"
	TypeMobileAgent      AgentType = "mobile_agent"
	TypeMultiagentCoding AgentType = "multiagent_coding"
	TypeNanocoder        AgentType = "nanocoder"
	TypeNoi              AgentType = "noi"
	TypeOctogen          AgentType = "octogen"
	TypeOllamaCode       AgentType = "ollama_code"
	TypeOpencodeCLI      AgentType = "opencode_cli"
	TypePlandex          AgentType = "plandex"
	TypePostgresMCP      AgentType = "postgres_mcp"
	TypeQwenCode         AgentType = "qwen_code"
	TypeShai             AgentType = "shai"
	TypeSnowCLI          AgentType = "snow_cli"
	TypeSpecKit          AgentType = "spec_kit"
	TypeSuperset         AgentType = "superset"
	TypeTaskweaver       AgentType = "taskweaver"
	TypeUIUXProMax       AgentType = "ui_ux_pro_max"
	TypeVtcode           AgentType = "vtcode"
	TypeWarp             AgentType = "warp"
	// Additional agent types
	TypeCursor           AgentType = "cursor"
	TypeKodu             AgentType = "kodu"
	TypeLovable          AgentType = "lovable"
	TypeGitHubSpark      AgentType = "github_spark"
	TypeSupermaven       AgentType = "supermaven"
	TypeCody             AgentType = "cody"
	TypeTabnine          AgentType = "tabnine"
	TypeJetBrainsAI      AgentType = "jetbrains_ai"
	TypeCopilotWorkspace AgentType = "copilot_workspace"
	TypeDeepSeek         AgentType = "deepseek"
	TypePromptfoo        AgentType = "promptfoo"
	TypeSmolagents       AgentType = "smolagents"
	TypeGPTR             AgentType = "gptr"
	TypeHoneycomb        AgentType = "honeycomb"
	TypeHunyuan          AgentType = "hunyuan"
	TypeCodeiumWindsurf  AgentType = "codeium_windsurf"
	TypeKimi             AgentType = "kimi"
	TypeWindsurf         AgentType = "windsurf"
	TypePerplexity       AgentType = "perplexity"
)

// AllAgentTypes returns all supported agent types
func AllAgentTypes() []AgentType {
	return []AgentType{
		TypeAider, TypeClaudeCode, TypeCodex, TypeOpenHands, TypeCline,
		TypeGeminiCLI, TypeAmazonQ, TypeKiro, TypeContinue, TypeAgentDeck,
		TypeBridle, TypeClaudePlugins, TypeClaudeSquad, TypeCodai,
		TypeCodenameGoose, TypeCodexSkills, TypeConduit, TypeCopilotCLI,
		TypeCrush, TypeDeepseekCLI, TypeFauxpilot, TypeForge, TypeGetShitDone,
		TypeGitMCP, TypeGptEngineer, TypeGptme, TypeJunie, TypeKiloCode,
		TypeMistralCode, TypeMobileAgent, TypeMultiagentCoding, TypeNanocoder,
		TypeNoi, TypeOctogen, TypeOllamaCode, TypeOpencodeCLI, TypePlandex,
		TypePostgresMCP, TypeQwenCode, TypeShai, TypeSnowCLI, TypeSpecKit,
		TypeSuperset, TypeTaskweaver, TypeUIUXProMax, TypeVtcode, TypeWarp,
		TypeCursor, TypeKodu, TypeLovable, TypeGitHubSpark, TypeSupermaven,
		TypeCody, TypeTabnine, TypeJetBrainsAI, TypeCopilotWorkspace, TypeDeepSeek,
		TypePromptfoo, TypeSmolagents, TypeGPTR, TypeHoneycomb, TypeHunyuan,
		TypeCodeiumWindsurf, TypeKimi, TypeWindsurf, TypePerplexity,
	}
}

// AgentInfo holds information about an agent
type AgentInfo struct {
	Type         AgentType
	Name         string
	Description  string
	Vendor       string
	Version      string
	Capabilities []string
	IsEnabled    bool
	Priority     int
}

// AgentIntegration defines the interface for CLI agent integrations
type AgentIntegration interface {
	Info() AgentInfo
	Initialize(ctx context.Context, config interface{}) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error)
	Health(ctx context.Context) error
	IsAvailable() bool
}

// Registry manages all agent integrations.
// Both stores are Pattern Alpha — AgentIntegration instances and
// the started bools are immutable at each individual entry level.
type Registry struct {
	agents  *safe.Store[AgentType, AgentIntegration]
	started *safe.Store[AgentType, bool]
}

// NewRegistry creates a new agent registry
func NewRegistry() *Registry {
	return &Registry{
		agents:  safe.NewStore[AgentType, AgentIntegration](),
		started: safe.NewStore[AgentType, bool](),
	}
}

// Register registers an agent integration
func (r *Registry) Register(agent AgentIntegration) error {
	info := agent.Info()
	if _, stored := r.agents.PutIfAbsent(info.Type, agent); !stored {
		return fmt.Errorf("agent %s already registered", info.Type)
	}
	return nil
}

// Get retrieves an agent integration
func (r *Registry) Get(agentType AgentType) (AgentIntegration, bool) {
	return r.agents.Get(agentType)
}

// List returns all registered agents
func (r *Registry) List() []AgentInfo {
	var infos []AgentInfo
	r.agents.Range(func(_ AgentType, agent AgentIntegration) bool {
		infos = append(infos, agent.Info())
		return true
	})
	return infos
}

// ListAvailable returns all available agents
func (r *Registry) ListAvailable() []AgentInfo {
	var infos []AgentInfo
	r.agents.Range(func(_ AgentType, agent AgentIntegration) bool {
		if agent.IsAvailable() {
			infos = append(infos, agent.Info())
		}
		return true
	})
	return infos
}

// StartAll starts all registered agents
func (r *Registry) StartAll(ctx context.Context) []error {
	var errs []error
	for _, agentType := range r.agents.Keys() {
		agent, ok := r.agents.Get(agentType)
		if !ok {
			continue
		}
		if err := agent.Start(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to start %s: %w", agentType, err))
		} else {
			r.started.Put(agentType, true)
		}
	}
	return errs
}

// StopAll stops all registered agents
func (r *Registry) StopAll(ctx context.Context) []error {
	var errs []error
	for _, agentType := range r.agents.Keys() {
		agent, ok := r.agents.Get(agentType)
		if !ok {
			continue
		}
		if err := agent.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop %s: %w", agentType, err))
		} else {
			r.started.Delete(agentType)
		}
	}
	return errs
}

// Execute executes a command on a specific agent
func (r *Registry) Execute(ctx context.Context, agentType AgentType, command string, params map[string]interface{}) (interface{}, error) {
	agent, ok := r.agents.Get(agentType)
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentType)
	}
	return agent.Execute(ctx, command, params)
}

// HealthCheck checks health of all agents
func (r *Registry) HealthCheck(ctx context.Context) map[AgentType]error {
	results := make(map[AgentType]error)
	r.agents.Range(func(agentType AgentType, agent AgentIntegration) bool {
		results[agentType] = agent.Health(ctx)
		return true
	})
	return results
}

// GetStats returns statistics about the registry
func (r *Registry) GetStats() map[string]interface{} {
	available := 0
	startedCount := 0
	total := 0

	r.agents.Range(func(agentType AgentType, agent AgentIntegration) bool {
		total++
		if agent.IsAvailable() {
			available++
		}
		if isStarted, ok := r.started.Get(agentType); ok && isStarted {
			startedCount++
		}
		return true
	})

	return map[string]interface{}{
		"total":     total,
		"available": available,
		"started":   startedCount,
	}
}

// Global registry instance
var (
	globalRegistry *Registry
	once           sync.Once
)

// GetGlobalRegistry returns the global registry instance
func GetGlobalRegistry() *Registry {
	once.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}
