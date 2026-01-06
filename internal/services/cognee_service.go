package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
)

// CogneeService provides comprehensive Cognee integration for LLM enhancement
type CogneeService struct {
	baseURL      string
	apiKey       string
	authToken    string // JWT token for Cognee API authentication
	client       *http.Client
	logger       *logrus.Logger
	config       *CogneeServiceConfig
	mu           sync.RWMutex
	isReady      bool
	stats        *CogneeStats
	feedbackLoop *FeedbackLoop
}

// CogneeServiceConfig holds all configuration for the Cognee service
type CogneeServiceConfig struct {
	// Core settings
	Enabled          bool          `json:"enabled"`
	BaseURL          string        `json:"base_url"`
	APIKey           string        `json:"api_key"`
	Timeout          time.Duration `json:"timeout"`
	AutoContainerize bool          `json:"auto_containerize"`

	// Authentication settings (for Cognee 0.5.0+)
	AuthEmail    string `json:"auth_email"`
	AuthPassword string `json:"auth_password"`

	// Memory enhancement settings
	AutoCognify            bool    `json:"auto_cognify"`
	EnhancePrompts         bool    `json:"enhance_prompts"`
	StoreResponses         bool    `json:"store_responses"`
	MaxContextSize         int     `json:"max_context_size"`
	RelevanceThreshold     float64 `json:"relevance_threshold"`
	TemporalAwareness      bool    `json:"temporal_awareness"`
	EnableFeedbackLoop     bool    `json:"enable_feedback_loop"`
	EnableGraphReasoning   bool    `json:"enable_graph_reasoning"`
	EnableCodeIntelligence bool    `json:"enable_code_intelligence"`

	// Search settings
	DefaultSearchLimit   int      `json:"default_search_limit"`
	DefaultDataset       string   `json:"default_dataset"`
	SearchTypes          []string `json:"search_types"` // CHUNKS, GRAPH_COMPLETION, RAG_COMPLETION, SUMMARIES
	CombineSearchResults bool     `json:"combine_search_results"`

	// Performance settings
	CacheEnabled    bool          `json:"cache_enabled"`
	CacheTTL        time.Duration `json:"cache_ttl"`
	MaxConcurrency  int           `json:"max_concurrency"`
	BatchSize       int           `json:"batch_size"`
	AsyncProcessing bool          `json:"async_processing"`
}

// CogneeStats tracks Cognee usage statistics
type CogneeStats struct {
	mu                     sync.RWMutex
	TotalMemoriesStored    int64
	TotalSearches          int64
	TotalCognifyOperations int64
	TotalInsightsQueries   int64
	TotalGraphCompletions  int64
	TotalCodeProcessed     int64
	TotalFeedbackReceived  int64
	AverageSearchLatency   time.Duration
	LastActivity           time.Time
	ErrorCount             int64
}

// FeedbackLoop manages the self-improvement feedback mechanism
type FeedbackLoop struct {
	mu        sync.RWMutex
	enabled   bool
	threshold float64
	history   []FeedbackEntry
}

// FeedbackEntry represents a single feedback interaction
type FeedbackEntry struct {
	QueryID      string    `json:"query_id"`
	Query        string    `json:"query"`
	Response     string    `json:"response"`
	Relevance    float64   `json:"relevance"`
	UserApproved bool      `json:"user_approved"`
	Timestamp    time.Time `json:"timestamp"`
}

// MemoryEntry represents a memory stored in Cognee
type MemoryEntry struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	ContentType string                 `json:"content_type"`
	Dataset     string                 `json:"dataset"`
	Metadata    map[string]interface{} `json:"metadata"`
	VectorID    string                 `json:"vector_id"`
	GraphNodes  []string               `json:"graph_nodes"`
	CreatedAt   time.Time              `json:"created_at"`
	Relevance   float64                `json:"relevance,omitempty"`
}

// EnhancedContext represents Cognee-enhanced context for LLM requests
type EnhancedContext struct {
	OriginalPrompt   string                   `json:"original_prompt"`
	EnhancedPrompt   string                   `json:"enhanced_prompt"`
	RelevantMemories []MemoryEntry            `json:"relevant_memories"`
	GraphInsights    []map[string]interface{} `json:"graph_insights"`
	TemporalContext  []TemporalEntry          `json:"temporal_context,omitempty"`
	CodeContext      []CodeContext            `json:"code_context,omitempty"`
	Confidence       float64                  `json:"confidence"`
	EnhancementType  string                   `json:"enhancement_type"`
}

// TemporalEntry represents time-aware context
type TemporalEntry struct {
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Relevance float64   `json:"relevance"`
}

// CodeContext represents code-related context
type CodeContext struct {
	Code        string                 `json:"code"`
	Language    string                 `json:"language"`
	Summary     string                 `json:"summary"`
	Entities    []string               `json:"entities"`
	Connections map[string]interface{} `json:"connections"`
}

// CogneeSearchResult represents a comprehensive search result
type CogneeSearchResult struct {
	Query            string                   `json:"query"`
	VectorResults    []MemoryEntry            `json:"vector_results"`
	GraphResults     []map[string]interface{} `json:"graph_results"`
	InsightsResults  []map[string]interface{} `json:"insights_results"`
	GraphCompletions []map[string]interface{} `json:"graph_completions"`
	CombinedContext  string                   `json:"combined_context"`
	TotalResults     int                      `json:"total_results"`
	SearchLatency    time.Duration            `json:"search_latency"`
	RelevanceScore   float64                  `json:"relevance_score"`
}

// NewCogneeService creates a new comprehensive Cognee service
func NewCogneeService(cfg *config.Config, logger *logrus.Logger) *CogneeService {
	if logger == nil {
		logger = logrus.New()
	}

	// Default auth credentials for Cognee (can be overridden via config)
	// Default: admin@superagent.ai / SuperAgentPass123 (as per CLAUDE.md)
	authEmail := os.Getenv("COGNEE_AUTH_EMAIL")
	if authEmail == "" {
		authEmail = "admin@superagent.ai"
	}
	authPassword := os.Getenv("COGNEE_AUTH_PASSWORD")
	if authPassword == "" {
		authPassword = "SuperAgentPass123"
	}

	serviceConfig := &CogneeServiceConfig{
		Enabled:                cfg.Cognee.Enabled,
		BaseURL:                cfg.Cognee.BaseURL,
		APIKey:                 cfg.Cognee.APIKey,
		Timeout:                cfg.Cognee.Timeout,
		AutoContainerize:       true,
		AuthEmail:              authEmail,
		AuthPassword:           authPassword,
		AutoCognify:            cfg.Cognee.AutoCognify,
		EnhancePrompts:         true,
		StoreResponses:         true,
		MaxContextSize:         4096,
		RelevanceThreshold:     0.7,
		TemporalAwareness:      true,
		EnableFeedbackLoop:     true,
		EnableGraphReasoning:   true,
		EnableCodeIntelligence: true,
		DefaultSearchLimit:     10,
		DefaultDataset:         "default",
		SearchTypes:            []string{"CHUNKS", "GRAPH_COMPLETION", "RAG_COMPLETION"},
		CombineSearchResults:   true,
		CacheEnabled:           true,
		CacheTTL:               30 * time.Minute,
		MaxConcurrency:         10,
		BatchSize:              50,
		AsyncProcessing:        true,
	}

	timeout := serviceConfig.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	service := &CogneeService{
		baseURL: serviceConfig.BaseURL,
		apiKey:  serviceConfig.APIKey,
		client: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
		config: serviceConfig,
		stats:  &CogneeStats{},
		feedbackLoop: &FeedbackLoop{
			enabled:   serviceConfig.EnableFeedbackLoop,
			threshold: 0.8,
			history:   make([]FeedbackEntry, 0),
		},
	}

	// Auto-containerize if enabled
	if serviceConfig.AutoContainerize && serviceConfig.Enabled {
		go func() {
			if err := service.EnsureRunning(context.Background()); err != nil {
				logger.WithError(err).Warn("Failed to auto-start Cognee containers")
			}
		}()
	}

	return service
}

// NewCogneeServiceWithConfig creates a Cognee service with explicit configuration
func NewCogneeServiceWithConfig(cfg *CogneeServiceConfig, logger *logrus.Logger) *CogneeService {
	if logger == nil {
		logger = logrus.New()
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &CogneeService{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
		config: cfg,
		stats:  &CogneeStats{},
		feedbackLoop: &FeedbackLoop{
			enabled:   cfg.EnableFeedbackLoop,
			threshold: 0.8,
			history:   make([]FeedbackEntry, 0),
		},
	}
}

// =====================================================
// CORE COGNEE OPERATIONS
// =====================================================

// EnsureRunning ensures Cognee is running, starting containers if needed
func (s *CogneeService) EnsureRunning(ctx context.Context) error {
	if s.IsHealthy(ctx) {
		s.mu.Lock()
		s.isReady = true
		s.mu.Unlock()

		// Ensure default dataset exists
		if err := s.EnsureDefaultDataset(ctx); err != nil {
			s.logger.WithError(err).Warn("Failed to ensure default dataset, searches may fail")
		}
		return nil
	}

	s.logger.Info("Cognee not running, attempting to start containers...")

	// Determine working directory - try current directory first, then known paths
	workDir, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(workDir, "docker-compose.yml")); err != nil {
		// Try known project paths
		knownPaths := []string{
			"/run/media/milosvasic/DATA4TB/Projects/HelixAgent",
			"/media/milosvasic/DATA4TB/Projects/HelixAgent",
		}
		for _, path := range knownPaths {
			if _, err := os.Stat(filepath.Join(path, "docker-compose.yml")); err == nil {
				workDir = path
				break
			}
		}
	}

	// Try container runtime in order: docker, podman
	var cmd *exec.Cmd
	var output []byte
	var err error

	// Try docker compose first
	if dockerPath, lookErr := exec.LookPath("docker"); lookErr == nil {
		cmd = exec.CommandContext(ctx, dockerPath, "compose", "--profile", "default", "up", "-d", "cognee", "chromadb", "postgres", "redis")
		cmd.Dir = workDir
		output, err = cmd.CombinedOutput()
		if err == nil {
			s.logger.Info("Started Cognee using docker compose")
			goto waitForHealth
		}

		// Try docker-compose fallback
		if dcPath, dcErr := exec.LookPath("docker-compose"); dcErr == nil {
			cmd = exec.CommandContext(ctx, dcPath, "--profile", "default", "up", "-d", "cognee", "chromadb", "postgres", "redis")
			cmd.Dir = workDir
			output, err = cmd.CombinedOutput()
			if err == nil {
				s.logger.Info("Started Cognee using docker-compose")
				goto waitForHealth
			}
		}
	}

	// Try podman-compose
	if pcPath, lookErr := exec.LookPath("podman-compose"); lookErr == nil {
		cmd = exec.CommandContext(ctx, pcPath, "--profile", "default", "up", "-d", "cognee", "chromadb", "postgres", "redis")
		cmd.Dir = workDir
		output, err = cmd.CombinedOutput()
		if err == nil {
			s.logger.Info("Started Cognee using podman-compose")
			goto waitForHealth
		}
	}

	// Try podman compose
	if podmanPath, lookErr := exec.LookPath("podman"); lookErr == nil {
		cmd = exec.CommandContext(ctx, podmanPath, "compose", "--profile", "default", "up", "-d", "cognee", "chromadb", "postgres", "redis")
		cmd.Dir = workDir
		output, err = cmd.CombinedOutput()
		if err == nil {
			s.logger.Info("Started Cognee using podman compose")
			goto waitForHealth
		}
	}

	// No container runtime found or all failed
	if err != nil {
		return fmt.Errorf("failed to start containers: %w, output: %s", err, string(output))
	}
	return fmt.Errorf("no container runtime found (tried docker, docker-compose, podman-compose, podman)")

waitForHealth:
	// Wait for services with exponential backoff
	maxWait := 60 * time.Second
	interval := 2 * time.Second
	start := time.Now()

	for time.Since(start) < maxWait {
		if s.IsHealthy(ctx) {
			s.mu.Lock()
			s.isReady = true
			s.mu.Unlock()
			s.logger.Info("Cognee services started successfully")

			// Ensure default dataset exists after startup
			if err := s.EnsureDefaultDataset(ctx); err != nil {
				s.logger.WithError(err).Warn("Failed to ensure default dataset, searches may fail")
			}
			return nil
		}
		time.Sleep(interval)
		doubled := interval * 2
		maxInterval := 10 * time.Second
		if doubled < maxInterval {
			interval = doubled
		} else {
			interval = maxInterval
		}
	}

	return fmt.Errorf("cognee services did not become healthy within %v", maxWait)
}

// IsHealthy checks if Cognee is healthy and responding
// Uses the root endpoint for faster response (the /health endpoint can be slow due to embedding tests)
func (s *CogneeService) IsHealthy(ctx context.Context) bool {
	// Use root endpoint which returns immediately with "Hello, World, I am alive!"
	url := fmt.Sprintf("%s/", s.baseURL)

	// Create a short-timeout client for health checks
	healthClient := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := healthClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Server is up, try to authenticate if we don't have a token
		s.mu.RLock()
		hasToken := s.authToken != ""
		s.mu.RUnlock()

		if !hasToken {
			// Try to get auth token in background
			go func() {
				if err := s.authenticate(context.Background()); err != nil {
					s.logger.WithError(err).Warn("Failed to authenticate with Cognee")
				}
			}()
		}
		return true
	}
	return false
}

// IsReady returns whether the service is ready
func (s *CogneeService) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isReady
}

// SetReady sets the ready state (for testing)
func (s *CogneeService) SetReady(ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isReady = ready
}

// =====================================================
// AUTHENTICATION OPERATIONS
// =====================================================

// authenticate registers and/or logs in to Cognee to get an auth token
func (s *CogneeService) authenticate(ctx context.Context) error {
	s.mu.Lock()
	if s.authToken != "" {
		s.mu.Unlock()
		return nil // Already authenticated
	}
	s.mu.Unlock()

	email := s.config.AuthEmail
	password := s.config.AuthPassword

	// Try to login first (in case user already exists)
	token, err := s.login(ctx, email, password)
	if err == nil {
		s.mu.Lock()
		s.authToken = token
		s.mu.Unlock()
		s.logger.Info("Authenticated with Cognee successfully")
		return nil
	}

	// Login failed, try to register then login
	s.logger.Debug("Login failed, attempting to register new user")
	if err := s.register(ctx, email, password); err != nil {
		// Registration might fail if user already exists, ignore
		s.logger.WithError(err).Debug("Registration failed (user may already exist)")
	}

	// Try login again
	token, err = s.login(ctx, email, password)
	if err != nil {
		return fmt.Errorf("failed to authenticate with Cognee: %w", err)
	}

	s.mu.Lock()
	s.authToken = token
	s.mu.Unlock()
	s.logger.Info("Authenticated with Cognee successfully after registration")
	return nil
}

// register creates a new user in Cognee
func (s *CogneeService) register(ctx context.Context, email, password string) error {
	url := fmt.Sprintf("%s/api/v1/auth/register", s.baseURL)

	reqBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// login authenticates with Cognee and returns the access token
func (s *CogneeService) login(ctx context.Context, email, password string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/auth/login", s.baseURL)

	// Use form-urlencoded for login as per Cognee's OAuth2 spec
	formData := fmt.Sprintf("username=%s&password=%s", email, password)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(formData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}

	return result.AccessToken, nil
}

// GetAuthToken returns the current auth token (for use by handlers)
func (s *CogneeService) GetAuthToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.authToken
}

// EnsureAuthenticated ensures we have a valid auth token
func (s *CogneeService) EnsureAuthenticated(ctx context.Context) error {
	s.mu.RLock()
	hasToken := s.authToken != ""
	s.mu.RUnlock()

	if hasToken {
		return nil
	}

	return s.authenticate(ctx)
}

// addAuthHeader adds the auth token to a request
func (s *CogneeService) addAuthHeader(req *http.Request) {
	s.mu.RLock()
	token := s.authToken
	s.mu.RUnlock()

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
}

// =====================================================
// MEMORY OPERATIONS
// =====================================================

// AddMemory stores content in Cognee's memory
func (s *CogneeService) AddMemory(ctx context.Context, content, dataset, contentType string, metadata map[string]interface{}) (*MemoryEntry, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("cognee service is disabled")
	}

	if dataset == "" {
		dataset = s.config.DefaultDataset
	}

	reqBody := map[string]interface{}{
		"content":      content,
		"dataset_name": dataset,
		"content_type": contentType,
		"metadata":     metadata,
	}

	if s.config.TemporalAwareness {
		reqBody["temporal_cognify"] = true
		reqBody["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/add", s.baseURL)
	resp, err := s.doRequest(ctx, "POST", url, data)
	if err != nil {
		s.stats.mu.Lock()
		s.stats.ErrorCount++
		s.stats.mu.Unlock()
		return nil, err
	}

	var result struct {
		ID         string   `json:"id"`
		VectorID   string   `json:"vector_id"`
		GraphNodes []string `json:"graph_nodes"`
		Status     string   `json:"status"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	s.stats.mu.Lock()
	s.stats.TotalMemoriesStored++
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()

	return &MemoryEntry{
		ID:          result.ID,
		Content:     content,
		ContentType: contentType,
		Dataset:     dataset,
		Metadata:    metadata,
		VectorID:    result.VectorID,
		GraphNodes:  result.GraphNodes,
		CreatedAt:   time.Now(),
	}, nil
}

// SearchMemory performs comprehensive memory search
func (s *CogneeService) SearchMemory(ctx context.Context, query string, dataset string, limit int) (*CogneeSearchResult, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("cognee service is disabled")
	}

	if dataset == "" {
		dataset = s.config.DefaultDataset
	}
	if limit == 0 {
		limit = s.config.DefaultSearchLimit
	}

	start := time.Now()
	result := &CogneeSearchResult{
		Query:            query,
		VectorResults:    make([]MemoryEntry, 0),
		GraphResults:     make([]map[string]interface{}, 0),
		InsightsResults:  make([]map[string]interface{}, 0),
		GraphCompletions: make([]map[string]interface{}, 0),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(s.config.SearchTypes))

	// Perform searches based on configured search types
	for _, searchType := range s.config.SearchTypes {
		wg.Add(1)
		go func(st string) {
			defer wg.Done()

			results, err := s.performSearch(ctx, query, dataset, limit, st)
			if err != nil {
				errChan <- err
				return
			}

			mu.Lock()
			defer mu.Unlock()

			switch st {
			case "CHUNKS", "CHUNKS_LEXICAL", "SUMMARIES":
				// Vector/chunk-based search results
				for _, r := range results {
					if entry, ok := r.(MemoryEntry); ok {
						result.VectorResults = append(result.VectorResults, entry)
					} else if m, ok := r.(map[string]interface{}); ok {
						// Convert map to MemoryEntry if possible
						entry := MemoryEntry{}
						if content, ok := m["content"].(string); ok {
							entry.Content = content
						}
						if id, ok := m["id"].(string); ok {
							entry.ID = id
						}
						if rel, ok := m["relevance"].(float64); ok {
							entry.Relevance = rel
						}
						result.VectorResults = append(result.VectorResults, entry)
					}
				}
			case "GRAPH_COMPLETION", "GRAPH_SUMMARY_COMPLETION", "GRAPH_COMPLETION_COT", "GRAPH_COMPLETION_CONTEXT_EXTENSION":
				// Graph-based completion results
				for _, r := range results {
					if m, ok := r.(map[string]interface{}); ok {
						result.GraphResults = append(result.GraphResults, m)
						result.GraphCompletions = append(result.GraphCompletions, m)
					}
				}
			case "RAG_COMPLETION", "TRIPLET_COMPLETION", "NATURAL_LANGUAGE", "FEELING_LUCKY":
				// RAG/Insights-style results
				for _, r := range results {
					if m, ok := r.(map[string]interface{}); ok {
						result.InsightsResults = append(result.InsightsResults, m)
					}
				}
			}
		}(searchType)
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	for err := range errChan {
		s.logger.WithError(err).Warn("Search error occurred")
	}

	// Calculate totals
	result.TotalResults = len(result.VectorResults) + len(result.GraphResults) +
		len(result.InsightsResults) + len(result.GraphCompletions)
	result.SearchLatency = time.Since(start)

	// Combine results into context if enabled
	if s.config.CombineSearchResults {
		result.CombinedContext = s.combineSearchResults(result)
		result.RelevanceScore = s.calculateRelevanceScore(result)
	}

	s.stats.mu.Lock()
	s.stats.TotalSearches++
	s.stats.LastActivity = time.Now()
	// Update average latency
	s.stats.AverageSearchLatency = (s.stats.AverageSearchLatency + result.SearchLatency) / 2
	s.stats.mu.Unlock()

	return result, nil
}

// performSearch executes a single search type with per-search timeout
func (s *CogneeService) performSearch(ctx context.Context, query, dataset string, limit int, searchType string) ([]interface{}, error) {
	// Apply per-search timeout (5 seconds) to prevent one slow search from blocking others
	searchTimeout := 5 * time.Second
	searchCtx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	reqBody := map[string]interface{}{
		"query":       query,
		"datasets":    []string{dataset},
		"limit":       limit,
		"search_type": searchType,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/search", s.baseURL)
	resp, err := s.doRequest(searchCtx, "POST", url, data)
	if err != nil {
		// Log but don't fail - return empty results on timeout
		s.logger.WithField("search_type", searchType).WithError(err).Warn("Search error occurred")
		return []interface{}{}, nil
	}

	var results struct {
		Results []interface{} `json:"results"`
	}
	if err := json.Unmarshal(resp, &results); err != nil {
		return nil, err
	}

	return results.Results, nil
}

// =====================================================
// LLM ENHANCEMENT OPERATIONS
// =====================================================

// EnhanceRequest enhances an LLM request with Cognee context
func (s *CogneeService) EnhanceRequest(ctx context.Context, req *models.LLMRequest) (*EnhancedContext, error) {
	if !s.config.Enabled || !s.config.EnhancePrompts {
		return &EnhancedContext{
			OriginalPrompt:  req.Prompt,
			EnhancedPrompt:  req.Prompt,
			EnhancementType: "none",
		}, nil
	}

	enhanced := &EnhancedContext{
		OriginalPrompt:   req.Prompt,
		RelevantMemories: make([]MemoryEntry, 0),
		GraphInsights:    make([]map[string]interface{}, 0),
		TemporalContext:  make([]TemporalEntry, 0),
		CodeContext:      make([]CodeContext, 0),
	}

	// Extract query from prompt or messages
	query := req.Prompt
	if len(req.Messages) > 0 {
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				query = req.Messages[i].Content
				break
			}
		}
	}

	// Search for relevant context
	searchResult, err := s.SearchMemory(ctx, query, s.config.DefaultDataset, s.config.DefaultSearchLimit)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to search memory for enhancement")
		enhanced.EnhancedPrompt = req.Prompt
		enhanced.EnhancementType = "failed"
		return enhanced, nil
	}

	// Populate enhanced context
	enhanced.RelevantMemories = searchResult.VectorResults
	enhanced.GraphInsights = searchResult.GraphResults
	enhanced.Confidence = searchResult.RelevanceScore

	// Build enhanced prompt
	enhanced.EnhancedPrompt = s.buildEnhancedPrompt(req.Prompt, searchResult)
	enhanced.EnhancementType = "full"

	// Add code context if applicable
	if s.config.EnableCodeIntelligence && containsCode(query) {
		codeContext, err := s.GetCodeContext(ctx, query)
		if err == nil {
			enhanced.CodeContext = codeContext
		}
	}

	return enhanced, nil
}

// ProcessResponse processes an LLM response through Cognee
func (s *CogneeService) ProcessResponse(ctx context.Context, req *models.LLMRequest, resp *models.LLMResponse) error {
	if !s.config.Enabled || !s.config.StoreResponses {
		return nil
	}

	// Store the conversation in memory
	conversationContent := fmt.Sprintf("User: %s\nAssistant: %s", req.Prompt, resp.Content)

	metadata := map[string]interface{}{
		"session_id":    req.SessionID,
		"user_id":       req.UserID,
		"provider":      resp.ProviderName,
		"model":         req.ModelParams.Model,
		"tokens_used":   resp.TokensUsed,
		"response_time": resp.ResponseTime,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}

	_, err := s.AddMemory(ctx, conversationContent, s.config.DefaultDataset, "conversation", metadata)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to store response in memory")
		return err
	}

	// Auto-cognify if enabled
	if s.config.AutoCognify {
		go func() {
			bgCtx := context.Background()
			if err := s.Cognify(bgCtx, []string{s.config.DefaultDataset}); err != nil {
				s.logger.WithError(err).Warn("Background cognify failed")
			}
		}()
	}

	return nil
}

// buildEnhancedPrompt creates an enhanced prompt with Cognee context
func (s *CogneeService) buildEnhancedPrompt(originalPrompt string, searchResult *CogneeSearchResult) string {
	if searchResult.TotalResults == 0 || searchResult.RelevanceScore < s.config.RelevanceThreshold {
		return originalPrompt
	}

	var contextParts []string

	// Add relevant memories
	if len(searchResult.VectorResults) > 0 {
		contextParts = append(contextParts, "## Relevant Context from Knowledge Base:")
		for i, mem := range searchResult.VectorResults {
			if i >= 5 {
				break
			}
			contextParts = append(contextParts, fmt.Sprintf("- %s", truncateText(mem.Content, 500)))
		}
	}

	// Add graph insights
	if len(searchResult.GraphResults) > 0 && s.config.EnableGraphReasoning {
		contextParts = append(contextParts, "\n## Knowledge Graph Insights:")
		for i, insight := range searchResult.GraphResults {
			if i >= 3 {
				break
			}
			if text, ok := insight["text"].(string); ok {
				contextParts = append(contextParts, fmt.Sprintf("- %s", truncateText(text, 300)))
			}
		}
	}

	if len(contextParts) == 0 {
		return originalPrompt
	}

	// Construct enhanced prompt
	enhancedPrompt := fmt.Sprintf(
		"%s\n\n---\n\n## User Query:\n%s",
		strings.Join(contextParts, "\n"),
		originalPrompt,
	)

	// Ensure we don't exceed max context size
	if len(enhancedPrompt) > s.config.MaxContextSize {
		enhancedPrompt = enhancedPrompt[:s.config.MaxContextSize]
	}

	return enhancedPrompt
}

// =====================================================
// COGNIFY OPERATIONS
// =====================================================

// Cognify processes data into knowledge graphs
func (s *CogneeService) Cognify(ctx context.Context, datasets []string) error {
	if !s.config.Enabled {
		return fmt.Errorf("cognee service is disabled")
	}

	if len(datasets) == 0 {
		datasets = []string{s.config.DefaultDataset}
	}

	reqBody := map[string]interface{}{
		"datasets": datasets,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/cognify", s.baseURL)
	_, err = s.doRequest(ctx, "POST", url, data)
	if err != nil {
		s.stats.mu.Lock()
		s.stats.ErrorCount++
		s.stats.mu.Unlock()
		return err
	}

	s.stats.mu.Lock()
	s.stats.TotalCognifyOperations++
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()

	return nil
}

// =====================================================
// INSIGHTS & GRAPH OPERATIONS
// =====================================================

// GetInsights retrieves insights using graph reasoning
func (s *CogneeService) GetInsights(ctx context.Context, query string, datasets []string, limit int) ([]map[string]interface{}, error) {
	if !s.config.Enabled || !s.config.EnableGraphReasoning {
		return nil, fmt.Errorf("cognee insights not enabled")
	}

	if len(datasets) == 0 {
		datasets = []string{s.config.DefaultDataset}
	}
	if limit == 0 {
		limit = s.config.DefaultSearchLimit
	}

	reqBody := map[string]interface{}{
		"query":       query,
		"datasets":    datasets,
		"limit":       limit,
		"search_type": "RAG_COMPLETION",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/search", s.baseURL)
	resp, err := s.doRequest(ctx, "POST", url, data)
	if err != nil {
		return nil, err
	}

	var result struct {
		Insights []map[string]interface{} `json:"insights"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	s.stats.mu.Lock()
	s.stats.TotalInsightsQueries++
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()

	return result.Insights, nil
}

// GetGraphCompletion performs LLM-powered graph completion
func (s *CogneeService) GetGraphCompletion(ctx context.Context, query string, datasets []string, limit int) ([]map[string]interface{}, error) {
	if !s.config.Enabled || !s.config.EnableGraphReasoning {
		return nil, fmt.Errorf("cognee graph completion not enabled")
	}

	if len(datasets) == 0 {
		datasets = []string{s.config.DefaultDataset}
	}
	if limit == 0 {
		limit = s.config.DefaultSearchLimit
	}

	reqBody := map[string]interface{}{
		"query":       query,
		"datasets":    datasets,
		"limit":       limit,
		"search_type": "GRAPH_COMPLETION",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/search", s.baseURL)
	resp, err := s.doRequest(ctx, "POST", url, data)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	s.stats.mu.Lock()
	s.stats.TotalGraphCompletions++
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()

	return result.Results, nil
}

// =====================================================
// CODE INTELLIGENCE OPERATIONS
// =====================================================

// ProcessCode indexes code through Cognee's code pipeline
func (s *CogneeService) ProcessCode(ctx context.Context, code, language, dataset string) (*CodeContext, error) {
	if !s.config.Enabled || !s.config.EnableCodeIntelligence {
		return nil, fmt.Errorf("cognee code intelligence not enabled")
	}

	if dataset == "" {
		dataset = s.config.DefaultDataset
	}

	reqBody := map[string]interface{}{
		"code":         code,
		"language":     language,
		"dataset_name": dataset,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/code-pipeline/index", s.baseURL)
	resp, err := s.doRequest(ctx, "POST", url, data)
	if err != nil {
		return nil, err
	}

	var result struct {
		Processed   bool                   `json:"processed"`
		Summary     string                 `json:"summary"`
		Entities    []string               `json:"entities"`
		Connections map[string]interface{} `json:"connections"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	s.stats.mu.Lock()
	s.stats.TotalCodeProcessed++
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()

	return &CodeContext{
		Code:        code,
		Language:    language,
		Summary:     result.Summary,
		Entities:    result.Entities,
		Connections: result.Connections,
	}, nil
}

// GetCodeContext retrieves code-related context
func (s *CogneeService) GetCodeContext(ctx context.Context, query string) ([]CodeContext, error) {
	if !s.config.EnableCodeIntelligence {
		return nil, nil
	}

	reqBody := map[string]interface{}{
		"query":       query,
		"search_type": "CODING_RULES",
		"limit":       5,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/search", s.baseURL)
	resp, err := s.doRequest(ctx, "POST", url, data)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []CodeContext `json:"results"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

// =====================================================
// DATASET MANAGEMENT
// =====================================================

// EnsureDefaultDataset creates the default dataset if it doesn't exist
// This prevents "No datasets found" errors during search operations
func (s *CogneeService) EnsureDefaultDataset(ctx context.Context) error {
	datasetName := s.config.DefaultDataset
	if datasetName == "" {
		datasetName = "default"
	}

	// Check if dataset exists
	datasets, err := s.ListDatasets(ctx)
	if err != nil {
		s.logger.WithError(err).Debug("Failed to list datasets, attempting to create default")
		// Continue to create - might be first time
	} else {
		// Check if default dataset exists
		for _, ds := range datasets {
			if name, ok := ds["name"].(string); ok && name == datasetName {
				s.logger.WithField("dataset", datasetName).Debug("Default dataset already exists")
				return nil
			}
		}
	}

	// Create default dataset
	s.logger.WithField("dataset", datasetName).Info("Creating default dataset for Cognee")
	err = s.CreateDataset(ctx, datasetName, "Default dataset for SuperAgent Cognee integration", map[string]interface{}{
		"created_by": "superagent",
		"auto_created": true,
		"created_at": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		// Check if it's a "already exists" type error
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "409") {
			s.logger.WithField("dataset", datasetName).Debug("Default dataset already exists (concurrent creation)")
			return nil
		}
		return fmt.Errorf("failed to create default dataset: %w", err)
	}

	s.logger.WithField("dataset", datasetName).Info("Default dataset created successfully")
	return nil
}

// CreateDataset creates a new dataset
func (s *CogneeService) CreateDataset(ctx context.Context, name, description string, metadata map[string]interface{}) error {
	reqBody := map[string]interface{}{
		"name":        name,
		"description": description,
		"metadata":    metadata,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/datasets", s.baseURL)
	_, err = s.doRequest(ctx, "POST", url, data)
	return err
}

// ListDatasets retrieves all datasets
func (s *CogneeService) ListDatasets(ctx context.Context) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/datasets", s.baseURL)
	resp, err := s.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Datasets []map[string]interface{} `json:"datasets"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Datasets, nil
}

// DeleteDataset removes a dataset
func (s *CogneeService) DeleteDataset(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/api/v1/datasets/%s", s.baseURL, name)
	_, err := s.doRequest(ctx, "DELETE", url, nil)
	return err
}

// =====================================================
// GRAPH VISUALIZATION
// =====================================================

// VisualizeGraph retrieves graph visualization data
func (s *CogneeService) VisualizeGraph(ctx context.Context, dataset, format string) (map[string]interface{}, error) {
	if dataset == "" {
		dataset = s.config.DefaultDataset
	}
	if format == "" {
		format = "json"
	}

	url := fmt.Sprintf("%s/api/v1/visualize?dataset=%s&format=%s", s.baseURL, dataset, format)
	resp, err := s.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// =====================================================
// FEEDBACK LOOP OPERATIONS
// =====================================================

// ProvideFeedback records user feedback for self-improvement
func (s *CogneeService) ProvideFeedback(ctx context.Context, queryID, query, response string, relevance float64, approved bool) error {
	if !s.config.EnableFeedbackLoop {
		return nil
	}

	entry := FeedbackEntry{
		QueryID:      queryID,
		Query:        query,
		Response:     response,
		Relevance:    relevance,
		UserApproved: approved,
		Timestamp:    time.Now(),
	}

	s.feedbackLoop.mu.Lock()
	s.feedbackLoop.history = append(s.feedbackLoop.history, entry)
	s.feedbackLoop.mu.Unlock()

	// Store feedback in Cognee for learning
	if approved {
		reqBody := map[string]interface{}{
			"query_id":         queryID,
			"query":            query,
			"response":         response,
			"relevance":        relevance,
			"approved":         approved,
			"save_interaction": true,
		}

		data, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}

		url := fmt.Sprintf("%s/api/v1/feedback", s.baseURL)
		_, err = s.doRequest(ctx, "POST", url, data)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to store feedback in Cognee")
		}
	}

	s.stats.mu.Lock()
	s.stats.TotalFeedbackReceived++
	s.stats.mu.Unlock()

	return nil
}

// =====================================================
// STATISTICS & MONITORING
// =====================================================

// GetStats returns current statistics
func (s *CogneeService) GetStats() *CogneeStats {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	return &CogneeStats{
		TotalMemoriesStored:    s.stats.TotalMemoriesStored,
		TotalSearches:          s.stats.TotalSearches,
		TotalCognifyOperations: s.stats.TotalCognifyOperations,
		TotalInsightsQueries:   s.stats.TotalInsightsQueries,
		TotalGraphCompletions:  s.stats.TotalGraphCompletions,
		TotalCodeProcessed:     s.stats.TotalCodeProcessed,
		TotalFeedbackReceived:  s.stats.TotalFeedbackReceived,
		AverageSearchLatency:   s.stats.AverageSearchLatency,
		LastActivity:           s.stats.LastActivity,
		ErrorCount:             s.stats.ErrorCount,
	}
}

// GetConfig returns the current configuration
func (s *CogneeService) GetConfig() *CogneeServiceConfig {
	return s.config
}

// =====================================================
// HELPER FUNCTIONS
// =====================================================

// doRequest performs an HTTP request to Cognee with automatic token refresh on 401
func (s *CogneeService) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	return s.doRequestWithRetry(ctx, method, url, body, true)
}

// doRequestWithRetry performs an HTTP request with optional retry on 401
func (s *CogneeService) doRequestWithRetry(ctx context.Context, method, url string, body []byte, allowRetry bool) ([]byte, error) {
	// Ensure we're authenticated before making API requests
	if err := s.EnsureAuthenticated(ctx); err != nil {
		s.logger.WithError(err).Warn("Failed to authenticate with Cognee, continuing anyway")
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	s.addAuthHeader(req) // Use auth token or API key

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle 401 Unauthorized - token may have expired
	if resp.StatusCode == http.StatusUnauthorized && allowRetry {
		s.logger.Info("Received 401 from Cognee, refreshing authentication token")

		// Clear the expired token
		s.clearAuthToken()

		// Re-authenticate
		if err := s.authenticate(ctx); err != nil {
			s.logger.WithError(err).Error("Failed to re-authenticate with Cognee")
			return nil, fmt.Errorf("cognee API error: %d - %s (re-auth failed: %v)", resp.StatusCode, string(respBody), err)
		}

		s.logger.Info("Successfully re-authenticated with Cognee, retrying request")

		// Retry the request once with new token
		return s.doRequestWithRetry(ctx, method, url, body, false)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("cognee API error: %d - %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// clearAuthToken clears the cached auth token to force re-authentication
func (s *CogneeService) clearAuthToken() {
	s.mu.Lock()
	s.authToken = ""
	s.mu.Unlock()
}

// combineSearchResults combines multiple search results into a single context string
func (s *CogneeService) combineSearchResults(result *CogneeSearchResult) string {
	var parts []string

	for _, mem := range result.VectorResults {
		parts = append(parts, mem.Content)
	}

	for _, insight := range result.InsightsResults {
		if text, ok := insight["text"].(string); ok {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, "\n\n")
}

// calculateRelevanceScore calculates overall relevance score
func (s *CogneeService) calculateRelevanceScore(result *CogneeSearchResult) float64 {
	if result.TotalResults == 0 {
		return 0
	}

	totalScore := 0.0
	count := 0

	for _, mem := range result.VectorResults {
		if mem.Relevance > 0 {
			totalScore += mem.Relevance
			count++
		}
	}

	if count == 0 {
		return 0.5 // Default score when no relevance info
	}

	return totalScore / float64(count)
}

// truncateText truncates text to a maximum length
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// containsCode checks if text likely contains code
func containsCode(text string) bool {
	codeIndicators := []string{
		"func ", "def ", "class ", "import ", "package ",
		"const ", "var ", "let ", "{", "}", "=>", "->",
		"public ", "private ", "return ", "if (", "for (",
	}
	for _, indicator := range codeIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}
	return false
}
