package challenge

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"dev.helix.agent/internal/services"
)

// TestSingleProviderMultiInstanceDebate tests the core single-provider multi-instance functionality
func TestSingleProviderMultiInstanceDebate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping single-provider debate test in short mode")
	}

	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	discovery := services.NewProviderDiscovery(logger, true)

	// Discover providers
	discovered, err := discovery.DiscoverProviders()
	if err != nil {
		t.Skipf("Provider discovery failed: %v", err)
	}

	t.Logf("Discovered %d providers", len(discovered))

	// Verify providers
	ctx := context.Background()
	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	results := discovery.VerifyAllProviders(verifyCtx)

	// Count healthy providers
	var healthyProviders []string
	for name, p := range results {
		if p.Verified && p.Status == services.ProviderStatusHealthy {
			healthyProviders = append(healthyProviders, name)
			t.Logf("Healthy provider: %s (score: %.2f)", name, p.Score)
		}
	}

	if len(healthyProviders) == 0 {
		t.Skip("No healthy providers available for testing")
	}

	// Register one provider to simulate single-provider scenario
	singleProvider := healthyProviders[0]
	p := results[singleProvider]
	require.NoError(t, registry.RegisterProvider(singleProvider, p.Provider))

	// Create debate service
	debateService := services.NewDebateServiceWithDeps(logger, registry, nil)

	t.Run("DetectSingleProviderMode", func(t *testing.T) {
		config := &services.DebateConfig{
			DebateID:  "test-detect-mode",
			Topic:     "Test topic",
			MaxRounds: 1,
			Timeout:   5 * time.Minute,
			Participants: []services.ParticipantConfig{
				{ParticipantID: "p1", Name: "P1", Role: "debater", LLMProvider: singleProvider},
				{ParticipantID: "p2", Name: "P2", Role: "critic", LLMProvider: singleProvider},
				{ParticipantID: "p3", Name: "P3", Role: "mediator", LLMProvider: singleProvider},
			},
		}

		isSingle, spc := debateService.IsSingleProviderMode(config)
		assert.True(t, isSingle, "Should detect single-provider mode")
		assert.NotNil(t, spc, "Should return single-provider config")
		assert.Equal(t, singleProvider, spc.ProviderName)
		assert.Equal(t, 3, spc.NumParticipants)
	})

	t.Run("CreateSingleProviderParticipants", func(t *testing.T) {
		spc := &services.SingleProviderConfig{
			ProviderName:      singleProvider,
			AvailableModels:   []string{"model1", "model2"},
			NumParticipants:   5,
			UseModelDiversity: true,
			UseTempDiversity:  true,
		}

		participants := debateService.CreateSingleProviderParticipants(spc, "Test topic")

		assert.Len(t, participants, 5)

		// Verify diversity
		temps := make(map[float64]int)
		models := make(map[string]int)
		prompts := make(map[string]int)

		for _, p := range participants {
			temps[p.Temperature]++
			models[p.LLMModel]++
			prompts[p.SystemPrompt]++

			// Each participant should have a system prompt
			assert.NotEmpty(t, p.SystemPrompt, "Participant should have system prompt")
			assert.NotEmpty(t, p.Name, "Participant should have name")
			assert.NotEmpty(t, p.Role, "Participant should have role")
		}

		// Should have temperature diversity
		assert.Greater(t, len(temps), 1, "Should have temperature diversity")

		// Should use multiple models
		assert.Greater(t, len(models), 1, "Should use multiple models")

		// Each participant should have unique system prompt
		assert.Equal(t, 5, len(prompts), "Each participant should have unique system prompt")
	})

	t.Run("ConductSingleProviderDebate_ThreeParticipants", func(t *testing.T) {
		debateCtx, debateCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer debateCancel()

		config := &services.DebateConfig{
			DebateID:  "single-provider-3p",
			Topic:     "What are the benefits and drawbacks of remote work?",
			MaxRounds: 2,
			Timeout:   5 * time.Minute,
			Participants: []services.ParticipantConfig{
				{ParticipantID: "p1", Name: "P1", Role: "proposer", LLMProvider: singleProvider},
				{ParticipantID: "p2", Name: "P2", Role: "opponent", LLMProvider: singleProvider},
				{ParticipantID: "p3", Name: "P3", Role: "mediator", LLMProvider: singleProvider},
			},
		}

		spc := &services.SingleProviderConfig{
			ProviderName:      singleProvider,
			AvailableModels:   debateService.GetAvailableModelsForProvider(singleProvider),
			NumParticipants:   3,
			UseModelDiversity: true,
			UseTempDiversity:  true,
		}

		result, err := debateService.ConductSingleProviderDebate(debateCtx, config, spc)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.True(t, result.Success)
		assert.Greater(t, len(result.AllResponses), 0)

		// Verify metadata
		assert.Equal(t, "single_provider", result.Metadata["mode"])
		assert.Equal(t, singleProvider, result.Metadata["provider"])

		// Check effective diversity
		if diversity, ok := result.Metadata["effective_diversity"].(float64); ok {
			t.Logf("Effective diversity: %.2f", diversity)
			// In single-provider mode, we want at least some diversity
			assert.Greater(t, diversity, 0.0, "Should have some response diversity")
		}

		t.Logf("Debate completed: %d responses, quality score: %.2f",
			len(result.AllResponses), result.QualityScore)
	})

	t.Run("ConductSingleProviderDebate_FiveParticipants", func(t *testing.T) {
		debateCtx, debateCancel := context.WithTimeout(ctx, 8*time.Minute)
		defer debateCancel()

		config := &services.DebateConfig{
			DebateID:  "single-provider-5p",
			Topic:     "Should artificial intelligence be regulated by governments?",
			MaxRounds: 2,
			Timeout:   8 * time.Minute,
			Participants: []services.ParticipantConfig{
				{ParticipantID: "p1", Name: "P1", Role: "analyst", LLMProvider: singleProvider},
				{ParticipantID: "p2", Name: "P2", Role: "proposer", LLMProvider: singleProvider},
				{ParticipantID: "p3", Name: "P3", Role: "critic", LLMProvider: singleProvider},
				{ParticipantID: "p4", Name: "P4", Role: "mediator", LLMProvider: singleProvider},
				{ParticipantID: "p5", Name: "P5", Role: "debater", LLMProvider: singleProvider},
			},
		}

		spc := &services.SingleProviderConfig{
			ProviderName:      singleProvider,
			AvailableModels:   debateService.GetAvailableModelsForProvider(singleProvider),
			NumParticipants:   5,
			UseModelDiversity: true,
			UseTempDiversity:  true,
		}

		result, err := debateService.ConductSingleProviderDebate(debateCtx, config, spc)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Skip if no results (external service issues)
		if len(result.AllResponses) == 0 {
			t.Skip("No responses from debate - external service may be unavailable")
		}
		assert.True(t, result.Success)
		assert.Equal(t, 5, result.Metadata["instance_count"])
		assert.GreaterOrEqual(t, len(result.AllResponses), 5, "Should have at least 5 responses (1 round)")

		t.Logf("5-participant debate: %d responses, quality: %.2f, rounds: %d",
			len(result.AllResponses), result.QualityScore, result.RoundsConducted)
	})

	t.Run("AutoConductDebate_SelectsCorrectMode", func(t *testing.T) {
		debateCtx, debateCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer debateCancel()

		config := &services.DebateConfig{
			DebateID:  "auto-mode-test",
			Topic:     "Is cloud computing better than on-premise infrastructure?",
			MaxRounds: 1,
			Timeout:   5 * time.Minute,
			Participants: []services.ParticipantConfig{
				{ParticipantID: "p1", Name: "P1", Role: "analyst", LLMProvider: singleProvider},
				{ParticipantID: "p2", Name: "P2", Role: "critic", LLMProvider: singleProvider},
				{ParticipantID: "p3", Name: "P3", Role: "mediator", LLMProvider: singleProvider},
			},
		}

		result, err := debateService.AutoConductDebate(debateCtx, config)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should automatically select single-provider mode
		assert.Equal(t, "single_provider", result.Metadata["mode"])
		assert.True(t, result.Success)
	})
}

// TestSingleProviderMultiInstanceDiversity tests the diversity mechanisms
func TestSingleProviderMultiInstanceDiversity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping diversity test in short mode")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	discovery := services.NewProviderDiscovery(logger, true)

	// Quick discovery
	discovered, err := discovery.DiscoverProviders()
	if err != nil || len(discovered) == 0 {
		t.Skip("No providers discovered")
	}

	// Verify and register first healthy provider
	ctx := context.Background()
	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	results := discovery.VerifyAllProviders(verifyCtx)

	var provider *services.DiscoveredProvider
	for _, p := range results {
		if p.Verified && p.Status == services.ProviderStatusHealthy {
			provider = p
			break
		}
	}

	if provider == nil {
		t.Skip("No healthy providers available")
	}

	require.NoError(t, registry.RegisterProvider(provider.Name, provider.Provider))
	debateService := services.NewDebateServiceWithDeps(logger, registry, nil)

	t.Run("TemperatureDiversity", func(t *testing.T) {
		spc := &services.SingleProviderConfig{
			ProviderName:      provider.Name,
			AvailableModels:   []string{"default"},
			NumParticipants:   5,
			UseModelDiversity: false,
			UseTempDiversity:  true,
		}

		participants := debateService.CreateSingleProviderParticipants(spc, "Test")

		temps := make([]float64, len(participants))
		for i, p := range participants {
			temps[i] = p.Temperature
		}

		// Check for temperature variation
		minTemp, maxTemp := temps[0], temps[0]
		for _, t := range temps {
			if t < minTemp {
				minTemp = t
			}
			if t > maxTemp {
				maxTemp = t
			}
		}

		assert.Greater(t, maxTemp-minTemp, 0.1, "Should have temperature variation")
		t.Logf("Temperature range: %.2f - %.2f", minTemp, maxTemp)
	})

	t.Run("SystemPromptDiversity", func(t *testing.T) {
		spc := &services.SingleProviderConfig{
			ProviderName:      provider.Name,
			AvailableModels:   []string{"default"},
			NumParticipants:   5,
			UseModelDiversity: false,
			UseTempDiversity:  false,
		}

		participants := debateService.CreateSingleProviderParticipants(spc, "Test")

		// Each participant should have a unique system prompt
		prompts := make(map[string]bool)
		for _, p := range participants {
			assert.NotEmpty(t, p.SystemPrompt)
			assert.False(t, prompts[p.SystemPrompt], "System prompts should be unique")
			prompts[p.SystemPrompt] = true

			// Verify prompt contains key elements
			assert.Contains(t, p.SystemPrompt, "perspective", "System prompt should mention perspective")
		}
	})

	t.Run("RoleDiversity", func(t *testing.T) {
		spc := &services.SingleProviderConfig{
			ProviderName:      provider.Name,
			AvailableModels:   []string{"default"},
			NumParticipants:   5,
			UseModelDiversity: false,
			UseTempDiversity:  true,
		}

		participants := debateService.CreateSingleProviderParticipants(spc, "Test")

		roles := make(map[string]int)
		for _, p := range participants {
			roles[p.Role]++
		}

		// Should have multiple different roles
		assert.Greater(t, len(roles), 2, "Should have multiple different roles")
		t.Logf("Role distribution: %v", roles)
	})
}

// TestSingleProviderDebateQuality tests the quality of responses
func TestSingleProviderDebateQuality(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping quality test in short mode")
	}

	logger := logrus.New()
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	discovery := services.NewProviderDiscovery(logger, true)

	// Setup provider
	discovered, _ := discovery.DiscoverProviders()
	if len(discovered) == 0 {
		t.Skip("No providers discovered")
	}

	ctx := context.Background()
	results := discovery.VerifyAllProviders(ctx)

	var provider *services.DiscoveredProvider
	for _, p := range results {
		if p.Verified && p.Status == services.ProviderStatusHealthy {
			provider = p
			break
		}
	}

	if provider == nil {
		t.Skip("No healthy providers")
	}

	require.NoError(t, registry.RegisterProvider(provider.Name, provider.Provider))
	debateService := services.NewDebateServiceWithDeps(logger, registry, nil)

	t.Run("ResponseQualityScores", func(t *testing.T) {
		debateCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		config := &services.DebateConfig{
			DebateID:  "quality-test",
			Topic:     "What makes a programming language good for beginners?",
			MaxRounds: 2,
			Timeout:   5 * time.Minute,
			Participants: []services.ParticipantConfig{
				{ParticipantID: "p1", Name: "P1", Role: "proposer", LLMProvider: provider.Name},
				{ParticipantID: "p2", Name: "P2", Role: "critic", LLMProvider: provider.Name},
				{ParticipantID: "p3", Name: "P3", Role: "mediator", LLMProvider: provider.Name},
			},
		}

		spc := &services.SingleProviderConfig{
			ProviderName:      provider.Name,
			AvailableModels:   debateService.GetAvailableModelsForProvider(provider.Name),
			NumParticipants:   3,
			UseModelDiversity: true,
			UseTempDiversity:  true,
		}

		result, err := debateService.ConductSingleProviderDebate(debateCtx, config, spc)
		require.NoError(t, err)

		// Skip if no results (external service issues)
		if len(result.AllResponses) == 0 {
			t.Skip("No responses from debate - external service may be unavailable")
		}

		// Check quality scores
		assert.Greater(t, result.QualityScore, 0.0)
		assert.Greater(t, result.FinalScore, 0.0)

		// Check individual response quality
		for _, resp := range result.AllResponses {
			assert.Greater(t, resp.QualityScore, 0.0, "Each response should have quality score")
			assert.NotEmpty(t, resp.Content, "Each response should have content")
		}

		t.Logf("Quality score: %.2f, Final score: %.2f", result.QualityScore, result.FinalScore)
	})

	t.Run("ContentDiversityInResponses", func(t *testing.T) {
		debateCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		config := &services.DebateConfig{
			DebateID:  "diversity-quality-test",
			Topic:     "What are the pros and cons of microservices architecture?",
			MaxRounds: 1,
			Timeout:   5 * time.Minute,
			Participants: []services.ParticipantConfig{
				{ParticipantID: "p1", Name: "P1", Role: "analyst", LLMProvider: provider.Name},
				{ParticipantID: "p2", Name: "P2", Role: "critic", LLMProvider: provider.Name},
				{ParticipantID: "p3", Name: "P3", Role: "proposer", LLMProvider: provider.Name},
			},
		}

		spc := &services.SingleProviderConfig{
			ProviderName:      provider.Name,
			AvailableModels:   debateService.GetAvailableModelsForProvider(provider.Name),
			NumParticipants:   3,
			UseModelDiversity: true,
			UseTempDiversity:  true,
		}

		result, err := debateService.ConductSingleProviderDebate(debateCtx, config, spc)
		require.NoError(t, err)

		// Check effective diversity
		diversity := result.Metadata["effective_diversity"].(float64)
		t.Logf("Effective diversity: %.4f", diversity)

		// Responses should have some diversity (not identical)
		assert.Greater(t, diversity, 0.3, "Responses should have reasonable diversity")
	})
}

// TestSingleProviderDebateEdgeCases tests edge cases
func TestSingleProviderDebateEdgeCases(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	debateService := services.NewDebateServiceWithDeps(logger, registry, nil)

	t.Run("NoProvidersAvailable", func(t *testing.T) {
		config := &services.DebateConfig{
			DebateID:  "no-providers",
			Topic:     "Test",
			MaxRounds: 1,
			Timeout:   time.Minute,
			Participants: []services.ParticipantConfig{
				{ParticipantID: "p1", LLMProvider: "nonexistent"},
			},
		}

		isSingle, spc := debateService.IsSingleProviderMode(config)
		assert.False(t, isSingle, "Should not detect single-provider mode with no providers")
		assert.Nil(t, spc)
	})

	t.Run("SingleParticipant", func(t *testing.T) {
		spc := &services.SingleProviderConfig{
			ProviderName:      "test-provider",
			AvailableModels:   []string{"model1"},
			NumParticipants:   1,
			UseModelDiversity: false,
			UseTempDiversity:  true,
		}

		participants := debateService.CreateSingleProviderParticipants(spc, "Test")
		assert.Len(t, participants, 1)
		assert.NotEmpty(t, participants[0].SystemPrompt)
	})

	t.Run("ManyParticipants", func(t *testing.T) {
		spc := &services.SingleProviderConfig{
			ProviderName:      "test-provider",
			AvailableModels:   []string{"model1", "model2", "model3"},
			NumParticipants:   10,
			UseModelDiversity: true,
			UseTempDiversity:  true,
		}

		participants := debateService.CreateSingleProviderParticipants(spc, "Test")
		assert.Len(t, participants, 10)

		// Models should cycle through available models
		modelCounts := make(map[string]int)
		for _, p := range participants {
			modelCounts[p.LLMModel]++
		}

		// Should use all 3 models
		assert.Equal(t, 3, len(modelCounts), "Should use all available models")
	})

	t.Run("EmptyModelList", func(t *testing.T) {
		spc := &services.SingleProviderConfig{
			ProviderName:      "test-provider",
			AvailableModels:   []string{},
			NumParticipants:   3,
			UseModelDiversity: true,
			UseTempDiversity:  true,
		}

		participants := debateService.CreateSingleProviderParticipants(spc, "Test")
		assert.Len(t, participants, 3)

		// Should fall back to "default" model
		for _, p := range participants {
			assert.Equal(t, "default", p.LLMModel)
		}
	})
}

// TestSingleProviderDebateReport generates a detailed test report
func TestSingleProviderDebateReport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping report test in short mode")
	}

	logger := logrus.New()
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	discovery := services.NewProviderDiscovery(logger, true)

	// Setup
	discovered, _ := discovery.DiscoverProviders()
	if len(discovered) == 0 {
		t.Skip("No providers discovered")
	}

	ctx := context.Background()
	results := discovery.VerifyAllProviders(ctx)

	var provider *services.DiscoveredProvider
	for _, p := range results {
		if p.Verified && p.Status == services.ProviderStatusHealthy {
			provider = p
			break
		}
	}

	if provider == nil {
		t.Skip("No healthy providers")
	}

	require.NoError(t, registry.RegisterProvider(provider.Name, provider.Provider))
	debateService := services.NewDebateServiceWithDeps(logger, registry, nil)

	t.Run("GenerateComprehensiveReport", func(t *testing.T) {
		debateCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		config := &services.DebateConfig{
			DebateID:  "comprehensive-report-test",
			Topic:     "What are the best practices for building scalable microservices?",
			MaxRounds: 3,
			Timeout:   10 * time.Minute,
			Participants: []services.ParticipantConfig{
				{ParticipantID: "p1", Name: "P1", Role: "analyst", LLMProvider: provider.Name},
				{ParticipantID: "p2", Name: "P2", Role: "proposer", LLMProvider: provider.Name},
				{ParticipantID: "p3", Name: "P3", Role: "critic", LLMProvider: provider.Name},
				{ParticipantID: "p4", Name: "P4", Role: "mediator", LLMProvider: provider.Name},
				{ParticipantID: "p5", Name: "P5", Role: "debater", LLMProvider: provider.Name},
			},
		}

		spc := &services.SingleProviderConfig{
			ProviderName:      provider.Name,
			AvailableModels:   debateService.GetAvailableModelsForProvider(provider.Name),
			NumParticipants:   5,
			UseModelDiversity: true,
			UseTempDiversity:  true,
		}

		result, err := debateService.ConductSingleProviderDebate(debateCtx, config, spc)
		require.NoError(t, err)

		// Generate report
		report := map[string]interface{}{
			"debate_id":           result.DebateID,
			"session_id":          result.SessionID,
			"topic":               result.Topic,
			"mode":                "single_provider",
			"provider":            provider.Name,
			"provider_score":      provider.Score,
			"rounds_conducted":    result.RoundsConducted,
			"total_responses":     len(result.AllResponses),
			"quality_score":       result.QualityScore,
			"final_score":         result.FinalScore,
			"duration_seconds":    result.Duration.Seconds(),
			"effective_diversity": result.Metadata["effective_diversity"],
			"models_used":         result.Metadata["models_used"],
			"consensus_reached":   result.Consensus.Reached,
			"agreement_level":     result.Consensus.AgreementLevel,
			"participants":        make([]map[string]interface{}, 0),
		}

		// Add participant details
		for _, p := range result.Participants {
			pReport := map[string]interface{}{
				"name":          p.ParticipantName,
				"role":          p.Role,
				"model":         p.LLMModel,
				"quality_score": p.QualityScore,
				"response_time": p.ResponseTime.Seconds(),
				"content_len":   len(p.Content),
			}
			report["participants"] = append(report["participants"].([]map[string]interface{}), pReport)
		}

		// Output report
		reportJSON, _ := json.MarshalIndent(report, "", "  ")
		t.Logf("Comprehensive Report:\n%s", string(reportJSON))

		// Write report to file
		reportPath := "/tmp/single_provider_debate_report.json"
		err = os.WriteFile(reportPath, reportJSON, 0644)
		if err == nil {
			t.Logf("Report written to: %s", reportPath)
		}

		// Assertions - skip if no results (external service issues)
		if len(result.AllResponses) == 0 {
			t.Skip("No responses from debate - external service may be unavailable")
		}
		assert.True(t, result.Success)
		assert.Greater(t, result.QualityScore, 0.5, "Quality should be above 0.5")
		assert.GreaterOrEqual(t, len(result.AllResponses), 5, "Should have at least 5 responses")
	})
}

