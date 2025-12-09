package llm

import (
	"sync"

	"github.com/superagent/superagent/internal/models"
)

// RunEnsemble executes a parallel ensemble of LLM providers and returns the aggregated responses
func RunEnsemble(req *models.LLMRequest) ([]*models.LLMResponse, *models.LLMResponse, error) {
	// Initialize providers with default configurations
	provs := []LLMProvider{
		NewDeepSeekProvider("", "", ""),
		NewClaudeProvider("", "", ""),
		NewGeminiProvider("", "", ""),
		&QwenProvider{},
		&ZaiProvider{},
	}

	var wg sync.WaitGroup
	respCh := make(chan *models.LLMResponse, len(provs))

	for _, p := range provs {
		pp := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := pp.Complete(req)
			if err == nil && r != nil {
				respCh <- r
			}
		}()
	}

	go func() {
		wg.Wait()
		close(respCh)
	}()

	var responses []*models.LLMResponse
	for r := range respCh {
		responses = append(responses, r)
	}

	// Choose the best by highest confidence if available
	var selected *models.LLMResponse
	max := -1.0
	for _, r := range responses {
		if r != nil && r.Confidence > max {
			max = r.Confidence
			selected = r
		}
	}
	if len(responses) == 0 {
		return nil, nil, nil
	}
	return responses, selected, nil
}
