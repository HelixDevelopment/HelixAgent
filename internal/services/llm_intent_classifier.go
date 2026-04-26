package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"github.com/sirupsen/logrus"
)

// providerCooldownDuration is how long a provider stays out of rotation
// after a 429 / quota / token-budget rejection. The intent classifier hot-loops
// on every chat request, so retrying a quota-exhausted provider on every call
// burns latency for nothing — and floods logs. Cooldown is per-process state.
const providerCooldownDuration = 5 * time.Minute

// LLMIntentClassifier uses actual LLMs to classify user intent
// NO HARDCODING - Pure AI semantic understanding
type LLMIntentClassifier struct {
	providerRegistry   *ProviderRegistry
	logger             *logrus.Logger
	fallbackClassifier *IntentClassifier // Fallback if LLM unavailable

	cooldownMu sync.RWMutex
	cooldowns  map[string]time.Time
}

// NewLLMIntentClassifier creates a new LLM-based intent classifier
func NewLLMIntentClassifier(registry *ProviderRegistry, logger *logrus.Logger) *LLMIntentClassifier {
	return &LLMIntentClassifier{
		providerRegistry:   registry,
		logger:             logger,
		fallbackClassifier: NewIntentClassifier(), // Fallback only
		cooldowns:          make(map[string]time.Time),
	}
}

// markProviderCooldown puts a provider on the bench for providerCooldownDuration.
func (lic *LLMIntentClassifier) markProviderCooldown(name string) {
	lic.cooldownMu.Lock()
	lic.cooldowns[strings.ToLower(name)] = time.Now().Add(providerCooldownDuration)
	lic.cooldownMu.Unlock()
}

// isProviderCooledDown reports whether a provider is currently on cooldown.
func (lic *LLMIntentClassifier) isProviderCooledDown(name string) bool {
	lic.cooldownMu.RLock()
	until, ok := lic.cooldowns[strings.ToLower(name)]
	lic.cooldownMu.RUnlock()
	return ok && time.Now().Before(until)
}

// isQuotaError reports whether the error string looks like a 429 / quota / token-budget rejection.
func isQuotaError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "429") ||
		strings.Contains(s, "too_many_requests") ||
		strings.Contains(s, "quota_exceeded") ||
		strings.Contains(s, "queue_exceeded") ||
		strings.Contains(s, "rate limit") ||
		strings.Contains(s, "tokens per day") ||
		strings.Contains(s, "requests per minute")
}

// LLMIntentResponse is the structured response from the LLM
type LLMIntentResponse struct {
	Intent           string   `json:"intent"`            // "confirmation", "refusal", "question", "request", "clarification", "unclear"
	Confidence       float64  `json:"confidence"`        // 0.0 to 1.0
	IsActionable     bool     `json:"is_actionable"`     // Should we proceed with action?
	ShouldProceed    bool     `json:"should_proceed"`    // Clear signal to execute
	Reasoning        string   `json:"reasoning"`         // Explanation of classification
	DetectedElements []string `json:"detected_elements"` // What semantic elements were found
}

// ClassifyIntentWithLLM uses an LLM to understand user intent semantically
// This is ZERO hardcoding - pure AI understanding
func (lic *LLMIntentClassifier) ClassifyIntentWithLLM(ctx context.Context, userMessage string, conversationContext string) (*IntentClassificationResult, error) {
	// Try to get a fast, lightweight LLM for classification
	provider, providerName, err := lic.getClassificationProvider()
	if err != nil {
		lic.logger.WithError(err).Warn("No LLM available for intent classification, using fallback")
		return lic.fallbackClassifier.EnhancedClassifyIntent(userMessage, conversationContext != ""), nil
	}

	// Build the intent classification prompt
	prompt := lic.buildIntentClassificationPrompt(userMessage, conversationContext)

	// Create request with timeout
	classifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	request := &models.LLMRequest{
		ID:     fmt.Sprintf("intent-classify-%d", time.Now().UnixNano()),
		Prompt: prompt,
		ModelParams: models.ModelParameters{
			MaxTokens:   500, // Keep it small for speed
			Temperature: 0.1, // Low temperature for consistent classification
		},
		Messages: []models.Message{
			{Role: "system", Content: lic.getSystemPrompt()},
			{Role: "user", Content: prompt},
		},
	}

	// Call the LLM
	response, err := provider.Complete(classifyCtx, request)
	if err != nil {
		if isQuotaError(err) && providerName != "" {
			lic.markProviderCooldown(providerName)
			lic.logger.WithFields(logrus.Fields{
				"provider": providerName,
				"cooldown": providerCooldownDuration.String(),
			}).Info("Intent classifier provider hit quota — benched, will retry after cooldown")
		} else {
			lic.logger.WithError(err).Warn("LLM intent classification failed, using fallback")
		}
		return lic.fallbackClassifier.EnhancedClassifyIntent(userMessage, conversationContext != ""), nil
	}

	// Parse the LLM response
	result, err := lic.parseLLMIntentResponse(response.Content)
	if err != nil {
		lic.logger.WithError(err).Warn("Failed to parse LLM intent response, using fallback")
		return lic.fallbackClassifier.EnhancedClassifyIntent(userMessage, conversationContext != ""), nil
	}

	lic.logger.WithFields(logrus.Fields{
		"user_message": truncateString(userMessage, 50),
		"intent":       result.Intent,
		"confidence":   result.Confidence,
		"actionable":   result.IsActionable,
		"reasoning":    truncateString(result.Reasoning, 100),
	}).Info("LLM classified user intent")

	// Convert to standard result
	return lic.convertToClassificationResult(result), nil
}

// getClassificationProvider gets a fast LLM for intent classification.
// Returns the provider, its registry name (for cooldown bookkeeping), and an error.
func (lic *LLMIntentClassifier) getClassificationProvider() (llm.LLMProvider, string, error) {
	if lic.providerRegistry == nil {
		return nil, "", fmt.Errorf("no provider registry available")
	}

	// Try fast providers first (in order of preference for classification)
	preferredProviders := []string{
		"cerebras", // Very fast
		"mistral",  // Fast
		"deepseek", // Fast
		"zen",      // Free
		"claude",   // Reliable
	}

	for _, name := range preferredProviders {
		if lic.isProviderCooledDown(name) {
			continue
		}
		provider, err := lic.providerRegistry.GetProvider(name)
		if err == nil && provider != nil {
			return provider, name, nil
		}
	}

	// Get any available provider, but skip test/mock providers
	var selected llm.LLMProvider
	var selectedName string
	for _, name := range lic.providerRegistry.providers.Keys() {
		// Skip providers that look like test/mock providers
		// This ensures we only use real LLM providers for intent classification
		nameLower := strings.ToLower(name)
		if strings.HasPrefix(nameLower, "provider") ||
			strings.Contains(nameLower, "mock") ||
			strings.Contains(nameLower, "test") ||
			nameLower == "primary" ||
			nameLower == "fallback" ||
			strings.HasPrefix(nameLower, "fallback") ||
			strings.HasPrefix(nameLower, "participant") ||
			strings.HasPrefix(nameLower, "agent") {
			continue
		}
		if lic.isProviderCooledDown(name) {
			continue
		}
		provider, err := lic.providerRegistry.GetProvider(name)
		if err == nil && provider != nil {
			selected = provider
			selectedName = name
			break
		}
	}
	if selected != nil {
		return selected, selectedName, nil
	}

	return nil, "", fmt.Errorf("no LLM providers available")
}

// getSystemPrompt returns the system prompt for intent classification
func (lic *LLMIntentClassifier) getSystemPrompt() string {
	return `You are an intent classifier for an AI coding assistant that has an AI Debate Ensemble.
Your job is to determine whether the user's message requires TOOL EXECUTION or ANALYSIS/DISCUSSION.

You MUST respond with ONLY a valid JSON object in this exact format:
{
  "intent": "confirmation|refusal|question|request|clarification|unclear",
  "confidence": 0.0-1.0,
  "is_actionable": true/false,
  "should_proceed": true/false,
  "reasoning": "brief explanation",
  "detected_elements": ["element1", "element2"]
}

CRITICAL: is_actionable means the user wants CONCRETE ACTIONS performed:
- Writing/creating/editing FILES on disk
- Running COMMANDS (tests, builds, scripts, deployments)
- Installing/configuring SOFTWARE
- Reading/scanning specific FILES or directories
- Creating REPORTS that get saved to the filesystem
- Making CODE CHANGES (add functions, fix bugs, refactor)

is_actionable is FALSE when the user wants ANALYSIS or DISCUSSION:
- Explaining concepts, patterns, or trade-offs
- Comparing approaches or architectures (pros/cons)
- Discussing best practices or design patterns
- Answering theoretical or informational questions
- Providing opinions, recommendations, or strategy
- Debating different viewpoints on a topic

is_actionable is also FALSE for:
- Simple yes/no questions ("Do you see my codebase?", "Can you help me?")
- Asking about capabilities ("What tools do you have?")
- Asking for information that doesn't require file/command execution

Examples:
- "Write a coverage report to docs/" -> is_actionable: true (file creation)
- "Explain microservices vs monoliths" -> is_actionable: false (analysis)
- "Run the tests and fix any failures" -> is_actionable: true (commands + code changes)
- "What are the trade-offs of event sourcing?" -> is_actionable: false (discussion)
- "Determine our code coverage and write the report" -> is_actionable: true (requires running tools + writing file)
- "Compare REST and GraphQL APIs" -> is_actionable: false (analysis/comparison)
- "Do you see my codebase?" -> is_actionable: false (simple question about capabilities)
- "Can you help me with this?" -> is_actionable: false (simple question)
- "What is this project about?" -> is_actionable: false (informational question)

This works in ANY human language. Analyze the semantic meaning, not just keywords.
Respond with ONLY the JSON object, no other text.`
}

// maxClassifierUserMessageChars caps the user message slice that goes
// into the classification prompt. Cerebras qwen-3-235b's classifier
// model has an 8192-token context; full chat messages with code blocks
// or long pastes blow past it (observed 9635 chars triggering 400
// "context_length_exceeded" — Finding #42). Intent of a message is
// fully recoverable from its leading characters; we don't need the
// whole payload.
const maxClassifierUserMessageChars = 4000

// buildIntentClassificationPrompt builds the prompt for classification.
// The user message is truncated to maxClassifierUserMessageChars to
// stay safely under provider context windows.
func (lic *LLMIntentClassifier) buildIntentClassificationPrompt(userMessage string, context string) string {
	var sb strings.Builder

	classifyMessage := userMessage
	if len(classifyMessage) > maxClassifierUserMessageChars {
		classifyMessage = classifyMessage[:maxClassifierUserMessageChars] + "...[truncated]"
	}

	sb.WriteString("Classify the intent of this user message:\n\n")
	sb.WriteString(fmt.Sprintf("USER MESSAGE: \"%s\"\n\n", classifyMessage))

	if context != "" {
		sb.WriteString("CONVERSATION CONTEXT:\n")
		sb.WriteString("(There were previous messages with recommendations/suggestions)\n\n")
	}

	sb.WriteString("Analyze the semantic meaning and return the JSON classification.")

	return sb.String()
}

// parseLLMIntentResponse parses the LLM's JSON response
func (lic *LLMIntentClassifier) parseLLMIntentResponse(content string) (*LLMIntentResponse, error) {
	// Clean the response - extract JSON if wrapped in other text
	content = strings.TrimSpace(content)

	// Try to find JSON object in response
	startIdx := strings.Index(content, "{")
	endIdx := strings.LastIndex(content, "}")

	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := content[startIdx : endIdx+1]

	var response LLMIntentResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate required fields
	if response.Intent == "" {
		return nil, fmt.Errorf("intent field is empty")
	}

	// Normalize confidence
	if response.Confidence < 0 {
		response.Confidence = 0
	}
	if response.Confidence > 1 {
		response.Confidence = 1
	}

	return &response, nil
}

// convertToClassificationResult converts LLM response to standard result
func (lic *LLMIntentClassifier) convertToClassificationResult(llmResult *LLMIntentResponse) *IntentClassificationResult {
	result := &IntentClassificationResult{
		Confidence:            llmResult.Confidence,
		IsActionable:          llmResult.IsActionable,
		RequiresClarification: false,
		Signals:               llmResult.DetectedElements,
	}

	// Map intent string to enum
	switch strings.ToLower(llmResult.Intent) {
	case "confirmation":
		result.Intent = IntentConfirmation
	case "refusal":
		result.Intent = IntentRefusal
	case "question":
		result.Intent = IntentQuestion
		result.RequiresClarification = true
	case "request":
		result.Intent = IntentRequest
	case "clarification":
		result.Intent = IntentClarification
		result.RequiresClarification = true
	default:
		result.Intent = IntentUnclear
		result.RequiresClarification = true
	}

	// Add LLM reasoning as signal
	if llmResult.Reasoning != "" {
		result.Signals = append(result.Signals, "llm_reason:"+truncateString(llmResult.Reasoning, 50))
	}

	// Use LLM's should_proceed if available
	if llmResult.ShouldProceed && result.Intent == IntentConfirmation {
		result.IsActionable = true
	}

	return result
}

// Helper function to truncate strings
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// QuickClassify does a fast classification without full LLM call
// Uses LLM if available, otherwise falls back
func (lic *LLMIntentClassifier) QuickClassify(ctx context.Context, message string, hasContext bool) *IntentClassificationResult {
	// For very short messages with context, use LLM for accuracy
	if len(message) < 50 && hasContext {
		result, err := lic.ClassifyIntentWithLLM(ctx, message, "previous recommendations")
		if err == nil {
			return result
		}
	}

	// Fallback to pattern-based for speed
	return lic.fallbackClassifier.EnhancedClassifyIntent(message, hasContext)
}

// CachedClassification stores recent classifications for performance
type CachedClassification struct {
	Message   string
	Result    *IntentClassificationResult
	Timestamp time.Time
}

// IntentClassificationCache provides caching for intent classification
type IntentClassificationCache struct {
	cache   map[string]*CachedClassification
	maxSize int
	ttl     time.Duration
}

// NewIntentClassificationCache creates a new cache
func NewIntentClassificationCache(maxSize int, ttl time.Duration) *IntentClassificationCache {
	return &IntentClassificationCache{
		cache:   make(map[string]*CachedClassification),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a cached classification if available
func (icc *IntentClassificationCache) Get(message string) (*IntentClassificationResult, bool) {
	key := strings.ToLower(strings.TrimSpace(message))
	cached, ok := icc.cache[key]
	if !ok {
		return nil, false
	}

	// Check TTL
	if time.Since(cached.Timestamp) > icc.ttl {
		delete(icc.cache, key)
		return nil, false
	}

	return cached.Result, true
}

// Set stores a classification in cache
func (icc *IntentClassificationCache) Set(message string, result *IntentClassificationResult) {
	key := strings.ToLower(strings.TrimSpace(message))

	// Evict old entries if at capacity
	if len(icc.cache) >= icc.maxSize {
		// Remove oldest entry
		var oldestKey string
		var oldestTime time.Time
		for k, v := range icc.cache {
			if oldestKey == "" || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
			}
		}
		if oldestKey != "" {
			delete(icc.cache, oldestKey)
		}
	}

	icc.cache[key] = &CachedClassification{
		Message:   message,
		Result:    result,
		Timestamp: time.Now(),
	}
}
