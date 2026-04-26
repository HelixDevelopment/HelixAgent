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
		ConfigLocation: "~/.config/crush/crush.json",
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
		Name:         "Cline",
		Description:  "Autonomous coding agent for VS Code",
		Language:     "TypeScript",
		ConfigFormat: "Proto/gRPC",
		APIPattern:   "OpenAI-compatible",
		EntryPoint:   "cline",
		Features:     []string{"vscode-extension", "browser-interaction", "autonomous-agent", "multi-model", "plan-mode", "act-mode", "sub-tasks"},
		// ToolSupport canonicalised from docs/research/CL4R1T4S/CLINE/Cline.md
		// (14 native tools: execute_command → Bash, read_file → Read,
		// write_to_file → Write, replace_in_file → Edit, search_files →
		// Grep, list_files → Glob, browser_action → Browser, web_fetch →
		// WebFetch, use_mcp_tool + access_mcp_resource + load_mcp_documentation
		// → MCP, ask_followup_question → Ask, attempt_completion → Complete,
		// new_task → Task, plan_mode_respond → Plan).
		// LSP-style tools (Symbols/References/Definition) come from Cline's
		// VS Code host, not the prompt itself, but are available when
		// Cline runs in that host (its primary deployment target).
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Browser", "WebFetch", "MCP", "Ask", "Complete", "Task", "Plan", "Symbols", "References", "Definition"},
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

	// =========================================================================
	// Additional 30 agents (from HelixCode/Example_Projects and external sources)
	// Total: 48 agents
	// =========================================================================

	"AgentDeck": {
		Name:           "AgentDeck",
		Description:    "Multi-agent orchestration deck for AI workflows",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "agent-deck",
		Features:       []string{"multi-agent", "orchestration", "workflow-automation", "agent-collaboration"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Task"},
		Protocols:      []string{"OpenAI", "MCP"},
		ConfigLocation: "~/.config/agent-deck/config.json",
		SystemPrompt:   "You are Agent Deck, a multi-agent orchestration platform. Coordinate AI agents to complete complex tasks.",
	},
	"Bridle": {
		Name:           "Bridle",
		Description:    "Lightweight AI coding assistant with permission controls",
		Language:       "TypeScript",
		ConfigFormat:   "YAML",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "bridle",
		Features:       []string{"permission-controls", "lightweight", "sandboxing", "tool-restrictions"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"},
		Protocols:      []string{"OpenAI", "MCP"},
		ConfigLocation: "~/.config/bridle/config.yaml",
		SystemPrompt:   "You are Bridle, a lightweight AI coding assistant with strict permission controls.",
	},
	"CheshireCat": {
		Name:           "CheshireCat",
		Description:    "Cheshire Cat AI framework with plugin architecture",
		Language:       "Python",
		ConfigFormat:   "JSON",
		APIPattern:     "Multi-provider",
		EntryPoint:     "cheshire-cat",
		Features:       []string{"plugin-system", "memory-management", "custom-embeddings", "web-interface"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebFetch"},
		Protocols:      []string{"OpenAI", "Anthropic", "Custom"},
		ConfigLocation: "~/.config/cheshire-cat/config.json",
		EnvVars:        map[string]string{"CCAT_API_KEY": "", "CCAT_HOST": "http://localhost:1865"},
		SystemPrompt:   "You are the Cheshire Cat, an AI framework with memory and plugin capabilities.",
	},
	"ClaudePlugins": {
		Name:           "ClaudePlugins",
		Description:    "Plugin and skills framework for Claude Code",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "Anthropic",
		EntryPoint:     "claude-plugins",
		Features:       []string{"plugin-marketplace", "hooks", "custom-commands", "skill-definitions"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Task"},
		Protocols:      []string{"Anthropic", "MCP"},
		ConfigLocation: "~/.claude/.claude-plugin/plugin.json",
		SystemPrompt:   "You are Claude Plugins, a framework for extending Claude Code with custom plugins and skills.",
	},
	"ClaudeSquad": {
		Name:           "ClaudeSquad",
		Description:    "Multi-agent squad for collaborative Claude instances",
		Language:       "Python",
		ConfigFormat:   "YAML",
		APIPattern:     "Anthropic",
		EntryPoint:     "claude-squad",
		Features:       []string{"multi-agent", "squad-coordination", "task-distribution", "collaborative-ai"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Task"},
		Protocols:      []string{"Anthropic", "MCP"},
		ConfigLocation: "~/.config/claude-squad/squad.yaml",
		EnvVars:        map[string]string{"ANTHROPIC_API_KEY": "", "SQUAD_SIZE": "3"},
		SystemPrompt:   "You are Claude Squad, a multi-agent system with collaborative Claude instances.",
	},
	"Codai": {
		Name:           "Codai",
		Description:    "AI-powered code assistant with review capabilities",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "codai",
		Features:       []string{"code-review", "refactoring", "documentation-generation", "test-generation"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Test"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/codai/config.json",
		SystemPrompt:   "You are Codai, an AI code assistant specializing in code review and refactoring.",
	},
	"Codex": {
		Name:           "Codex",
		Description:    "OpenAI Codex CLI for code generation",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI",
		EntryPoint:     "codex",
		Features:       []string{"code-generation", "natural-language-to-code", "multi-language"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/codex/config.json",
		EnvVars:        map[string]string{"OPENAI_API_KEY": ""},
		SystemPrompt:   "You are Codex, an AI system for generating code from natural language descriptions.",
	},
	"CodexSkills": {
		Name:           "CodexSkills",
		Description:    "Skill-based extension for Codex with custom capabilities",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI",
		EntryPoint:     "codex-skills",
		Features:       []string{"skill-definitions", "custom-capabilities", "skill-chaining", "context-awareness"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Task"},
		Protocols:      []string{"OpenAI", "MCP"},
		ConfigLocation: "~/.config/codex-skills/skills.json",
		SystemPrompt:   "You are Codex Skills, an enhanced Codex with custom skill definitions.",
	},
	"Conduit": {
		Name:           "Conduit",
		Description:    "Pipeline-based AI workflow orchestrator",
		Language:       "Go",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "conduit",
		Features:       []string{"pipeline-workflows", "data-streaming", "transform-chains", "parallel-execution"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Task"},
		Protocols:      []string{"OpenAI", "MCP"},
		ConfigLocation: "~/.config/conduit/config.json",
		SystemPrompt:   "You are Conduit, a pipeline-based AI workflow orchestrator.",
	},
	"Emdash": {
		Name:           "Emdash",
		Description:    "Minimalist AI writing and coding assistant",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "emdash",
		Features:       []string{"minimalist", "writing-assistance", "markdown-support", "distraction-free"},
		ToolSupport:    []string{"Read", "Write", "Edit"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/emdash/config.json",
		SystemPrompt:   "You are Emdash, a minimalist AI assistant for writing and coding.",
	},
	"FauxPilot": {
		Name:           "FauxPilot",
		Description:    "Self-hosted GitHub Copilot alternative",
		Language:       "Python",
		ConfigFormat:   "YAML",
		APIPattern:     "Copilot-compatible",
		EntryPoint:     "fauxpilot",
		Features:       []string{"self-hosted", "copilot-compatible", "local-models", "privacy-focused"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob"},
		Protocols:      []string{"OpenAI", "Copilot"},
		ConfigLocation: "~/.config/fauxpilot/config.yaml",
		EnvVars:        map[string]string{"FAUXPILOT_HOST": "http://localhost:5000"},
		SystemPrompt:   "You are FauxPilot, a self-hosted GitHub Copilot alternative.",
	},
	"GetShitDone": {
		Name:           "GetShitDone",
		Description:    "Task-focused AI assistant for productivity",
		Language:       "Python",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "gsd",
		Features:       []string{"task-management", "todo-lists", "productivity", "deadline-tracking"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Task"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/gsd/config.json",
		SystemPrompt:   "You are Get Shit Done, a task-focused AI assistant for maximum productivity.",
	},
	"GitHubCopilotCLI": {
		Name:           "GitHubCopilotCLI",
		Description:    "GitHub Copilot CLI for terminal assistance",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "GitHub",
		EntryPoint:     "gh copilot",
		Features:       []string{"shell-suggestions", "git-commands", "gh-cli-integration", "explain-commands"},
		ToolSupport:    []string{"Bash", "Git"},
		Protocols:      []string{"GitHub", "Copilot"},
		ConfigLocation: "~/.config/gh-copilot/config.json",
		EnvVars:        map[string]string{"GITHUB_TOKEN": ""},
		SystemPrompt:   "You are GitHub Copilot CLI, providing terminal command suggestions and explanations.",
	},
	"GitHubSpecKit": {
		Name:           "GitHubSpecKit",
		Description:    "Specification-driven development toolkit",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "GitHub",
		EntryPoint:     "spec-kit",
		Features:       []string{"spec-generation", "api-documentation", "schema-validation", "openapi-support"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Git"},
		Protocols:      []string{"GitHub", "OpenAI"},
		ConfigLocation: "~/.config/spec-kit/config.json",
		SystemPrompt:   "You are GitHub Spec Kit, a specification-driven development toolkit.",
	},
	"GitMCP": {
		Name:           "GitMCP",
		Description:    "Git operations via MCP protocol",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "MCP",
		EntryPoint:     "git-mcp",
		Features:       []string{"git-operations", "mcp-native", "repository-management", "branch-automation"},
		ToolSupport:    []string{"Git", "Bash", "Read", "Write"},
		Protocols:      []string{"MCP"},
		ConfigLocation: "~/.config/git-mcp/config.json",
		SystemPrompt:   "You are GitMCP, providing Git operations through the MCP protocol.",
	},
	"GPTME": {
		Name:           "GPTME",
		Description:    "Personal AI assistant in your terminal",
		Language:       "Python",
		ConfigFormat:   "TOML",
		APIPattern:     "Multi-provider",
		EntryPoint:     "gptme",
		Features:       []string{"personal-assistant", "conversation-history", "tool-use", "code-execution"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebFetch"},
		Protocols:      []string{"OpenAI", "Anthropic", "Ollama"},
		ConfigLocation: "~/.config/gptme/config.toml",
		SystemPrompt:   "You are GPTME, a personal AI assistant in your terminal.",
	},
	"MobileAgent": {
		Name:           "MobileAgent",
		Description:    "Mobile app development AI assistant",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "mobile-agent",
		Features:       []string{"react-native", "flutter", "ios", "android", "cross-platform"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/mobile-agent/config.json",
		SystemPrompt:   "You are Mobile Agent, an AI assistant for mobile app development.",
	},
	"MultiagentCoding": {
		Name:           "MultiagentCoding",
		Description:    "Multi-agent system for collaborative coding",
		Language:       "Python",
		ConfigFormat:   "YAML",
		APIPattern:     "Multi-provider",
		EntryPoint:     "mac",
		Features:       []string{"multi-agent", "collaborative-coding", "role-based-agents", "consensus-building"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Task"},
		Protocols:      []string{"OpenAI", "Anthropic"},
		ConfigLocation: "~/.config/mac/config.yaml",
		SystemPrompt:   "You are Multi-Agent Coding System, coordinating multiple AI agents for collaborative development.",
	},
	"Nanocoder": {
		Name:           "Nanocoder",
		Description:    "Lightweight code generation assistant",
		Language:       "Rust",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "nanocoder",
		Features:       []string{"lightweight", "fast", "minimal-dependencies", "embedded-friendly"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/nanocoder/config.json",
		SystemPrompt:   "You are Nanocoder, a lightweight and fast code generation assistant.",
	},
	"Noi": {
		Name:           "Noi",
		Description:    "AI-powered browser extension for development",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "Multi-provider",
		EntryPoint:     "noi",
		Features:       []string{"browser-extension", "web-scraping", "dom-interaction", "ai-browsing"},
		ToolSupport:    []string{"WebFetch", "Read", "Write"},
		Protocols:      []string{"OpenAI", "Anthropic"},
		ConfigLocation: "~/.config/noi/config.json",
		SystemPrompt:   "You are Noi, an AI-powered browser extension for development assistance.",
	},
	"Octogen": {
		Name:           "Octogen",
		Description:    "Code generation and project scaffolding",
		Language:       "Python",
		ConfigFormat:   "YAML",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "octogen",
		Features:       []string{"project-scaffolding", "template-generation", "boilerplate", "code-templates"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/octogen/config.yaml",
		SystemPrompt:   "You are Octogen, an AI for code generation and project scaffolding.",
	},
	"OpenHands": {
		Name:           "OpenHands",
		Description:    "Open-source AI software development platform",
		Language:       "Python",
		ConfigFormat:   "TOML",
		APIPattern:     "Multi-provider",
		EntryPoint:     "openhands",
		Features:       []string{"software-development", "open-source", "browser-automation", "sandboxed-execution"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "WebFetch", "Task"},
		Protocols:      []string{"OpenAI", "Anthropic", "MCP"},
		ConfigLocation: "~/.config/openhands/config.toml",
		EnvVars:        map[string]string{"OPENAI_API_KEY": "", "ANTHROPIC_API_KEY": ""},
		SystemPrompt:   "You are OpenHands, an open-source AI software development platform.",
	},
	"PostgresMCP": {
		Name:           "PostgresMCP",
		Description:    "PostgreSQL database operations via MCP",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "MCP",
		EntryPoint:     "postgres-mcp",
		Features:       []string{"database-queries", "schema-management", "migrations", "sql-generation"},
		ToolSupport:    []string{"Read", "Write", "Task"},
		Protocols:      []string{"MCP"},
		ConfigLocation: "~/.config/postgres-mcp/config.json",
		EnvVars:        map[string]string{"DATABASE_URL": ""},
		SystemPrompt:   "You are PostgresMCP, providing PostgreSQL database operations through MCP.",
	},
	"Shai": {
		Name:           "Shai",
		Description:    "Shell AI assistant for command-line help",
		Language:       "Go",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "shai",
		Features:       []string{"shell-commands", "command-explanation", "shell-scripting", "terminal-help"},
		ToolSupport:    []string{"Bash"},
		Protocols:      []string{"OpenAI"},
		ConfigLocation: "~/.config/shai/config.json",
		SystemPrompt:   "You are Shai, a shell AI assistant for command-line help.",
	},
	"SnowCLI": {
		Name:           "SnowCLI",
		Description:    "Snowflake data platform AI assistant",
		Language:       "Python",
		ConfigFormat:   "YAML",
		APIPattern:     "Snowflake",
		EntryPoint:     "snow",
		Features:       []string{"snowflake-queries", "data-analysis", "warehouse-management", "sql-generation"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Task"},
		Protocols:      []string{"Snowflake", "OpenAI"},
		ConfigLocation: "~/.config/snow/config.yaml",
		EnvVars:        map[string]string{"SNOWFLAKE_ACCOUNT": "", "SNOWFLAKE_USER": ""},
		SystemPrompt:   "You are SnowCLI, an AI assistant for the Snowflake data platform.",
	},
	"TaskWeaver": {
		Name:           "TaskWeaver",
		Description:    "Code-first AI agent framework by Microsoft",
		Language:       "Python",
		ConfigFormat:   "YAML",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "taskweaver",
		Features:       []string{"code-first", "plugin-system", "data-analysis", "stateful-execution"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Task"},
		Protocols:      []string{"OpenAI", "Azure"},
		ConfigLocation: "~/.config/taskweaver/config.yaml",
		EnvVars:        map[string]string{"OPENAI_API_KEY": "", "AZURE_OPENAI_KEY": ""},
		SystemPrompt:   "You are TaskWeaver, a code-first AI agent framework for complex task execution.",
	},
	"UIUXProMax": {
		Name:           "UIUXProMax",
		Description:    "UI/UX design AI assistant",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "uiux-pro",
		Features:       []string{"ui-design", "ux-analysis", "figma-integration", "design-systems"},
		ToolSupport:    []string{"Read", "Write", "Edit", "WebFetch"},
		Protocols:      []string{"OpenAI", "Figma"},
		ConfigLocation: "~/.config/uiux-pro/config.json",
		SystemPrompt:   "You are UI/UX Pro Max, an AI assistant for UI/UX design and analysis.",
	},
	"VTCode": {
		Name:           "VTCode",
		Description:    "Voice-to-code AI assistant",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "OpenAI-compatible",
		EntryPoint:     "vtcode",
		Features:       []string{"voice-input", "speech-to-code", "transcription", "voice-commands"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"},
		Protocols:      []string{"OpenAI", "Whisper"},
		ConfigLocation: "~/.config/vtcode/config.json",
		EnvVars:        map[string]string{"OPENAI_API_KEY": ""},
		SystemPrompt:   "You are VTCode, converting voice input to code in real-time.",
	},
	"Warp": {
		Name:           "Warp",
		Description:    "AI-powered terminal with built-in assistant",
		Language:       "Rust",
		ConfigFormat:   "YAML",
		APIPattern:     "Warp",
		EntryPoint:     "warp",
		Features:       []string{"modern-terminal", "ai-assistant", "command-suggestions", "workflow-blocks"},
		ToolSupport:    []string{"Bash", "Git"},
		Protocols:      []string{"Warp", "OpenAI"},
		ConfigLocation: "~/.warp/config.yaml",
		SystemPrompt:   "You are Warp AI, an AI assistant built into the Warp terminal.",
	},
	"Continue": {
		Name:           "Continue",
		Description:    "Open-source AI code assistant for IDEs",
		Language:       "TypeScript",
		ConfigFormat:   "JSON",
		APIPattern:     "Multi-provider",
		EntryPoint:     "continue",
		Features:       []string{"vscode-extension", "jetbrains-plugin", "context-providers", "slash-commands", "mcp-support"},
		ToolSupport:    []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Task"},
		Protocols:      []string{"OpenAI", "Anthropic", "Ollama", "MCP"},
		ConfigLocation: "~/.continue/config.json",
		EnvVars:        map[string]string{"OPENAI_API_KEY": "", "ANTHROPIC_API_KEY": ""},
		SystemPrompt:   "You are Continue, an open-source AI code assistant for VS Code and JetBrains IDEs.",
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
