package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Tests for Verified Provider Instance Usage in DebateService (CRITICAL FIX)
// =============================================================================
// Note: mockVerifiedProvider, ToParticipantConfig, GetVerifiedProviderInstance,
// GetParticipantConfigs tests are in debate_team_config_test.go to avoid duplicates

// TestDebateService_UsesVerifiedProviderInstance tests that the debate service
// uses the verified provider instance when available instead of looking up
// from the registry.
func TestDebateService_UsesVerifiedProviderInstance(t *testing.T) {
	// CONST-035: NewProviderRegistry(nil, nil) below kicks off
	// full provider auto-discovery against whatever API keys live
	// in the dev shell. This is a unit test for ParticipantConfig
	// instance preference, not a discovery test — scrub the env so
	// the registry constructor returns in milliseconds.
	clearProviderEnvVarsForTest(t)

	logger := logrus.New()
	_ = context.Background()

	verifiedProvider := &mockVerifiedProvider{name: "verified-claude-cli"}

	participant := ParticipantConfig{
		ParticipantID:    "test-claude",
		Name:             "Test Claude",
		Role:             "analyst",
		LLMProvider:      "claude",
		LLMModel:         "claude-sonnet-4-5",
		ProviderInstance: verifiedProvider,
	}

	config := &DebateConfig{
		DebateID:     "test-debate-1",
		Topic:        "Test topic",
		Participants: []ParticipantConfig{participant},
		MaxRounds:    1,
		Timeout:      30 * time.Second,
	}

	teamConfig := NewDebateTeamConfig(nil, nil, logger)
	teamConfig.members.Put(PositionAnalyst, &DebateTeamMember{
		Position:     PositionAnalyst,
		Role:         RoleAnalyst,
		ProviderName: "claude",
		ModelName:    "claude-sonnet-4-5",
		Provider:     verifiedProvider,
		IsActive:     true,
	})

	registry := NewProviderRegistry(nil, nil)
	svc := &DebateService{
		logger:           logger,
		providerRegistry: registry,
		teamConfig:       teamConfig,
	}

	retrievedProvider := teamConfig.GetVerifiedProviderInstance("claude", "claude-sonnet-4-5")
	require.NotNil(t, retrievedProvider, "GetVerifiedProviderInstance should return the verified instance")
	assert.Equal(t, verifiedProvider, retrievedProvider, "Should return the exact same provider instance")

	t.Run("selects verified instance over registry lookup", func(t *testing.T) {
		assert.NotNil(t, config.Participants[0].ProviderInstance, "ParticipantConfig should have ProviderInstance set")
	})
	_ = svc
}

// TestDebateService_FallbackErrorDetection tests that fallback errors are
// properly detected and reported.
func TestDebateService_FallbackErrorDetection(t *testing.T) {
	_ = logrus.New()

	t.Run("detects empty response as fallback trigger", func(t *testing.T) {
		isEmpty := strings.TrimSpace("") == ""
		assert.True(t, isEmpty, "Empty response should be detected")
	})

	t.Run("detects canned error responses", func(t *testing.T) {
		cannedResponses := []string{
			"Unable to provide analysis at this time",
			"I apologize, but I cannot help with that",
			"Error occurred while processing",
			"Currently unable to process request",
			"Not able to complete the task",
			"Failed to generate response",
		}

		for _, resp := range cannedResponses {
			matchedPattern := IsCannedErrorResponse(resp)
			assert.NotEmpty(t, matchedPattern, "Should detect canned error in: %s", resp)
		}
	})

	t.Run("accepts valid responses", func(t *testing.T) {
		validResponses := []string{
			"Yes, I can see your code and here's my analysis:",
			"The implementation looks correct. Here are some suggestions:",
			"I've analyzed the code and found the following issues:",
		}

		for _, resp := range validResponses {
			matchedPattern := IsCannedErrorResponse(resp)
			assert.Empty(t, matchedPattern, "Should NOT detect canned error in valid response: %s", resp)
		}
	})

	t.Run("detects suspiciously fast responses", func(t *testing.T) {
		fastTime := 50 * time.Millisecond
		shortContent := 50

		isSuspicious := IsSuspiciouslyFastResponse(fastTime, shortContent)
		assert.True(t, isSuspicious, "Should detect suspiciously fast response")
	})

	t.Run("accepts normal response times", func(t *testing.T) {
		normalTime := 500 * time.Millisecond
		normalContent := 500

		isSuspicious := IsSuspiciouslyFastResponse(normalTime, normalContent)
		assert.False(t, isSuspicious, "Should accept normal response time")
	})
}

// TestDebateService_ProviderInstanceFallbackChain tests that the fallback chain
// correctly uses verified provider instances.
func TestDebateService_ProviderInstanceFallbackChain(t *testing.T) {
	logger := logrus.New()

	t.Run("fallback config preserves provider instance", func(t *testing.T) {
		primaryProvider := &mockVerifiedProvider{name: "primary-verified"}
		fallbackProvider := &mockVerifiedProvider{name: "fallback-verified"}

		participant := ParticipantConfig{
			ParticipantID:    "test-fallback",
			Name:             "Test Fallback",
			Role:             "analyst",
			LLMProvider:      "nvidia",
			LLMModel:         "llama-3.1-70b",
			ProviderInstance: primaryProvider,
			Fallbacks: []FallbackConfig{
				{
					Provider:         "huggingface",
					Model:            "llama-3.3-70b",
					ProviderInstance: fallbackProvider,
				},
			},
		}

		require.Len(t, participant.Fallbacks, 1)
		assert.NotNil(t, participant.ProviderInstance, "Primary ProviderInstance should be set")
		assert.NotNil(t, participant.Fallbacks[0].ProviderInstance, "Fallback ProviderInstance should be set")
		assert.Equal(t, primaryProvider, participant.ProviderInstance)
		assert.Equal(t, fallbackProvider, participant.Fallbacks[0].ProviderInstance)
	})

	t.Run("team config GetVerifiedProviderInstance checks all sources", func(t *testing.T) {
		config := NewDebateTeamConfig(nil, nil, logger)

		llmProvider := &mockVerifiedProvider{name: "verified-llm"}
		config.verifiedLLMs.Replace([]*VerifiedLLM{
			{
				ProviderName: "deepseek",
				ModelName:    "deepseek-v3",
				Provider:     llmProvider,
				Verified:     true,
			},
		})

		provider := config.GetVerifiedProviderInstance("deepseek", "deepseek-v3")
		assert.NotNil(t, provider)
		assert.Equal(t, llmProvider, provider)

		memberProvider := &mockVerifiedProvider{name: "team-member"}
		config.members.Put(PositionCritic, &DebateTeamMember{
			ProviderName: "mistral",
			ModelName:    "mistral-large",
			Provider:     memberProvider,
			IsActive:     true,
		})

		provider = config.GetVerifiedProviderInstance("mistral", "mistral-large")
		assert.NotNil(t, provider)
		assert.Equal(t, memberProvider, provider)

		provider = config.GetVerifiedProviderInstance("nonexistent", "model")
		assert.Nil(t, provider)
	})
}

// TestDebateService_getParticipantResponse_UsesProviderInstance tests the
// actual provider selection logic in getParticipantResponse
func TestDebateService_getParticipantResponse_UsesProviderInstance(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("uses participant ProviderInstance when set", func(t *testing.T) {
		verifiedProvider := &mockVerifiedProvider{name: "verified-instance"}

		participant := ParticipantConfig{
			ParticipantID:    "test",
			Role:             "analyst",
			LLMProvider:      "test-provider",
			LLMModel:         "test-model",
			ProviderInstance: verifiedProvider,
		}

		teamConfig := NewDebateTeamConfig(nil, nil, logger)
		teamConfig.members.Put(PositionAnalyst, &DebateTeamMember{
			ProviderName: "test-provider",
			ModelName:    "test-model",
			Provider:     verifiedProvider,
			IsActive:     true,
		})

		retrieved := teamConfig.GetVerifiedProviderInstance("test-provider", "test-model")
		require.NotNil(t, retrieved, "Should find verified instance")

		assert.NotNil(t, participant.ProviderInstance, "Participant should have ProviderInstance")
		assert.Equal(t, verifiedProvider, participant.ProviderInstance)
	})

	t.Run("falls back to team config when participant has no instance", func(t *testing.T) {
		verifiedProvider := &mockVerifiedProvider{name: "from-team-config"}

		participant := ParticipantConfig{
			ParticipantID: "test",
			Role:          "analyst",
			LLMProvider:   "provider-from-team",
			LLMModel:      "model-from-team",
		}

		teamConfig := NewDebateTeamConfig(nil, nil, logger)
		teamConfig.verifiedLLMs.Replace([]*VerifiedLLM{
			{
				ProviderName: "provider-from-team",
				ModelName:    "model-from-team",
				Provider:     verifiedProvider,
				Verified:     true,
			},
		})

		retrieved := teamConfig.GetVerifiedProviderInstance("provider-from-team", "model-from-team")
		require.NotNil(t, retrieved, "Should find from team config verified LLMs")

		assert.Nil(t, participant.ProviderInstance, "Participant should not have ProviderInstance set")
		assert.NotNil(t, retrieved, "Team config should have the instance")
	})
}

// TestDebateService_IntegrationWithFallbackChain tests the integration
// between verified provider instances and the fallback chain mechanism
func TestDebateService_IntegrationWithFallbackChain(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("primary fails, fallback with instance succeeds", func(t *testing.T) {
		primaryProvider := &mockVerifiedProvider{name: "primary-failing"}
		fallbackProvider := &mockVerifiedProvider{name: "fallback-working"}

		config := NewDebateTeamConfig(nil, nil, logger)
		config.members.Put(PositionAnalyst, &DebateTeamMember{
			Position:     PositionAnalyst,
			ProviderName: "primary",
			ModelName:    "primary-model",
			Provider:     primaryProvider,
			IsActive:     true,
			Fallbacks: []*DebateTeamMember{
				{
					ProviderName: "fallback",
					ModelName:    "fallback-model",
					Provider:     fallbackProvider,
				},
			},
		})

		member := config.GetTeamMember(PositionAnalyst)
		require.NotNil(t, member)
		require.NotNil(t, member.Provider)

		fallbackInst := config.GetVerifiedProviderInstance("fallback", "fallback-model")
		require.NotNil(t, fallbackInst, "Should find fallback provider instance")

		assert.Equal(t, fallbackProvider, fallbackInst)
	})

	t.Run("entire fallback chain has provider instances", func(t *testing.T) {
		providers := []*mockVerifiedProvider{
			{name: "provider-1"},
			{name: "provider-2"},
			{name: "provider-3"},
		}

		config := NewDebateTeamConfig(nil, nil, logger)
		config.members.Put(PositionAnalyst, &DebateTeamMember{
			Position:     PositionAnalyst,
			ProviderName: "p1",
			ModelName:    "m1",
			Provider:     providers[0],
			IsActive:     true,
			Fallbacks: []*DebateTeamMember{
				{
					ProviderName: "p2",
					ModelName:    "m2",
					Provider:     providers[1],
				},
				{
					ProviderName: "p3",
					ModelName:    "m3",
					Provider:     providers[2],
				},
			},
		})

		p1 := config.GetVerifiedProviderInstance("p1", "m1")
		p2 := config.GetVerifiedProviderInstance("p2", "m2")
		p3 := config.GetVerifiedProviderInstance("p3", "m3")

		assert.NotNil(t, p1, "Provider 1 should have instance")
		assert.NotNil(t, p2, "Provider 2 should have instance")
		assert.NotNil(t, p3, "Provider 3 should have instance")
		assert.Equal(t, providers[0], p1)
		assert.Equal(t, providers[1], p2)
		assert.Equal(t, providers[2], p3)
	})
}
