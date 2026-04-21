// Package ensemble_bench contains pure in-process benchmarks of the
// llm.RunEnsembleWithProviders primitive. These benchmarks measure
// semaphore/scheduler overhead with synthetic in-memory providers and do NOT
// require a running HelixAgent (CONST-030 unit-tier: mocks allowed).
package ensemble_bench

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

// benchMockProvider implements llm.LLMProvider for benchmark use.
type benchMockProvider struct {
	response *models.LLMResponse
	delay    time.Duration
}

func (m *benchMockProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.response, nil
}

func (m *benchMockProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	if m.response != nil {
		ch <- m.response
	}
	close(ch)
	return ch, nil
}

func (m *benchMockProvider) HealthCheck() error {
	return nil
}

func (m *benchMockProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{SupportsStreaming: true}
}

func (m *benchMockProvider) ValidateConfig(
	config map[string]interface{},
) (bool, []string) {
	return true, nil
}

// Ensure interface compliance.
var _ llm.LLMProvider = (*benchMockProvider)(nil)

// BenchmarkEnsemble_Semaphore_5Providers benchmarks ensemble execution with
// the weighted semaphore limiting 5 concurrent providers.
func BenchmarkEnsemble_Semaphore_5Providers(b *testing.B) {
	providers := make([]llm.LLMProvider, 5)
	for i := range providers {
		providers[i] = &benchMockProvider{
			response: &models.LLMResponse{
				Content:    "bench response",
				Confidence: 0.8,
			},
		}
	}

	req := &models.LLMRequest{Prompt: "benchmark"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = llm.RunEnsembleWithProviders(ctx, req, providers)
	}
}

// BenchmarkEnsemble_Semaphore_20Providers benchmarks ensemble execution with
// 20 providers to exercise the semaphore limiting path.
func BenchmarkEnsemble_Semaphore_20Providers(b *testing.B) {
	providers := make([]llm.LLMProvider, 20)
	for i := range providers {
		providers[i] = &benchMockProvider{
			response: &models.LLMResponse{
				Content:    "bench response",
				Confidence: float64(i) / 20.0,
			},
		}
	}

	req := &models.LLMRequest{Prompt: "benchmark"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = llm.RunEnsembleWithProviders(ctx, req, providers)
	}
}

// BenchmarkEnsemble_WithLatency benchmarks ensemble with simulated 1ms
// provider latency to measure real-world semaphore scheduling overhead.
func BenchmarkEnsemble_WithLatency(b *testing.B) {
	providers := make([]llm.LLMProvider, 5)
	for i := range providers {
		providers[i] = &benchMockProvider{
			delay: 1 * time.Millisecond,
			response: &models.LLMResponse{
				Content:    "bench response",
				Confidence: 0.9,
			},
		}
	}

	req := &models.LLMRequest{Prompt: "benchmark"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = llm.RunEnsembleWithProviders(ctx, req, providers)
	}
}
