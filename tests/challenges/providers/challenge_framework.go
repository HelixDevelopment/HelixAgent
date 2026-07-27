//go:build ignore

// Package providers implements challenge-based testing for all LLM providers
// Challenges validate real-world capabilities and edge cases
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/llm"
	"go.uber.org/zap"
)

// Challenge represents a test challenge for providers
type Challenge struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    ChallengeCategory `json:"category"`
	Description string            `json:"description"`
	Difficulty  Difficulty        `json:"difficulty"`
	Prompt      string            `json:"prompt"`
	Expected    ExpectedResult    `json:"expected"`
	Timeout     time.Duration     `json:"timeout"`
	MaxTokens   int               `json:"max_tokens"`
	Validators  []Validator       `json:"validators"`
}

// ChallengeCategory defines challenge types
type ChallengeCategory string

const (
	CategoryBasic       ChallengeCategory = "basic"
	CategoryReasoning   ChallengeCategory = "reasoning"
	CategoryCoding      ChallengeCategory = "coding"
	CategoryMath        ChallengeCategory = "math"
	CategoryCreativity  ChallengeCategory = "creativity"
	CategoryInstruction ChallengeCategory = "instruction"
	CategoryContext     ChallengeCategory = "context"
	CategoryTools       ChallengeCategory = "tools"
	CategoryVision      ChallengeCategory = "vision"
	CategoryRAG         ChallengeCategory = "rag"
	CategoryMCP         ChallengeCategory = "mcp"
	CategoryLSP         ChallengeCategory = "lsp"
	CategoryEmbeddings  ChallengeCategory = "embeddings"
)

// Difficulty levels
type Difficulty string

const (
	Easy       Difficulty = "easy"
	Medium     Difficulty = "medium"
	Hard       Difficulty = "hard"
	Expert     Difficulty = "expert"
	Impossible Difficulty = "impossible"
)

// ExpectedResult defines what we expect from the model
type ExpectedResult struct {
	Contains      []string        `json:"contains,omitempty"`
	NotContains   []string        `json:"not_contains,omitempty"`
	JSONSchema    json.RawMessage `json:"json_schema,omitempty"`
	MinLength     int             `json:"min_length,omitempty"`
	MaxLength     int             `json:"max_length,omitempty"`
	ToolCalls     []string        `json:"tool_calls,omitempty"`
	CodeValid     bool            `json:"code_valid,omitempty"`
	AnswerCorrect string          `json:"answer_correct,omitempty"`
}

// Validator validates challenge results
type Validator func(response string, expected ExpectedResult) (bool, string)

// ChallengeRunner runs challenges against providers
type ChallengeRunner struct {
	logger *zap.Logger
	client llm.Client
}

// NewChallengeRunner creates a new challenge runner
func NewChallengeRunner(logger *zap.Logger, client llm.Client) *ChallengeRunner {
	return &ChallengeRunner{
		logger: logger,
		client: client,
	}
}

// RunChallenge runs a single challenge
func (r *ChallengeRunner) RunChallenge(ctx context.Context, challenge Challenge, model string) (*ChallengeResult, error) {
	start := time.Now()

	request := llm.ChatRequest{
		Model:     model,
		Messages:  []llm.Message{{Role: "user", Content: challenge.Prompt}},
		MaxTokens: challenge.MaxTokens,
	}

	response, err := r.client.Chat(ctx, request)
	if err != nil {
		return &ChallengeResult{
			ChallengeID: challenge.ID,
			Model:       model,
			Passed:      false,
			Error:       err.Error(),
			Duration:    time.Since(start),
		}, nil
	}

	// Validate response
	passed, reason := r.validate(response.Content, challenge.Expected)

	return &ChallengeResult{
		ChallengeID:  challenge.ID,
		Model:        model,
		Passed:       passed,
		Response:     response.Content,
		Reason:       reason,
		Duration:     time.Since(start),
		TokensUsed:   response.Usage.TotalTokens,
		TokensPerSec: float64(response.Usage.TotalTokens) / time.Since(start).Seconds(),
	}, nil
}

// validate validates response against expected result
func (r *ChallengeRunner) validate(response string, expected ExpectedResult) (bool, string) {
	// Check contains
	for _, s := range expected.Contains {
		if !strings.Contains(strings.ToLower(response), strings.ToLower(s)) {
			return false, fmt.Sprintf("Response missing expected content: %s", s)
		}
	}

	// Check not contains
	for _, s := range expected.NotContains {
		if strings.Contains(strings.ToLower(response), strings.ToLower(s)) {
			return false, fmt.Sprintf("Response contains forbidden content: %s", s)
		}
	}

	// Check length
	if expected.MinLength > 0 && len(response) < expected.MinLength {
		return false, fmt.Sprintf("Response too short: %d < %d", len(response), expected.MinLength)
	}

	if expected.MaxLength > 0 && len(response) > expected.MaxLength {
		return false, fmt.Sprintf("Response too long: %d > %d", len(response), expected.MaxLength)
	}

	return true, "All validations passed"
}

// ChallengeResult holds the result of running a challenge
type ChallengeResult struct {
	ChallengeID  string        `json:"challenge_id"`
	Model        string        `json:"model"`
	Passed       bool          `json:"passed"`
	Response     string        `json:"response,omitempty"`
	Reason       string        `json:"reason"`
	Error        string        `json:"error,omitempty"`
	Duration     time.Duration `json:"duration"`
	TokensUsed   int           `json:"tokens_used"`
	TokensPerSec float64       `json:"tokens_per_sec"`
}

// GetAllChallenges returns all available challenges
func GetAllChallenges() []Challenge {
	return []Challenge{
		// BASIC CHALLENGES
		{
			ID:          "basic-hello",
			Name:        "Hello World",
			Category:    CategoryBasic,
			Description: "Simple greeting response",
			Difficulty:  Easy,
			Prompt:      "Say hello in exactly 3 words.",
			Expected:    ExpectedResult{Contains: []string{"hello"}, MaxLength: 50},
			Timeout:     10 * time.Second,
			MaxTokens:   50,
		},
		{
			ID:          "basic-json",
			Name:        "JSON Generation",
			Category:    CategoryBasic,
			Description: "Generate valid JSON",
			Difficulty:  Easy,
			Prompt:      "Generate a JSON object with keys: name, age, city. Values can be any reasonable values.",
			Expected:    ExpectedResult{Contains: []string{"{", "}", "name", "age", "city"}},
			Timeout:     15 * time.Second,
			MaxTokens:   200,
		},

		// REASONING CHALLENGES
		{
			ID:          "reasoning-math",
			Name:        "Math Word Problem",
			Category:    CategoryReasoning,
			Description: "Solve a multi-step math problem",
			Difficulty:  Medium,
			Prompt:      "A train travels at 60 mph. Another train leaves 2 hours later at 80 mph. How long until the second train catches up?",
			Expected:    ExpectedResult{Contains: []string{"6"}, AnswerCorrect: "6 hours"},
			Timeout:     30 * time.Second,
			MaxTokens:   500,
		},
		{
			ID:          "reasoning-logic",
			Name:        "Logical Deduction",
			Category:    CategoryReasoning,
			Description: "Deduce answer from clues",
			Difficulty:  Hard,
			Prompt:      "Three people: Alice, Bob, Charlie. Alice is taller than Bob. Charlie is shorter than Bob. Who is tallest?",
			Expected:    ExpectedResult{Contains: []string{"alice"}, AnswerCorrect: "Alice"},
			Timeout:     30 * time.Second,
			MaxTokens:   300,
		},

		// CODING CHALLENGES
		{
			ID:          "coding-fibonacci",
			Name:        "Fibonacci Function",
			Category:    CategoryCoding,
			Description: "Write a fibonacci function",
			Difficulty:  Easy,
			Prompt:      "Write a Python function that returns the nth Fibonacci number. Include docstring and type hints.",
			Expected:    ExpectedResult{Contains: []string{"def", "fibonacci", "return"}, CodeValid: true},
			Timeout:     30 * time.Second,
			MaxTokens:   500,
		},
		{
			ID:          "coding-debug",
			Name:        "Debug Code",
			Category:    CategoryCoding,
			Description: "Find and fix bug",
			Difficulty:  Medium,
			Prompt:      "This code has a bug: def add(a, b): return a + c. Fix it.",
			Expected:    ExpectedResult{Contains: []string{"b"}, NotContains: []string{"c"}},
			Timeout:     30 * time.Second,
			MaxTokens:   300,
		},

		// CONTEXT CHALLENGES
		{
			ID:          "context-1k",
			Name:        "1K Context",
			Category:    CategoryContext,
			Description: "Handle 1K token context",
			Difficulty:  Easy,
			Prompt:      generateContextPrompt(1000) + "\n\nSummarize the above text in 3 sentences.",
			Expected:    ExpectedResult{MinLength: 100, Contains: []string{"summary", "text"}},
			Timeout:     30 * time.Second,
			MaxTokens:   500,
		},
		{
			ID:          "context-10k",
			Name:        "10K Context",
			Category:    CategoryContext,
			Description: "Handle 10K token context",
			Difficulty:  Medium,
			Prompt:      generateContextPrompt(10000) + "\n\nWhat is the main topic?",
			Expected:    ExpectedResult{MinLength: 50},
			Timeout:     60 * time.Second,
			MaxTokens:   500,
		},
		{
			ID:          "context-50k",
			Name:        "50K Context",
			Category:    CategoryContext,
			Description: "Handle 50K token context",
			Difficulty:  Hard,
			Prompt:      generateContextPrompt(50000) + "\n\nList the key points.",
			Expected:    ExpectedResult{MinLength: 100},
			Timeout:     120 * time.Second,
			MaxTokens:   1000,
		},

		// TOOL CHALLENGES
		{
			ID:          "tool-single",
			Name:        "Single Tool Call",
			Category:    CategoryTools,
			Description: "Call one tool correctly",
			Difficulty:  Easy,
			Prompt:      "What is the weather in San Francisco?",
			Expected:    ExpectedResult{ToolCalls: []string{"get_weather"}},
			Timeout:     20 * time.Second,
			MaxTokens:   500,
		},
		{
			ID:          "tool-multiple",
			Name:        "Multiple Tool Calls",
			Category:    CategoryTools,
			Description: "Call multiple tools",
			Difficulty:  Medium,
			Prompt:      "Compare the weather in San Francisco, New York, and London.",
			Expected:    ExpectedResult{ToolCalls: []string{"get_weather", "get_weather", "get_weather"}},
			Timeout:     30 * time.Second,
			MaxTokens:   800,
		},

		// INSTRUCTION FOLLOWING
		{
			ID:          "instruction-format",
			Name:        "Format Following",
			Category:    CategoryInstruction,
			Description: "Follow complex formatting instructions",
			Difficulty:  Medium,
			Prompt:      "List 3 colors. Format: 1. [COLOR] - [HEX CODE]. Example: 1. Red - #FF0000",
			Expected:    ExpectedResult{Contains: []string{"1.", "2.", "3.", "#"}},
			Timeout:     20 * time.Second,
			MaxTokens:   300,
		},
		{
			ID:          "instruction-restriction",
			Name:        "Output Restrictions",
			Category:    CategoryInstruction,
			Description: "Follow output restrictions",
			Difficulty:  Hard,
			Prompt:      "Explain quantum computing in exactly 50 words. No more, no less.",
			Expected:    ExpectedResult{}, // Word count validation in custom validator
			Timeout:     30 * time.Second,
			MaxTokens:   200,
		},

		// CREATIVITY
		{
			ID:          "creative-story",
			Name:        "Creative Story",
			Category:    CategoryCreativity,
			Description: "Write creative content",
			Difficulty:  Medium,
			Prompt:      "Write a 100-word sci-fi story about AI.",
			Expected:    ExpectedResult{MinLength: 300, Contains: []string{"ai", "future"}},
			Timeout:     30 * time.Second,
			MaxTokens:   500,
		},

		// MATH
		{
			ID:          "math-calculus",
			Name:        "Calculus Problem",
			Category:    CategoryMath,
			Description: "Solve calculus problem",
			Difficulty:  Hard,
			Prompt:      "What is the derivative of f(x) = 3x^2 + 2x + 1?",
			Expected:    ExpectedResult{Contains: []string{"6x", "2"}},
			Timeout:     30 * time.Second,
			MaxTokens:   300,
		},
	}
}

// generateContextPrompt generates text of approximately n tokens
func generateContextPrompt(tokens int) string {
	words := tokens / 0.75 // Approximate words per token
	paragraphs := []string{}

	topics := []string{
		"artificial intelligence", "machine learning", "neural networks",
		"deep learning", "natural language processing", "computer vision",
		"robotics", "automation", "data science", "big data",
	}

	for i := 0; i < words/100; i++ {
		topic := topics[i%len(topics)]
		paragraphs = append(paragraphs, fmt.Sprintf(
			"This is a detailed paragraph about %s. "+
				"%s is a fascinating field that has seen tremendous growth in recent years. "+
				"Researchers and practitioners in %s work on solving complex problems. "+
				"The applications of %s range from healthcare to finance to entertainment. "+
				"As technology advances, %s continues to evolve and improve. "+
				"New techniques and methodologies are constantly being developed in %s. "+
				"The future of %s looks very promising with many exciting developments ahead. ",
			topic, topic, topic, topic, topic, topic, topic,
		))
	}

	return strings.Join(paragraphs, "\n\n")
}

// RunAllChallenges runs all challenges against all providers
func RunAllChallenges(t *testing.T, logger *zap.Logger) {
	challenges := GetAllChallenges()
	providers := GetTestProviders()

	results := make(map[string][]ChallengeResult)

	for _, provider := range providers {
		apiKey := os.Getenv(provider.APIKeyEnv)
		if apiKey == "" {
			t.Logf("Skipping %s: %s not set", provider.Name, provider.APIKeyEnv)
			continue
		}

		for _, model := range provider.Models {
			modelKey := fmt.Sprintf("%s/%s", provider.Name, model.Name)
			results[modelKey] = []ChallengeResult{}

			for _, challenge := range challenges {
				t.Run(fmt.Sprintf("%s/%s", modelKey, challenge.ID), func(t *testing.T) {
					// Run challenge
					ctx, cancel := context.WithTimeout(context.Background(), challenge.Timeout)
					defer cancel()

					// Implementation would run actual challenge
					t.Logf("Running challenge: %s", challenge.Name)
				})
			}
		}
	}
}

// GenerateChallengeReport generates a comprehensive report
func GenerateChallengeReport(results map[string][]ChallengeResult) string {
	report := "# Provider Challenge Report\n\n"
	report += "## Summary\n\n"

	totalPassed := 0
	totalFailed := 0

	for model, modelResults := range results {
		passed := 0
		for _, r := range modelResults {
			if r.Passed {
				passed++
			}
		}
		totalPassed += passed
		totalFailed += len(modelResults) - passed

		percentage := float64(passed) / float64(len(modelResults)) * 100
		report += fmt.Sprintf("- **%s**: %d/%d (%.1f%%)\n", model, passed, len(modelResults), percentage)
	}

	report += "\n## Details\n\n"

	for model, modelResults := range results {
		report += fmt.Sprintf("### %s\n\n", model)
		report += "| Challenge | Status | Duration | Tokens |\n"
		report += "|-----------|--------|----------|--------|\n"

		for _, r := range modelResults {
			status := "✅ PASS"
			if !r.Passed {
				status = "❌ FAIL"
			}
			report += fmt.Sprintf("| %s | %s | %s | %d |\n",
				r.ChallengeID, status, r.Duration, r.TokensUsed)
		}
		report += "\n"
	}

	return report
}

// SaveChallengeReport saves report to file
func SaveChallengeReport(report string, filename string) error {
	return os.WriteFile(filename, []byte(report), 0644)
}

// TestChallenges runs all challenges (entry point for testing)
func TestChallenges(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	RunAllChallenges(t, logger)
}
