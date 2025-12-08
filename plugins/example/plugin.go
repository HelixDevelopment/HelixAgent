package main

import (
	"context"
	"fmt"
	"time"

	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/plugins"
)

// Plugin is the exported plugin instance
var Plugin plugins.LLMPlugin = &ExamplePlugin{}

// ExamplePlugin demonstrates a basic LLM provider plugin
type ExamplePlugin struct {
	config map[string]interface{}
}

func (p *ExamplePlugin) Name() string {
	return "example"
}

func (p *ExamplePlugin) Version() string {
	return "1.0.0"
}

func (p *ExamplePlugin) Capabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:         []string{"example-model"},
		SupportedFeatures:       []string{"streaming"},
		SupportedRequestTypes:   []string{"code_generation", "reasoning"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          false,
		Limits: models.ModelLimits{
			MaxTokens:             4096,
			MaxInputLength:        2048,
			MaxOutputLength:       2048,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"author":  "SuperAgent Team",
			"license": "MIT",
		},
	}
}

func (p *ExamplePlugin) Init(config map[string]interface{}) error {
	p.config = config
	fmt.Printf("Example plugin initialized with config: %v\n", config)
	return nil
}

func (p *ExamplePlugin) Shutdown(ctx context.Context) error {
	fmt.Println("Example plugin shutting down")
	return nil
}

func (p *ExamplePlugin) HealthCheck(ctx context.Context) error {
	// Simulate health check
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (p *ExamplePlugin) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	// Simulate LLM processing
	time.Sleep(500 * time.Millisecond)

	response := &models.LLMResponse{
		RequestID:      req.ID,
		ProviderID:     "example",
		ProviderName:   "example",
		Content:        fmt.Sprintf("Example response to: %s", req.Prompt),
		Confidence:     0.8,
		TokensUsed:     150,
		ResponseTime:   500,
		FinishReason:   "stop",
		Selected:       false,
		SelectionScore: 0.0,
		CreatedAt:      time.Now(),
	}

	return response, nil
}

func (p *ExamplePlugin) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse)

	go func() {
		defer close(ch)

		// Simulate streaming response
		words := []string{"This", "is", "an", "example", "streaming", "response"}
		for i, word := range words {
			select {
			case <-ctx.Done():
				return
			default:
				response := &models.LLMResponse{
					RequestID:      req.ID,
					ProviderID:     "example",
					ProviderName:   "example",
					Content:        word + " ",
					Confidence:     0.8,
					TokensUsed:     i + 1,
					ResponseTime:   int64((i + 1) * 100),
					FinishReason:   "",
					Selected:       false,
					SelectionScore: 0.0,
					CreatedAt:      time.Now(),
				}
				ch <- response
				time.Sleep(100 * time.Millisecond)
			}
		}

		// Final response
		final := &models.LLMResponse{
			RequestID:      req.ID,
			ProviderID:     "example",
			ProviderName:   "example",
			Content:        "",
			Confidence:     0.8,
			TokensUsed:     len(words),
			ResponseTime:   int64(len(words) * 100),
			FinishReason:   "stop",
			Selected:       false,
			SelectionScore: 0.0,
			CreatedAt:      time.Now(),
		}
		ch <- final
	}()

	return ch, nil
}
