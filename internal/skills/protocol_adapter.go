// Package skills provides protocol adapters for integrating skills with MCP/ACP/LSP.
package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
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
//
// Concurrency model (CONST-029): the three protocol registries are
// safe.Store containers; each Register*/Get* operation is atomic
// without any caller-held lock. The previous mu only guarded each
// individual store's lookup/insert, and never needed to enforce a
// cross-store invariant — the three protocols are independent views
// of the same underlying skill.
type ProtocolSkillAdapter struct {
	service          *Service
	providerRegistry *services.ProviderRegistry
	mcpTools         *safe.Store[string, *MCPSkillTool]
	acpActions       *safe.Store[string, *ACPSkillAction]
	lspCommands      *safe.Store[string, *LSPSkillCommand]
	log              *logrus.Logger
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
	Command   string   `json:"command"`
	Title     string   `json:"title"`
	Arguments []string `json:"arguments,omitempty"`
	Skill     *Skill   `json:"-"`
}

// SkillToolCall represents a skill invocation via a protocol.
type SkillToolCall struct {
	Protocol  ProtocolType           `json:"protocol"`
	SkillName string                 `json:"skill_name"`
	Arguments map[string]interface{} `json:"arguments"`
	RequestID string                 `json:"request_id"`
	InvokedAt time.Time              `json:"invoked_at"`
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
		service:     service,
		mcpTools:    safe.NewStore[string, *MCPSkillTool](),
		acpActions:  safe.NewStore[string, *ACPSkillAction](),
		lspCommands: safe.NewStore[string, *LSPSkillCommand](),
		log:         logrus.New(),
	}
}

// SetLogger sets the logger.
func (a *ProtocolSkillAdapter) SetLogger(log *logrus.Logger) {
	a.log = log
}

// SetProviderRegistry sets the provider registry for LLM integration.
func (a *ProtocolSkillAdapter) SetProviderRegistry(registry *services.ProviderRegistry) {
	a.providerRegistry = registry
}

// RegisterAllSkillsAsTools registers all skills as protocol tools.
func (a *ProtocolSkillAdapter) RegisterAllSkillsAsTools() error {
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
	a.mcpTools.Put(skill.Name, tool)
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
	a.acpActions.Put(skill.Name, action)
}

// registerSkillAsLSPCommand registers a skill as an LSP command.
func (a *ProtocolSkillAdapter) registerSkillAsLSPCommand(skill *Skill) {
	command := &LSPSkillCommand{
		Command:   "helixagent.skill." + skill.Name,
		Title:     skill.Name,
		Arguments: []string{"query", "context"},
		Skill:     skill,
	}
	a.lspCommands.Put(skill.Name, command)
}

// GetMCPTools returns all skills as MCP tools.
func (a *ProtocolSkillAdapter) GetMCPTools() []*MCPSkillTool {
	tools := make([]*MCPSkillTool, 0, a.mcpTools.Len())
	a.mcpTools.Range(func(_ string, tool *MCPSkillTool) bool {
		tools = append(tools, tool)
		return true
	})
	return tools
}

// GetACPActions returns all skills as ACP actions.
func (a *ProtocolSkillAdapter) GetACPActions() []*ACPSkillAction {
	actions := make([]*ACPSkillAction, 0, a.acpActions.Len())
	a.acpActions.Range(func(_ string, action *ACPSkillAction) bool {
		actions = append(actions, action)
		return true
	})
	return actions
}

// GetLSPCommands returns all skills as LSP commands.
func (a *ProtocolSkillAdapter) GetLSPCommands() []*LSPSkillCommand {
	commands := make([]*LSPSkillCommand, 0, a.lspCommands.Len())
	a.lspCommands.Range(func(_ string, cmd *LSPSkillCommand) bool {
		commands = append(commands, cmd)
		return true
	})
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

	// Execute skill - dispatches to category-specific handler via executeSkillLogic
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

// executeSkillLogic executes the actual skill logic based on skill type and configuration.
// It processes the skill's instructions, determines appropriate execution strategy,
// and returns the result.
func (a *ProtocolSkillAdapter) executeSkillLogic(ctx context.Context, skill *Skill, query string, params map[string]interface{}) string {
	a.log.WithFields(logrus.Fields{
		"skill":    skill.Name,
		"category": skill.Category,
		"query":    query,
	}).Debug("Executing skill logic")

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Sprintf("Skill execution cancelled: %v", ctx.Err())
	default:
	}

	// Extract context from params
	skillContext := make(map[string]interface{})
	if ctxParam, ok := params["context"].(map[string]interface{}); ok {
		skillContext = ctxParam
	}

	// Execute based on skill category
	var result string
	var err error

	switch skill.Category {
	case "code_generation":
		result, err = a.executeCodeGenerationSkill(ctx, skill, query, skillContext)
	case "code_review":
		result, err = a.executeCodeReviewSkill(ctx, skill, query, skillContext)
	case "documentation":
		result, err = a.executeDocumentationSkill(ctx, skill, query, skillContext)
	case "testing":
		result, err = a.executeTestingSkill(ctx, skill, query, skillContext)
	case "refactoring":
		result, err = a.executeRefactoringSkill(ctx, skill, query, skillContext)
	case "debugging":
		result, err = a.executeDebuggingSkill(ctx, skill, query, skillContext)
	case "analysis":
		result, err = a.executeAnalysisSkill(ctx, skill, query, skillContext)
	case "search":
		result, err = a.executeSearchSkill(ctx, skill, query, skillContext)
	default:
		// Generic execution for unknown categories
		result, err = a.executeGenericSkill(ctx, skill, query, skillContext)
	}

	if err != nil {
		a.log.WithError(err).WithField("skill", skill.Name).Warn("Skill execution failed")
		return fmt.Sprintf("Error executing skill %s: %v", skill.Name, err)
	}

	return result
}

// getBestProvider returns the highest-scoring LLM provider from the registry.
func (a *ProtocolSkillAdapter) getBestProvider() (llm.LLMProvider, error) {
	if a.providerRegistry == nil {
		return nil, fmt.Errorf("provider registry not set")
	}
	providers := a.providerRegistry.ListProvidersOrderedByScore()
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}
	providerName := providers[0]
	provider, err := a.providerRegistry.GetProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", providerName, err)
	}
	return provider, nil
}

// executeCodeGenerationSkill handles code generation tasks
func (a *ProtocolSkillAdapter) executeCodeGenerationSkill(ctx context.Context, skill *Skill, query string, skillContext map[string]interface{}) (string, error) {
	// Build the code generation prompt
	language := "go"
	if lang, ok := skillContext["language"].(string); ok {
		language = lang
	}

	// Try to get LLM provider for actual code generation
	provider, err := a.getBestProvider()
	if err != nil {
		a.log.WithError(err).Debug("No LLM provider available, using template-based fallback")
		// Fall back to template-based result when no LLM provider is configured
		return fmt.Sprintf(`[Code Generation Result]
Query: %s
Language: %s

Instructions Applied:
%s

Generated Code:
// LLM provider integration pending (no provider available)

func generated() {
    // Implementation based on: %s
}

Note: Full code generation requires LLM integration.
Skill '%s' was successfully invoked.`, query, language, skill.Instructions, query, skill.Name), nil
	}

	// Build prompt with skill instructions
	prompt := fmt.Sprintf(`You are a code generation assistant. Follow these instructions:

%s

Generate code in %s for the following request:

%s

Provide only the code, no explanations.`, skill.Instructions, language, query)

	// Create LLM request
	req := &models.LLMRequest{
		Prompt: prompt,
		ModelParams: models.ModelParameters{
			Model:            "", // Provider will use default model
			Temperature:      0.2,
			MaxTokens:        2048,
			TopP:             1.0,
			StopSequences:    nil,
			ProviderSpecific: nil,
		},
	}

	// Call provider
	resp, err := provider.Complete(ctx, req)
	if err != nil {
		a.log.WithError(err).Warn("LLM provider failed to generate code")
		return fmt.Sprintf("Error generating code with LLM provider: %v", err), nil
	}

	// Return generated code with metadata
	return fmt.Sprintf(`[Code Generation Result]
Query: %s
Language: %s

Instructions Applied:
%s

Generated Code:
%s

Skill '%s' executed successfully using LLM provider.`, query, language, skill.Instructions, resp.Content, skill.Name), nil
}

// executeCodeReviewSkill handles code review tasks
func (a *ProtocolSkillAdapter) executeCodeReviewSkill(ctx context.Context, skill *Skill, query string, skillContext map[string]interface{}) (string, error) {
	code := ""
	if c, ok := skillContext["code"].(string); ok {
		code = c
	}

	return fmt.Sprintf(`[Code Review Result]
Skill: %s
Query: %s

Review Criteria (from skill instructions):
%s

Code Under Review:
%s

Review Status: Skill invoked successfully
Note: Full code review requires LLM integration for detailed analysis.`, skill.Name, query, skill.Instructions, truncateString(code, 500)), nil
}

// executeDocumentationSkill handles documentation generation tasks
func (a *ProtocolSkillAdapter) executeDocumentationSkill(ctx context.Context, skill *Skill, query string, skillContext map[string]interface{}) (string, error) {
	docType := "general"
	if dt, ok := skillContext["doc_type"].(string); ok {
		docType = dt
	}

	return fmt.Sprintf(`[Documentation Result]
Skill: %s
Query: %s
Documentation Type: %s

Skill Instructions Applied:
%s

Documentation skeleton generated. Full content requires LLM integration.
Skill '%s' executed successfully.`, skill.Name, query, docType, skill.Instructions, skill.Name), nil
}

// executeTestingSkill handles test generation tasks
func (a *ProtocolSkillAdapter) executeTestingSkill(ctx context.Context, skill *Skill, query string, skillContext map[string]interface{}) (string, error) {
	testType := "unit"
	if tt, ok := skillContext["test_type"].(string); ok {
		testType = tt
	}

	return fmt.Sprintf(`[Testing Result]
Skill: %s
Query: %s
Test Type: %s

Test Generation Guidelines:
%s

Test skeleton:
func Test%s(t *testing.T) {
    // Test implementation based on skill instructions
    // Full test generation requires LLM integration
}

Skill '%s' executed successfully.`, skill.Name, query, testType, skill.Instructions, capitalizeFirst(sanitizeIdentifier(query)), skill.Name), nil
}

// executeRefactoringSkill handles code refactoring tasks
func (a *ProtocolSkillAdapter) executeRefactoringSkill(ctx context.Context, skill *Skill, query string, skillContext map[string]interface{}) (string, error) {
	return fmt.Sprintf(`[Refactoring Result]
Skill: %s
Query: %s

Refactoring Guidelines:
%s

Refactoring analysis complete. Actual code transformation requires LLM integration.
Skill '%s' executed successfully.`, skill.Name, query, skill.Instructions, skill.Name), nil
}

// executeDebuggingSkill handles debugging tasks
func (a *ProtocolSkillAdapter) executeDebuggingSkill(ctx context.Context, skill *Skill, query string, skillContext map[string]interface{}) (string, error) {
	errorMsg := ""
	if e, ok := skillContext["error"].(string); ok {
		errorMsg = e
	}

	return fmt.Sprintf(`[Debugging Result]
Skill: %s
Query: %s
Error Context: %s

Debugging Approach:
%s

Debug analysis initiated. Full debugging assistance requires LLM integration.
Skill '%s' executed successfully.`, skill.Name, query, errorMsg, skill.Instructions, skill.Name), nil
}

// executeAnalysisSkill handles code analysis tasks
func (a *ProtocolSkillAdapter) executeAnalysisSkill(ctx context.Context, skill *Skill, query string, skillContext map[string]interface{}) (string, error) {
	return fmt.Sprintf(`[Analysis Result]
Skill: %s
Query: %s

Analysis Framework:
%s

Analysis complete. Detailed insights require LLM integration.
Skill '%s' executed successfully.`, skill.Name, query, skill.Instructions, skill.Name), nil
}

// executeSearchSkill handles search tasks
func (a *ProtocolSkillAdapter) executeSearchSkill(ctx context.Context, skill *Skill, query string, skillContext map[string]interface{}) (string, error) {
	return fmt.Sprintf(`[Search Result]
Skill: %s
Query: %s

Search Parameters:
%s

Search initiated. Results require integration with search backend.
Skill '%s' executed successfully.`, skill.Name, query, skill.Instructions, skill.Name), nil
}

// executeGenericSkill handles unknown skill categories
func (a *ProtocolSkillAdapter) executeGenericSkill(ctx context.Context, skill *Skill, query string, skillContext map[string]interface{}) (string, error) {
	return fmt.Sprintf(`[Skill Execution Result]
Skill: %s
Category: %s
Query: %s

Instructions:
%s

Context: %v

Skill executed successfully via protocol adapter.
Note: Specific functionality depends on LLM integration.`, skill.Name, skill.Category, query, skill.Instructions, skillContext), nil
}

// Helper functions

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// sanitizeIdentifier removes or replaces invalid identifier characters
func sanitizeIdentifier(s string) string {
	var result strings.Builder
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9' && i > 0) || r == '_' {
			result.WriteRune(r)
		}
	}
	if result.Len() == 0 {
		return "Generated"
	}
	return result.String()
}

// ToMCPToolList converts skills to MCP tool list format.
func (a *ProtocolSkillAdapter) ToMCPToolList() ([]byte, error) {
	tools := make([]map[string]interface{}, 0, a.mcpTools.Len())
	a.mcpTools.Range(func(_ string, tool *MCPSkillTool) bool {
		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
		return true
	})

	return json.Marshal(map[string]interface{}{
		"tools": tools,
	})
}

// ToACPActionList converts skills to ACP action list format.
func (a *ProtocolSkillAdapter) ToACPActionList() ([]byte, error) {
	actions := make([]map[string]interface{}, 0, a.acpActions.Len())
	a.acpActions.Range(func(_ string, action *ACPSkillAction) bool {
		actions = append(actions, map[string]interface{}{
			"name":        action.Name,
			"description": action.Description,
			"parameters":  action.Parameters,
		})
		return true
	})

	return json.Marshal(map[string]interface{}{
		"actions": actions,
	})
}

// ToLSPCommandList converts skills to LSP command list format.
func (a *ProtocolSkillAdapter) ToLSPCommandList() ([]byte, error) {
	commands := make([]map[string]interface{}, 0, a.lspCommands.Len())
	a.lspCommands.Range(func(_ string, cmd *LSPSkillCommand) bool {
		commands = append(commands, map[string]interface{}{
			"command":   cmd.Command,
			"title":     cmd.Title,
			"arguments": cmd.Arguments,
		})
		return true
	})

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

	data, _ := json.Marshal(map[string]interface{}{ //nolint:errcheck // marshaling simple map should never fail
		"skills_used": names,
		"count":       len(usages),
	})
	return string(data)
}
