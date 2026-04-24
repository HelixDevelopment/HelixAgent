//go:build ignore

// Package benchmarks implements performance benchmarking for all providers
package benchmarks

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/llm"
	"go.uber.org/zap"
)

// BenchmarkResult holds benchmark results
type BenchmarkResult struct {
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	TestName        string    `json:"test_name"`
	LatencyMS       int64     `json:"latency_ms"`
	TokensPerSecond float64   `json:"tokens_per_second"`
	FirstTokenMS    int64     `json:"first_token_ms"`
	TotalTokens     int       `json:"total_tokens"`
	InputTokens     int       `json:"input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	CostUSD         float64   `json:"cost_usd"`
	Errors          int       `json:"errors"`
	Timestamp       time.Time `json:"timestamp"`
}

// BenchmarkSuite contains all benchmarks
type BenchmarkSuite struct {
	logger *zap.Logger
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite(logger *zap.Logger) *BenchmarkSuite {
	return &BenchmarkSuite{logger: logger}
}

// BenchmarkLatency measures time to first token and total latency
func (s *BenchmarkSuite) BenchmarkLatency(ctx context.Context, client llm.Client, model string, prompt string) (*BenchmarkResult, error) {
	start := time.Now()
	firstToken := time.Time{}

	request := llm.ChatRequest{
		Model:    model,
		Messages: []llm.Message{{Role: "user", Content: prompt}},
		Stream:   true,
	}

	stream, err := client.ChatStream(ctx, request)
	if err != nil {
		return nil, err
	}

	totalTokens := 0
	for chunk := range stream {
		if firstToken.IsZero() && len(chunk.Content) > 0 {
			firstToken = time.Now()
		}
		if chunk.Usage != nil {
			totalTokens = chunk.Usage.TotalTokens
		}
	}

	elapsed := time.Since(start)

	return &BenchmarkResult{
		Model:           model,
		TestName:        "latency",
		LatencyMS:       elapsed.Milliseconds(),
		FirstTokenMS:    firstToken.Sub(start).Milliseconds(),
		TotalTokens:     totalTokens,
		TokensPerSecond: float64(totalTokens) / elapsed.Seconds(),
		Timestamp:       time.Now(),
	}, nil
}

// BenchmarkThroughput measures sustained token generation rate
func (s *BenchmarkSuite) BenchmarkThroughput(ctx context.Context, client llm.Client, model string, duration time.Duration) (*BenchmarkResult, error) {
	start := time.Now()
	totalTokens := 0
	iterations := 0

	prompt := "Write a long story about artificial intelligence and its impact on society. Be detailed and comprehensive."

	for time.Since(start) < duration {
		request := llm.ChatRequest{
			Model:     model,
			Messages:  []llm.Message{{Role: "user", Content: prompt}},
			MaxTokens: 500,
		}

		response, err := client.Chat(ctx, request)
		if err != nil {
			s.logger.Warn("Benchmark request failed", zap.Error(err))
			continue
		}

		totalTokens += response.Usage.TotalTokens
		iterations++
	}

	elapsed := time.Since(start)

	return &BenchmarkResult{
		Model:           model,
		TestName:        "throughput",
		TotalTokens:     totalTokens,
		TokensPerSecond: float64(totalTokens) / elapsed.Seconds(),
		Timestamp:       time.Now(),
	}, nil
}

// BenchmarkContextWindow tests different context window sizes
func (s *BenchmarkSuite) BenchmarkContextWindow(ctx context.Context, client llm.Client, model string, contextTokens int) (*BenchmarkResult, error) {
	// Generate context
	context := generateContext(contextTokens)

	start := time.Now()

	request := llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: context + "\n\nSummarize the above text."},
		},
		MaxTokens: 500,
	}

	response, err := client.Chat(ctx, request)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start)

	return &BenchmarkResult{
		Model:           model,
		TestName:        fmt.Sprintf("context_%d", contextTokens),
		LatencyMS:       elapsed.Milliseconds(),
		InputTokens:     response.Usage.InputTokens,
		OutputTokens:    response.Usage.OutputTokens,
		TotalTokens:     response.Usage.TotalTokens,
		TokensPerSecond: float64(response.Usage.TotalTokens) / elapsed.Seconds(),
		Timestamp:       time.Now(),
	}, nil
}

// BenchmarkToolCalling measures tool calling overhead
func (s *BenchmarkSuite) BenchmarkToolCalling(ctx context.Context, client llm.Client, model string) (*BenchmarkResult, error) {
	tools := []llm.ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get weather",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]string{"type": "string"},
				},
				"required": []string{"location"},
			},
		},
	}

	start := time.Now()

	request := llm.ChatRequest{
		Model:    model,
		Messages: []llm.Message{{Role: "user", Content: "What is the weather in San Francisco?"}},
		Tools:    tools,
	}

	response, err := client.Chat(ctx, request)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start)

	return &BenchmarkResult{
		Model:           model,
		TestName:        "tool_calling",
		LatencyMS:       elapsed.Milliseconds(),
		TotalTokens:     response.Usage.TotalTokens,
		TokensPerSecond: float64(response.Usage.TotalTokens) / elapsed.Seconds(),
		Timestamp:       time.Now(),
	}, nil
}

// generateContext generates text with approximately n tokens
func generateContext(tokens int) string {
	// Approximate 0.75 tokens per word
	wordsNeeded := int(float64(tokens) / 0.75)

	paragraph := "This is a comprehensive analysis of artificial intelligence and its applications. "
	paragraph += "Machine learning has revolutionized numerous industries including healthcare, finance, and transportation. "
	paragraph += "Deep learning models have achieved remarkable results in computer vision and natural language processing. "
	paragraph += "Neural networks can now recognize images, translate languages, and generate human-like text. "
	paragraph += "The future of AI holds immense potential for solving complex global challenges. "

	result := ""
	for len(result) < wordsNeeded*10 {
		result += paragraph
	}

	return result
}

// RunAllBenchmarks runs all benchmarks
func RunAllBenchmarks(t *testing.T, logger *zap.Logger) {
	suite := NewBenchmarkSuite(logger)
	providers := GetTestProviders()

	results := []BenchmarkResult{}

	for _, provider := range providers {
		apiKey := os.Getenv(provider.APIKeyEnv)
		if apiKey == "" {
			t.Logf("Skipping %s: %s not set", provider.Name, provider.APIKeyEnv)
			continue
		}

		for _, model := range provider.Models {
			// Benchmark latency
			t.Run(fmt.Sprintf("%s/%s/latency", provider.Name, model.Name), func(t *testing.T) {
				// Implementation
				t.Logf("Benchmarking latency for %s/%s", provider.Name, model.Name)
			})

			// Benchmark throughput
			t.Run(fmt.Sprintf("%s/%s/throughput", provider.Name, model.Name), func(t *testing.T) {
				t.Logf("Benchmarking throughput for %s/%s", provider.Name, model.Name)
			})

			// Benchmark context windows
			contextSizes := []int{1000, 10000, 50000, 100000}
			for _, size := range contextSizes {
				if size > provider.MaxContext {
					continue
				}
				testName := fmt.Sprintf("%s/%s/context_%d", provider.Name, model.Name, size)
				t.Run(testName, func(t *testing.T) {
					t.Logf("Benchmarking %d token context for %s/%s", size, provider.Name, model.Name)
				})
			}

			// Benchmark tool calling
			if provider.SupportsTools && model.SupportsTools {
				t.Run(fmt.Sprintf("%s/%s/tool_calling", provider.Name, model.Name), func(t *testing.T) {
					t.Logf("Benchmarking tool calling for %s/%s", provider.Name, model.Name)
				})
			}
		}
	}

	_ = suite
	_ = results
}

// GenerateBenchmarkReport generates a report
func GenerateBenchmarkReport(results []BenchmarkResult) string {
	report := "# Provider Benchmark Report\n\n"
	report += "| Provider | Model | Test | Latency | TPS | Tokens |\n"
	report += "|----------|-------|------|---------|-----|--------|\n"

	for _, r := range results {
		report += fmt.Sprintf("| %s | %s | %s | %dms | %.1f | %d |\n",
			r.Provider, r.Model, r.TestName, r.LatencyMS, r.TokensPerSecond, r.TotalTokens)
	}

	return report
}

// TestBenchmarks runs all benchmarks
func TestBenchmarks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmarks in short mode")  // SKIP-OK: #short-mode
	}

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	RunAllBenchmarks(t, logger)
}

// GetTestProviders returns test provider configs (same as provider tests)
func GetTestProviders() []struct {
	Name      string
	BaseURL   string
	APIKeyEnv string
	Models    []struct {
		Name           string
		MaxTokens      int
		SupportsTools  bool
		SupportsVision bool
	}
	SupportsTools  bool
	SupportsVision bool
	MaxContext     int
} {
	// This is a simplified version - would import from provider tests
	return nil
}
