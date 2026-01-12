package agents

// CLIAgent represents a CLI coding agent that can integrate with HelixAgent
type CLIAgent struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Language       string            `json:"language"`
	ConfigFormat   string            `json:"config_format"`
	APIPattern     string            `json:"api_pattern"`
	EntryPoint     string            `json:"entry_point"`
	Features       []string          `json:"features"`
	ToolSupport    []string          `json:"tool_support"`
	Protocols      []string          `json:"protocols"`
	ConfigLocation string            `json:"config_location"`
	EnvVars        map[string]string `json:"env_vars"`
	SystemPrompt   string            `json:"system_prompt"`
}

// CLIAgentRegistry contains all supported CLI agents
var CLIAgentRegistry = map[string]*CLIAgent{
	// Already supported agents
	"OpenCode": {
		Name:           "OpenCode",
		Description:    "OpenCode AI coding assistant",
		Language:       "Go",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "opencode",
		Features:       []string{"code-completion", "chat", "tool-use", "mcp"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Test", "Lint"},
		Protocols:      []string{"OpenAI", "MCP"},
		ConfigLocation: "~/.config/opencode/opencode.json",
		SystemPrompt:   "You are an AI coding assistant. Help the user with their coding questions.",
	},
	"Crush": {
		Name:           "Crush",
		Description:    "Terminal-based AI assistant for shell commands",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "crush",
		Features:       []string{"shell-commands", "terminal-integration", "streaming"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Glob", "Grep"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/crush/config.json",
		SystemPrompt:   "You are Crush, a terminal-based AI assistant. You help with shell commands and system tasks.",
	},
	"HelixCode": {
		Name:           "HelixCode",
		Description:    "Distributed AI development platform assistant",
		Language:       "Go",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "helixcode",
		Features:       []string{"architecture", "distributed-ai", "ensemble"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Test", "Lint", "Task"},
		Protocols:      []string{"OpenAI", "MCP", "ACP"},
		ConfigLocation: "~/.config/helixcode/config.json",
		SystemPrompt:   "You are the HelixCode distributed AI development platform assistant. You help with software architecture and coding.",
	},
	"Kiro": {
		Name:           "Kiro",
		Description:    "AI coding agent with comprehensive tool support",
		Language:       "Python",
		ConfigFormat:   "YAML",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "kiro",
		Features:       []string{"3-phase-methodology", "steering-files", "multi-ai-workflows"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Test", "Lint", "PR", "Issue", "Workflow"},
		Protocols:      []string{"OpenAI", "MCP"},
		ConfigLocation: "~/.kiro/steering/",
		SystemPrompt:   "You are Kiro, an AI coding agent that helps developers write better code. You have access to tools for code analysis, git operations, and testing.",
	},

	// New agents from HelixCode/Example_Projects
	"Aider": {
		Name:           "Aider",
		Description:    "AI pair programming in your terminal",
		Language:       "Python",
		ConfigFormat:   "TOML",
		APIPattern:     "Multi-provider",
		EntryPoint:     "aider",
		Features:       []string{"git-integration", "auto-commits", "codebase-mapping", "voice-to-code", "image-support"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Diff"},
		Protocols:      []string{"OpenAI", "Anthropic", "DeepSeek", "Gemini"},
		ConfigLocation: "~/.aider.conf.yml",
		EnvVars:        map[string]string{"AIDER_MODEL": "gpt-4", "OPENAI_API_KEY": ""},
		SystemPrompt:   "You are Aider, an AI pair programmer. Help the user edit their code with git-integrated changes.",
	},
	"ClaudeCode": {
		Name:           "ClaudeCode",
		Description:    "Anthropic's official CLI for Claude",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "Anthropic",
		EntryPoint:     "claude",
		Features:       []string{"codebase-understanding", "git-workflow", "plugin-system", "github-integration"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Task"},
		Protocols:      []string{"Anthropic", "MCP"},
		ConfigLocation: "~/.claude/",
		EnvVars:        map[string]string{"ANTHROPIC_API_KEY": ""},
		SystemPrompt:   "You are Claude Code, Anthropic's official CLI for Claude. You are an interactive CLI tool that helps users with software engineering tasks.",
	},
	"Cline": {
		Name:           "Cline",
		Description:    "Autonomous coding agent for VS Code",
		Language:       "TypeScript",
		ConfigFormat:   "Proto/gRPC",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "cline",
		Features:       []string{"vscode-extension", "browser-interaction", "autonomous-agent", "multi-model"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "WebFetch", "Symbols", "References", "Definition"},
		Protocols:      []string{"OpenAI", "MCP", "gRPC"},
		ConfigLocation: "~/.cline/config.json",
		SystemPrompt:   "You are Cline, an autonomous coding agent. You can browse the web, interact with files, and execute commands to help the user.",
	},
	"CodenameGoose": {
		Name:           "CodenameGoose",
		Description:    "Profile-based AI coding assistant in Rust",
		Language:       "Rust",
		ConfigFormat:   "YAML",
		APIPattern:     "Multi-provider",
		EntryPoint:     "goose",
		Features:       []string{"profile-system", "extension-plugins", "multi-model-per-profile", "ripgrep-integration"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "TreeView"},
		Protocols:      []string{"OpenAI", "Anthropic"},
		ConfigLocation: "~/.config/goose/profile.yaml",
		SystemPrompt:   "You are Goose, an AI coding assistant with profile-based configuration. Help the user with their coding tasks.",
	},
	"DeepSeekCLI": {
		Name:           "DeepSeekCLI",
		Description:    "DeepSeek AI coding assistant CLI",
		Language:       "TypeScript",
		ConfigFormat:   "ENV",
		APIPattern:     "DeepSeek/Ollama",
		EntryPoint:     "deepseek",
		Features:       []string{"local-model-support", "ollama-integration", "cloud-api"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"},
		Protocols:      []string{"DeepSeek", "Ollama"},
		ConfigLocation: "~/.deepseek/.env",
		EnvVars:        map[string]string{"DEEPSEEK_API_KEY": "", "DEEPSEEK_USE_LOCAL": "false", "OLLAMA_HOST": "http://localhost:11434"},
		SystemPrompt:   "You are DeepSeek CLI, an AI-powered coding assistant. Help the user with code generation and analysis.",
	},
	"Forge": {
		Name:           "Forge",
		Description:    "Workflow-based AI agent orchestration",
		Language:       "Rust",
		ConfigFormat:   "YAML+JSON Schema",
		APIPattern:     "Multi-provider",
		EntryPoint:     "forge",
		Features:       []string{"workflow-orchestration", "multi-agent", "context-compaction", "tool-failure-tracking"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Test", "Lint", "Task"},
		Protocols:      []string{"OpenAI", "Anthropic"},
		ConfigLocation: "~/.config/forge/forge.yaml",
		SystemPrompt:   "You are Forge, an AI agent orchestrator. Execute workflows with multiple agents and tools.",
	},
	"GeminiCLI": {
		Name:           "GeminiCLI",
		Description:    "Google Gemini CLI coding assistant",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "Google",
		EntryPoint:     "gemini",
		Features:       []string{"gemini-api", "gcp-integration", "docker-support"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git"},
		Protocols:      []string{"Gemini"},
		ConfigLocation: "~/.config/gemini/config.json",
		EnvVars:        map[string]string{"GOOGLE_API_KEY": "", "GOOGLE_PROJECT_ID": ""},
		SystemPrompt:   "You are Gemini CLI, a Google AI coding assistant. Help the user with their coding questions.",
	},
	"GPTEngineer": {
		Name:           "GPTEngineer",
		Description:    "End-to-end code generation agent",
		Language:       "Python",
		ConfigFormat:   "YAML",
		APIPattern:     "OpenAI",
		EntryPoint:     "gpt-engineer",
		Features:       []string{"project-scaffolding", "multi-file-generation", "full-project-creation"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Git"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/gpt-engineer/config.yaml",
		EnvVars:        map[string]string{"OPENAI_API_KEY": ""},
		SystemPrompt:   "You are GPT Engineer, an end-to-end code generation agent. Generate complete projects from specifications.",
	},
	"KiloCode": {
		Name:           "KiloCode",
		Description:    "Multi-provider AI coding assistant with 50+ LLM support",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "Multi-provider",
		EntryPoint:     "kilo",
		Features:       []string{"50+-providers", "vscode-extension", "jetbrains-plugin", "mcp-support", "turbo-monorepo"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Test", "Lint", "Diff", "TreeView", "FileInfo", "Symbols", "References", "Definition", "PR", "Issue", "Workflow"},
		Protocols:      []string{"OpenAI", "Anthropic", "OpenRouter", "AWS", "GCP", "Azure", "MCP"},
		ConfigLocation: "~/.config/kilo-code/config.json",
		SystemPrompt:   "You are Kilo Code, a multi-provider AI coding assistant supporting 50+ LLM providers.",
	},
	"MistralCode": {
		Name:           "MistralCode",
		Description:    "Mistral AI coding assistant CLI",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "Mistral",
		EntryPoint:     "mistral",
		Features:       []string{"mistral-api", "code-generation", "code-explanation"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"},
		Protocols:      []string{"Mistral"},
		ConfigLocation: "~/.config/mistral/config.json",
		EnvVars:        map[string]string{"MISTRAL_API_KEY": ""},
		SystemPrompt:   "You are Mistral Code, a Mistral AI coding assistant. Help the user with code generation and analysis.",
	},
	"OllamaCode": {
		Name:           "OllamaCode",
		Description:    "Local LLM coding assistant via Ollama",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "Ollama",
		EntryPoint:     "ollama-code",
		Features:       []string{"local-models", "no-cloud-api", "privacy-focused"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git"},
		Protocols:      []string{"Ollama"},
		ConfigLocation: "~/.config/ollama-code/config.json",
		EnvVars:        map[string]string{"OLLAMA_HOST": "http://localhost:11434", "OLLAMA_MODEL": "codellama"},
		SystemPrompt:   "You are Ollama Code, a local AI coding assistant. Help the user without sending data to the cloud.",
	},
	"Plandex": {
		Name:           "Plandex",
		Description:    "Plan-based AI development workflow",
		Language:       "Go",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "plandex",
		Features:       []string{"plan-based-development", "multi-file-changes", "version-control"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Git", "Task"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.plandex/config.json",
		EnvVars:        map[string]string{"OPENAI_API_KEY": ""},
		SystemPrompt:   "You are Plandex, a plan-based AI development assistant. Help the user plan and implement changes.",
	},
	"QwenCode": {
		Name:           "QwenCode",
		Description:    "Alibaba Qwen AI coding assistant",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "Qwen",
		EntryPoint:     "qwen",
		Features:       []string{"qwen-api", "code-generation", "localization"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git"},
		Protocols:      []string{"Qwen"},
		ConfigLocation: "~/.config/qwen/config.json",
		EnvVars:        map[string]string{"QWEN_API_KEY": "", "DASHSCOPE_API_KEY": ""},
		SystemPrompt:   "You are Qwen Code, an Alibaba AI coding assistant. Help the user with code generation and analysis.",
	},
	"AmazonQ": {
		Name:           "AmazonQ",
		Description:    "Amazon Q Developer CLI with MCP support",
		Language:       "Rust",
		ConfigFormat:   "JSON",
		APIPattern:     "AWS",
		EntryPoint:     "q",
		Features:       []string{"mcp-servers", "built-in-tools", "aws-integration", "knowledge-base"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Test", "Lint", "WebFetch", "Task"},
		Protocols:      []string{"AWS", "MCP"},
		ConfigLocation: "~/.aws/q/config.json",
		EnvVars:        map[string]string{"AWS_REGION": "", "AWS_PROFILE": ""},
		SystemPrompt:   "You are Amazon Q Developer, an AI assistant for software development with AWS integration.",
	},
}

// GetAgent returns a CLI agent by name (case-insensitive)
func GetAgent(name string) (*CLIAgent, bool) {
	// Try exact match first
	if agent, ok := CLIAgentRegistry[name]; ok {
		return agent, true
	}
	// Try case-insensitive match
	for key, agent := range CLIAgentRegistry {
		if equalFold(key, name) {
			return agent, true
		}
	}
	return nil, false
}

// GetAllAgents returns all registered CLI agents
func GetAllAgents() []*CLIAgent {
	agents := make([]*CLIAgent, 0, len(CLIAgentRegistry))
	for _, agent := range CLIAgentRegistry {
		agents = append(agents, agent)
	}
	return agents
}

// GetAgentNames returns all agent names
func GetAgentNames() []string {
	names := make([]string, 0, len(CLIAgentRegistry))
	for name := range CLIAgentRegistry {
		names = append(names, name)
	}
	return names
}

// equalFold is a simple case-insensitive string comparison
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Category constants for agents
const (
	CategoryTerminal   = "terminal"
	CategoryVSCode     = "vscode"
	CategoryJetBrains  = "jetbrains"
	CategoryStandalone = "standalone"
	CategoryCloud      = "cloud"
	CategoryLocal      = "local"
)

// GetAgentsByProtocol returns agents that support a specific protocol
func GetAgentsByProtocol(protocol string) []*CLIAgent {
	var agents []*CLIAgent
	for _, agent := range CLIAgentRegistry {
		for _, p := range agent.Protocols {
			if equalFold(p, protocol) {
				agents = append(agents, agent)
				break
			}
		}
	}
	return agents
}

// GetAgentsByTool returns agents that support a specific tool
func GetAgentsByTool(tool string) []*CLIAgent {
	var agents []*CLIAgent
	for _, agent := range CLIAgentRegistry {
		for _, t := range agent.ToolSupport {
			if equalFold(t, tool) {
				agents = append(agents, agent)
				break
			}
		}
	}
	return agents
}
