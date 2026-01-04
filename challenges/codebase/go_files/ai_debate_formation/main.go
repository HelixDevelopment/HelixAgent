// Package main implements the AI Debate Group Formation challenge.
// This challenge forms an optimal AI debate group from top-scoring models.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ModelScore holds scoring information for an LLM model.
type ModelScore struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Provider       string             `json:"provider"`
	Score          float64            `json:"score"`
	Capabilities   []string           `json:"capabilities,omitempty"`
	ScoreBreakdown map[string]float64 `json:"score_breakdown,omitempty"`
}

// DebateGroupMember represents a member of the AI debate group.
type DebateGroupMember struct {
	Position  int          `json:"position"`
	Role      string       `json:"role"` // "primary" or "fallback_N"
	Model     ModelScore   `json:"model"`
	Fallbacks []ModelScore `json:"fallbacks,omitempty"`
}

// DebateConfiguration holds debate-specific settings.
type DebateConfiguration struct {
	DebateRounds       int     `json:"debate_rounds"`
	ConsensusThreshold float64 `json:"consensus_threshold"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	FallbackStrategy   string  `json:"fallback_strategy"`
}

// DebateGroup represents the complete AI debate group configuration.
type DebateGroup struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	CreatedAt     time.Time           `json:"created_at"`
	Members       []DebateGroupMember `json:"members"`
	TotalModels   int                 `json:"total_models"`
	AverageScore  float64             `json:"average_score"`
	Configuration DebateConfiguration `json:"configuration"`
}

// FormationConfig holds configuration for debate group formation.
type FormationConfig struct {
	PrimaryCount        int               `json:"primary_count"`
	FallbacksPerPrimary int               `json:"fallbacks_per_primary"`
	MinimumScore        float64           `json:"minimum_score"`
	PreferDiversity     bool              `json:"prefer_diversity"`
	SelectionWeights    SelectionWeights  `json:"selection_weights"`
}

// SelectionWeights defines weights for model selection criteria.
type SelectionWeights struct {
	VerificationScore  float64 `json:"verification_score"`
	CapabilityCoverage float64 `json:"capability_coverage"`
	ResponseSpeed      float64 `json:"response_speed"`
	ProviderDiversity  float64 `json:"provider_diversity"`
}

// ChallengeResult holds the complete challenge output.
type ChallengeResult struct {
	ChallengeID   string            `json:"challenge_id"`
	ChallengeName string            `json:"challenge_name"`
	Timestamp     time.Time         `json:"timestamp"`
	Duration      time.Duration     `json:"duration"`
	Status        string            `json:"status"`
	DebateGroup   *DebateGroup      `json:"debate_group"`
	Assertions    []AssertionResult `json:"assertions"`
	Metrics       FormationMetrics  `json:"metrics"`
}

// AssertionResult holds the outcome of an assertion.
type AssertionResult struct {
	Type    string `json:"type"`
	Target  string `json:"target"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// FormationMetrics holds metrics about the formation process.
type FormationMetrics struct {
	ModelsConsidered    int      `json:"models_considered"`
	ModelsSelected      int      `json:"models_selected"`
	ProvidersUsed       int      `json:"providers_used"`
	AveragePrimaryScore float64  `json:"average_primary_score"`
	AverageFallbackScore float64 `json:"average_fallback_score"`
	CapabilityCoverage  float64  `json:"capability_coverage"`
	ProviderDiversity   float64  `json:"provider_diversity"`
}

// Default configuration
func defaultFormationConfig() FormationConfig {
	primaryCount := 5
	fallbacksPerPrimary := 2

	// Allow override from environment
	if val := os.Getenv("DEBATE_GROUP_SIZE"); val != "" {
		fmt.Sscanf(val, "%d", &primaryCount)
	}
	if val := os.Getenv("DEBATE_FALLBACKS_PER_MEMBER"); val != "" {
		fmt.Sscanf(val, "%d", &fallbacksPerPrimary)
	}

	return FormationConfig{
		PrimaryCount:        primaryCount,
		FallbacksPerPrimary: fallbacksPerPrimary,
		MinimumScore:        6.0,
		PreferDiversity:     true,
		SelectionWeights: SelectionWeights{
			VerificationScore:  0.4,
			CapabilityCoverage: 0.3,
			ResponseSpeed:      0.2,
			ProviderDiversity:  0.1,
		},
	}
}

// Load scored models from provider verification
func loadScoredModels(dependencyPath string) ([]ModelScore, error) {
	modelsFile := filepath.Join(dependencyPath, "results", "models_scored.json")

	data, err := os.ReadFile(modelsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read models file: %w", err)
	}

	var models []ModelScore
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("failed to parse models file: %w", err)
	}

	return models, nil
}

// FormDebateGroup creates an optimal debate group from scored models.
func FormDebateGroup(models []ModelScore, config FormationConfig) (*DebateGroup, FormationMetrics) {
	metrics := FormationMetrics{
		ModelsConsidered: len(models),
	}

	// Filter by minimum score
	var eligibleModels []ModelScore
	for _, m := range models {
		if m.Score >= config.MinimumScore {
			eligibleModels = append(eligibleModels, m)
		}
	}

	// Sort by score (descending)
	sort.Slice(eligibleModels, func(i, j int) bool {
		return eligibleModels[i].Score > eligibleModels[j].Score
	})

	// Calculate required models
	totalRequired := config.PrimaryCount * (1 + config.FallbacksPerPrimary)

	// Select models with diversity consideration
	var selectedModels []ModelScore
	usedProviders := make(map[string]int)

	if config.PreferDiversity {
		// First pass: select diverse primaries
		for len(selectedModels) < config.PrimaryCount && len(eligibleModels) > 0 {
			for i, m := range eligibleModels {
				if usedProviders[m.Provider] == 0 || len(selectedModels) >= config.PrimaryCount/2 {
					selectedModels = append(selectedModels, m)
					usedProviders[m.Provider]++
					eligibleModels = append(eligibleModels[:i], eligibleModels[i+1:]...)
					break
				}
			}
			if len(selectedModels) == 0 && len(eligibleModels) > 0 {
				// No diverse option, take highest score
				selectedModels = append(selectedModels, eligibleModels[0])
				usedProviders[eligibleModels[0].Provider]++
				eligibleModels = eligibleModels[1:]
			}
		}

		// Add remaining models for fallbacks
		selectedModels = append(selectedModels, eligibleModels...)
	} else {
		// Simple: take top scoring models
		if len(eligibleModels) > totalRequired {
			selectedModels = eligibleModels[:totalRequired]
		} else {
			selectedModels = eligibleModels
		}
	}

	// Limit to required
	if len(selectedModels) > totalRequired {
		selectedModels = selectedModels[:totalRequired]
	}

	// Build debate group members
	var members []DebateGroupMember
	modelIndex := 0

	for position := 1; position <= config.PrimaryCount && modelIndex < len(selectedModels); position++ {
		member := DebateGroupMember{
			Position: position,
			Role:     "primary",
			Model:    selectedModels[modelIndex],
		}
		modelIndex++

		// Assign fallbacks
		for fb := 0; fb < config.FallbacksPerPrimary && modelIndex < len(selectedModels); fb++ {
			member.Fallbacks = append(member.Fallbacks, selectedModels[modelIndex])
			modelIndex++
		}

		members = append(members, member)
	}

	// Calculate metrics
	metrics.ModelsSelected = len(selectedModels)
	uniqueProviders := make(map[string]bool)
	allCapabilities := make(map[string]bool)

	var primaryScoreSum, fallbackScoreSum float64
	primaryCount, fallbackCount := 0, 0

	for _, m := range members {
		uniqueProviders[m.Model.Provider] = true
		primaryScoreSum += m.Model.Score
		primaryCount++
		for _, cap := range m.Model.Capabilities {
			allCapabilities[cap] = true
		}

		for _, fb := range m.Fallbacks {
			uniqueProviders[fb.Provider] = true
			fallbackScoreSum += fb.Score
			fallbackCount++
			for _, cap := range fb.Capabilities {
				allCapabilities[cap] = true
			}
		}
	}

	metrics.ProvidersUsed = len(uniqueProviders)
	if primaryCount > 0 {
		metrics.AveragePrimaryScore = primaryScoreSum / float64(primaryCount)
	}
	if fallbackCount > 0 {
		metrics.AverageFallbackScore = fallbackScoreSum / float64(fallbackCount)
	}

	// Calculate capability coverage (how many expected capabilities are covered)
	expectedCapabilities := []string{"code_generation", "reasoning", "vision", "function_calling", "code_completion"}
	coveredCount := 0
	for _, cap := range expectedCapabilities {
		if allCapabilities[cap] {
			coveredCount++
		}
	}
	metrics.CapabilityCoverage = float64(coveredCount) / float64(len(expectedCapabilities))

	// Calculate provider diversity (0-1 scale)
	if metrics.ModelsSelected > 0 {
		metrics.ProviderDiversity = float64(metrics.ProvidersUsed) / float64(min(metrics.ModelsSelected, 10))
	}

	// Calculate average score
	avgScore := float64(0)
	if len(selectedModels) > 0 {
		totalScore := float64(0)
		for _, m := range selectedModels {
			totalScore += m.Score
		}
		avgScore = totalScore / float64(len(selectedModels))
	}

	// Create debate group
	group := &DebateGroup{
		ID:           fmt.Sprintf("dg_%s", time.Now().Format("20060102_150405")),
		Name:         "SuperAgent AI Debate Group",
		CreatedAt:    time.Now(),
		Members:      members,
		TotalModels:  len(selectedModels),
		AverageScore: avgScore,
		Configuration: DebateConfiguration{
			DebateRounds:       3,
			ConsensusThreshold: 0.7,
			TimeoutSeconds:     60,
			FallbackStrategy:   "sequential",
		},
	}

	return group, metrics
}

// Validate the formed debate group
func validateDebateGroup(group *DebateGroup, config FormationConfig) []AssertionResult {
	var results []AssertionResult

	// Assert: Exactly N primary members
	primaryAssertion := AssertionResult{
		Type:   "exact_count",
		Target: "primary_members",
	}
	if len(group.Members) == config.PrimaryCount {
		primaryAssertion.Passed = true
		primaryAssertion.Message = fmt.Sprintf("Exactly %d primary members", config.PrimaryCount)
	} else {
		primaryAssertion.Passed = false
		primaryAssertion.Message = fmt.Sprintf("Expected %d primary members, got %d", config.PrimaryCount, len(group.Members))
	}
	results = append(results, primaryAssertion)

	// Assert: Exactly M fallbacks per primary
	fallbackAssertion := AssertionResult{
		Type:   "exact_count",
		Target: "fallbacks_per_primary",
	}
	allHaveCorrectFallbacks := true
	for _, m := range group.Members {
		if len(m.Fallbacks) != config.FallbacksPerPrimary {
			allHaveCorrectFallbacks = false
			break
		}
	}
	if allHaveCorrectFallbacks {
		fallbackAssertion.Passed = true
		fallbackAssertion.Message = fmt.Sprintf("Each primary has %d fallbacks", config.FallbacksPerPrimary)
	} else {
		fallbackAssertion.Passed = false
		fallbackAssertion.Message = fmt.Sprintf("Not all primaries have %d fallbacks", config.FallbacksPerPrimary)
	}
	results = append(results, fallbackAssertion)

	// Assert: No duplicate models
	noDuplicatesAssertion := AssertionResult{
		Type:   "no_duplicates",
		Target: "all_models",
	}
	modelIDs := make(map[string]bool)
	hasDuplicates := false
	for _, m := range group.Members {
		if modelIDs[m.Model.ID] {
			hasDuplicates = true
			break
		}
		modelIDs[m.Model.ID] = true
		for _, fb := range m.Fallbacks {
			if modelIDs[fb.ID] {
				hasDuplicates = true
				break
			}
			modelIDs[fb.ID] = true
		}
	}
	if !hasDuplicates {
		noDuplicatesAssertion.Passed = true
		noDuplicatesAssertion.Message = "No duplicate models in group"
	} else {
		noDuplicatesAssertion.Passed = false
		noDuplicatesAssertion.Message = "Duplicate models found in group"
	}
	results = append(results, noDuplicatesAssertion)

	// Assert: Minimum average score threshold
	scoreAssertion := AssertionResult{
		Type:   "min_score",
		Target: "average_group_score",
	}
	minAvgScore := 7.0
	if group.AverageScore >= minAvgScore {
		scoreAssertion.Passed = true
		scoreAssertion.Message = fmt.Sprintf("Average score %.2f >= %.2f", group.AverageScore, minAvgScore)
	} else {
		scoreAssertion.Passed = false
		scoreAssertion.Message = fmt.Sprintf("Average score %.2f < %.2f", group.AverageScore, minAvgScore)
	}
	results = append(results, scoreAssertion)

	return results
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Generate formation report
func generateReport(result ChallengeResult) string {
	var sb strings.Builder
	group := result.DebateGroup

	sb.WriteString("# AI Debate Group Formation Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", result.Timestamp.Format(time.RFC3339)))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Status | %s |\n", strings.ToUpper(result.Status)))
	sb.WriteString(fmt.Sprintf("| Group ID | %s |\n", group.ID))
	sb.WriteString(fmt.Sprintf("| Total Models | %d |\n", group.TotalModels))
	sb.WriteString(fmt.Sprintf("| Primary Members | %d |\n", len(group.Members)))
	sb.WriteString(fmt.Sprintf("| Average Score | %.2f |\n", group.AverageScore))
	sb.WriteString(fmt.Sprintf("| Duration | %v |\n", result.Duration))

	sb.WriteString("\n## Formation Metrics\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Models Considered | %d |\n", result.Metrics.ModelsConsidered))
	sb.WriteString(fmt.Sprintf("| Models Selected | %d |\n", result.Metrics.ModelsSelected))
	sb.WriteString(fmt.Sprintf("| Providers Used | %d |\n", result.Metrics.ProvidersUsed))
	sb.WriteString(fmt.Sprintf("| Average Primary Score | %.2f |\n", result.Metrics.AveragePrimaryScore))
	sb.WriteString(fmt.Sprintf("| Average Fallback Score | %.2f |\n", result.Metrics.AverageFallbackScore))
	sb.WriteString(fmt.Sprintf("| Capability Coverage | %.0f%% |\n", result.Metrics.CapabilityCoverage*100))
	sb.WriteString(fmt.Sprintf("| Provider Diversity | %.0f%% |\n", result.Metrics.ProviderDiversity*100))

	sb.WriteString("\n## Debate Group Members\n\n")
	for _, m := range group.Members {
		sb.WriteString(fmt.Sprintf("### Position %d: %s (Primary)\n\n", m.Position, m.Model.Name))
		sb.WriteString(fmt.Sprintf("- **Model ID**: %s\n", m.Model.ID))
		sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", m.Model.Provider))
		sb.WriteString(fmt.Sprintf("- **Score**: %.2f\n", m.Model.Score))
		if len(m.Model.Capabilities) > 0 {
			sb.WriteString(fmt.Sprintf("- **Capabilities**: %s\n", strings.Join(m.Model.Capabilities, ", ")))
		}

		if len(m.Fallbacks) > 0 {
			sb.WriteString("\n**Fallbacks:**\n\n")
			for i, fb := range m.Fallbacks {
				sb.WriteString(fmt.Sprintf("1. **Fallback %d**: %s (Score: %.2f, Provider: %s)\n",
					i+1, fb.Name, fb.Score, fb.Provider))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Configuration\n\n")
	sb.WriteString(fmt.Sprintf("```json\n"))
	configJSON, _ := json.MarshalIndent(group.Configuration, "", "  ")
	sb.WriteString(string(configJSON))
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Assertion Results\n\n")
	sb.WriteString("| Assertion | Target | Passed | Message |\n")
	sb.WriteString("|-----------|--------|--------|--------|\n")
	for _, a := range result.Assertions {
		passedStr := "No"
		if a.Passed {
			passedStr = "Yes"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", a.Type, a.Target, passedStr, a.Message))
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("*Generated by SuperAgent Challenges*\n")

	return sb.String()
}

func main() {
	resultsDir := flag.String("results-dir", "", "Directory to store results")
	dependencyDir := flag.String("dependency-dir", "", "Path to provider_verification results")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *resultsDir == "" {
		log.Fatal("--results-dir is required")
	}

	start := time.Now()

	// Create directories
	resultsPath := filepath.Join(*resultsDir, "results")
	logsPath := filepath.Join(*resultsDir, "logs")
	if err := os.MkdirAll(resultsPath, 0755); err != nil {
		log.Fatalf("Failed to create results directory: %v", err)
	}
	if err := os.MkdirAll(logsPath, 0755); err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}

	// Determine dependency path
	depPath := *dependencyDir
	if depPath == "" {
		// Try to find latest provider_verification results
		depPath = findLatestResults("provider_verification")
	}

	if depPath == "" {
		log.Fatal("Could not find provider_verification results. Run provider_verification first.")
	}

	if *verbose {
		log.Printf("Loading models from: %s", depPath)
	}

	// Load scored models
	models, err := loadScoredModels(depPath)
	if err != nil {
		log.Fatalf("Failed to load models: %v", err)
	}

	if *verbose {
		log.Printf("Loaded %d scored models", len(models))
	}

	// Form debate group
	config := defaultFormationConfig()
	group, metrics := FormDebateGroup(models, config)

	// Validate
	assertions := validateDebateGroup(group, config)

	// Determine status
	status := "passed"
	for _, a := range assertions {
		if !a.Passed {
			status = "failed"
			break
		}
	}

	// Build result
	result := ChallengeResult{
		ChallengeID:   "ai_debate_formation",
		ChallengeName: "AI Debate Group Formation",
		Timestamp:     time.Now(),
		Duration:      time.Since(start),
		Status:        status,
		DebateGroup:   group,
		Assertions:    assertions,
		Metrics:       metrics,
	}

	// Write outputs
	groupFile := filepath.Join(resultsPath, "debate_group.json")
	groupData, _ := json.MarshalIndent(group, "", "  ")
	if err := os.WriteFile(groupFile, groupData, 0644); err != nil {
		log.Printf("Warning: Failed to write debate group: %v", err)
	}

	membersFile := filepath.Join(resultsPath, "member_assignments.json")
	membersData, _ := json.MarshalIndent(group.Members, "", "  ")
	if err := os.WriteFile(membersFile, membersData, 0644); err != nil {
		log.Printf("Warning: Failed to write member assignments: %v", err)
	}

	reportFile := filepath.Join(resultsPath, "formation_report.md")
	report := generateReport(result)
	if err := os.WriteFile(reportFile, []byte(report), 0644); err != nil {
		log.Printf("Warning: Failed to write report: %v", err)
	}

	// Print summary
	fmt.Printf("\n=== AI Debate Group Formation Complete ===\n")
	fmt.Printf("Status: %s\n", strings.ToUpper(result.Status))
	fmt.Printf("Group ID: %s\n", group.ID)
	fmt.Printf("Members: %d primaries, %d total models\n", len(group.Members), group.TotalModels)
	fmt.Printf("Average Score: %.2f\n", group.AverageScore)
	fmt.Printf("Providers: %d\n", metrics.ProvidersUsed)
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Results: %s\n", resultsPath)

	// Assertions summary
	passedCount := 0
	for _, a := range assertions {
		if a.Passed {
			passedCount++
		}
	}
	fmt.Printf("Assertions: %d/%d passed\n", passedCount, len(assertions))

	if result.Status == "failed" {
		os.Exit(1)
	}
}

// findLatestResults finds the most recent results directory for a challenge
func findLatestResults(challengeID string) string {
	basePath := filepath.Join("results", challengeID)

	// Also check from challenges directory
	altPath := filepath.Join("..", "results", challengeID)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		basePath = altPath
	}

	// Walk to find the most recent timestamp directory
	var latestPath string
	var latestTime time.Time

	filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Check if this looks like a timestamp directory (YYYYMMDD_HHMMSS)
			name := info.Name()
			if len(name) == 15 && name[8] == '_' {
				if info.ModTime().After(latestTime) {
					latestTime = info.ModTime()
					latestPath = path
				}
			}
		}
		return nil
	})

	return latestPath
}
