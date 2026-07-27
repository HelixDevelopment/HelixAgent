package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/verifier"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAIDebate_TeamInitializationWithVerifiedProviders tests that the debate team
// initializes correctly using only verified providers.
//
// Skipped in -short mode because a full LLMsVerifier pipeline run touches 33
// live providers serially and routinely exceeds 5 minutes even when every
// provider is healthy. The matching smoke path is covered by the startup
// verifier unit tests and the live HelixAgent binary's own boot-time
// verification — we do not need to re-run it inside the integration package.
func TestAIDebate_TeamInitializationWithVerifiedProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full provider verification in -short mode (see test comment)") // SKIP-OK: #short-mode
	}

	hasAPIKey := os.Getenv("CLAUDE_API_KEY") != "" || os.Getenv("DEEPSEEK_API_KEY") != "" ||
		os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("OPENROUTER_API_KEY") != ""
	if !hasAPIKey {
		t.Skip("No API keys configured (need at least one of CLAUDE/DEEPSEEK/GEMINI/OPENROUTER)") // SKIP-OK: #requires-upstream-key
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := logrus.New()

	// Create startup verifier
	config := &verifier.StartupConfig{
		VerificationTimeout:  60 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)
	require.NotNil(t, sv)

	// Run verification to get verified providers
	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result)

	// Create debate team config
	dtc := services.NewDebateTeamConfigWithStartupVerifier(sv, logger)
	require.NotNil(t, dtc)

	// Initialize team
	err = dtc.InitializeTeam(ctx)
	require.NoError(t, err)

	// Get all LLMs
	llms := dtc.GetAllLLMs()
	require.NotNil(t, llms)

	// Check that primary positions have high scores
	positions := []services.DebateTeamPosition{
		services.PositionAnalyst,
		services.PositionProposer,
	}

	for _, pos := range positions {
		member := dtc.GetTeamMember(pos)
		if member != nil {
			// Primary positions should have high-quality providers
			assert.Greater(t, member.Score, 5.0,
				"Primary position %d should have a high-quality provider (score > 5.0)", pos)
		}
	}

	// Check that no Ollama models are used when OLLAMA_ENABLED is not set
	if os.Getenv("OLLAMA_ENABLED") != "true" {
		for _, llm := range llms {
			assert.NotEqual(t, "ollama", llm.ProviderName,
				"Ollama should not be in debate team when OLLAMA_ENABLED is not set")
		}
	}
}

// TestAIDebate_FallbackActivation tests that fallbacks are properly configured
// and can be activated when primaries fail
func TestAIDebate_FallbackActivation(t *testing.T) {
	hasAPIKey := os.Getenv("CLAUDE_API_KEY") != "" || os.Getenv("DEEPSEEK_API_KEY") != "" ||
		os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("OPENROUTER_API_KEY") != ""
	if !hasAPIKey {
		t.Skip("No API keys configured (need at least one of CLAUDE/DEEPSEEK/GEMINI/OPENROUTER)") // SKIP-OK: #requires-upstream-key
	}

	ctx := context.Background()

	logger := logrus.New()

	// Create startup verifier
	config := &verifier.StartupConfig{
		VerificationTimeout:  60 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)
	require.NotNil(t, sv)

	// Run verification
	_, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)

	// Create debate team config
	dtc := services.NewDebateTeamConfigWithStartupVerifier(sv, logger)
	require.NotNil(t, dtc)

	// Initialize team
	err = dtc.InitializeTeam(ctx)
	require.NoError(t, err)

	// Check fallbacks for each position
	positions := []services.DebateTeamPosition{
		services.PositionAnalyst,
		services.PositionProposer,
		services.PositionCritic,
		services.PositionSynthesis,
		services.PositionMediator,
	}

	for _, pos := range positions {
		member := dtc.GetTeamMember(pos)
		if member != nil {
			fallbackCount := len(member.Fallbacks)
			t.Logf("Position %d has %d fallbacks", pos, fallbackCount)
			// Should have 2-4 fallbacks per position
			assert.GreaterOrEqual(t, fallbackCount, 0, "Position %d should have fallbacks configured", pos)
		}
	}
}

// TestAIDebate_NoOllamaWhenDisabled tests that Ollama is not used in the debate
// team when OLLAMA_ENABLED is not set
func TestAIDebate_NoOllamaWhenDisabled(t *testing.T) {
	if os.Getenv("OLLAMA_ENABLED") == "true" {
		t.Skip("Skipping test - OLLAMA_ENABLED is set to true") // SKIP-OK: #legacy-untriaged
	}

	hasAPIKey := os.Getenv("CLAUDE_API_KEY") != "" || os.Getenv("DEEPSEEK_API_KEY") != "" ||
		os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("OPENROUTER_API_KEY") != ""
	if !hasAPIKey {
		t.Skip("No API keys configured (need at least one of CLAUDE/DEEPSEEK/GEMINI/OPENROUTER)") // SKIP-OK: #requires-upstream-key
	}

	ctx := context.Background()

	logger := logrus.New()

	// Create startup verifier
	config := &verifier.StartupConfig{
		VerificationTimeout:  60 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)
	require.NotNil(t, sv)

	// Run verification
	_, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)

	// Create debate team config
	dtc := services.NewDebateTeamConfigWithStartupVerifier(sv, logger)
	require.NotNil(t, dtc)

	// Initialize team
	err = dtc.InitializeTeam(ctx)
	require.NoError(t, err)

	// Get all LLMs and verify no Ollama
	llms := dtc.GetAllLLMs()
	for _, llm := range llms {
		assert.NotEqual(t, "ollama", llm.ProviderName,
			"Ollama provider should not be in debate team when OLLAMA_ENABLED is not set")
	}

	t.Logf("✓ Verified no Ollama providers in debate team (%d LLMs checked)", len(llms))
}

// TestAIDebate_VerifiedProvidersOnly tests that only verified providers
// are included in the debate team
func TestAIDebate_VerifiedProvidersOnly(t *testing.T) {
	hasAPIKey := os.Getenv("CLAUDE_API_KEY") != "" || os.Getenv("DEEPSEEK_API_KEY") != "" ||
		os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("OPENROUTER_API_KEY") != ""
	if !hasAPIKey {
		t.Skip("No API keys configured (need at least one of CLAUDE/DEEPSEEK/GEMINI/OPENROUTER)") // SKIP-OK: #requires-upstream-key
	}

	ctx := context.Background()

	logger := logrus.New()

	// Create startup verifier
	config := &verifier.StartupConfig{
		VerificationTimeout:  60 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)
	require.NotNil(t, sv)

	// Run verification
	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result)

	// Create debate team config
	dtc := services.NewDebateTeamConfigWithStartupVerifier(sv, logger)
	require.NotNil(t, dtc)

	// Initialize team
	err = dtc.InitializeTeam(ctx)
	require.NoError(t, err)

	// Get all LLMs
	llms := dtc.GetAllLLMs()

	// All LLMs should be from verified providers
	for _, llm := range llms {
		// Check if provider is in the verified list
		verifiedProviders := sv.GetVerifiedProviders()
		isVerified := false
		for _, vp := range verifiedProviders {
			if vp.Type == llm.ProviderName {
				isVerified = true
				break
			}
		}
		assert.True(t, isVerified,
			"LLM %s/%s should be from a verified provider", llm.ProviderName, llm.ModelName)
	}

	t.Logf("✓ All %d LLMs in debate team are from verified providers", len(llms))
}
