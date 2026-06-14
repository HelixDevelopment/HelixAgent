package services

import (
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestEnhancedIntentClassifier_Initialization tests initialization
func TestEnhancedIntentClassifier_Initialization(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	classifier := NewEnhancedIntentClassifier(registry, logger)

	assert.NotNil(t, classifier)
	assert.NotNil(t, classifier.llmClassifier)
	assert.NotNil(t, classifier.providerRegistry)
	assert.NotNil(t, classifier.logger)
}

// TestEnhancedIntentClassifier_QuickClassify_Granularity tests quick granularity detection
func TestEnhancedIntentClassifier_QuickClassify_Granularity(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	tests := []struct {
		name                string
		message             string
		expectedGranularity WorkGranularity
	}{
		{
			"Single Action",
			"Add a log statement",
			GranularitySingleAction,
		},
		{
			"Small Creation",
			"Fix the typo in the function",
			GranularitySmallCreation,
		},
		{
			"Big Creation - Long Message",
			"Build a comprehensive authentication system with JWT tokens, refresh tokens, OAuth integration, rate limiting, session management, and comprehensive audit logging with detailed user activity tracking",
			GranularityBigCreation,
		},
		{
			"Whole Functionality",
			"Build the entire payment processing system",
			GranularityWholeFunctionality,
		},
		{
			"Refactoring",
			"Refactor the entire codebase to use microservices",
			GranularityRefactoring,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.QuickClassify(tt.message)
			assert.Equal(t, tt.expectedGranularity, result.Granularity)
			assert.True(t, result.GranularityScore > 0)
		})
	}
}

// TestEnhancedIntentClassifier_QuickClassify_ActionType tests quick action type detection
func TestEnhancedIntentClassifier_QuickClassify_ActionType(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	tests := []struct {
		name               string
		message            string
		expectedActionType ActionType
	}{
		{
			"Creation",
			"Create a new user authentication module",
			ActionCreation,
		},
		{
			"Debugging",
			"Debug the memory leak in the cache",
			ActionDebugging,
		},
		{
			"Fixing",
			"Fix the broken API endpoint",
			ActionFixing,
		},
		{
			"Improvements",
			"Improve the performance of the query",
			ActionImprovements,
		},
		{
			"Refactoring",
			"Refactor the database layer",
			ActionRefactoring,
		},
		{
			"Analysis",
			"Investigate the slow response time",
			ActionDebugging,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.QuickClassify(tt.message)
			assert.Equal(t, tt.expectedActionType, result.ActionType)
			assert.True(t, result.ActionTypeScore > 0)
		})
	}
}

// TestEnhancedIntentClassifier_ShouldUseSpecKit tests SpecKit decision logic
func TestEnhancedIntentClassifier_ShouldUseSpecKit(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	tests := []struct {
		name                    string
		result                  *EnhancedIntentResult
		expectedRequiresSpecKit bool
	}{
		{
			"Whole Functionality - Requires SpecKit",
			&EnhancedIntentResult{
				Granularity:      GranularityWholeFunctionality,
				GranularityScore: 0.9,
			},
			true,
		},
		{
			"Refactoring - Requires SpecKit",
			&EnhancedIntentResult{
				Granularity:      GranularityRefactoring,
				GranularityScore: 0.8,
			},
			true,
		},
		{
			"Big Creation High Score - Requires SpecKit",
			&EnhancedIntentResult{
				Granularity:      GranularityBigCreation,
				GranularityScore: 0.85,
			},
			true,
		},
		{
			// RECONCILED per §11.4.120: under the default-on policy a
			// big-creation development request routes through spec by DEFAULT,
			// regardless of granularity score.
			"Big Creation Low Score - Default Spec",
			&EnhancedIntentResult{
				Granularity:      GranularityBigCreation,
				GranularityScore: 0.6,
			},
			true,
		},
		{
			// RECONCILED per §11.4.120: a small-creation development request
			// (ordinary "implement feature X") now defaults to the spec flow.
			"Small Creation - Default Spec",
			&EnhancedIntentResult{
				Granularity:      GranularitySmallCreation,
				GranularityScore: 0.7,
			},
			true,
		},
		{
			// Trivial-bypass invariant preserved: single-action one-off stays
			// lightweight and never spec-drives.
			"Single Action - No SpecKit (trivial bypass)",
			&EnhancedIntentResult{
				Granularity:      GranularitySingleAction,
				GranularityScore: 0.5,
			},
			false,
		},
		{
			// Trivial-bypass invariant: pure conversation never spec-drives.
			"Conversation - No SpecKit (trivial bypass)",
			&EnhancedIntentResult{
				ActionType:  ActionConversation,
				Granularity: GranularitySmallCreation,
			},
			false,
		},
		{
			// Trivial-bypass invariant: pure analysis/review never spec-drives.
			"Analysis - No SpecKit (trivial bypass)",
			&EnhancedIntentResult{
				ActionType:  ActionAnalysis,
				Granularity: GranularityBigCreation,
			},
			false,
		},
		{
			"Refactoring Action Type - Requires SpecKit",
			&EnhancedIntentResult{
				Granularity:      GranularityBigCreation,
				GranularityScore: 0.75,
				ActionType:       ActionRefactoring,
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requiresSpecKit := classifier.shouldUseSpecKit(tt.result)
			assert.Equal(t, tt.expectedRequiresSpecKit, requiresSpecKit)
		})
	}
}

// TestEnhancedIntentClassifier_GenerateSpecKitReason tests reason generation
func TestEnhancedIntentClassifier_GenerateSpecKitReason(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	tests := []struct {
		name           string
		result         *EnhancedIntentResult
		expectedPhrase string // Expected phrase in reason
	}{
		{
			"Whole Functionality",
			&EnhancedIntentResult{
				Granularity: GranularityWholeFunctionality,
			},
			"complete functionality",
		},
		{
			"Refactoring",
			&EnhancedIntentResult{
				Granularity: GranularityRefactoring,
			},
			"major refactoring",
		},
		{
			"Big Creation",
			&EnhancedIntentResult{
				Granularity: GranularityBigCreation,
			},
			"substantial feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := classifier.generateSpecKitReason(tt.result)
			assert.Contains(t, reason, tt.expectedPhrase)
		})
	}
}

// TestEnhancedIntentClassifier_QuickClassify_WithSpecKit tests QuickClassify sets SpecKit flag
func TestEnhancedIntentClassifier_QuickClassify_WithSpecKit(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	tests := []struct {
		name          string
		message       string
		expectSpecKit bool
	}{
		{
			"Refactoring triggers SpecKit",
			"Refactor the entire authentication system",
			true,
		},
		{
			// RECONCILED per §11.4.120: under the spec-driven-default policy a
			// fix request is development-class (SmallCreation + ActionFixing) and
			// routes through the 7-phase flow by default. Genuinely trivial
			// one-off turns (conversation / pure analysis / single-action that is
			// not a fix/build) still bypass — see the trivial-bypass tests.
			"Fix request defaults to SpecKit",
			"Fix typo in comment",
			true,
		},
		{
			"Whole system triggers SpecKit",
			"Build the complete admin dashboard",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.QuickClassify(tt.message)
			assert.Equal(t, tt.expectSpecKit, result.RequiresSpecKit)
			if tt.expectSpecKit {
				assert.NotEmpty(t, result.SpecKitReason)
			}
		})
	}
}

// TestEnhancedIntentClassifier_ExtractJSON tests JSON extraction from markdown
func TestEnhancedIntentClassifier_ExtractJSON(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			"Plain JSON",
			`{"intent": "request"}`,
			`{"intent": "request"}`,
		},
		{
			"JSON with markdown",
			"```json\n{\"intent\": \"request\"}\n```",
			`{"intent": "request"}`,
		},
		{
			"JSON with backticks only",
			"```\n{\"intent\": \"request\"}\n```",
			`{"intent": "request"}`,
		},
		{
			"JSON with extra text",
			"Here is the result:\n```json\n{\"intent\": \"request\"}\n```\nEnd",
			`{"intent": "request"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.extractJSON(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEnhancedIntentClassifier_ClassifyEnhancedIntent_NoProvider tests behavior without provider
func TestEnhancedIntentClassifier_ClassifyEnhancedIntent_NoProvider(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil) // Registry may auto-discover providers from environment
	classifier := NewEnhancedIntentClassifier(registry, logger)

	ctx := context.Background()
	_, err := classifier.ClassifyEnhancedIntent(ctx, "Test request", "", nil)

	// If providers available from environment, might not error
	// If no providers, should error
	if err != nil {
		// Should mention provider or LLM issue
		assert.True(t,
			strings.Contains(err.Error(), "no provider") ||
				strings.Contains(err.Error(), "LLM") ||
				strings.Contains(err.Error(), "failed to parse"),
			"Expected provider-related error, got: %v", err)
	}
	// If no error, providers were available and classification succeeded (OK for this test)
}

// TestEnhancedIntentClassifier_GetProvider tests provider selection
func TestEnhancedIntentClassifier_GetProvider(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	provider, err := classifier.getProvider()
	// May return provider if environment has API keys, or error if not
	if err != nil {
		assert.Contains(t, err.Error(), "no providers available")
		assert.Nil(t, provider)
	} else {
		assert.NotNil(t, provider)
	}
}

// TestContainsAny tests the helper function
func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substrs  []string
		expected bool
	}{
		{
			"Contains match",
			"refactor the code",
			[]string{"refactor", "restructure"},
			true,
		},
		{
			"No match",
			"fix the bug",
			[]string{"refactor", "restructure"},
			false,
		},
		{
			"Multiple matches",
			"refactor and restructure",
			[]string{"refactor", "restructure"},
			true,
		},
		{
			"Empty substrs",
			"test",
			[]string{},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.s, tt.substrs...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEnhancedIntentClassifier_QuickClassify_SignalsPopulated tests that signals are populated
func TestEnhancedIntentClassifier_QuickClassify_SignalsPopulated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	result := classifier.QuickClassify("Create a new feature")

	assert.NotNil(t, result.Signals)
	assert.Contains(t, result.Signals, "quick_classification")
}

// TestEnhancedIntentClassifier_QuickClassify_ConfidenceRange tests confidence is in valid range
func TestEnhancedIntentClassifier_QuickClassify_ConfidenceRange(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	result := classifier.QuickClassify("Test request")

	assert.GreaterOrEqual(t, result.Confidence, 0.0)
	assert.LessOrEqual(t, result.Confidence, 1.0)
	assert.GreaterOrEqual(t, result.GranularityScore, 0.0)
	assert.LessOrEqual(t, result.GranularityScore, 1.0)
	assert.GreaterOrEqual(t, result.ActionTypeScore, 0.0)
	assert.LessOrEqual(t, result.ActionTypeScore, 1.0)
}

// TestEnhancedIntentClassifier_BuildEnhancedPrompt tests prompt building
func TestEnhancedIntentClassifier_BuildEnhancedPrompt(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	userMessage := "Build a new feature"
	conversationContext := "Previous discussion about architecture"
	codebaseContext := map[string]interface{}{
		"has_tests": true,
		"coverage":  85,
	}

	prompt := classifier.buildEnhancedPrompt(userMessage, conversationContext, codebaseContext)

	assert.Contains(t, prompt, userMessage)
	assert.Contains(t, prompt, conversationContext)
	assert.Contains(t, prompt, "granularity")
	assert.Contains(t, prompt, "action_type")
	assert.Contains(t, prompt, "JSON")
}

// TestEnhancedIntentClassifier_GetEnhancedSystemPrompt tests system prompt
func TestEnhancedIntentClassifier_GetEnhancedSystemPrompt(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	prompt := classifier.getEnhancedSystemPrompt()

	assert.Contains(t, prompt, "intent classifier")
	assert.Contains(t, prompt, "JSON")
	assert.Contains(t, prompt, "granularity")
	assert.Contains(t, prompt, "action")
}
