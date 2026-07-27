package services

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	runtime.GOMAXPROCS(2)
}

// mockLLMProvider implements llm.LLMProvider for testing purposes.
type mockLLMProvider struct {
	completeFunc func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
	healthErr    error
}

func (m *mockLLMProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return &models.LLMResponse{Content: ""}, nil
}

func (m *mockLLMProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse)
	close(ch)
	return ch, nil
}

func (m *mockLLMProvider) HealthCheck() error {
	return m.healthErr
}

func (m *mockLLMProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportsStreaming: true,
	}
}

func (m *mockLLMProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

// newLLMClassifierTestLogger creates a silent logger for classifier tests.
func newLLMClassifierTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	return logger
}

// =============================================================================
// NewLLMIntentClassifier Tests
// =============================================================================

func TestNewLLMIntentClassifier_NilRegistry(t *testing.T) {
	logger := newLLMClassifierTestLogger()

	lic := NewLLMIntentClassifier(nil, logger)

	require.NotNil(t, lic)
	assert.Nil(t, lic.providerRegistry)
	assert.NotNil(t, lic.fallbackClassifier)
	assert.Equal(t, logger, lic.logger)
}

func TestNewLLMIntentClassifier_WithRegistry(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	regCfg := &RegistryConfig{
		DefaultTimeout: 10 * time.Second,
		Providers:      make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)

	lic := NewLLMIntentClassifier(registry, logger)

	require.NotNil(t, lic)
	assert.Equal(t, registry, lic.providerRegistry)
	assert.NotNil(t, lic.fallbackClassifier)
}

// =============================================================================
// parseLLMIntentResponse Tests
// =============================================================================

func TestLLMIntentClassifier_parseLLMIntentResponse_ValidJSON(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	tests := []struct {
		name           string
		input          string
		expectedIntent string
		expectedConf   float64
		expectErr      bool
	}{
		{
			name: "confirmation intent",
			input: `{
				"intent": "confirmation",
				"confidence": 0.95,
				"is_actionable": true,
				"should_proceed": true,
				"reasoning": "User clearly approves",
				"detected_elements": ["yes", "approval"]
			}`,
			expectedIntent: "confirmation",
			expectedConf:   0.95,
			expectErr:      false,
		},
		{
			name: "refusal intent",
			input: `{
				"intent": "refusal",
				"confidence": 0.8,
				"is_actionable": false,
				"should_proceed": false,
				"reasoning": "User declines",
				"detected_elements": ["no"]
			}`,
			expectedIntent: "refusal",
			expectedConf:   0.8,
			expectErr:      false,
		},
		{
			name: "question intent",
			input: `{
				"intent": "question",
				"confidence": 0.7,
				"is_actionable": false,
				"should_proceed": false,
				"reasoning": "User is asking",
				"detected_elements": ["what", "?"]
			}`,
			expectedIntent: "question",
			expectedConf:   0.7,
			expectErr:      false,
		},
		{
			name: "request intent",
			input: `{
				"intent": "request",
				"confidence": 0.85,
				"is_actionable": true,
				"should_proceed": false,
				"reasoning": "New request",
				"detected_elements": ["please", "create"]
			}`,
			expectedIntent: "request",
			expectedConf:   0.85,
			expectErr:      false,
		},
		{
			name: "clarification intent",
			input: `{
				"intent": "clarification",
				"confidence": 0.6,
				"is_actionable": false,
				"should_proceed": false,
				"reasoning": "Needs more info",
				"detected_elements": ["explain"]
			}`,
			expectedIntent: "clarification",
			expectedConf:   0.6,
			expectErr:      false,
		},
		{
			name: "unclear intent",
			input: `{
				"intent": "unclear",
				"confidence": 0.3,
				"is_actionable": false,
				"should_proceed": false,
				"reasoning": "Cannot determine",
				"detected_elements": []
			}`,
			expectedIntent: "unclear",
			expectedConf:   0.3,
			expectErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lic.parseLLMIntentResponse(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedIntent, result.Intent)
			assert.Equal(t, tt.expectedConf, result.Confidence)
		})
	}
}

func TestLLMIntentClassifier_parseLLMIntentResponse_JSONWrappedInText(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	input := `Here is the classification:
	{"intent": "confirmation", "confidence": 0.9, "is_actionable": true, "should_proceed": true, "reasoning": "yes", "detected_elements": []}
	That's my analysis.`

	result, err := lic.parseLLMIntentResponse(input)
	require.NoError(t, err)
	assert.Equal(t, "confirmation", result.Intent)
	assert.Equal(t, 0.9, result.Confidence)
}

func TestLLMIntentClassifier_parseLLMIntentResponse_InvalidJSON(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	tests := []struct {
		name  string
		input string
	}{
		{"no JSON at all", "This is just plain text"},
		{"incomplete JSON", "{intent: confirmation"},
		{"empty string", ""},
		{"only whitespace", "   \t\n  "},
		{"braces but no valid JSON", "{ not valid json }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lic.parseLLMIntentResponse(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestLLMIntentClassifier_parseLLMIntentResponse_EmptyIntent(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	input := `{"intent": "", "confidence": 0.5}`

	_, err := lic.parseLLMIntentResponse(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "intent field is empty")
}

func TestLLMIntentClassifier_parseLLMIntentResponse_ConfidenceClamping(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	t.Run("negative confidence clamped to 0", func(t *testing.T) {
		input := `{"intent": "confirmation", "confidence": -0.5}`
		result, err := lic.parseLLMIntentResponse(input)
		require.NoError(t, err)
		assert.Equal(t, 0.0, result.Confidence)
	})

	t.Run("confidence above 1 clamped to 1", func(t *testing.T) {
		input := `{"intent": "confirmation", "confidence": 1.5}`
		result, err := lic.parseLLMIntentResponse(input)
		require.NoError(t, err)
		assert.Equal(t, 1.0, result.Confidence)
	})

	t.Run("normal confidence unchanged", func(t *testing.T) {
		input := `{"intent": "confirmation", "confidence": 0.75}`
		result, err := lic.parseLLMIntentResponse(input)
		require.NoError(t, err)
		assert.Equal(t, 0.75, result.Confidence)
	})
}

// =============================================================================
// convertToClassificationResult Tests
// =============================================================================

func TestLLMIntentClassifier_convertToClassificationResult(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	tests := []struct {
		name                       string
		llmResult                  *LLMIntentResponse
		expectedIntent             UserIntent
		expectedRequiresClarify    bool
		expectedIsActionable       bool
		expectedReasoningInSignals bool
	}{
		{
			name: "confirmation maps correctly",
			llmResult: &LLMIntentResponse{
				Intent:        "confirmation",
				Confidence:    0.9,
				IsActionable:  true,
				ShouldProceed: true,
				Reasoning:     "User confirmed",
			},
			expectedIntent:             IntentConfirmation,
			expectedRequiresClarify:    false,
			expectedIsActionable:       true,
			expectedReasoningInSignals: true,
		},
		{
			name: "refusal maps correctly",
			llmResult: &LLMIntentResponse{
				Intent:       "refusal",
				Confidence:   0.85,
				IsActionable: false,
				Reasoning:    "User said no",
			},
			expectedIntent:             IntentRefusal,
			expectedRequiresClarify:    false,
			expectedIsActionable:       false,
			expectedReasoningInSignals: true,
		},
		{
			name: "question sets requires clarification",
			llmResult: &LLMIntentResponse{
				Intent:     "question",
				Confidence: 0.7,
				Reasoning:  "User asked something",
			},
			expectedIntent:             IntentQuestion,
			expectedRequiresClarify:    true,
			expectedReasoningInSignals: true,
		},
		{
			name: "request maps correctly",
			llmResult: &LLMIntentResponse{
				Intent:       "request",
				Confidence:   0.8,
				IsActionable: true,
			},
			expectedIntent:          IntentRequest,
			expectedRequiresClarify: false,
			expectedIsActionable:    true,
		},
		{
			name: "clarification sets requires clarification",
			llmResult: &LLMIntentResponse{
				Intent:     "clarification",
				Confidence: 0.6,
			},
			expectedIntent:          IntentClarification,
			expectedRequiresClarify: true,
		},
		{
			name: "unknown intent maps to unclear",
			llmResult: &LLMIntentResponse{
				Intent:     "something_else",
				Confidence: 0.3,
			},
			expectedIntent:          IntentUnclear,
			expectedRequiresClarify: true,
		},
		{
			name: "empty reasoning not added to signals",
			llmResult: &LLMIntentResponse{
				Intent:     "confirmation",
				Confidence: 0.9,
				Reasoning:  "",
			},
			expectedIntent:             IntentConfirmation,
			expectedReasoningInSignals: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lic.convertToClassificationResult(tt.llmResult)

			assert.Equal(t, tt.expectedIntent, result.Intent)
			assert.Equal(t, tt.llmResult.Confidence, result.Confidence)
			assert.Equal(t, tt.expectedRequiresClarify, result.RequiresClarification)

			if tt.expectedReasoningInSignals {
				found := false
				for _, sig := range result.Signals {
					if len(sig) > 11 && sig[:11] == "llm_reason:" {
						found = true
						break
					}
				}
				assert.True(t, found, "Reasoning should be in signals")
			}
		})
	}
}

func TestLLMIntentClassifier_convertToClassificationResult_DetectedElements(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	llmResult := &LLMIntentResponse{
		Intent:           "confirmation",
		Confidence:       0.95,
		DetectedElements: []string{"yes", "approval", "go_ahead"},
	}

	result := lic.convertToClassificationResult(llmResult)

	// DetectedElements should be in Signals
	for _, elem := range llmResult.DetectedElements {
		assert.Contains(t, result.Signals, elem)
	}
}

func TestLLMIntentClassifier_convertToClassificationResult_ShouldProceedOverride(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	llmResult := &LLMIntentResponse{
		Intent:        "confirmation",
		Confidence:    0.9,
		IsActionable:  false, // initially false
		ShouldProceed: true,  // but should_proceed overrides for confirmation
	}

	result := lic.convertToClassificationResult(llmResult)

	assert.True(t, result.IsActionable,
		"ShouldProceed=true with confirmation intent should set IsActionable=true")
}

// =============================================================================
// buildIntentClassificationPrompt Tests
// =============================================================================

func TestLLMIntentClassifier_buildIntentClassificationPrompt(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	t.Run("without context", func(t *testing.T) {
		prompt := lic.buildIntentClassificationPrompt("Hello", "")
		assert.Contains(t, prompt, "Hello")
		assert.Contains(t, prompt, "USER MESSAGE")
		assert.NotContains(t, prompt, "CONVERSATION CONTEXT")
	})

	t.Run("with context", func(t *testing.T) {
		prompt := lic.buildIntentClassificationPrompt("Yes", "previous messages")
		assert.Contains(t, prompt, "Yes")
		assert.Contains(t, prompt, "CONVERSATION CONTEXT")
	})

	t.Run("truncates oversized user message to fit provider context", func(t *testing.T) {
		// Finding #42: Cerebras qwen-3-235b returns 400 context_length_exceeded
		// when full chat messages with code blocks exceed the classifier model's
		// 8192-token window. Cap user message at maxClassifierUserMessageChars.
		huge := strings.Repeat("a", maxClassifierUserMessageChars+1000)
		prompt := lic.buildIntentClassificationPrompt(huge, "")
		assert.Less(t, len(prompt), maxClassifierUserMessageChars+500,
			"prompt should be bounded near maxClassifierUserMessageChars")
		assert.Contains(t, prompt, "...[truncated]",
			"truncation marker should be present")
	})
}

// =============================================================================
// getSystemPrompt Tests
// =============================================================================

func TestLLMIntentClassifier_getSystemPrompt(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	prompt := lic.getSystemPrompt()

	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "intent classifier")
	assert.Contains(t, prompt, "confirmation")
	assert.Contains(t, prompt, "refusal")
	assert.Contains(t, prompt, "question")
	assert.Contains(t, prompt, "request")
	assert.Contains(t, prompt, "clarification")
	assert.Contains(t, prompt, "unclear")
	assert.Contains(t, prompt, "JSON")
}

// =============================================================================
// getClassificationProvider Tests
// =============================================================================

func TestLLMIntentClassifier_getClassificationProvider_NilRegistry(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	provider, name, err := lic.getClassificationProvider()
	// require, not assert: err.Error() below nil-derefs (panicking the whole
	// package binary and masking every later test) when the assertion fails.
	require.Error(t, err)
	assert.Nil(t, provider)
	assert.Empty(t, name)
	assert.Contains(t, err.Error(), "no provider registry")
}

func TestLLMIntentClassifier_getClassificationProvider_EmptyRegistry(t *testing.T) {
	// The synthesized "helixllm" default provider is enabled by the ambient
	// USE_HELIX_LLM env var, which would otherwise make this test's verdict
	// depend on host state. Pin it so the registry is genuinely empty.
	t.Setenv("USE_HELIX_LLM", "")

	logger := newLLMClassifierTestLogger()
	regCfg := &RegistryConfig{
		DefaultTimeout: 10 * time.Second,
		Providers:      make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)

	lic := NewLLMIntentClassifier(registry, logger)

	provider, name, err := lic.getClassificationProvider()
	// require, not assert: see NilRegistry above — a nil err must fail this
	// test cleanly, never panic the binary.
	require.Error(t, err)
	assert.Nil(t, provider)
	assert.Empty(t, name)
	assert.Contains(t, err.Error(), "no LLM providers available")
}

func TestLLMIntentClassifier_getClassificationProvider_SkipsMockProviders(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	regCfg := &RegistryConfig{
		DefaultTimeout:        10 * time.Second,
		MaxConcurrentRequests: 5,
		Providers:             make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)

	// Register providers with names that should be skipped
	mockNames := []string{"mock-provider", "test-llm", "provider1", "primary", "fallback1", "participant1", "agent-test"}
	for _, name := range mockNames {
		_ = registry.RegisterProvider(name, &mockLLMProvider{})
	}

	lic := NewLLMIntentClassifier(registry, logger)

	provider, name, err := lic.getClassificationProvider()
	assert.Error(t, err, "Should fail because all providers are mock/test-like")
	assert.Nil(t, provider)
	assert.Empty(t, name)
}

func TestLLMIntentClassifier_getClassificationProvider_ReturnsRealProvider(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	regCfg := &RegistryConfig{
		DefaultTimeout:        10 * time.Second,
		MaxConcurrentRequests: 5,
		Providers:             make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)

	// Register a provider with a real-sounding name
	_ = registry.RegisterProvider("deepseek", &mockLLMProvider{})

	lic := NewLLMIntentClassifier(registry, logger)

	provider, name, err := lic.getClassificationProvider()
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "deepseek", name)
}

func TestLLMIntentClassifier_getClassificationProvider_SkipsCooledDownProvider(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	regCfg := &RegistryConfig{
		DefaultTimeout:        10 * time.Second,
		MaxConcurrentRequests: 5,
		Providers:             make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)
	require.NoError(t, registry.RegisterProvider("cerebras", &mockLLMProvider{}))
	require.NoError(t, registry.RegisterProvider("mistral", &mockLLMProvider{}))

	lic := NewLLMIntentClassifier(registry, logger)

	// Cerebras is preferred and would be returned first.
	_, name, err := lic.getClassificationProvider()
	require.NoError(t, err)
	assert.Equal(t, "cerebras", name, "without cooldown, cerebras should be picked")

	// Bench cerebras → mistral should be selected next.
	lic.markProviderCooldown("cerebras")
	_, name, err = lic.getClassificationProvider()
	require.NoError(t, err)
	assert.Equal(t, "mistral", name, "with cerebras cooled down, mistral should be picked")
}

func TestLLMIntentClassifier_isQuotaError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("transient network blip"), false},
		{fmt.Errorf("HTTP 429 too many requests"), true},
		{fmt.Errorf("provider returned token_quota_exceeded"), true},
		{fmt.Errorf("Tokens per day limit exceeded - too many tokens processed"), true},
		{fmt.Errorf("queue_exceeded: try again later"), true},
		{fmt.Errorf("Rate limit hit"), true},
	}
	for _, c := range cases {
		got := isQuotaError(c.err)
		assert.Equal(t, c.want, got, "isQuotaError(%v)", c.err)
	}
}

// =============================================================================
// ClassifyIntentWithLLM Tests
// =============================================================================

func TestLLMIntentClassifier_ClassifyIntentWithLLM_NoProvider(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	result, err := lic.ClassifyIntentWithLLM(context.Background(), "Yes, go ahead!", "context")
	require.NoError(t, err)
	assert.NotNil(t, result, "Should fall back to pattern-based classifier")
}

func TestLLMIntentClassifier_ClassifyIntentWithLLM_WithMockedProvider(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	regCfg := &RegistryConfig{
		DefaultTimeout:        10 * time.Second,
		MaxConcurrentRequests: 5,
		Providers:             make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)

	mock := &mockLLMProvider{
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				Content: `{"intent": "confirmation", "confidence": 0.95, "is_actionable": true, "should_proceed": true, "reasoning": "User said yes", "detected_elements": ["yes"]}`,
			}, nil
		},
	}
	_ = registry.RegisterProvider("cerebras", mock)

	lic := NewLLMIntentClassifier(registry, logger)

	result, err := lic.ClassifyIntentWithLLM(context.Background(), "Yes!", "previous context")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, IntentConfirmation, result.Intent)
	assert.True(t, result.IsActionable)
}

func TestLLMIntentClassifier_ClassifyIntentWithLLM_ProviderReturnsInvalidJSON(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	regCfg := &RegistryConfig{
		DefaultTimeout:        10 * time.Second,
		MaxConcurrentRequests: 5,
		Providers:             make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)

	mock := &mockLLMProvider{
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				Content: "I cannot classify this properly",
			}, nil
		},
	}
	_ = registry.RegisterProvider("cerebras", mock)

	lic := NewLLMIntentClassifier(registry, logger)

	result, err := lic.ClassifyIntentWithLLM(context.Background(), "Yes!", "context")
	require.NoError(t, err, "Should fall back to pattern-based classifier")
	assert.NotNil(t, result)
}

func TestLLMIntentClassifier_ClassifyIntentWithLLM_ProviderError(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	regCfg := &RegistryConfig{
		DefaultTimeout:        10 * time.Second,
		MaxConcurrentRequests: 5,
		Providers:             make(map[string]*ProviderConfig),
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(regCfg, nil)

	mock := &mockLLMProvider{
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return nil, assert.AnError
		},
	}
	_ = registry.RegisterProvider("cerebras", mock)

	lic := NewLLMIntentClassifier(registry, logger)

	result, err := lic.ClassifyIntentWithLLM(context.Background(), "Yes!", "context")
	require.NoError(t, err, "Should fall back to pattern-based classifier")
	assert.NotNil(t, result)
}

// =============================================================================
// QuickClassify Tests
// =============================================================================

func TestLLMIntentClassifier_QuickClassify_ShortMessageWithContext(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	// Short message with context: attempts LLM first, falls back to pattern
	result := lic.QuickClassify(context.Background(), "Yes!", true)

	assert.NotNil(t, result)
	assert.True(t, result.IsConfirmation() || result.ShouldProceed())
}

func TestLLMIntentClassifier_QuickClassify_LongMessage(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	// Long message goes straight to pattern-based
	longMsg := "This is a very long message that exceeds the fifty character limit so it will use the pattern-based fallback classifier directly."
	result := lic.QuickClassify(context.Background(), longMsg, false)

	assert.NotNil(t, result)
}

func TestLLMIntentClassifier_QuickClassify_ShortMessageNoContext(t *testing.T) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)

	// Short message without context: goes to pattern-based directly
	result := lic.QuickClassify(context.Background(), "No", false)

	assert.NotNil(t, result)
}

// =============================================================================
// IntentClassificationCache Tests
// =============================================================================

func TestNewIntentClassificationCache_Creation(t *testing.T) {
	tests := []struct {
		name    string
		maxSize int
		ttl     time.Duration
	}{
		{"small cache", 10, 1 * time.Minute},
		{"medium cache", 100, 5 * time.Minute},
		{"large cache", 1000, 30 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewIntentClassificationCache(tt.maxSize, tt.ttl)
			require.NotNil(t, cache)
			assert.Equal(t, tt.maxSize, cache.maxSize)
			assert.Equal(t, tt.ttl, cache.ttl)
			assert.NotNil(t, cache.cache)
			assert.Empty(t, cache.cache)
		})
	}
}

func TestIntentClassificationCache_Set_And_Get(t *testing.T) {
	cache := NewIntentClassificationCache(10, 5*time.Minute)

	result := &IntentClassificationResult{
		Intent:       IntentConfirmation,
		Confidence:   0.95,
		IsActionable: true,
		Signals:      []string{"yes"},
	}

	cache.Set("test message", result)

	retrieved, found := cache.Get("test message")
	assert.True(t, found)
	require.NotNil(t, retrieved)
	assert.Equal(t, IntentConfirmation, retrieved.Intent)
	assert.Equal(t, 0.95, retrieved.Confidence)
	assert.True(t, retrieved.IsActionable)
}

func TestIntentClassificationCache_Get_CaseInsensitive(t *testing.T) {
	cache := NewIntentClassificationCache(10, 5*time.Minute)

	result := &IntentClassificationResult{
		Intent:     IntentRefusal,
		Confidence: 0.8,
	}

	cache.Set("Test Message", result)

	tests := []struct {
		name  string
		key   string
		found bool
	}{
		{"lowercase", "test message", true},
		{"uppercase", "TEST MESSAGE", true},
		{"mixed case", "Test Message", true},
		{"with whitespace trim", "  test message  ", true},
		{"different message", "different", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found := cache.Get(tt.key)
			assert.Equal(t, tt.found, found)
		})
	}
}

func TestIntentClassificationCache_Get_MissForNonExistent(t *testing.T) {
	cache := NewIntentClassificationCache(10, 5*time.Minute)

	result, found := cache.Get("nonexistent")
	assert.False(t, found)
	assert.Nil(t, result)
}

func TestLLMIntentClassificationCache_TTLExpiration(t *testing.T) {
	cache := NewIntentClassificationCache(10, 1*time.Millisecond)

	cache.Set("expiring", &IntentClassificationResult{
		Intent: IntentConfirmation,
	})

	// Should exist immediately
	_, found := cache.Get("expiring")
	assert.True(t, found)

	// Wait for TTL
	time.Sleep(5 * time.Millisecond)

	// Should be expired
	_, found = cache.Get("expiring")
	assert.False(t, found, "Entry should be expired after TTL")
}

func TestLLMIntentClassificationCache_TTLExpiration_DeletesEntry(t *testing.T) {
	cache := NewIntentClassificationCache(10, 1*time.Millisecond)

	cache.Set("expiring", &IntentClassificationResult{
		Intent: IntentConfirmation,
	})

	time.Sleep(5 * time.Millisecond)

	// Get triggers deletion of expired entry
	_, _ = cache.Get("expiring")

	// Internal map should no longer contain the entry
	assert.NotContains(t, cache.cache, "expiring")
}

func TestLLMIntentClassificationCache_MaxSizeEviction(t *testing.T) {
	cache := NewIntentClassificationCache(3, 5*time.Minute)

	// Fill cache to capacity
	cache.Set("msg-1", &IntentClassificationResult{Intent: IntentConfirmation})
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	cache.Set("msg-2", &IntentClassificationResult{Intent: IntentRefusal})
	time.Sleep(1 * time.Millisecond)
	cache.Set("msg-3", &IntentClassificationResult{Intent: IntentQuestion})

	// Cache is full, adding another should evict oldest (msg-1)
	cache.Set("msg-4", &IntentClassificationResult{Intent: IntentRequest})

	assert.Equal(t, 3, len(cache.cache))

	// msg-4 should exist
	_, found := cache.Get("msg-4")
	assert.True(t, found)
}

func TestIntentClassificationCache_Set_OverwriteExisting(t *testing.T) {
	cache := NewIntentClassificationCache(10, 5*time.Minute)

	// Set initial value
	cache.Set("msg", &IntentClassificationResult{
		Intent:     IntentQuestion,
		Confidence: 0.5,
	})

	// Overwrite with new value
	cache.Set("msg", &IntentClassificationResult{
		Intent:     IntentConfirmation,
		Confidence: 0.9,
	})

	retrieved, found := cache.Get("msg")
	assert.True(t, found)
	assert.Equal(t, IntentConfirmation, retrieved.Intent)
	assert.Equal(t, 0.9, retrieved.Confidence)
}

// =============================================================================
// CachedClassification Tests
// =============================================================================

func TestCachedClassification_Fields(t *testing.T) {
	now := time.Now()
	cached := CachedClassification{
		Message: "test message",
		Result: &IntentClassificationResult{
			Intent:     IntentConfirmation,
			Confidence: 0.9,
		},
		Timestamp: now,
	}

	assert.Equal(t, "test message", cached.Message)
	assert.Equal(t, IntentConfirmation, cached.Result.Intent)
	assert.Equal(t, now, cached.Timestamp)
}

// =============================================================================
// LLMIntentResponse Tests
// =============================================================================

func TestLLMIntentResponse_Fields(t *testing.T) {
	resp := LLMIntentResponse{
		Intent:           "confirmation",
		Confidence:       0.95,
		IsActionable:     true,
		ShouldProceed:    true,
		Reasoning:        "User said yes",
		DetectedElements: []string{"yes", "approval"},
	}

	assert.Equal(t, "confirmation", resp.Intent)
	assert.Equal(t, 0.95, resp.Confidence)
	assert.True(t, resp.IsActionable)
	assert.True(t, resp.ShouldProceed)
	assert.Equal(t, "User said yes", resp.Reasoning)
	assert.Len(t, resp.DetectedElements, 2)
}

// =============================================================================
// truncateString Tests
// =============================================================================

func TestLLMIntentClassifier_TruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 5, "hello..."},
		{"empty string", "", 10, ""},
		{"zero max length", "hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkIntentClassificationCache_Set(b *testing.B) {
	cache := NewIntentClassificationCache(1000, 5*time.Minute)
	result := &IntentClassificationResult{
		Intent:     IntentConfirmation,
		Confidence: 0.9,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("test message", result)
	}
}

func BenchmarkIntentClassificationCache_Get(b *testing.B) {
	cache := NewIntentClassificationCache(1000, 5*time.Minute)
	cache.Set("test message", &IntentClassificationResult{
		Intent:     IntentConfirmation,
		Confidence: 0.9,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get("test message")
	}
}

func BenchmarkLLMIntentClassifier_parseLLMIntentResponse(b *testing.B) {
	logger := newLLMClassifierTestLogger()
	lic := NewLLMIntentClassifier(nil, logger)
	input := `{"intent": "confirmation", "confidence": 0.95, "is_actionable": true, "should_proceed": true, "reasoning": "test", "detected_elements": ["yes"]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = lic.parseLLMIntentResponse(input)
	}
}

// Ensure the unused import is used (llm is needed for interface compliance check).
var _ llm.LLMProvider = (*mockLLMProvider)(nil)
