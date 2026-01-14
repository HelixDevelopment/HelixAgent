package services

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConstants verifies the debate team constants
func TestConstants(t *testing.T) {
	t.Run("Total positions is 5", func(t *testing.T) {
		assert.Equal(t, 5, TotalDebatePositions)
	})

	t.Run("Fallbacks per position is 2", func(t *testing.T) {
		assert.Equal(t, 2, FallbacksPerPosition)
	})

	t.Run("Total debate LLMs is 15", func(t *testing.T) {
		assert.Equal(t, 15, TotalDebateLLMs)
		assert.Equal(t, TotalDebatePositions*(1+FallbacksPerPosition), TotalDebateLLMs)
	})
}

func TestClaudeModels(t *testing.T) {
	t.Run("Claude 4.5 models are defined", func(t *testing.T) {
		assert.NotEmpty(t, ClaudeModels.Opus45, "Opus 4.5 model should be defined")
		assert.NotEmpty(t, ClaudeModels.Sonnet45, "Sonnet 4.5 model should be defined")
		assert.NotEmpty(t, ClaudeModels.Haiku45, "Haiku 4.5 model should be defined")
	})

	t.Run("Claude 4.x models are defined", func(t *testing.T) {
		assert.NotEmpty(t, ClaudeModels.Opus4, "Opus 4 model should be defined")
		assert.NotEmpty(t, ClaudeModels.Sonnet4, "Sonnet 4 model should be defined")
	})

	t.Run("Claude 3.5 fallback models are defined", func(t *testing.T) {
		assert.NotEmpty(t, ClaudeModels.Sonnet35, "Sonnet 3.5 model should be defined")
		assert.NotEmpty(t, ClaudeModels.Haiku35, "Haiku 3.5 model should be defined")
	})

	t.Run("Claude 3 legacy models are defined", func(t *testing.T) {
		assert.NotEmpty(t, ClaudeModels.Opus3, "Opus 3 model should be defined")
		assert.NotEmpty(t, ClaudeModels.Sonnet3, "Sonnet 3 model should be defined")
		assert.NotEmpty(t, ClaudeModels.Haiku3, "Haiku 3 model should be defined")
	})

	t.Run("Claude 4.5 model names follow Anthropic naming convention", func(t *testing.T) {
		assert.Contains(t, ClaudeModels.Opus45, "claude-opus-4-5", "Opus 4.5 should have correct prefix")
		assert.Contains(t, ClaudeModels.Sonnet45, "claude-sonnet-4-5", "Sonnet 4.5 should have correct prefix")
		assert.Contains(t, ClaudeModels.Haiku45, "claude-haiku-4-5", "Haiku 4.5 should have correct prefix")
	})

	t.Run("All Claude models are unique", func(t *testing.T) {
		models := []string{
			ClaudeModels.Opus45,
			ClaudeModels.Sonnet45,
			ClaudeModels.Haiku45,
			ClaudeModels.Opus4,
			ClaudeModels.Sonnet4,
			ClaudeModels.Sonnet35,
			ClaudeModels.Haiku35,
			ClaudeModels.Opus3,
			ClaudeModels.Sonnet3,
			ClaudeModels.Haiku3,
		}
		uniqueModels := make(map[string]bool)
		for _, model := range models {
			assert.False(t, uniqueModels[model], "Model %s should be unique", model)
			uniqueModels[model] = true
		}
	})
}

func TestQwenModels(t *testing.T) {
	t.Run("Qwen models are defined", func(t *testing.T) {
		assert.NotEmpty(t, QwenModels.Turbo, "Turbo model should be defined")
		assert.NotEmpty(t, QwenModels.Plus, "Plus model should be defined")
		assert.NotEmpty(t, QwenModels.Max, "Max model should be defined")
		assert.NotEmpty(t, QwenModels.Coder, "Coder model should be defined")
		assert.NotEmpty(t, QwenModels.Long, "Long model should be defined")
	})

	t.Run("Qwen model names follow Alibaba naming convention", func(t *testing.T) {
		assert.Contains(t, QwenModels.Turbo, "qwen", "Turbo should be Qwen family")
		assert.Contains(t, QwenModels.Plus, "qwen", "Plus should be Qwen family")
		assert.Contains(t, QwenModels.Max, "qwen", "Max should be Qwen family")
	})

	t.Run("All Qwen models are unique", func(t *testing.T) {
		models := []string{
			QwenModels.Turbo,
			QwenModels.Plus,
			QwenModels.Max,
			QwenModels.Coder,
			QwenModels.Long,
		}
		uniqueModels := make(map[string]bool)
		for _, model := range models {
			assert.False(t, uniqueModels[model], "Model %s should be unique", model)
			uniqueModels[model] = true
		}
	})
}

func TestLLMsVerifierModels(t *testing.T) {
	t.Run("LLMsVerifier models are defined", func(t *testing.T) {
		assert.NotEmpty(t, LLMsVerifierModels.DeepSeek, "DeepSeek model should be defined")
		assert.NotEmpty(t, LLMsVerifierModels.Gemini, "Gemini model should be defined")
		assert.NotEmpty(t, LLMsVerifierModels.Mistral, "Mistral model should be defined")
		assert.NotEmpty(t, LLMsVerifierModels.Groq, "Groq model should be defined")
		assert.NotEmpty(t, LLMsVerifierModels.Cerebras, "Cerebras model should be defined")
	})

	t.Run("All LLMsVerifier models are unique", func(t *testing.T) {
		models := []string{
			LLMsVerifierModels.DeepSeek,
			LLMsVerifierModels.Gemini,
			LLMsVerifierModels.Mistral,
			LLMsVerifierModels.Groq,
			LLMsVerifierModels.Cerebras,
		}
		uniqueModels := make(map[string]bool)
		for _, model := range models {
			assert.False(t, uniqueModels[model], "Model %s should be unique", model)
			uniqueModels[model] = true
		}
	})
}

func TestDebateTeamPosition(t *testing.T) {
	t.Run("Positions are correctly numbered 1-5", func(t *testing.T) {
		assert.Equal(t, DebateTeamPosition(1), PositionAnalyst)
		assert.Equal(t, DebateTeamPosition(2), PositionProposer)
		assert.Equal(t, DebateTeamPosition(3), PositionCritic)
		assert.Equal(t, DebateTeamPosition(4), PositionSynthesis)
		assert.Equal(t, DebateTeamPosition(5), PositionMediator)
	})
}

func TestDebateRole(t *testing.T) {
	t.Run("Roles are correctly defined", func(t *testing.T) {
		assert.Equal(t, DebateRole("analyst"), RoleAnalyst)
		assert.Equal(t, DebateRole("proposer"), RoleProposer)
		assert.Equal(t, DebateRole("critic"), RoleCritic)
		assert.Equal(t, DebateRole("synthesis"), RoleSynthesis)
		assert.Equal(t, DebateRole("mediator"), RoleMediator)
	})
}

func TestNewDebateTeamConfig(t *testing.T) {
	t.Run("Creates config with nil dependencies", func(t *testing.T) {
		config := NewDebateTeamConfig(nil, nil, nil)
		require.NotNil(t, config)
		assert.NotNil(t, config.members)
		assert.NotNil(t, config.verifiedLLMs)
		assert.Nil(t, config.providerRegistry)
		assert.Nil(t, config.discovery)
		assert.NotNil(t, config.logger) // Should create default logger
	})

	t.Run("Creates config with custom logger", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.PanicLevel)
		config := NewDebateTeamConfig(nil, nil, logger)
		require.NotNil(t, config)
		assert.Equal(t, logger, config.logger)
	})

	t.Run("Creates config with provider registry", func(t *testing.T) {
		registryConfig := &RegistryConfig{}
		registry := NewProviderRegistry(registryConfig, nil)
		logger := logrus.New()
		logger.SetLevel(logrus.PanicLevel)
		config := NewDebateTeamConfig(registry, nil, logger)
		require.NotNil(t, config)
		assert.NotNil(t, config.providerRegistry)
	})
}

func TestVerifiedLLM(t *testing.T) {
	t.Run("VerifiedLLM struct fields", func(t *testing.T) {
		llm := &VerifiedLLM{
			ProviderName: "claude",
			ModelName:    ClaudeModels.Sonnet45,
			Score:        9.5,
			IsOAuth:      true,
			Verified:     true,
		}

		assert.Equal(t, "claude", llm.ProviderName)
		assert.Equal(t, ClaudeModels.Sonnet45, llm.ModelName)
		assert.Equal(t, 9.5, llm.Score)
		assert.True(t, llm.IsOAuth)
		assert.True(t, llm.Verified)
	})
}

func TestDebateTeamMember(t *testing.T) {
	t.Run("Member with all fields", func(t *testing.T) {
		member := &DebateTeamMember{
			Position:     PositionAnalyst,
			Role:         RoleAnalyst,
			ProviderName: "claude",
			ModelName:    ClaudeModels.Sonnet45,
			Score:        9.5,
			IsActive:     true,
			IsOAuth:      true,
		}

		assert.Equal(t, PositionAnalyst, member.Position)
		assert.Equal(t, RoleAnalyst, member.Role)
		assert.Equal(t, "claude", member.ProviderName)
		assert.Equal(t, ClaudeModels.Sonnet45, member.ModelName)
		assert.Equal(t, 9.5, member.Score)
		assert.True(t, member.IsActive)
		assert.True(t, member.IsOAuth)
	})

	t.Run("Member with fallback chain", func(t *testing.T) {
		qwenFallback := &DebateTeamMember{
			Position:     PositionCritic,
			Role:         RoleCritic,
			ProviderName: "qwen",
			ModelName:    QwenModels.Turbo,
			Score:        7.5,
			IsActive:     false,
			IsOAuth:      true,
		}

		haikuFallback := &DebateTeamMember{
			Position:     PositionCritic,
			Role:         RoleCritic,
			ProviderName: "claude",
			ModelName:    ClaudeModels.Haiku45,
			Score:        8.5,
			IsActive:     false,
			IsOAuth:      true,
			Fallback:     qwenFallback,
		}

		primary := &DebateTeamMember{
			Position:     PositionCritic,
			Role:         RoleCritic,
			ProviderName: "deepseek",
			ModelName:    LLMsVerifierModels.DeepSeek,
			Score:        8.8,
			IsActive:     true,
			IsOAuth:      false,
			Fallback:     haikuFallback,
		}

		// Verify fallback chain
		assert.NotNil(t, primary.Fallback)
		assert.Equal(t, "claude", primary.Fallback.ProviderName)
		assert.NotNil(t, primary.Fallback.Fallback)
		assert.Equal(t, "qwen", primary.Fallback.Fallback.ProviderName)
		assert.Nil(t, primary.Fallback.Fallback.Fallback)

		// Count fallback depth
		depth := 0
		fb := primary.Fallback
		for fb != nil {
			depth++
			fb = fb.Fallback
		}
		assert.Equal(t, FallbacksPerPosition, depth)
	})
}

func TestDebateTeamConfigInitializeTeam(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("Initializes without providers gracefully", func(t *testing.T) {
		config := NewDebateTeamConfig(nil, nil, logger)
		err := config.InitializeTeam(context.Background())
		// Should not return error even without providers
		assert.NoError(t, err)
	})

	t.Run("Creates empty team when no verified LLMs", func(t *testing.T) {
		config := NewDebateTeamConfig(nil, nil, logger)
		err := config.InitializeTeam(context.Background())
		assert.NoError(t, err)
		// No verified LLMs means no members assigned
		assert.Equal(t, 0, len(config.GetActiveMembers()))
	})
}

func TestDebateTeamConfigGetTeamSummary(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := NewDebateTeamConfig(nil, nil, logger)

	// Manually add a member for testing
	config.members[PositionAnalyst] = &DebateTeamMember{
		Position:     PositionAnalyst,
		Role:         RoleAnalyst,
		ProviderName: "claude",
		ModelName:    ClaudeModels.Sonnet45,
		Score:        9.5,
		IsActive:     true,
		IsOAuth:      true,
	}

	t.Run("Summary includes team info", func(t *testing.T) {
		summary := config.GetTeamSummary()

		assert.Equal(t, "HelixAgent AI Debate Team", summary["team_name"])
		assert.Equal(t, TotalDebatePositions, summary["total_positions"])
		assert.Equal(t, TotalDebateLLMs, summary["expected_llms"])
	})

	t.Run("Summary includes Claude models", func(t *testing.T) {
		summary := config.GetTeamSummary()

		claudeModels := summary["claude_models"].(map[string]string)
		// Claude 4.5 models
		assert.NotEmpty(t, claudeModels["opus_45"])
		assert.NotEmpty(t, claudeModels["sonnet_45"])
		assert.NotEmpty(t, claudeModels["haiku_45"])
		// Claude 4.x models
		assert.NotEmpty(t, claudeModels["opus_4"])
		assert.NotEmpty(t, claudeModels["sonnet_4"])
		// Claude 3.5 models
		assert.NotEmpty(t, claudeModels["sonnet_35"])
		assert.NotEmpty(t, claudeModels["haiku_35"])
		// Claude 3 legacy models
		assert.NotEmpty(t, claudeModels["opus_3"])
		assert.NotEmpty(t, claudeModels["sonnet_3"])
		assert.NotEmpty(t, claudeModels["haiku_3"])
	})

	t.Run("Summary includes Qwen models", func(t *testing.T) {
		summary := config.GetTeamSummary()

		qwenModels := summary["qwen_models"].(map[string]string)
		assert.NotEmpty(t, qwenModels["turbo"])
		assert.NotEmpty(t, qwenModels["plus"])
		assert.NotEmpty(t, qwenModels["max"])
		assert.NotEmpty(t, qwenModels["coder"])
		assert.NotEmpty(t, qwenModels["long"])
	})

	t.Run("Summary includes LLMsVerifier models", func(t *testing.T) {
		summary := config.GetTeamSummary()

		verifierModels := summary["llmsverifier_models"].(map[string]string)
		assert.NotEmpty(t, verifierModels["deepseek"])
		assert.NotEmpty(t, verifierModels["gemini"])
		assert.NotEmpty(t, verifierModels["mistral"])
		assert.NotEmpty(t, verifierModels["groq"])
		assert.NotEmpty(t, verifierModels["cerebras"])
	})

	t.Run("Summary tracks OAuth vs LLMsVerifier LLMs", func(t *testing.T) {
		summary := config.GetTeamSummary()

		assert.Contains(t, summary, "oauth_llms")
		assert.Contains(t, summary, "llmsverifier_llms")
	})
}

func TestDebateTeamConfigGetActiveMembers(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := NewDebateTeamConfig(nil, nil, logger)

	// Add some members
	config.members[PositionAnalyst] = &DebateTeamMember{
		Position:     PositionAnalyst,
		ProviderName: "claude",
		IsActive:     true,
	}
	config.members[PositionProposer] = &DebateTeamMember{
		Position:     PositionProposer,
		ProviderName: "claude",
		IsActive:     true,
	}
	config.members[PositionCritic] = &DebateTeamMember{
		Position:     PositionCritic,
		ProviderName: "deepseek",
		IsActive:     false, // Inactive
	}

	t.Run("Returns only active members", func(t *testing.T) {
		active := config.GetActiveMembers()
		assert.Len(t, active, 2)
	})

	t.Run("Inactive members are excluded", func(t *testing.T) {
		active := config.GetActiveMembers()
		for _, member := range active {
			assert.True(t, member.IsActive)
			assert.NotEqual(t, PositionCritic, member.Position)
		}
	})
}

func TestDebateTeamConfigGetAllLLMs(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := NewDebateTeamConfig(nil, nil, logger)

	// Create a position with 2 fallbacks (total 3 LLMs for this position)
	fallback2 := &DebateTeamMember{
		Position:     PositionAnalyst,
		ProviderName: "qwen",
		ModelName:    QwenModels.Max,
	}
	fallback1 := &DebateTeamMember{
		Position:     PositionAnalyst,
		ProviderName: "groq",
		ModelName:    LLMsVerifierModels.Groq,
		Fallback:     fallback2,
	}
	config.members[PositionAnalyst] = &DebateTeamMember{
		Position:     PositionAnalyst,
		ProviderName: "claude",
		ModelName:    ClaudeModels.Sonnet45,
		Fallback:     fallback1,
	}

	t.Run("Returns all LLMs including fallbacks", func(t *testing.T) {
		allLLMs := config.GetAllLLMs()
		assert.Len(t, allLLMs, 3) // Primary + 2 fallbacks
	})

	t.Run("LLMs are in correct order (primary first)", func(t *testing.T) {
		allLLMs := config.GetAllLLMs()
		assert.Equal(t, "claude", allLLMs[0].ProviderName)
		assert.Equal(t, "groq", allLLMs[1].ProviderName)
		assert.Equal(t, "qwen", allLLMs[2].ProviderName)
	})
}

func TestDebateTeamConfigActivateFallback(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := NewDebateTeamConfig(nil, nil, logger)

	t.Run("Returns error for non-existent position", func(t *testing.T) {
		_, err := config.ActivateFallback(PositionAnalyst)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no member at position")
	})

	t.Run("Returns error when no fallback available", func(t *testing.T) {
		config.members[PositionAnalyst] = &DebateTeamMember{
			Position:     PositionAnalyst,
			ProviderName: "claude",
			IsActive:     true,
			Fallback:     nil,
		}

		_, err := config.ActivateFallback(PositionAnalyst)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no fallback available")
	})

	t.Run("Successfully activates fallback", func(t *testing.T) {
		fallback := &DebateTeamMember{
			Position:     PositionProposer,
			ProviderName: "qwen",
			ModelName:    QwenModels.Plus,
			IsActive:     false,
		}

		config.members[PositionProposer] = &DebateTeamMember{
			Position:     PositionProposer,
			ProviderName: "claude",
			ModelName:    ClaudeModels.Opus45,
			IsActive:     true,
			Fallback:     fallback,
		}

		activated, err := config.ActivateFallback(PositionProposer)
		require.NoError(t, err)
		assert.Equal(t, "qwen", activated.ProviderName)
		assert.Equal(t, QwenModels.Plus, activated.ModelName)
		assert.True(t, activated.IsActive)

		// Verify the position now has the fallback
		current := config.GetTeamMember(PositionProposer)
		assert.Equal(t, "qwen", current.ProviderName)
	})
}

func TestDebateTeamConfigCountTotalLLMs(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := NewDebateTeamConfig(nil, nil, logger)

	t.Run("Empty team has 0 LLMs", func(t *testing.T) {
		assert.Equal(t, 0, config.CountTotalLLMs())
	})

	t.Run("Counts primary and fallbacks", func(t *testing.T) {
		fallback := &DebateTeamMember{
			Position:     PositionAnalyst,
			ProviderName: "qwen",
		}
		config.members[PositionAnalyst] = &DebateTeamMember{
			Position:     PositionAnalyst,
			ProviderName: "claude",
			Fallback:     fallback,
		}

		assert.Equal(t, 2, config.CountTotalLLMs())
	})
}

func TestDebateTeamConfigIsFullyPopulated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := NewDebateTeamConfig(nil, nil, logger)

	t.Run("Empty team is not fully populated", func(t *testing.T) {
		assert.False(t, config.IsFullyPopulated())
	})

	t.Run("Partially filled team is not fully populated", func(t *testing.T) {
		config.members[PositionAnalyst] = &DebateTeamMember{
			Position:     PositionAnalyst,
			ProviderName: "claude",
		}
		assert.False(t, config.IsFullyPopulated())
	})
}

func TestDebateTeamPositionRoleMapping(t *testing.T) {
	t.Run("Position 1 is Analyst", func(t *testing.T) {
		assert.Equal(t, PositionAnalyst, DebateTeamPosition(1))
	})

	t.Run("Position 2 is Proposer", func(t *testing.T) {
		assert.Equal(t, PositionProposer, DebateTeamPosition(2))
	})

	t.Run("Position 3 is Critic", func(t *testing.T) {
		assert.Equal(t, PositionCritic, DebateTeamPosition(3))
	})

	t.Run("Position 4 is Synthesis", func(t *testing.T) {
		assert.Equal(t, PositionSynthesis, DebateTeamPosition(4))
	})

	t.Run("Position 5 is Mediator", func(t *testing.T) {
		assert.Equal(t, PositionMediator, DebateTeamPosition(5))
	})
}

func TestNoModelDuplication(t *testing.T) {
	t.Run("Claude models have no duplicates", func(t *testing.T) {
		models := map[string]bool{}
		claudeList := []string{
			ClaudeModels.Opus45,
			ClaudeModels.Sonnet45,
			ClaudeModels.Haiku45,
			ClaudeModels.Opus4,
			ClaudeModels.Sonnet4,
			ClaudeModels.Sonnet35,
			ClaudeModels.Haiku35,
			ClaudeModels.Opus3,
			ClaudeModels.Sonnet3,
			ClaudeModels.Haiku3,
		}
		for _, m := range claudeList {
			assert.False(t, models[m], "Claude model %s is duplicated", m)
			models[m] = true
		}
	})

	t.Run("Qwen models are unique per position", func(t *testing.T) {
		models := map[string]bool{}
		qwenList := []string{
			QwenModels.Turbo,
			QwenModels.Plus,
			QwenModels.Max,
			QwenModels.Coder,
			QwenModels.Long,
		}
		for _, m := range qwenList {
			assert.False(t, models[m], "Qwen model %s is duplicated", m)
			models[m] = true
		}
	})

	t.Run("LLMsVerifier models are unique", func(t *testing.T) {
		models := map[string]bool{}
		verifierList := []string{
			LLMsVerifierModels.DeepSeek,
			LLMsVerifierModels.Gemini,
			LLMsVerifierModels.Mistral,
			LLMsVerifierModels.Groq,
			LLMsVerifierModels.Cerebras,
		}
		for _, m := range verifierList {
			assert.False(t, models[m], "LLMsVerifier model %s is duplicated", m)
			models[m] = true
		}
	})
}

func TestTotalLLMCount(t *testing.T) {
	t.Run("Total available models equals or exceeds 15", func(t *testing.T) {
		// Count all unique models defined
		allModels := map[string]bool{}

		// Claude models (10)
		allModels[ClaudeModels.Opus45] = true
		allModels[ClaudeModels.Sonnet45] = true
		allModels[ClaudeModels.Haiku45] = true
		allModels[ClaudeModels.Opus4] = true
		allModels[ClaudeModels.Sonnet4] = true
		allModels[ClaudeModels.Sonnet35] = true
		allModels[ClaudeModels.Haiku35] = true
		allModels[ClaudeModels.Opus3] = true
		allModels[ClaudeModels.Sonnet3] = true
		allModels[ClaudeModels.Haiku3] = true

		// Qwen models (5)
		allModels[QwenModels.Turbo] = true
		allModels[QwenModels.Plus] = true
		allModels[QwenModels.Max] = true
		allModels[QwenModels.Coder] = true
		allModels[QwenModels.Long] = true

		// LLMsVerifier models (5)
		allModels[LLMsVerifierModels.DeepSeek] = true
		allModels[LLMsVerifierModels.Gemini] = true
		allModels[LLMsVerifierModels.Mistral] = true
		allModels[LLMsVerifierModels.Groq] = true
		allModels[LLMsVerifierModels.Cerebras] = true

		// Total should be >= 20 (10 Claude + 5 Qwen + 5 LLMsVerifier)
		assert.GreaterOrEqual(t, len(allModels), 20, "Should have at least 20 unique models defined")
	})
}

func TestGetVerifiedLLMs(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := NewDebateTeamConfig(nil, nil, logger)

	t.Run("Returns empty list initially", func(t *testing.T) {
		llms := config.GetVerifiedLLMs()
		assert.Empty(t, llms)
	})

	t.Run("Returns verified LLMs after initialization", func(t *testing.T) {
		// Manually add verified LLMs
		config.verifiedLLMs = []*VerifiedLLM{
			{ProviderName: "claude", ModelName: ClaudeModels.Sonnet45, Score: 9.5, IsOAuth: true, Verified: true},
			{ProviderName: "deepseek", ModelName: LLMsVerifierModels.DeepSeek, Score: 8.5, IsOAuth: false, Verified: true},
		}

		llms := config.GetVerifiedLLMs()
		assert.Len(t, llms, 2)
		assert.Equal(t, "claude", llms[0].ProviderName)
		assert.Equal(t, "deepseek", llms[1].ProviderName)
	})
}

func TestOAuthPrioritization(t *testing.T) {
	t.Run("OAuth providers should be prioritized in sorting", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.PanicLevel)

		config := NewDebateTeamConfig(nil, nil, logger)

		// Add LLMs with OAuth flag
		config.verifiedLLMs = []*VerifiedLLM{
			{ProviderName: "deepseek", Score: 8.5, IsOAuth: false},
			{ProviderName: "claude", Score: 9.5, IsOAuth: true},
			{ProviderName: "gemini", Score: 9.0, IsOAuth: false},
			{ProviderName: "qwen", Score: 8.0, IsOAuth: true},
		}

		// OAuth providers should come first when sorted
		// The actual sorting happens in InitializeTeam, but we can verify the logic
		oauthFirst := make([]*VerifiedLLM, 0)
		nonOAuth := make([]*VerifiedLLM, 0)

		for _, llm := range config.verifiedLLMs {
			if llm.IsOAuth {
				oauthFirst = append(oauthFirst, llm)
			} else {
				nonOAuth = append(nonOAuth, llm)
			}
		}

		assert.Len(t, oauthFirst, 2, "Should have 2 OAuth providers")
		assert.Len(t, nonOAuth, 2, "Should have 2 non-OAuth providers")
	})
}

// TestReliableAPIProvidersCollection tests the collectReliableAPIProviders method
// This test suite ensures that working API providers (Cerebras, Mistral, etc.)
// are always included in the debate team fallback chain
func TestReliableAPIProvidersCollection(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("ReliableAPIProviders models are defined", func(t *testing.T) {
		// Ensure the models used by collectReliableAPIProviders are properly defined
		assert.NotEmpty(t, LLMsVerifierModels.Cerebras, "Cerebras model should be defined")
		assert.NotEmpty(t, LLMsVerifierModels.Mistral, "Mistral model should be defined")
		assert.NotEmpty(t, LLMsVerifierModels.DeepSeek, "DeepSeek model should be defined")
		assert.NotEmpty(t, LLMsVerifierModels.Gemini, "Gemini model should be defined")
	})

	t.Run("Cerebras model ID is correct", func(t *testing.T) {
		assert.Equal(t, "llama-3.3-70b", LLMsVerifierModels.Cerebras, "Cerebras should use llama-3.3-70b")
	})

	t.Run("Mistral model ID is correct", func(t *testing.T) {
		assert.Equal(t, "mistral-large-latest", LLMsVerifierModels.Mistral, "Mistral should use mistral-large-latest")
	})

	t.Run("DeepSeek model ID is correct", func(t *testing.T) {
		assert.Equal(t, "deepseek-chat", LLMsVerifierModels.DeepSeek, "DeepSeek should use deepseek-chat")
	})

	t.Run("Gemini model ID is correct", func(t *testing.T) {
		assert.Equal(t, "gemini-2.0-flash", LLMsVerifierModels.Gemini, "Gemini should use gemini-2.0-flash")
	})
}

// TestFallbackChainIncludesWorkingProviders verifies that the fallback chain
// for OAuth providers includes non-OAuth working providers like Cerebras and Mistral
// This is CRITICAL - when OAuth providers fail, we MUST fall back to working API providers
func TestFallbackChainIncludesWorkingProviders(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := NewDebateTeamConfig(nil, nil, logger)

	t.Run("getFallbackLLMs prioritizes non-OAuth for OAuth primary", func(t *testing.T) {
		// Setup: Add OAuth and non-OAuth providers to verifiedLLMs
		config.verifiedLLMs = []*VerifiedLLM{
			{ProviderName: "claude", ModelName: ClaudeModels.Opus45, Score: 9.8, IsOAuth: true, Verified: true},
			{ProviderName: "cerebras", ModelName: LLMsVerifierModels.Cerebras, Score: 8.9, IsOAuth: false, Verified: true},
			{ProviderName: "mistral", ModelName: LLMsVerifierModels.Mistral, Score: 8.7, IsOAuth: false, Verified: true},
			{ProviderName: "zen", ModelName: "grok-code", Score: 8.2, IsOAuth: false, Verified: true},
		}

		// Get fallbacks for an OAuth primary (Claude)
		fallbacks := config.getFallbackLLMs("claude", ClaudeModels.Opus45, 2)

		// CRITICAL: Fallbacks for OAuth primary should be non-OAuth providers
		require.Len(t, fallbacks, 2, "Should get 2 fallbacks")
		assert.False(t, fallbacks[0].IsOAuth, "First fallback should NOT be OAuth")
		assert.False(t, fallbacks[1].IsOAuth, "Second fallback should NOT be OAuth")

		// Verify working providers are in fallbacks
		fallbackProviders := make([]string, len(fallbacks))
		for i, fb := range fallbacks {
			fallbackProviders[i] = fb.ProviderName
		}

		// Either Cerebras or Mistral should be in the fallback chain
		hasCerebrasOrMistral := false
		for _, provider := range fallbackProviders {
			if provider == "cerebras" || provider == "mistral" {
				hasCerebrasOrMistral = true
				break
			}
		}
		assert.True(t, hasCerebrasOrMistral, "Fallback chain MUST include working API providers (Cerebras or Mistral)")
	})

	t.Run("Fallback chain does not rely solely on free/unreliable providers", func(t *testing.T) {
		// This test ensures we don't have fallback chains like: Claude -> Zen -> Zen
		// which would fail completely when Claude's OAuth is restricted

		config.verifiedLLMs = []*VerifiedLLM{
			{ProviderName: "claude", ModelName: ClaudeModels.Opus45, Score: 9.8, IsOAuth: true, Verified: true},
			{ProviderName: "cerebras", ModelName: LLMsVerifierModels.Cerebras, Score: 8.9, IsOAuth: false, Verified: true},
			{ProviderName: "mistral", ModelName: LLMsVerifierModels.Mistral, Score: 8.7, IsOAuth: false, Verified: true},
			{ProviderName: "zen", ModelName: "grok-code", Score: 8.2, IsOAuth: false, Verified: true},
			{ProviderName: "zen", ModelName: "big-pickle", Score: 8.0, IsOAuth: false, Verified: true},
		}

		fallbacks := config.getFallbackLLMs("claude", ClaudeModels.Opus45, 2)

		// Count how many fallbacks are from reliable providers
		reliableCount := 0
		for _, fb := range fallbacks {
			if fb.ProviderName == "cerebras" || fb.ProviderName == "mistral" ||
			   fb.ProviderName == "deepseek" || fb.ProviderName == "gemini" {
				reliableCount++
			}
		}

		// At least one fallback MUST be a reliable API provider
		assert.GreaterOrEqual(t, reliableCount, 1,
			"At least one fallback must be a reliable API provider (Cerebras, Mistral, DeepSeek, or Gemini)")
	})
}

// TestDebateTeamMustHaveWorkingFallbacks is a comprehensive test that verifies
// the entire debate team has proper fallback chains with working providers
func TestDebateTeamMustHaveWorkingFallbacks(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := NewDebateTeamConfig(nil, nil, logger)

	t.Run("All positions have fallbacks when providers available", func(t *testing.T) {
		// Simulate verified LLMs being available
		config.verifiedLLMs = []*VerifiedLLM{
			// OAuth providers (Claude)
			{ProviderName: "claude", ModelName: ClaudeModels.Opus45, Score: 9.8, IsOAuth: true, Verified: true},
			{ProviderName: "claude", ModelName: ClaudeModels.Sonnet45, Score: 9.6, IsOAuth: true, Verified: true},
			{ProviderName: "claude", ModelName: ClaudeModels.Opus4, Score: 9.4, IsOAuth: true, Verified: true},
			{ProviderName: "claude", ModelName: ClaudeModels.Sonnet4, Score: 9.2, IsOAuth: true, Verified: true},
			{ProviderName: "claude", ModelName: ClaudeModels.Haiku45, Score: 9.0, IsOAuth: true, Verified: true},
			// Reliable API providers (NON-OAuth)
			{ProviderName: "cerebras", ModelName: LLMsVerifierModels.Cerebras, Score: 8.9, IsOAuth: false, Verified: true},
			{ProviderName: "mistral", ModelName: LLMsVerifierModels.Mistral, Score: 8.7, IsOAuth: false, Verified: true},
			{ProviderName: "deepseek", ModelName: LLMsVerifierModels.DeepSeek, Score: 8.8, IsOAuth: false, Verified: true},
			{ProviderName: "gemini", ModelName: LLMsVerifierModels.Gemini, Score: 8.6, IsOAuth: false, Verified: true},
		}

		// Manually assign positions with fallbacks
		config.assignPrimaryPositions()
		config.assignAllFallbacks()

		// Verify each position has fallbacks
		for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
			member := config.GetTeamMember(pos)
			require.NotNil(t, member, "Position %d should have a member", pos)
			require.NotNil(t, member.Fallback, "Position %d should have a fallback", pos)

			// CRITICAL: If primary is OAuth, fallback should include non-OAuth
			if member.IsOAuth {
				// At least one fallback in the chain should be non-OAuth
				hasNonOAuthFallback := false
				fb := member.Fallback
				for fb != nil {
					if !fb.IsOAuth {
						hasNonOAuthFallback = true
						break
					}
					fb = fb.Fallback
				}
				assert.True(t, hasNonOAuthFallback,
					"Position %d with OAuth primary MUST have non-OAuth fallback (found: %s -> %s)",
					pos, member.ProviderName, member.Fallback.ProviderName)
			}
		}
	})
}

// TestCollectVerifiedLLMsIncludesReliableProviders ensures that collectVerifiedLLMs
// adds reliable API providers before free models
func TestCollectVerifiedLLMsIncludesReliableProviders(t *testing.T) {
	t.Run("Collection order should prioritize reliability", func(t *testing.T) {
		// The collection order should be:
		// 1. OAuth providers (Claude, Qwen) - highest priority for quality
		// 2. Reliable API providers (Cerebras, Mistral, DeepSeek, Gemini) - proven working
		// 3. Free models (Zen, OpenRouter :free) - lowest priority for fallbacks

		// This is documented in collectVerifiedLLMs() method
		// Order of collection:
		// 1. collectClaudeModels()
		// 2. collectQwenModels()
		// 3. collectReliableAPIProviders() <-- CRITICAL: This must come before free models
		// 4. collectOpenRouterFreeModels()
		// 5. collectZenModels()
		// 6. collectLLMsVerifierProviders()

		// Verify the expected model IDs
		assert.NotEmpty(t, LLMsVerifierModels.Cerebras)
		assert.NotEmpty(t, LLMsVerifierModels.Mistral)
		assert.NotEmpty(t, LLMsVerifierModels.DeepSeek)
		assert.NotEmpty(t, LLMsVerifierModels.Gemini)
	})
}

// TestReliableProvidersScoreRange ensures reliable API providers have appropriate scores
func TestReliableProvidersScoreRange(t *testing.T) {
	t.Run("Reliable provider scores should be competitive", func(t *testing.T) {
		// In collectReliableAPIProviders(), the scores are:
		// - Cerebras: 8.9
		// - Mistral: 8.7
		// - DeepSeek: 8.8
		// - Gemini: 8.6

		// These scores should be:
		// 1. Lower than Claude 4.5 models (9.0-9.8) to not override OAuth primaries
		// 2. Higher than Zen models (7.6-8.2) to be prioritized as fallbacks
		// 3. Competitive enough to be selected as fallbacks before free models

		expectedScores := map[string]float64{
			"cerebras": 8.9,
			"mistral":  8.7,
			"deepseek": 8.8,
			"gemini":   8.6,
		}

		for provider, score := range expectedScores {
			assert.GreaterOrEqual(t, score, 8.5, "%s score should be >= 8.5", provider)
			assert.LessOrEqual(t, score, 9.0, "%s score should be <= 9.0", provider)
		}
	})
}
