// Package sglang provides HTTP client for the SGLang service.
// SGLang provides RadixAttention for efficient prefix caching.
package sglang

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client is an HTTP client for the SGLang service.
type Client struct {
	baseURL    string
	httpClient *http.Client

	// Session management
	mu       sync.RWMutex
	sessions map[string]*Session
}

// Session represents a multi-turn conversation session.
type Session struct {
	ID           string
	SystemPrompt string
	History      []Message
	CreatedAt    time.Time
	LastUsedAt   time.Time
}

// Message represents a conversation message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClientConfig holds configuration for the SGLang client.
type ClientConfig struct {
	BaseURL string
	Timeout time.Duration
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL: "http://localhost:30000",
		Timeout: 120 * time.Second,
	}
}

// NewClient creates a new SGLang client.
func NewClient(config *ClientConfig) *Client {
	if config == nil {
		config = DefaultConfig()
	}
	return &Client{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		sessions: make(map[string]*Session),
	}
}

// CompletionRequest represents a completion request.
type CompletionRequest struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// CompletionChoice represents a completion choice.
type CompletionChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CompletionResponse represents a completion response.
type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   Usage              `json:"usage"`
}

// PrefixCacheRequest represents a prefix cache warming request.
type PrefixCacheRequest struct {
	Prefix   string `json:"prefix"`
	Priority int    `json:"priority,omitempty"` // Higher = more important
}

// PrefixCacheResponse represents prefix cache status.
type PrefixCacheResponse struct {
	Cached    bool   `json:"cached"`
	CacheID   string `json:"cache_id,omitempty"`
	TokenSize int    `json:"token_size,omitempty"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	Model     string `json:"model,omitempty"`
	GPUMemory string `json:"gpu_memory,omitempty"`
}

// Health checks the service health.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/health", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// SGLang might return plain text
		return &HealthResponse{Status: "healthy"}, nil
	}
	return &result, nil
}

// Complete performs a chat completion.
func (c *Client) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 500
	}

	resp, err := c.doRequest(ctx, "POST", "/v1/chat/completions", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// CompleteSimple performs a simple completion with a single prompt.
func (c *Client) CompleteSimple(ctx context.Context, prompt string) (string, error) {
	result, err := c.Complete(ctx, &CompletionRequest{
		Messages: []Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no completion generated")
}

// CompleteWithSystem performs a completion with a system prompt.
func (c *Client) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	result, err := c.Complete(ctx, &CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return "", err
	}
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no completion generated")
}

// Session management for prefix caching

// CreateSession creates a new conversation session.
func (c *Client) CreateSession(ctx context.Context, sessionID, systemPrompt string) (*Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	session := &Session{
		ID:           sessionID,
		SystemPrompt: systemPrompt,
		History:      []Message{},
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
	}

	// Pre-warm the prefix cache with the system prompt
	if systemPrompt != "" {
		_, _ = c.WarmPrefix(ctx, systemPrompt)
	}

	c.sessions[sessionID] = session
	return session, nil
}

// GetSession retrieves a session by ID.
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, ok := c.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// ContinueSession continues a conversation in an existing session.
func (c *Client) ContinueSession(ctx context.Context, sessionID, userMessage string) (string, error) {
	c.mu.Lock()
	session, ok := c.sessions[sessionID]
	if !ok {
		c.mu.Unlock()
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	// Build messages with system prompt and history
	messages := []Message{}
	if session.SystemPrompt != "" {
		messages = append(messages, Message{Role: "system", Content: session.SystemPrompt})
	}
	messages = append(messages, session.History...)
	messages = append(messages, Message{Role: "user", Content: userMessage})

	c.mu.Unlock()

	// Perform completion
	result, err := c.Complete(ctx, &CompletionRequest{
		Messages: messages,
	})
	if err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no completion generated")
	}

	assistantMessage := result.Choices[0].Message.Content

	// Update session history
	c.mu.Lock()
	session.History = append(session.History, Message{Role: "user", Content: userMessage})
	session.History = append(session.History, Message{Role: "assistant", Content: assistantMessage})
	session.LastUsedAt = time.Now()
	c.mu.Unlock()

	return assistantMessage, nil
}

// DeleteSession deletes a session.
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.sessions, sessionID)
	return nil
}

// ListSessions returns all active sessions.
func (c *Client) ListSessions(ctx context.Context) []*Session {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sessions := make([]*Session, 0, len(c.sessions))
	for _, session := range c.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// WarmPrefix pre-warms the prefix cache with common prefixes.
func (c *Client) WarmPrefix(ctx context.Context, prefix string) (*PrefixCacheResponse, error) {
	// SGLang automatically handles prefix caching, so we just make a completion
	// with the prefix to warm the cache
	_, err := c.Complete(ctx, &CompletionRequest{
		Messages:  []Message{{Role: "system", Content: prefix}},
		MaxTokens: 1, // Minimal generation to just cache the prefix
	})
	if err != nil {
		return nil, err
	}

	return &PrefixCacheResponse{
		Cached:    true,
		TokenSize: len(prefix) / 4, // Rough estimate
	}, nil
}

// WarmPrefixes warms multiple prefixes in parallel.
func (c *Client) WarmPrefixes(ctx context.Context, prefixes []string) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(prefixes))

	for _, prefix := range prefixes {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			if _, err := c.WarmPrefix(ctx, p); err != nil {
				errors <- err
			}
		}(prefix)
	}

	wg.Wait()
	close(errors)

	// Return first error if any
	for err := range errors {
		return err
	}
	return nil
}

// CleanupSessions removes stale sessions.
func (c *Client) CleanupSessions(ctx context.Context, maxAge time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, session := range c.sessions {
		if session.LastUsedAt.Before(cutoff) {
			delete(c.sessions, id)
			removed++
		}
	}

	return removed
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

// IsAvailable checks if the service is available.
func (c *Client) IsAvailable(ctx context.Context) bool {
	health, err := c.Health(ctx)
	return err == nil && health.Status == "healthy"
}
