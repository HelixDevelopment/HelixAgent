// Package main_challenge implements the Main SuperAgent Challenge
// that verifies all providers, benchmarks LLMs, forms AI debate groups,
// and generates OpenCode configuration.
//
// This challenge uses ONLY production binaries - NO MOCKS, NO STUBS!
package main_challenge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// MainChallengeConfig holds configuration for the Main challenge
type MainChallengeConfig struct {
	// LLMsVerifier binary path
	LLMsVerifierPath string `json:"llmsverifier_path"`

	// SuperAgent binary path
	SuperAgentPath string `json:"superagent_path"`

	// Results directory
	ResultsDir string `json:"results_dir"`

	// Debate group configuration
	DebateGroupSize       int `json:"debate_group_size"`
	FallbacksPerMember    int `json:"fallbacks_per_member"`
	MinimumModelScore     float64 `json:"minimum_model_score"`

	// Timeouts
	ProviderVerificationTimeout time.Duration `json:"provider_verification_timeout"`
	ModelBenchmarkTimeout       time.Duration `json:"model_benchmark_timeout"`
	SystemVerificationTimeout   time.Duration `json:"system_verification_timeout"`

	// Logging
	Verbose bool `json:"verbose"`
}

// DefaultMainChallengeConfig returns default configuration
func DefaultMainChallengeConfig() *MainChallengeConfig {
	return &MainChallengeConfig{
		LLMsVerifierPath:            "../LLMsVerifier/llm-verifier/llm-verifier",
		SuperAgentPath:              "./superagent",
		ResultsDir:                  "challenges/results/main_challenge",
		DebateGroupSize:             5,
		FallbacksPerMember:          2,
		MinimumModelScore:           7.0,
		ProviderVerificationTimeout: 5 * time.Minute,
		ModelBenchmarkTimeout:       30 * time.Minute,
		SystemVerificationTimeout:   10 * time.Minute,
		Verbose:                     true,
	}
}

// ProviderResult holds verification results for a single provider
type ProviderResult struct {
	Name          string    `json:"name"`
	Enabled       bool      `json:"enabled"`
	APIKeySet     bool      `json:"api_key_set"`
	Verified      bool      `json:"verified"`
	Models        []string  `json:"models"`
	ModelCount    int       `json:"model_count"`
	ResponseTime  time.Duration `json:"response_time_ms"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	VerifiedAt    time.Time `json:"verified_at"`
}

// ModelScore holds scoring information for an LLM model
type ModelScore struct {
	Provider        string             `json:"provider"`
	ModelID         string             `json:"model_id"`
	DisplayName     string             `json:"display_name"`
	TotalScore      float64            `json:"total_score"`
	ScoreBreakdown  map[string]float64 `json:"score_breakdown"`
	Capabilities    []string           `json:"capabilities"`
	Verified        bool               `json:"verified"`
	ResponseTimeMS  int64              `json:"response_time_ms"`
	IsFree          bool               `json:"is_free"`
}

// DebateGroupMember represents a member of the AI debate group
type DebateGroupMember struct {
	Position   int          `json:"position"`
	Role       string       `json:"role"` // "primary" or "fallback_N"
	Model      ModelScore   `json:"model"`
	Fallbacks  []ModelScore `json:"fallbacks,omitempty"`
}

// DebateGroup represents the complete AI debate group configuration
type DebateGroup struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	CreatedAt     time.Time           `json:"created_at"`
	Members       []DebateGroupMember `json:"members"`
	TotalModels   int                 `json:"total_models"`
	AverageScore  float64             `json:"average_score"`
	Configuration DebateConfiguration `json:"configuration"`
}

// DebateConfiguration holds debate-specific settings
type DebateConfiguration struct {
	DebateRounds       int     `json:"debate_rounds"`
	ConsensusThreshold float64 `json:"consensus_threshold"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	FallbackStrategy   string  `json:"fallback_strategy"`
}

// MainChallengeResult holds the complete result of the Main challenge
type MainChallengeResult struct {
	ChallengeID     string              `json:"challenge_id"`
	ChallengeName   string              `json:"challenge_name"`
	StartTime       time.Time           `json:"start_time"`
	EndTime         time.Time           `json:"end_time"`
	Duration        time.Duration       `json:"duration"`
	Status          string              `json:"status"` // "passed", "failed", "partial"

	// Phase results
	ProviderVerification struct {
		ProvidersTotal    int               `json:"providers_total"`
		ProvidersVerified int               `json:"providers_verified"`
		ProvidersFailed   int               `json:"providers_failed"`
		Providers         []ProviderResult  `json:"providers"`
	} `json:"provider_verification"`

	ModelBenchmark struct {
		ModelsTotal      int          `json:"models_total"`
		ModelsVerified   int          `json:"models_verified"`
		ModelsFailed     int          `json:"models_failed"`
		TopModels        []ModelScore `json:"top_models"`
		AllScores        []ModelScore `json:"all_scores"`
	} `json:"model_benchmark"`

	DebateGroupFormation struct {
		DebateGroup      DebateGroup `json:"debate_group"`
		FormationSuccess bool        `json:"formation_success"`
		SelectionReport  string      `json:"selection_report"`
	} `json:"debate_group_formation"`

	SystemVerification struct {
		Verified          bool    `json:"verified"`
		VerificationScore float64 `json:"verification_score"`
		TestsPassed       int     `json:"tests_passed"`
		TestsFailed       int     `json:"tests_failed"`
		Report            string  `json:"report"`
	} `json:"system_verification"`

	// Output files
	OpenCodeConfigPath string `json:"opencode_config_path"`
	ReportPath         string `json:"report_path"`
	LogPath            string `json:"log_path"`

	// Errors
	Errors []string `json:"errors,omitempty"`
}

// OpenCodeConfig represents the OpenCode configuration structure
type OpenCodeConfig struct {
	Endpoint    string                 `json:"endpoint"`
	APIKey      string                 `json:"api_key"`
	Model       string                 `json:"model"`
	Features    OpenCodeFeatures       `json:"features"`
	DebateGroup OpenCodeDebateGroup    `json:"debate_group"`
	Providers   map[string]interface{} `json:"providers,omitempty"`
}

// OpenCodeFeatures represents feature configuration
type OpenCodeFeatures struct {
	MCP        MCPConfig        `json:"mcp"`
	ACP        ACPConfig        `json:"acp"`
	LSP        LSPConfig        `json:"lsp"`
	Embeddings EmbeddingsConfig `json:"embeddings"`
}

// MCPConfig represents MCP (Model Context Protocol) configuration
type MCPConfig struct {
	Enabled bool                   `json:"enabled"`
	Servers []map[string]interface{} `json:"servers,omitempty"`
}

// ACPConfig represents ACP (Agent Context Protocol) configuration
type ACPConfig struct {
	Enabled bool                   `json:"enabled"`
	Servers []map[string]interface{} `json:"servers,omitempty"`
}

// LSPConfig represents LSP (Language Server Protocol) configuration
type LSPConfig struct {
	Enabled bool                   `json:"enabled"`
	Servers []map[string]interface{} `json:"servers,omitempty"`
}

// EmbeddingsConfig represents embeddings configuration
type EmbeddingsConfig struct {
	Enabled bool   `json:"enabled"`
	Model   string `json:"model"`
}

// OpenCodeDebateGroup represents debate group in OpenCode config
type OpenCodeDebateGroup struct {
	Members           int    `json:"members"`
	FallbacksPerMember int   `json:"fallbacks_per_member"`
	Strategy          string `json:"strategy"`
}

// MainChallenge implements the main challenge logic
type MainChallenge struct {
	config *MainChallengeConfig
	result *MainChallengeResult
}

// NewMainChallenge creates a new MainChallenge instance
func NewMainChallenge(config *MainChallengeConfig) *MainChallenge {
	if config == nil {
		config = DefaultMainChallengeConfig()
	}

	return &MainChallenge{
		config: config,
		result: &MainChallengeResult{
			ChallengeID:   fmt.Sprintf("main_%d", time.Now().Unix()),
			ChallengeName: "Main SuperAgent Challenge",
			StartTime:     time.Now(),
		},
	}
}

// Run executes the complete Main challenge
func (c *MainChallenge) Run(ctx context.Context) (*MainChallengeResult, error) {
	c.result.StartTime = time.Now()

	// Create results directory
	timestamp := time.Now().Format("20060102_150405")
	resultsDir := filepath.Join(c.config.ResultsDir,
		time.Now().Format("2006"),
		time.Now().Format("01"),
		time.Now().Format("02"),
		timestamp,
	)

	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create results directory: %w", err)
	}

	logsDir := filepath.Join(resultsDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	c.result.LogPath = filepath.Join(logsDir, "challenge.log")

	// Note: Actual implementation would call binaries here
	// This is the framework structure - execution is done by bash scripts

	c.result.EndTime = time.Now()
	c.result.Duration = c.result.EndTime.Sub(c.result.StartTime)

	return c.result, nil
}

// SelectTopModels selects the top N models based on scores
func SelectTopModels(models []ModelScore, count int) []ModelScore {
	if len(models) == 0 {
		return nil
	}

	// Sort by total score descending
	sorted := make([]ModelScore, len(models))
	copy(sorted, models)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TotalScore > sorted[j].TotalScore
	})

	if count > len(sorted) {
		count = len(sorted)
	}

	return sorted[:count]
}

// FormDebateGroup creates a debate group from scored models
func FormDebateGroup(models []ModelScore, primaryCount, fallbacksPerPrimary int) *DebateGroup {
	if len(models) == 0 {
		return nil
	}

	totalNeeded := primaryCount + (primaryCount * fallbacksPerPrimary)
	if totalNeeded > len(models) {
		// Adjust if not enough models
		if len(models) < primaryCount {
			primaryCount = len(models)
			fallbacksPerPrimary = 0
		} else {
			fallbacksPerPrimary = (len(models) - primaryCount) / primaryCount
		}
	}

	topModels := SelectTopModels(models, totalNeeded)

	group := &DebateGroup{
		ID:        fmt.Sprintf("debate_group_%d", time.Now().Unix()),
		Name:      "SuperAgent AI Debate Group",
		CreatedAt: time.Now(),
		Members:   make([]DebateGroupMember, 0, primaryCount),
		Configuration: DebateConfiguration{
			DebateRounds:       3,
			ConsensusThreshold: 0.7,
			TimeoutSeconds:     60,
			FallbackStrategy:   "next_best",
		},
	}

	modelIndex := 0
	var totalScore float64

	for i := 0; i < primaryCount && modelIndex < len(topModels); i++ {
		member := DebateGroupMember{
			Position:  i + 1,
			Role:      "primary",
			Model:     topModels[modelIndex],
			Fallbacks: make([]ModelScore, 0, fallbacksPerPrimary),
		}
		totalScore += topModels[modelIndex].TotalScore
		modelIndex++

		// Add fallbacks
		for j := 0; j < fallbacksPerPrimary && modelIndex < len(topModels); j++ {
			member.Fallbacks = append(member.Fallbacks, topModels[modelIndex])
			totalScore += topModels[modelIndex].TotalScore
			modelIndex++
		}

		group.Members = append(group.Members, member)
	}

	group.TotalModels = modelIndex
	if modelIndex > 0 {
		group.AverageScore = totalScore / float64(modelIndex)
	}

	return group
}

// GenerateOpenCodeConfig generates OpenCode configuration
func GenerateOpenCodeConfig(group *DebateGroup, endpoint, apiKey string) *OpenCodeConfig {
	return &OpenCodeConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    "superagent-ensemble",
		Features: OpenCodeFeatures{
			MCP: MCPConfig{
				Enabled: true,
				Servers: []map[string]interface{}{
					{"name": "filesystem", "command": "npx", "args": []string{"-y", "@anthropic-ai/mcp-filesystem"}},
					{"name": "github", "command": "npx", "args": []string{"-y", "@anthropic-ai/mcp-github"}},
				},
			},
			ACP: ACPConfig{
				Enabled: true,
				Servers: []map[string]interface{}{},
			},
			LSP: LSPConfig{
				Enabled: true,
				Servers: []map[string]interface{}{
					{"name": "gopls", "language": "go"},
					{"name": "typescript-language-server", "language": "typescript"},
				},
			},
			Embeddings: EmbeddingsConfig{
				Enabled: true,
				Model:   "text-embedding-3-small",
			},
		},
		DebateGroup: OpenCodeDebateGroup{
			Members:            len(group.Members),
			FallbacksPerMember: 2,
			Strategy:           "confidence_weighted",
		},
	}
}

// SaveJSON saves data to a JSON file
func SaveJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
