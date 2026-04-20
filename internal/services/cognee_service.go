package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/adapters/containers"
	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/models"
	"github.com/sirupsen/logrus"
)

// CogneeService provides comprehensive Cognee integration for LLM enhancement
type CogneeService struct {
	baseURL           string
	apiKey            string
	authToken         string // JWT token for Cognee API authentication
	client            *http.Client
	logger            *logrus.Logger
	config            *CogneeServiceConfig
	ContainerAdapter  *containers.Adapter
	mu                sync.RWMutex
	isReady           bool
	stats             *CogneeStats
	feedbackLoop      *FeedbackLoop
	lastSearchWarning time.Time // Rate limit search warnings
	lastStoreWarning  time.Time // Rate limit store warnings
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

// FeedbackLoop manages the self-improvement feedback mechanism.
//
// Concurrency model (CONST-029): history → *safe.Slice. mu dropped.
type FeedbackLoop struct {
	enabled   bool
	threshold float64
	history   *safe.Slice[FeedbackEntry]
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

// findProjectRoot dynamically locates the HelixAgent project root
// by searching for docker-compose.yml starting from current directory and going up
// This ensures no hardcoded paths are needed
func findProjectRoot() string {
	// Start from current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Search upward from cwd for docker-compose.yml
	dir := cwd
	for {
		composePath := filepath.Join(dir, "docker-compose.yml")
		if _, err := os.Stat(composePath); err == nil {
			return dir
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, not found
			break
		}
		dir = parent
	}

	// Also check if executable path gives us a hint
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		// Check the executable's directory and parent directories
		dir := execDir
		for i := 0; i < 5; i++ { // Check up to 5 levels up
			composePath := filepath.Join(dir, "docker-compose.yml")
			if _, err := os.Stat(composePath); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return ""
}

// NewCogneeService creates a new comprehensive Cognee service
func NewCogneeService(cfg *config.Config, logger *logrus.Logger) *CogneeService {
	if logger == nil {
		logger = logrus.New()
	}

	// Default auth credentials for Cognee (can be overridden via config)
	// Default: admin@helixagent.ai / HelixAgentPass123 (as per CLAUDE.md)
	authEmail := os.Getenv("COGNEE_AUTH_EMAIL")
	if authEmail == "" {
		authEmail = "admin@helixagent.ai"
	}
	authPassword := os.Getenv("COGNEE_AUTH_PASSWORD")
	if authPassword == "" {
		authPassword = "HelixAgentPass123" // #nosec G101 - dev-only default for local Cognee container
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
			history:   safe.NewSlice[FeedbackEntry](),
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

	// Seed Cognee with initial data to prevent "empty knowledge graph" errors
	if serviceConfig.Enabled {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := service.SeedInitialData(ctx); err != nil {
				logger.WithError(err).Debug("Failed to seed Cognee with initial data (non-critical)")
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
			history:   safe.NewSlice[FeedbackEntry](),
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

	// Determine working directory dynamically - no hardcoded paths
	workDir := findProjectRoot()
	if workDir == "" {
		return fmt.Errorf("cannot find HelixAgent project root (docker-compose.yml not found)")
	}
	s.logger.WithField("workDir", workDir).Debug("Found project root for container startup")

	composeFile := filepath.Join(workDir, "docker-compose.yml")

	// Use Containers module adapter for compose up.
	if s.ContainerAdapter == nil {
		return fmt.Errorf("container adapter not initialized")
	}
	if err := s.ContainerAdapter.ComposeUp(ctx, composeFile, "default"); err != nil {
		return fmt.Errorf("failed to start Cognee containers: %w", err)
	}
	s.logger.Info("Started Cognee via Containers module adapter")

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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		// Server is up, try to authenticate if we don't have a token
		s.mu.RLock()
		hasToken := s.authToken != ""
		s.mu.RUnlock()

		if !hasToken {
			// Try to get auth token in background
			//nolint:gosec // G118: background Cognee auth retry, intentionally decoupled from the triggering request
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
	if regErr := s.register(ctx, email, password); regErr != nil {
		// Registration might fail if user already exists, ignore
		s.logger.WithError(regErr).Debug("Registration failed (user may already exist)")
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

	reqBody, _ := json.Marshal(map[string]string{ //nolint:errcheck
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("registration failed with status %d - failed to read response body: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// login authenticates with Cognee and returns the access token
func (s *CogneeService) login(ctx context.Context, email, password string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/auth/login", s.baseURL)

	// Use form-urlencoded for login as per Cognee's OAuth2 spec
	form := neturl.Values{}
	form.Set("username", email)
	form.Set("password", password)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("login failed with status %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
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

// AddMemory stores content in Cognee's memory using a two-phase approach:
// 1. Primary: Use the /api/v1/add endpoint (multipart form-data) which reliably stores data
// 2. Fallback: Try /api/v1/memify for graph enrichment (best-effort, may fail with non-OpenAI LLMs)
func (s *CogneeService) AddMemory(ctx context.Context, content, dataset, contentType string, metadata map[string]interface{}) (*MemoryEntry, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("cognee service is disabled")
	}

	if dataset == "" {
		dataset = s.config.DefaultDataset
	}

	// Phase 1: Store data using the reliable /api/v1/add endpoint (multipart form-data)
	entry, err := s.addMemoryViaAdd(ctx, content, dataset, contentType, metadata)
	if err != nil {
		// Fallback to memify endpoint
		logrus.WithError(err).Debug("Add endpoint failed, trying memify fallback")
		return s.addMemoryViaMemify(ctx, content, dataset, contentType, metadata)
	}

	// Phase 2: Best-effort enrichment via memify (non-blocking)
	//nolint:gosec // G118: best-effort enrichment with its own 30s timeout, intentionally decoupled from the caller context
	go func() {
		enrichCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.enrichViaMemify(enrichCtx, content, dataset); err != nil {
			logrus.WithError(err).Debug("Memify enrichment failed (non-critical)")
		}
	}()

	return entry, nil
}

// addMemoryViaAdd stores content using Cognee's /api/v1/add endpoint with multipart form-data
func (s *CogneeService) addMemoryViaAdd(ctx context.Context, content, dataset, contentType string, metadata map[string]interface{}) (*MemoryEntry, error) {
	// Build multipart form body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the text content as a file upload
	part, err := writer.CreateFormFile("data", "memory.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, writeErr := part.Write([]byte(content)); writeErr != nil {
		return nil, fmt.Errorf("failed to write content: %w", writeErr)
	}

	// Add dataset name
	if fieldErr := writer.WriteField("datasetName", dataset); fieldErr != nil {
		return nil, fmt.Errorf("failed to write dataset field: %w", fieldErr)
	}

	if closeErr := writer.Close(); closeErr != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", closeErr)
	}

	url := fmt.Sprintf("%s/api/v1/add", s.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	s.addAuthHeader(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("add request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("add endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response to extract IDs
	var addResult struct {
		Status        string `json:"status"`
		PipelineRunID string `json:"pipeline_run_id"`
		DatasetID     string `json:"dataset_id"`
		DatasetName   string `json:"dataset_name"`
	}
	if err := json.Unmarshal(body, &addResult); err != nil {
		// Response may be in a different format, just use what we have
		logrus.WithError(err).Debug("Failed to parse add response, using raw")
	}

	s.stats.mu.Lock()
	s.stats.TotalMemoriesStored++
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()

	return &MemoryEntry{
		ID:          addResult.DatasetID,
		Content:     content,
		ContentType: contentType,
		Dataset:     dataset,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}, nil
}

// addMemoryViaMemify stores content using Cognee's /api/v1/memify endpoint (JSON)
func (s *CogneeService) addMemoryViaMemify(ctx context.Context, content, dataset, contentType string, metadata map[string]interface{}) (*MemoryEntry, error) {
	reqBody := map[string]interface{}{
		"data":        content,
		"datasetName": dataset,
	}

	if s.config.TemporalAwareness {
		reqBody["runInBackground"] = false
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/memify", s.baseURL)
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

// enrichViaMemify runs the memify enrichment pipeline on content (best-effort)
func (s *CogneeService) enrichViaMemify(ctx context.Context, content, dataset string) error {
	reqBody := map[string]interface{}{
		"data":        content,
		"datasetName": dataset,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/memify", s.baseURL)
	_, err = s.doRequest(ctx, "POST", url, data)
	return err
}

// SearchMemory performs comprehensive memory search
func (s *CogneeService) SearchMemory(ctx context.Context, query string, dataset string, limit int) (*CogneeSearchResult, error) {
	// Return empty results when disabled (not an error - just no data)
	if !s.config.Enabled {
		s.logger.Debug("Cognee disabled, returning empty search results")
		return &CogneeSearchResult{
			Query:            query,
			VectorResults:    make([]MemoryEntry, 0),
			GraphResults:     make([]map[string]interface{}, 0),
			InsightsResults:  make([]map[string]interface{}, 0),
			GraphCompletions: make([]map[string]interface{}, 0),
		}, nil
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

	// Collect any errors (rate-limit warnings to once per 30 seconds to avoid spam)
	var searchErrors []error
	for err := range errChan {
		searchErrors = append(searchErrors, err)
	}
	if len(searchErrors) > 0 {
		s.mu.Lock()
		now := time.Now()
		shouldLog := s.lastSearchWarning.IsZero() || now.Sub(s.lastSearchWarning) > 30*time.Second
		if shouldLog {
			s.lastSearchWarning = now
			s.mu.Unlock()
			s.logger.WithField("error_count", len(searchErrors)).Warn("Search errors occurred (rate-limited)")
		} else {
			s.mu.Unlock()
		}
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
	// Apply per-search timeout to prevent one slow search from blocking others
	// Use configured timeout, with a reasonable default and maximum
	searchTimeout := s.config.Timeout
	if searchTimeout == 0 {
		searchTimeout = 30 * time.Second // Default to 30 seconds if not configured
	}
	// Cap at 60 seconds to prevent excessively long searches
	maxTimeout := 60 * time.Second
	if searchTimeout > maxTimeout {
		searchTimeout = maxTimeout
	}
	searchCtx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	reqBody := map[string]interface{}{
		"query":      query,
		"datasets":   []string{dataset},
		"topK":       limit,
		"searchType": searchType,
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

	// Cognee returns a raw JSON array, not a wrapped object
	var rawResults []interface{}
	if err := json.Unmarshal(resp, &rawResults); err != nil {
		// Try wrapped format as fallback
		var wrapped struct {
			Results []interface{} `json:"results"`
		}
		if err2 := json.Unmarshal(resp, &wrapped); err2 != nil {
			return nil, err
		}
		return wrapped.Results, nil
	}

	return rawResults, nil
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
		// Rate limit this warning to once per 30 seconds
		s.mu.Lock()
		now := time.Now()
		shouldLog := s.lastStoreWarning.IsZero() || now.Sub(s.lastStoreWarning) > 30*time.Second
		if shouldLog {
			s.lastStoreWarning = now
			s.mu.Unlock()
			s.logger.WithError(err).Warn("Failed to store response in memory (rate-limited)")
		} else {
			s.mu.Unlock()
		}
		return err
	}

	// Auto-cognify if enabled (with timeout to prevent hanging goroutines)
	if s.config.AutoCognify {
		//nolint:gosec // G118: auto-cognify with its own 60s timeout, intentionally decoupled from the caller context
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
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

	// Use a dedicated longer-timeout client for cognify (LLM processing can be slow)
	cognifyClient := &http.Client{Timeout: 120 * time.Second}

	url := fmt.Sprintf("%s/api/v1/cognify", s.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create cognify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	s.addAuthHeader(req)

	resp, err := cognifyClient.Do(req)
	if err != nil {
		s.stats.mu.Lock()
		s.stats.ErrorCount++
		s.stats.mu.Unlock()
		return fmt.Errorf("cognify request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read cognify response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("cognify API error: %d - %s", resp.StatusCode, string(body))
	}

	s.stats.mu.Lock()
	s.stats.TotalCognifyOperations++
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()

	return nil
}

// parseCogneeSearchResults handles Cognee's variable response format.
// Cognee may return: ["string1", "string2"] or [{"key": "value"}, ...] or {"results": [...]}
func parseCogneeSearchResults(resp []byte) ([]map[string]interface{}, error) {
	// Try as array of objects first
	var objResults []map[string]interface{}
	if err := json.Unmarshal(resp, &objResults); err == nil {
		return objResults, nil
	}

	// Try as array of strings (RAG_COMPLETION returns plain text answers)
	var strResults []string
	if err := json.Unmarshal(resp, &strResults); err == nil {
		results := make([]map[string]interface{}, len(strResults))
		for i, s := range strResults {
			results[i] = map[string]interface{}{"text": s}
		}
		return results, nil
	}

	// Try as array of mixed types
	var rawResults []interface{}
	if err := json.Unmarshal(resp, &rawResults); err == nil {
		results := make([]map[string]interface{}, 0, len(rawResults))
		for _, item := range rawResults {
			switch v := item.(type) {
			case map[string]interface{}:
				results = append(results, v)
			case string:
				results = append(results, map[string]interface{}{"text": v})
			default:
				results = append(results, map[string]interface{}{"value": v})
			}
		}
		return results, nil
	}

	// Try as wrapped object with known keys ({"insights": [...]} or {"results": [...]})
	var wrapped map[string]interface{}
	if err := json.Unmarshal(resp, &wrapped); err == nil {
		for _, key := range []string{"insights", "results", "completions", "data"} {
			if arr, ok := wrapped[key]; ok {
				if items, ok := arr.([]interface{}); ok {
					results := make([]map[string]interface{}, 0, len(items))
					for _, item := range items {
						switch v := item.(type) {
						case map[string]interface{}:
							results = append(results, v)
						case string:
							results = append(results, map[string]interface{}{"text": v})
						default:
							results = append(results, map[string]interface{}{"value": v})
						}
					}
					return results, nil
				}
			}
		}
	}

	// Try as single string response
	var singleStr string
	if err := json.Unmarshal(resp, &singleStr); err == nil {
		return []map[string]interface{}{{"text": singleStr}}, nil
	}

	return nil, fmt.Errorf("unable to parse Cognee search response: %s", string(resp[:min(len(resp), 200)]))
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
		"query":      query,
		"datasets":   datasets,
		"topK":       limit,
		"searchType": "RAG_COMPLETION",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// Use a dedicated client with reasonable timeout for LLM-based search
	insightsClient := &http.Client{Timeout: 30 * time.Second}
	url := fmt.Sprintf("%s/api/v1/search", s.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.addAuthHeader(req)

	httpResp, err := insightsClient.Do(req)
	if err != nil {
		// Return empty results on timeout instead of error
		s.logger.WithError(err).Warn("Insights search timeout, returning empty results")
		return []map[string]interface{}{}, nil
	}
	defer func() { _ = httpResp.Body.Close() }()

	resp, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode >= 400 {
		s.logger.WithField("status", httpResp.StatusCode).Warn("Insights search returned error status")
		return []map[string]interface{}{}, nil
	}

	rawResults, err := parseCogneeSearchResults(resp)
	if err != nil {
		return nil, err
	}

	s.stats.mu.Lock()
	s.stats.TotalInsightsQueries++
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()

	return rawResults, nil
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
		"query":      query,
		"datasets":   datasets,
		"topK":       limit,
		"searchType": "GRAPH_COMPLETION",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// Use a dedicated client with reasonable timeout for LLM-based search
	graphClient := &http.Client{Timeout: 30 * time.Second}
	url := fmt.Sprintf("%s/api/v1/search", s.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.addAuthHeader(req)

	httpResp, err := graphClient.Do(req)
	if err != nil {
		// Return empty results on timeout instead of error
		s.logger.WithError(err).Warn("Graph completion search timeout, returning empty results")
		return []map[string]interface{}{}, nil
	}
	defer func() { _ = httpResp.Body.Close() }()

	resp, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode >= 400 {
		s.logger.WithField("status", httpResp.StatusCode).Warn("Graph completion search returned error status")
		return []map[string]interface{}{}, nil
	}

	rawResults, err := parseCogneeSearchResults(resp)
	if err != nil {
		return nil, err
	}

	s.stats.mu.Lock()
	s.stats.TotalGraphCompletions++
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()

	return rawResults, nil
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
		"query":      query,
		"searchType": "CODING_RULES",
		"limit":      5,
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

	// Cognee returns a raw JSON array
	var rawResults []CodeContext
	if err := json.Unmarshal(resp, &rawResults); err != nil {
		// Try wrapped format as fallback
		var wrapped struct {
			Results []CodeContext `json:"results"`
		}
		if err2 := json.Unmarshal(resp, &wrapped); err2 != nil {
			return nil, err
		}
		return wrapped.Results, nil
	}

	return rawResults, nil
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
	err = s.CreateDataset(ctx, datasetName, "Default dataset for HelixAgent Cognee integration", map[string]interface{}{
		"created_by":   "helixagent",
		"auto_created": true,
		"created_at":   time.Now().Format(time.RFC3339),
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

	// Cognee API can return either an array directly or an object with datasets field
	// Try array first (direct response from Cognee)
	var datasets []map[string]interface{}
	if err := json.Unmarshal(resp, &datasets); err == nil {
		return datasets, nil
	}

	// Fallback to object format
	var result struct {
		Datasets []map[string]interface{} `json:"datasets"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse datasets response: %w", err)
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

	s.feedbackLoop.history.Append(entry)

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
	defer func() { _ = resp.Body.Close() }()

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

	// Handle HTTP 409 Conflict (empty knowledge graph) gracefully
	// This happens when Cognee has no data - return empty response instead of error
	if resp.StatusCode == http.StatusConflict {
		s.logger.WithFields(logrus.Fields{
			"url":    url,
			"status": resp.StatusCode,
			"body":   string(respBody),
		}).Debug("Cognee knowledge graph empty (409) - returning empty results")
		return []byte("[]"), nil // Return empty JSON array
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

// SeedInitialData seeds Cognee with initial data to prevent "empty knowledge graph" errors
// This is called during service initialization to ensure the vector database is initialized
func (s *CogneeService) SeedInitialData(ctx context.Context) error {
	if !s.config.Enabled {
		return nil
	}

	s.logger.Debug("Seeding Cognee with initial data to initialize vector database")

	// Wait for service to be healthy
	healthCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Poll for health every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-healthCtx.Done():
			s.logger.Warn("Timed out waiting for Cognee to be healthy, skipping seed")
			return fmt.Errorf("cognee not healthy after 15s")
		case <-ticker.C:
			if s.IsHealthy(healthCtx) {
				goto seedData
			}
		}
	}

seedData:
	// Ensure we're authenticated
	if err := s.EnsureAuthenticated(ctx); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	// Seed with minimal system context to initialize the vector database
	seedContent := `HelixAgent is an AI-powered ensemble LLM service that combines responses from multiple language models.
It supports 10 LLM providers (Claude, DeepSeek, Gemini, Mistral, OpenRouter, Qwen, ZAI, Zen, Cerebras, Ollama) with dynamic provider selection.
The AI Debate system uses multi-round debate between providers for consensus with 5 positions (analyst, proposer, critic, synthesis, mediator).
Cognee provides knowledge graph and memory capabilities for enhanced context-aware responses.`

	// Create dedicated HTTP client for seeding to avoid race condition with shared client
	// Use longer timeout for Cognee cold start (10-15s) + processing time
	seedClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Build multipart form body directly here with dedicated client
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("data", "helixagent_seed.txt")
	if err != nil {
		s.logger.WithError(err).Warn("Failed to create seed form file")
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, writeErr := part.Write([]byte(seedContent)); writeErr != nil {
		s.logger.WithError(writeErr).Warn("Failed to write seed content")
		return fmt.Errorf("failed to write content: %w", writeErr)
	}

	if fieldErr := writer.WriteField("datasetName", "system"); fieldErr != nil {
		s.logger.WithError(fieldErr).Warn("Failed to write dataset field")
		return fmt.Errorf("failed to write dataset field: %w", fieldErr)
	}

	if closeErr := writer.Close(); closeErr != nil {
		s.logger.WithError(closeErr).Warn("Failed to close multipart writer")
		return fmt.Errorf("failed to close multipart writer: %w", closeErr)
	}

	// Ensure we're authenticated before making request
	if authErr := s.EnsureAuthenticated(ctx); authErr != nil {
		s.logger.WithError(authErr).Warn("Failed to authenticate for seeding")
		return fmt.Errorf("failed to authenticate: %w", authErr)
	}

	url := fmt.Sprintf("%s/api/v1/add", s.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to create seed request")
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add auth header
	s.mu.RLock()
	token := s.authToken
	s.mu.RUnlock()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	// Execute request with dedicated client (no race condition)
	resp, err := seedClient.Do(req)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to seed Cognee with initial data (non-critical)")
		return fmt.Errorf("seed request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to read seed response")
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		s.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(body),
		}).Warn("Seed request returned error status")
		return fmt.Errorf("seed API error: %d - %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Successfully seeded Cognee with initial data (vector database initialized)")
	return nil
}
