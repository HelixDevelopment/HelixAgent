// Package search implements web search capabilities
// Ported from SearchForYou - LLM-powered web search
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// WebSearchProvider defines the web search provider interface
type WebSearchProvider interface {
	Search(ctx context.Context, query string, options WebSearchOptions) (*WebSearchResult, error)
	Name() string
}

// WebSearchOptions contains search configuration
type WebSearchOptions struct {
	Limit         int
	Timeout       time.Duration
	IncludeAnswer bool
	RecencyDays   int
	SafeSearch    bool
}

// WebSearchResult contains search results
type WebSearchResult struct {
	Query      string        `json:"query"`
	Answer     string        `json:"answer,omitempty"`
	Results    []SearchItem  `json:"results"`
	TotalCount int           `json:"total_count"`
	Provider   string        `json:"provider"`
	SearchTime time.Duration `json:"search_time"`
}

// SearchItem represents a single search result
type SearchItem struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Snippet     string  `json:"snippet"`
	Content     string  `json:"content,omitempty"`
	PublishedAt string  `json:"published_at,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

// TavilyProvider implements Tavily AI search
type TavilyProvider struct {
	logger *zap.Logger
	apiKey string
	client *http.Client
}

// NewTavilyProvider creates a new Tavily search provider
func NewTavilyProvider(logger *zap.Logger, apiKey string) *TavilyProvider {
	return &TavilyProvider{
		logger: logger,
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (p *TavilyProvider) Name() string {
	return "tavily"
}

// Search performs a web search using Tavily
func (p *TavilyProvider) Search(ctx context.Context, query string, options WebSearchOptions) (*WebSearchResult, error) {
	start := time.Now()

	type request struct {
		APIKey        string `json:"api_key"`
		Query         string `json:"query"`
		SearchDepth   string `json:"search_depth,omitempty"`
		IncludeAnswer bool   `json:"include_answer,omitempty"`
		MaxResults    int    `json:"max_results,omitempty"`
		Days          int    `json:"days,omitempty"`
	}

	type response struct {
		Query   string `json:"query"`
		Answer  string `json:"answer,omitempty"`
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}

	reqBody := request{
		APIKey:        p.apiKey,
		Query:         query,
		SearchDepth:   "advanced",
		IncludeAnswer: options.IncludeAnswer,
		MaxResults:    options.Limit,
		Days:          options.RecencyDays,
	}

	if reqBody.MaxResults == 0 {
		reqBody.MaxResults = 5
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search API returned status %d", resp.StatusCode)
	}

	var searchResp response
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	results := make([]SearchItem, 0, len(searchResp.Results))
	for _, r := range searchResp.Results {
		results = append(results, SearchItem{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Content: r.Content,
			Score:   r.Score,
		})
	}

	p.logger.Info("Web search completed",
		zap.String("provider", "tavily"),
		zap.String("query", query),
		zap.Int("results", len(results)),
		zap.Duration("duration", time.Since(start)),
	)

	return &WebSearchResult{
		Query:      query,
		Answer:     searchResp.Answer,
		Results:    results,
		TotalCount: len(results),
		Provider:   "tavily",
		SearchTime: time.Since(start),
	}, nil
}

// PerplexityProvider implements Perplexity AI search
type PerplexityProvider struct {
	logger *zap.Logger
	apiKey string
	client *http.Client
}

// NewPerplexityProvider creates a new Perplexity search provider
func NewPerplexityProvider(logger *zap.Logger, apiKey string) *PerplexityProvider {
	return &PerplexityProvider{
		logger: logger,
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (p *PerplexityProvider) Name() string {
	return "perplexity"
}

// Search performs a web search using Perplexity
func (p *PerplexityProvider) Search(ctx context.Context, query string, options WebSearchOptions) (*WebSearchResult, error) {
	start := time.Now()

	type request struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	type response struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				Citations []struct {
					URL   string `json:"url"`
					Title string `json:"title"`
				} `json:"citations"`
			} `json:"message"`
		} `json:"choices"`
	}

	reqBody := request{
		Model: "sonar-pro",
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "user", Content: query},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.perplexity.ai/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search API returned status %d", resp.StatusCode)
	}

	var searchResp response
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResp.Choices) == 0 {
		return nil, fmt.Errorf("no results from Perplexity")
	}

	results := make([]SearchItem, 0)
	for _, citation := range searchResp.Choices[0].Message.Citations {
		results = append(results, SearchItem{
			Title: citation.Title,
			URL:   citation.URL,
		})
	}

	p.logger.Info("Web search completed",
		zap.String("provider", "perplexity"),
		zap.String("query", query),
		zap.Int("results", len(results)),
		zap.Duration("duration", time.Since(start)),
	)

	return &WebSearchResult{
		Query:      query,
		Answer:     searchResp.Choices[0].Message.Content,
		Results:    results,
		TotalCount: len(results),
		Provider:   "perplexity",
		SearchTime: time.Since(start),
	}, nil
}
