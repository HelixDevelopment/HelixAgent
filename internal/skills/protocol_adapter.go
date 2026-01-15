// Package skills provides protocol adapters for integrating skills with MCP/ACP/LSP.
package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ProtocolType represents the type of protocol.
type ProtocolType string

const (
	ProtocolMCP ProtocolType = "mcp"
	ProtocolACP ProtocolType = "acp"
	ProtocolLSP ProtocolType = "lsp"
)

// ProtocolSkillAdapter adapts skills for use with various protocols.
type ProtocolSkillAdapter struct {
	service      *Service
	mcpTools     map[string]*MCPSkillTool
	acpActions   map[string]*ACPSkillAction
	lspCommands  map[string]*LSPSkillCommand
	mu           sync.RWMutex
	log          *logrus.Logger
}

// MCPSkillTool wraps a skill as an MCP tool.
type MCPSkillTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Skill       *Skill                 `json:"-"`
}

// ACPSkillAction wraps a skill as an ACP action.
type ACPSkillAction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Skill       *Skill                 `json:"-"`
}

// LSPSkillCommand wraps a skill as an LSP command.
type LSPSkillCommand struct {
	Command     string   `json:"command"`
	Title       string   `json:"title"`
	Arguments   []string `json:"arguments,omitempty"`
	Skill       *Skill   `json:"-"`
}

// SkillToolCall represents a skill invocation via a protocol.
type SkillToolCall struct {
	Protocol   ProtocolType           `json:"protocol"`
	SkillName  string                 `json:"skill_name"`
	Arguments  map[string]interface{} `json:"arguments"`
	RequestID  string                 `json:"request_id"`
	InvokedAt  time.Time              `json:"invoked_at"`
}

// SkillToolResult represents the result of a skill invocation.
type SkillToolResult struct {
	Protocol    ProtocolType `json:"protocol"`
	SkillName   string       `json:"skill_name"`
	Content     string       `json:"content"`
	SkillsUsed  []SkillUsage `json:"skills_used"`
	IsError     bool         `json:"is_error,omitempty"`
	Error       string       `json:"error,omitempty"`
	CompletedAt time.Time    `json:"completed_at"`
}

// NewProtocolSkillAdapter creates a new protocol adapter.
func NewProtocolSkillAdapter(service *Service) *ProtocolSkillAdapter {
	return &ProtocolSkillAdapter{
		service:    service,
		mcpTools:   make(map[string]*MCPSkillTool),
		acpActions: make(map[string]*ACPSkillAction),
		lspCommands: make(map[string]*LSPSkillCommand),
		log:        logrus.New(),
	}
}

// SetLogger sets the logger.
func (a *ProtocolSkillAdapter) SetLogger(log *logrus.Logger) {
	a.log = log
}

// RegisterAllSkillsAsTools registers all skills as protocol tools.
func (a *ProtocolSkillAdapter) RegisterAllSkillsAsTools() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	skills := a.service.GetAllSkills()
	for _, skill := range skills {
		a.registerSkillAsMCPTool(skill)
		a.registerSkillAsACPAction(skill)
		a.registerSkillAsLSPCommand(skill)
	}

	a.log.WithField("count", len(skills)).Info("Registered skills as protocol tools")
	return nil
}

// registerSkillAsMCPTool registers a skill as an MCP tool.
func (a *ProtocolSkillAdapter) registerSkillAsMCPTool(skill *Skill) {
	tool := &MCPSkillTool{
		Name:        "skill_" + skill.Name,
		Description: skill.Description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The query or prompt for the skill",
				},
				"context": map[string]interface{}{
					"type":        "object",
					"description": "Additional context for the skill",
				},
			},
			"required": []string{"query"},
		},
		Skill: skill,
	}
	a.mcpTools[skill.Name] = tool
}

// registerSkillAsACPAction registers a skill as an ACP action.
func (a *ProtocolSkillAdapter) registerSkillAsACPAction(skill *Skill) {
	action := &ACPSkillAction{
		Name:        "skill." + skill.Name,
		Description: skill.Description,
		Parameters: map[string]interface{}{
			"query":   "string",
			"context": "object",
		},
		Skill: skill,
	}
	a.acpActions[skill.Name] = action
}

// registerSkillAsLSPCommand registers a skill as an LSP command.
func (a *ProtocolSkillAdapter) registerSkillAsLSPCommand(skill *Skill) {
	command := &LSPSkillCommand{
		Command:   "helixagent.skill." + skill.Name,
		Title:     skill.Name,
		Arguments: []string{"query", "context"},
		Skill:     skill,
	}
	a.lspCommands[skill.Name] = command
}

// GetMCPTools returns all skills as MCP tools.
func (a *ProtocolSkillAdapter) GetMCPTools() []*MCPSkillTool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tools := make([]*MCPSkillTool, 0, len(a.mcpTools))
	for _, tool := range a.mcpTools {
		tools = append(tools, tool)
	}
	return tools
}

// GetACPActions returns all skills as ACP actions.
func (a *ProtocolSkillAdapter) GetACPActions() []*ACPSkillAction {
	a.mu.RLock()
	defer a.mu.RUnlock()

	actions := make([]*ACPSkillAction, 0, len(a.acpActions))
	for _, action := range a.acpActions {
		actions = append(actions, action)
	}
	return actions
}

// GetLSPCommands returns all skills as LSP commands.
func (a *ProtocolSkillAdapter) GetLSPCommands() []*LSPSkillCommand {
	a.mu.RLock()
	defer a.mu.RUnlock()

	commands := make([]*LSPSkillCommand, 0, len(a.lspCommands))
	for _, cmd := range a.lspCommands {
		commands = append(commands, cmd)
	}
	return commands
}

// InvokeMCPTool invokes a skill through the MCP protocol.
func (a *ProtocolSkillAdapter) InvokeMCPTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*SkillToolResult, error) {
	return a.invokeSkill(ctx, ProtocolMCP, toolName, arguments)
}

// InvokeACPAction invokes a skill through the ACP protocol.
func (a *ProtocolSkillAdapter) InvokeACPAction(ctx context.Context, actionName string, parameters map[string]interface{}) (*SkillToolResult, error) {
	return a.invokeSkill(ctx, ProtocolACP, actionName, parameters)
}

// InvokeLSPCommand invokes a skill through the LSP protocol.
func (a *ProtocolSkillAdapter) InvokeLSPCommand(ctx context.Context, command string, arguments []interface{}) (*SkillToolResult, error) {
	// Convert arguments to map
	params := make(map[string]interface{})
	if len(arguments) > 0 {
		if query, ok := arguments[0].(string); ok {
			params["query"] = query
		}
	}
	if len(arguments) > 1 {
		params["context"] = arguments[1]
	}
	return a.invokeSkill(ctx, ProtocolLSP, command, params)
}

// invokeSkill invokes a skill through any protocol.
func (a *ProtocolSkillAdapter) invokeSkill(ctx context.Context, protocol ProtocolType, identifier string, params map[string]interface{}) (*SkillToolResult, error) {
	// Extract skill name from identifier
	skillName := a.extractSkillName(protocol, identifier)

	// Get the skill
	skill, ok := a.service.GetSkill(skillName)
	if !ok {
		return &SkillToolResult{
			Protocol:    protocol,
			SkillName:   skillName,
			IsError:     true,
			Error:       fmt.Sprintf("skill not found: %s", skillName),
			CompletedAt: time.Now(),
		}, nil
	}

	// Create request ID
	requestID := fmt.Sprintf("%s-%s-%d", protocol, skillName, time.Now().UnixNano())

	// Create match for tracking
	match := &SkillMatch{
		Skill:          skill,
		Confidence:     1.0, // Explicit invocation
		MatchedTrigger: string(protocol) + "_call",
		MatchType:      MatchTypeExact,
	}

	// Start tracking
	_ = a.service.StartSkillExecution(requestID, skill, match)

	// Execute skill (placeholder - actual execution depends on skill type)
	query := ""
	if q, ok := params["query"].(string); ok {
		query = q
	}

	content := a.executeSkillLogic(ctx, skill, query, params)

	// Complete tracking
	usage := a.service.CompleteSkillExecution(requestID, true, "")

	// Build result with skill usage
	usages := []SkillUsage{}
	if usage != nil {
		usages = append(usages, *usage)
	}

	return &SkillToolResult{
		Protocol:    protocol,
		SkillName:   skillName,
		Content:     content,
		SkillsUsed:  usages,
		CompletedAt: time.Now(),
	}, nil
}

// extractSkillName extracts the skill name from a protocol identifier.
func (a *ProtocolSkillAdapter) extractSkillName(protocol ProtocolType, identifier string) string {
	switch protocol {
	case ProtocolMCP:
		// skill_name -> name
		if len(identifier) > 6 && identifier[:6] == "skill_" {
			return identifier[6:]
		}
	case ProtocolACP:
		// skill.name -> name
		if len(identifier) > 6 && identifier[:6] == "skill." {
			return identifier[6:]
		}
	case ProtocolLSP:
		// helixagent.skill.name -> name
		if len(identifier) > 17 && identifier[:17] == "helixagent.skill." {
			return identifier[17:]
		}
	}
	return identifier
}

// executeSkillLogic executes the actual skill logic.
func (a *ProtocolSkillAdapter) executeSkillLogic(ctx context.Context, skill *Skill, query string, params map[string]interface{}) string {
	// This is a placeholder - actual implementation would:
	// 1. Parse the skill's instructions
	// 2. Determine what tools to use
	// 3. Execute the workflow
	// 4. Return the result

	response := fmt.Sprintf(
		"[Skill: %s]\n"+
		"Category: %s\n"+
		"Query: %s\n\n"+
		"Instructions:\n%s\n\n"+
		"This skill was invoked via protocol. "+
		"Full implementation would execute the skill's workflow.",
		skill.Name,
		skill.Category,
		query,
		skill.Instructions,
	)

	return response
}

// ToMCPToolList converts skills to MCP tool list format.
func (a *ProtocolSkillAdapter) ToMCPToolList() ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tools := make([]map[string]interface{}, 0, len(a.mcpTools))
	for _, tool := range a.mcpTools {
		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}

	return json.Marshal(map[string]interface{}{
		"tools": tools,
	})
}

// ToACPActionList converts skills to ACP action list format.
func (a *ProtocolSkillAdapter) ToACPActionList() ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	actions := make([]map[string]interface{}, 0, len(a.acpActions))
	for _, action := range a.acpActions {
		actions = append(actions, map[string]interface{}{
			"name":        action.Name,
			"description": action.Description,
			"parameters":  action.Parameters,
		})
	}

	return json.Marshal(map[string]interface{}{
		"actions": actions,
	})
}

// ToLSPCommandList converts skills to LSP command list format.
func (a *ProtocolSkillAdapter) ToLSPCommandList() ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	commands := make([]map[string]interface{}, 0, len(a.lspCommands))
	for _, cmd := range a.lspCommands {
		commands = append(commands, map[string]interface{}{
			"command":   cmd.Command,
			"title":     cmd.Title,
			"arguments": cmd.Arguments,
		})
	}

	return json.Marshal(map[string]interface{}{
		"commands": commands,
	})
}

// GetSkillUsageHeader returns HTTP header content for skill usage.
func GetSkillUsageHeader(usages []SkillUsage) string {
	if len(usages) == 0 {
		return ""
	}

	names := make([]string, len(usages))
	for i, u := range usages {
		names[i] = u.SkillName
	}

	data, _ := json.Marshal(map[string]interface{}{
		"skills_used": names,
		"count":       len(usages),
	})
	return string(data)
}
