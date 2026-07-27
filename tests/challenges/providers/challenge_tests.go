//go:build ignore

// Package providers implements real challenge tests for all providers
package providers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ChallengeCategory represents types of challenges
type ChallengeCategory int

const (
	CategoryShortRequest ChallengeCategory = iota
	CategoryBigRequest
	CategoryToolUse
	CategoryMCP
	CategoryLSP
	CategoryEmbeddings
	CategoryRAG
	CategoryACP
	CategoryVision
	CategoryStreaming
	CategoryJSON
	CategoryParallel
	CategoryDebate
	CategoryAgent
	CategoryCode
	CategoryMath
	CategoryReasoning
)

// DifficultyLevel represents challenge difficulty
type DifficultyLevel int

const (
	DifficultyEasy DifficultyLevel = iota
	DifficultyMedium
	DifficultyHard
	DifficultyExtreme
	DifficultyImpossible
)

// Challenge represents a single challenge test
type Challenge struct {
	Name        string
	Category    ChallengeCategory
	Difficulty  DifficultyLevel
	Description string
}

// TestResult holds challenge results
type TestResult struct {
	Success bool
	Latency time.Duration
	Tokens  int
	Error   error
	Details string
}

// ProviderChallengeRunner runs challenges against providers
type ProviderChallengeRunner struct {
	Provider ProviderConfig
	Client   llm.Client
	Logger   *zap.Logger
}

// ProviderConfig for challenge tests
type ProviderConfig struct {
	Name           string
	EnvVar         string
	Type           string
	Model          string
	SupportsTools  bool
	SupportsVision bool
}

var challengeProviders = []ProviderConfig{
	{Name: "OpenAI", EnvVar: "OPENAI_API_KEY", Type: "openai", Model: "gpt-4o", SupportsTools: true, SupportsVision: true},
	{Name: "Anthropic", EnvVar: "ANTHROPIC_API_KEY", Type: "anthropic", Model: "claude-3-5-sonnet-20241022", SupportsTools: true, SupportsVision: true},
	{Name: "DeepSeek", EnvVar: "DEEPSEEK_API_KEY", Type: "deepseek", Model: "deepseek-chat", SupportsTools: true, SupportsVision: false},
	{Name: "Groq", EnvVar: "GROQ_API_KEY", Type: "groq", Model: "llama-3.1-70b-versatile", SupportsTools: true, SupportsVision: true},
	{Name: "Mistral", EnvVar: "MISTRAL_API_KEY", Type: "mistral", Model: "mistral-large-latest", SupportsTools: true, SupportsVision: false},
	{Name: "Gemini", EnvVar: "GEMINI_API_KEY", Type: "gemini", Model: "gemini-2.0-flash-exp", SupportsTools: true, SupportsVision: true},
}

// runChallenge executes a challenge and returns results
func (r *ProviderChallengeRunner) runChallenge(ctx context.Context, challenge Challenge) TestResult {
	start := time.Now()
	result := TestResult{Success: false}

	switch challenge.Category {
	case CategoryShortRequest:
		result = r.challengeShortRequest(ctx)
	case CategoryBigRequest:
		result = r.challengeBigRequest(ctx, challenge.Difficulty)
	case CategoryToolUse:
		result = r.challengeToolUse(ctx)
	case CategoryStreaming:
		result = r.challengeStreaming(ctx)
	case CategoryJSON:
		result = r.challengeJSON(ctx)
	case CategoryVision:
		result = r.challengeVision(ctx)
	case CategoryParallel:
		result = r.challengeParallel(ctx)
	case CategoryCode:
		result = r.challengeCode(ctx)
	case CategoryMath:
		result = r.challengeMath(ctx)
	case CategoryReasoning:
		result = r.challengeReasoning(ctx)
	}

	result.Latency = time.Since(start)
	return result
}

func (r *ProviderChallengeRunner) challengeShortRequest(ctx context.Context) TestResult {
	resp, err := r.Client.Chat(ctx, llm.ChatRequest{
		Model: r.Provider.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "Say 'test'"},
		},
		MaxTokens: 10,
	})
	if err != nil {
		return TestResult{Success: false, Error: err}
	}
	return TestResult{
		Success: strings.Contains(strings.ToLower(resp.Content), "test"),
		Tokens:  resp.Usage.TotalTokens,
		Details: resp.Content,
	}
}

func (r *ProviderChallengeRunner) challengeBigRequest(ctx context.Context, difficulty DifficultyLevel) TestResult {
	tokenCounts := map[DifficultyLevel]int{
		DifficultyEasy:       1000,
		DifficultyMedium:     10000,
		DifficultyHard:       50000,
		DifficultyExtreme:    100000,
		DifficultyImpossible: 200000,
	}

	tokens := tokenCounts[difficulty]
	bigText := generateBigText(tokens)

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := r.Client.Chat(ctx, llm.ChatRequest{
		Model: r.Provider.Model,
		Messages: []llm.Message{
			{Role: "system", Content: "Summarize the following text:"},
			{Role: "user", Content: bigText},
		},
		MaxTokens: 500,
	})

	if err != nil {
		return TestResult{Success: false, Error: err}
	}

	return TestResult{
		Success: len(resp.Content) > 0,
		Latency: time.Since(start),
		Tokens:  resp.Usage.TotalTokens,
		Details: fmt.Sprintf("Processed %d input tokens", tokens),
	}
}

func (r *ProviderChallengeRunner) challengeToolUse(ctx context.Context) TestResult {
	if !r.Provider.SupportsTools {
		return TestResult{Success: false, Details: "Provider doesn't support tools"}
	}

	tools := []llm.ToolDefinition{
		{
			Name:        "calculate",
			Description: "Calculate a math expression",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]string{"type": "string"},
				},
				"required": []string{"expression"},
			},
		},
	}

	resp, err := r.Client.Chat(ctx, llm.ChatRequest{
		Model: r.Provider.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "What is 123 * 456?"},
		},
		Tools:     tools,
		MaxTokens: 100,
	})
	if err != nil {
		return TestResult{Success: false, Error: err}
	}

	return TestResult{
		Success: len(resp.ToolCalls) > 0,
		Details: fmt.Sprintf("Tool calls: %d", len(resp.ToolCalls)),
	}
}

func (r *ProviderChallengeRunner) challengeStreaming(ctx context.Context) TestResult {
	stream, err := r.Client.ChatStream(ctx, llm.ChatRequest{
		Model: r.Provider.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "Count from 1 to 10"},
		},
		MaxTokens: 100,
	})
	if err != nil {
		return TestResult{Success: false, Error: err}
	}

	var chunks int
	for chunk := range stream {
		if chunk.Error != nil {
			return TestResult{Success: false, Error: chunk.Error}
		}
		chunks++
	}

	return TestResult{
		Success: chunks > 5,
		Details: fmt.Sprintf("Received %d chunks", chunks),
	}
}

func (r *ProviderChallengeRunner) challengeJSON(ctx context.Context) TestResult {
	resp, err := r.Client.Chat(ctx, llm.ChatRequest{
		Model: r.Provider.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "Generate JSON with 'name' and 'value' fields. Output only JSON."},
		},
		MaxTokens:      100,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return TestResult{Success: false, Error: err}
	}

	// Basic JSON validation
	isJSON := strings.HasPrefix(strings.TrimSpace(resp.Content), "{") &&
		strings.HasSuffix(strings.TrimSpace(resp.Content), "}")

	return TestResult{
		Success: isJSON,
		Details: resp.Content,
	}
}

func (r *ProviderChallengeRunner) challengeVision(ctx context.Context) TestResult {
	if !r.Provider.SupportsVision {
		return TestResult{Success: false, Details: "Provider doesn't support vision"}
	}

	// Red pixel image
	imageBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

	resp, err := r.Client.Chat(ctx, llm.ChatRequest{
		Model: r.Provider.Model,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: "What color is this?",
				Images:  []string{imageBase64},
			},
		},
		MaxTokens: 50,
	})
	if err != nil {
		return TestResult{Success: false, Error: err}
	}

	return TestResult{
		Success: len(resp.Content) > 0,
		Details: resp.Content,
	}
}

func (r *ProviderChallengeRunner) challengeParallel(ctx context.Context) TestResult {
	var wg sync.WaitGroup
	successes := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := r.Client.Chat(ctx, llm.ChatRequest{
				Model: r.Provider.Model,
				Messages: []llm.Message{
					{Role: "user", Content: fmt.Sprintf("Say '%d'", idx)},
				},
				MaxTokens: 10,
			})
			successes <- (err == nil && len(resp.Content) > 0)
		}(i)
	}

	go func() {
		wg.Wait()
		close(successes)
	}()

	successCount := 0
	for s := range successes {
		if s {
			successCount++
		}
	}

	return TestResult{
		Success: successCount == 5,
		Details: fmt.Sprintf("%d/5 parallel requests succeeded", successCount),
	}
}

func (r *ProviderChallengeRunner) challengeCode(ctx context.Context) TestResult {
	resp, err := r.Client.Chat(ctx, llm.ChatRequest{
		Model: r.Provider.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "Write a Go function to reverse a string"},
		},
		MaxTokens: 200,
	})
	if err != nil {
		return TestResult{Success: false, Error: err}
	}

	// Check if response contains Go code elements
	hasCode := strings.Contains(resp.Content, "func") &&
		(strings.Contains(resp.Content, "string") || strings.Contains(resp.Content, "rune"))

	return TestResult{
		Success: hasCode,
		Details: fmt.Sprintf("Response length: %d chars", len(resp.Content)),
	}
}

func (r *ProviderChallengeRunner) challengeMath(ctx context.Context) TestResult {
	resp, err := r.Client.Chat(ctx, llm.ChatRequest{
		Model: r.Provider.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "Calculate: (15 * 23) + (47 * 8) = ?"},
		},
		MaxTokens: 100,
	})
	if err != nil {
		return TestResult{Success: false, Error: err}
	}

	// Expected: 345 + 376 = 721
	containsAnswer := strings.Contains(resp.Content, "721")

	return TestResult{
		Success: containsAnswer,
		Details: resp.Content,
	}
}

func (r *ProviderChallengeRunner) challengeReasoning(ctx context.Context) TestResult {
	resp, err := r.Client.Chat(ctx, llm.ChatRequest{
		Model: r.Provider.Model,
		Messages: []llm.Message{
			{Role: "user", Content: "If it takes 5 machines 5 minutes to make 5 widgets, how long does it take 100 machines to make 100 widgets? Explain your reasoning."},
		},
		MaxTokens: 200,
	})
	if err != nil {
		return TestResult{Success: false, Error: err}
	}

	// Should contain "5 minutes" or "five minutes"
	correct := strings.Contains(strings.ToLower(resp.Content), "5 minute") ||
		strings.Contains(strings.ToLower(resp.Content), "five minute")

	return TestResult{
		Success: correct,
		Details: resp.Content,
	}
}

// TestChallenges_ShortRequest runs short request challenge
func TestChallenges_ShortRequest(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	for _, p := range challengeProviders {
		t.Run(p.Name, func(t *testing.T) {
			apiKey := os.Getenv(p.EnvVar)
			if apiKey == "" {
				t.Skipf("%s not set", p.EnvVar)
			}

			runner := &ProviderChallengeRunner{
				Provider: p,
				Client:   llm.NewClient(p.Type, apiKey, logger),
				Logger:   logger,
			}

			ctx := context.Background()
			result := runner.runChallenge(ctx, Challenge{
				Name:       "Short Request",
				Category:   CategoryShortRequest,
				Difficulty: DifficultyEasy,
			})

			assert.True(t, result.Success, "Challenge failed: %v", result.Error)
			t.Logf("%s: %v, Tokens: %d", p.Name, result.Latency, result.Tokens)
		})
	}
}

// TestChallenges_BigRequest runs big request challenges
func TestChallenges_BigRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping big request in short mode") // SKIP-OK: #short-mode
	}

	logger, _ := zap.NewDevelopment()

	difficulties := []DifficultyLevel{DifficultyEasy, DifficultyMedium}

	for _, diff := range difficulties {
		t.Run(fmt.Sprintf("Difficulty_%d", diff), func(t *testing.T) {
			for _, p := range challengeProviders {
				t.Run(p.Name, func(t *testing.T) {
					apiKey := os.Getenv(p.EnvVar)
					if apiKey == "" {
						t.Skipf("%s not set", p.EnvVar)
					}

					runner := &ProviderChallengeRunner{
						Provider: p,
						Client:   llm.NewClient(p.Type, apiKey, logger),
						Logger:   logger,
					}

					ctx := context.Background()
					result := runner.runChallenge(ctx, Challenge{
						Name:       "Big Request",
						Category:   CategoryBigRequest,
						Difficulty: diff,
					})

					assert.True(t, result.Success, "Challenge failed: %v", result.Error)
					t.Logf("%s: %v - %s", p.Name, result.Latency, result.Details)
				})
			}
		})
	}
}

// TestChallenges_AllCategories runs all challenge categories
func TestChallenges_AllCategories(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full challenge suite in short mode") // SKIP-OK: #short-mode
	}

	logger, _ := zap.NewDevelopment()

	challenges := []Challenge{
		{Name: "Tool Use", Category: CategoryToolUse, Difficulty: DifficultyMedium},
		{Name: "Streaming", Category: CategoryStreaming, Difficulty: DifficultyEasy},
		{Name: "JSON", Category: CategoryJSON, Difficulty: DifficultyEasy},
		{Name: "Vision", Category: CategoryVision, Difficulty: DifficultyMedium},
		{Name: "Parallel", Category: CategoryParallel, Difficulty: DifficultyMedium},
		{Name: "Code", Category: CategoryCode, Difficulty: DifficultyEasy},
		{Name: "Math", Category: CategoryMath, Difficulty: DifficultyEasy},
		{Name: "Reasoning", Category: CategoryReasoning, Difficulty: DifficultyMedium},
	}

	for _, p := range challengeProviders {
		apiKey := os.Getenv(p.EnvVar)
		if apiKey == "" {
			continue
		}

		t.Run(p.Name, func(t *testing.T) {
			runner := &ProviderChallengeRunner{
				Provider: p,
				Client:   llm.NewClient(p.Type, apiKey, logger),
				Logger:   logger,
			}

			ctx := context.Background()
			passed := 0

			for _, challenge := range challenges {
				result := runner.runChallenge(ctx, challenge)
				if result.Success {
					passed++
				}
				t.Logf("  %s: %v", challenge.Name, result.Success)
			}

			t.Logf("%s: %d/%d challenges passed", p.Name, passed, len(challenges))
		})
	}
}

// generateBigText generates text of approximately n tokens
func generateBigText(tokens int) string {
	word := "word "
	wordsNeeded := tokens / 2 // Approximate
	result := make([]byte, 0, wordsNeeded*len(word))
	for i := 0; i < wordsNeeded; i++ {
		result = append(result, word...)
	}
	return string(result)
}
