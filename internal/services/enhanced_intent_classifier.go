package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"github.com/sirupsen/logrus"
)

// WorkGranularity represents the scale of the requested work
type WorkGranularity string

const (
	GranularitySingleAction       WorkGranularity = "single_action"       // Single, discrete action
	GranularitySmallCreation      WorkGranularity = "small_creation"      // Small feature/fix
	GranularityBigCreation        WorkGranularity = "big_creation"        // Substantial feature
	GranularityWholeFunctionality WorkGranularity = "whole_functionality" // Complete system/module
	GranularityRefactoring        WorkGranularity = "refactoring"         // Major restructuring
)

// ActionType represents the type of work requested
type ActionType string

const (
	ActionConversation ActionType = "conversation" // Chat, greetings, questions — no code work
	ActionCreation     ActionType = "creation"     // Building something new
	ActionDebugging    ActionType = "debugging"    // Finding problems
	ActionFixing       ActionType = "fixing"       // Resolving issues
	ActionImprovements ActionType = "improvements" // Enhancements
	ActionSingleOp     ActionType = "single_op"    // One-off operation
	ActionRefactoring  ActionType = "refactoring"  // Code restructuring
	ActionAnalysis     ActionType = "analysis"     // Investigation/review
)

// EnhancedIntentResult contains comprehensive intent classification
type EnhancedIntentResult struct {
	// Basic Intent
	Intent       UserIntent `json:"intent"`
	Confidence   float64    `json:"confidence"`
	IsActionable bool       `json:"is_actionable"`

	// Granularity Detection
	Granularity      WorkGranularity `json:"granularity"`
	GranularityScore float64         `json:"granularity_score"`

	// Action Type Detection
	ActionType      ActionType `json:"action_type"`
	ActionTypeScore float64    `json:"action_type_score"`

	// SpecKit Decision
	RequiresSpecKit bool   `json:"requires_speckit"`
	SpecKitReason   string `json:"speckit_reason,omitempty"`

	// Analysis Details
	DetectedKeywords []string `json:"detected_keywords"`
	Reasoning        string   `json:"reasoning"`
	Signals          []string `json:"signals"`

	// Context
	HasExistingCode bool   `json:"has_existing_code"`
	EstimatedScope  string `json:"estimated_scope"`
}

// EnhancedIntentClassifier provides comprehensive intent analysis with granularity detection
type EnhancedIntentClassifier struct {
	llmClassifier    *LLMIntentClassifier
	providerRegistry *ProviderRegistry
	logger           *logrus.Logger
}

// NewEnhancedIntentClassifier creates a new enhanced intent classifier
func NewEnhancedIntentClassifier(registry *ProviderRegistry, logger *logrus.Logger) *EnhancedIntentClassifier {
	return &EnhancedIntentClassifier{
		llmClassifier:    NewLLMIntentClassifier(registry, logger),
		providerRegistry: registry,
		logger:           logger,
	}
}

// ClassifyEnhancedIntent performs comprehensive intent classification
func (eic *EnhancedIntentClassifier) ClassifyEnhancedIntent(ctx context.Context, userMessage string, conversationContext string, codebaseContext map[string]interface{}) (*EnhancedIntentResult, error) {
	// Get a provider for classification
	provider, err := eic.getProvider()
	if err != nil {
		return nil, fmt.Errorf("no provider available for intent classification: %w", err)
	}

	// Build comprehensive prompt
	prompt := eic.buildEnhancedPrompt(userMessage, conversationContext, codebaseContext)

	// Create request
	classifyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	request := &models.LLMRequest{
		ID:     fmt.Sprintf("enhanced-intent-%d", time.Now().UnixNano()),
		Prompt: prompt,
		ModelParams: models.ModelParameters{
			MaxTokens:   1000,
			Temperature: 0.2,
		},
		Messages: []models.Message{
			{Role: "system", Content: eic.getEnhancedSystemPrompt()},
			{Role: "user", Content: prompt},
		},
	}

	// Call LLM
	response, err := provider.Complete(classifyCtx, request)
	if err != nil {
		return nil, fmt.Errorf("LLM classification failed: %w", err)
	}

	// Parse response
	result, err := eic.parseEnhancedResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse enhanced intent response: %w", err)
	}

	// Determine if SpecKit is required
	result.RequiresSpecKit = eic.shouldUseSpecKit(result)
	if result.RequiresSpecKit {
		result.SpecKitReason = eic.generateSpecKitReason(result)
	}

	eic.logger.WithFields(logrus.Fields{
		"granularity":      result.Granularity,
		"action_type":      result.ActionType,
		"requires_speckit": result.RequiresSpecKit,
		"confidence":       result.Confidence,
	}).Info("Enhanced intent classified")

	return result, nil
}

// buildEnhancedPrompt creates a comprehensive prompt for intent classification
func (eic *EnhancedIntentClassifier) buildEnhancedPrompt(userMessage, conversationContext string, codebaseContext map[string]interface{}) string {
	prompt := fmt.Sprintf(`Analyze this user request and classify its intent with the following dimensions:

User Request: "%s"

Conversation Context: %s

Codebase Context: %v

Please provide a JSON response with the following structure:
{
  "intent": "confirmation|refusal|question|request|clarification|unclear",
  "confidence": 0.0-1.0,
  "is_actionable": true|false,
  "granularity": "single_action|small_creation|big_creation|whole_functionality|refactoring",
  "granularity_score": 0.0-1.0,
  "action_type": "creation|debugging|fixing|improvements|single_op|refactoring|analysis",
  "action_type_score": 0.0-1.0,
  "detected_keywords": ["keyword1", "keyword2"],
  "reasoning": "Explanation of classification",
  "signals": ["signal1", "signal2"],
  "has_existing_code": true|false,
  "estimated_scope": "Brief scope estimate"
}

Granularity Guidelines:
- single_action: One discrete task (e.g., "add a log statement", "fix typo")
- small_creation: Small feature or fix (e.g., "add validation to form", "create helper function")
- big_creation: Substantial feature (e.g., "add user authentication", "implement caching layer")
- whole_functionality: Complete system/module (e.g., "build payment processing", "create admin dashboard")
- refactoring: Major restructuring (e.g., "refactor to microservices", "migrate to new framework")

Action Type Guidelines:
- creation: Building something new from scratch
- debugging: Finding and investigating problems
- fixing: Resolving known issues
- improvements: Enhancing existing functionality
- single_op: One-off operation (query, report, etc.)
- refactoring: Code restructuring
- analysis: Investigation, review, research

Be precise and confident in your classification.`, userMessage, conversationContext, codebaseContext)

	return prompt
}

// getEnhancedSystemPrompt returns the system prompt for enhanced classification
func (eic *EnhancedIntentClassifier) getEnhancedSystemPrompt() string {
	return `You are an expert intent classifier. Your role is to determine whether a user message requires software development work or is a simple conversational interaction.

CRITICAL: Many messages are simple greetings, questions, or chat — these must NOT be classified as development work.

1. Classify the user's intent: conversation, question, confirmation, refusal, request, clarification, unclear
2. Determine granularity/scale: single_action (includes all conversational/question messages), small_creation, big_creation, whole_functionality, refactoring
3. Identify action type: conversation (chat/greeting/question), creation, debugging, fixing, improvements, single_operation, refactoring, analysis
4. Assess whether existing code is being modified or new code is being created
5. Estimate the scope of work

IMPORTANT RULES:
- Greetings ("hello", "hi", "hey"), simple questions ("do you see my codebase?", "what is X?", "how does Y work?"), and general chat MUST be classified as: intent="question" or intent="conversation", granularity="single_action", action_type="conversation"
- Only classify as big_creation/whole_functionality/refactoring when the user EXPLICITLY requests building, creating, or restructuring substantial code
- When in doubt, prefer lower granularity — single_action or small_creation
- Short messages (under 100 characters) are almost always single_action

You MUST respond with ONLY valid JSON - no markdown, no explanations, just the JSON object.

Be analytical and precise. Consider:
- Simple questions, greetings, and chat → granularity: single_action, action_type: conversation
- Keywords like "build", "create", "implement" suggest creation
- Keywords like "debug", "investigate", "find" suggest debugging
- Keywords like "fix", "resolve", "correct" suggest fixing
- Keywords like "improve", "enhance", "optimize" suggest improvements
- Keywords like "refactor", "restructure", "migrate" suggest refactoring
- Scale indicators like "entire", "complete", "whole" affect granularity
- Context about existing code affects has_existing_code

Always provide your best assessment with confidence scores.`
}

// parseEnhancedResponse parses the LLM response into EnhancedIntentResult
func (eic *EnhancedIntentClassifier) parseEnhancedResponse(content string) (*EnhancedIntentResult, error) {
	// Extract JSON from response (LLM might add markdown)
	jsonStr := eic.extractJSON(content)

	var result EnhancedIntentResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &result, nil
}

// extractJSON extracts JSON from response that might contain markdown
func (eic *EnhancedIntentClassifier) extractJSON(content string) string {
	// Remove markdown code blocks
	content = strings.TrimSpace(content)

	// Check for ```json ... ```
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + len("```json")
		end := strings.LastIndex(content, "```")
		if end > start {
			content = content[start:end]
		}
	} else if strings.Contains(content, "```") {
		start := strings.Index(content, "```") + len("```")
		end := strings.LastIndex(content, "```")
		if end > start {
			content = content[start:end]
		}
	}

	return strings.TrimSpace(content)
}

// shouldUseSpecKit determines whether the HelixSpecifier 7-phase spec-driven
// flow handles the request.
//
// DEFAULT-ON POLICY (operator mandate 2026-06-14: "spec driven development which
// MUST BE default way of the development and planning by HelixAgent"): spec-driven
// is the DEFAULT for any development/planning-class request — feature creation,
// implementation, fixing-that-produces-code, improvements, and refactoring all
// route through the 7-phase flow by default. The bias is inverted versus the
// historical opt-in heuristic: instead of "spec only for whole-functionality /
// major-refactor / big-creation", the engine now specs by default and only a
// TRIVIAL set of intents bypass it.
//
// Trivial bypass (do not spec "what is 2+2"): pure conversation, pure analysis /
// investigation, and single-action / one-off operations stay lightweight. These
// are the only paths that skip the spec flow. The ensemble / agentic / plain-chat
// paths are unaffected — they consult RequiresSpecKit and continue to handle the
// bypass classes themselves.
func (eic *EnhancedIntentClassifier) shouldUseSpecKit(result *EnhancedIntentResult) bool {
	// --- Trivial-bypass escape (the ONLY non-spec paths) ---
	// Conversational chat/greetings/questions and pure analysis/review never spec.
	if result.ActionType == ActionConversation || result.ActionType == ActionAnalysis {
		return false
	}
	// One-off discrete operations (single command / single edit) stay lightweight.
	if result.Granularity == GranularitySingleAction || result.ActionType == ActionSingleOp {
		return false
	}

	// --- Development / planning class → spec-driven by DEFAULT ---
	// Any request that builds, changes, fixes, improves, or restructures code
	// (i.e. survived the trivial-bypass escape and carries a development-class
	// action OR a code-producing granularity) defaults to the 7-phase spec flow.
	switch result.ActionType {
	case ActionCreation, ActionRefactoring, ActionImprovements, ActionFixing, ActionDebugging:
		return true
	}
	switch result.Granularity {
	case GranularitySmallCreation, GranularityBigCreation,
		GranularityWholeFunctionality, GranularityRefactoring:
		return true
	}

	// Anything left is neither trivial nor recognisably development-class;
	// keep it lightweight (no spec) rather than over-spec an unclassified turn.
	return false
}

// generateSpecKitReason generates explanation for why SpecKit is needed
func (eic *EnhancedIntentClassifier) generateSpecKitReason(result *EnhancedIntentResult) string {
	reasons := []string{}

	if result.Granularity == GranularityWholeFunctionality {
		reasons = append(reasons, "requires building complete functionality")
	}

	if result.Granularity == GranularityRefactoring {
		reasons = append(reasons, "involves major refactoring")
	}

	if result.Granularity == GranularityBigCreation {
		reasons = append(reasons, "substantial feature creation detected")
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "complex work requiring structured approach")
	}

	return strings.Join(reasons, "; ")
}

// getProvider gets an LLM provider for classification
func (eic *EnhancedIntentClassifier) getProvider() (llm.LLMProvider, error) {
	if eic.providerRegistry == nil {
		return nil, fmt.Errorf("provider registry not initialized")
	}

	// Try to get fastest provider first
	providers := eic.providerRegistry.ListProviders()
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Prefer fast, cheap providers for classification
	preferredOrder := []string{"cerebras", "deepseek", "mistral", "gemini"}

	for _, preferred := range preferredOrder {
		for _, p := range providers {
			if strings.Contains(strings.ToLower(p), preferred) {
				provider, err := eic.providerRegistry.GetProvider(p)
				if err == nil {
					return provider, nil
				}
			}
		}
	}

	// Fallback to first available
	provider, err := eic.providerRegistry.GetProvider(providers[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	return provider, nil
}

// QuickClassify performs a quick classification without full LLM call (fallback)
func (eic *EnhancedIntentClassifier) QuickClassify(userMessage string) *EnhancedIntentResult {
	result := &EnhancedIntentResult{
		Intent:       IntentRequest,
		Confidence:   0.5,
		IsActionable: true,
		Signals:      []string{"quick_classification"},
	}

	lowerMsg := strings.ToLower(userMessage)

	// Quick granularity detection
	// Check refactor first as it's more specific than "entire"
	if containsAny(lowerMsg, "refactor", "restructure", "migrate", "rewrite") {
		result.Granularity = GranularityRefactoring
		result.GranularityScore = 0.7
		result.RequiresSpecKit = true
	} else if containsAny(lowerMsg, "entire", "complete", "whole", "full system", "all of") {
		result.Granularity = GranularityWholeFunctionality
		result.GranularityScore = 0.7
	} else if containsAny(lowerMsg, "build", "create", "implement", "add") && len(userMessage) > 100 {
		result.Granularity = GranularityBigCreation
		result.GranularityScore = 0.6
	} else if containsAny(lowerMsg, "fix", "bug", "issue", "problem") {
		result.Granularity = GranularitySmallCreation
		result.GranularityScore = 0.6
	} else {
		result.Granularity = GranularitySingleAction
		result.GranularityScore = 0.5
	}

	// Quick action type detection
	if containsAny(lowerMsg, "debug", "investigate", "find", "trace") {
		result.ActionType = ActionDebugging
		result.ActionTypeScore = 0.7
	} else if containsAny(lowerMsg, "fix", "resolve", "correct") {
		result.ActionType = ActionFixing
		result.ActionTypeScore = 0.7
	} else if containsAny(lowerMsg, "improve", "enhance", "optimize", "better") {
		result.ActionType = ActionImprovements
		result.ActionTypeScore = 0.7
	} else if containsAny(lowerMsg, "refactor", "restructure") {
		result.ActionType = ActionRefactoring
		result.ActionTypeScore = 0.7
	} else if containsAny(lowerMsg, "build", "create", "implement", "develop") {
		result.ActionType = ActionCreation
		result.ActionTypeScore = 0.7
	} else {
		result.ActionType = ActionSingleOp
		result.ActionTypeScore = 0.5
	}

	result.RequiresSpecKit = eic.shouldUseSpecKit(result)
	if result.RequiresSpecKit {
		result.SpecKitReason = eic.generateSpecKitReason(result)
	}

	return result
}
