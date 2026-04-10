package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/config"
	llm "dev.helix.agent/internal/llm/cognee"
	"dev.helix.agent/internal/models"
)

// memoryCacheEntry represents a cached memory entry with timestamp tracking
type memoryCacheEntry struct {
	sources   []models.MemorySource
	createdAt time.Time
	expiresAt time.Time
}

// CleanupStats represents statistics from a cache cleanup operation
type CleanupStats struct {
	EntriesRemoved int
	EntriesKept    int
	TimeTaken      time.Duration
	CleanupTime    time.Time
}

// MemoryService provides memory enhancement capabilities using Cognee
type MemoryService struct {
	client           *llm.Client
	enabled          bool
	dataset          string
	cache            map[string]*memoryCacheEntry
	cacheMu          sync.RWMutex
	ttl              time.Duration
	cleanupInterval  time.Duration
	lastCleanup      time.Time
	lastCleanupStats *CleanupStats
	stopCh           chan struct{}
	stopped          bool
	wg               sync.WaitGroup
}

// DefaultCleanupInterval is the default interval for cache cleanup
const DefaultCleanupInterval = 1 * time.Minute

// NewMemoryService creates a new memory service instance
func NewMemoryService(cfg *config.Config) *MemoryService {
	return NewMemoryServiceWithOptions(cfg, 5*time.Minute, DefaultCleanupInterval)
}

// NewMemoryServiceWithOptions creates a new memory service instance with custom TTL and cleanup interval.
// The service is enabled when MemoryEnabled is true (default) AND Cognee is configured.
// When MemoryEnabled is true but Cognee is not configured, cache-only mode is used
// with Mem0 as the primary memory backend.
func NewMemoryServiceWithOptions(cfg *config.Config, ttl, cleanupInterval time.Duration) *MemoryService {
	if cfg == nil || !cfg.MemoryEnabled || !cfg.Cognee.Enabled || !cfg.Cognee.AutoCognify {
		ms := &MemoryService{
			enabled:         false,
			cache:           make(map[string]*memoryCacheEntry),
			ttl:             ttl,
			cleanupInterval: cleanupInterval,
			stopCh:          make(chan struct{}),
		}
		// Start background cleanup even for disabled service (for when cache is still used)
		ms.wg.Add(1)
		go ms.cleanupRoutine()
		return ms
	}

	ms := &MemoryService{
		client: llm.NewClient(&config.Config{
			Cognee: cfg.Cognee,
		}),
		enabled:         true,
		dataset:         "default",
		cache:           make(map[string]*memoryCacheEntry),
		ttl:             ttl,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}

	// Start background cleanup goroutine with lifecycle tracking
	ms.wg.Add(1)
	go ms.cleanupRoutine()

	return ms
}

// AddMemory adds content to memory using Cognee
func (m *MemoryService) AddMemory(ctx context.Context, req *MemoryRequest) error {
	if !m.enabled {
		return fmt.Errorf("memory service is disabled")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", req.ContentType, strings.ToLower(req.Content[:func() int {
		if len(req.Content) < 50 {
			return len(req.Content)
		} else {
			return 50
		}
	}()]))

	// Check if we have a valid cached entry
	if sources := m.getCachedSources(cacheKey); len(sources) > 0 {
		// Return cached results
		return nil
	}

	// Add to Cognee
	memReq := &llm.MemoryRequest{
		Content:     req.Content,
		DatasetName: req.DatasetName,
		ContentType: req.ContentType,
	}

	resp, err := m.client.AddMemory(memReq)
	if err != nil {
		return fmt.Errorf("failed to add memory to Cognee: %w", err)
	}

	// Convert to MemorySource format
	sources := m.convertToMemorySources(resp)

	// Cache the results with expiration
	m.setCachedSources(cacheKey, sources)

	return nil
}

// SearchMemory searches memory for relevant content
func (m *MemoryService) SearchMemory(ctx context.Context, req *SearchRequest) ([]models.MemorySource, error) {
	if !m.enabled {
		return nil, fmt.Errorf("memory service is disabled")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("search:%s", strings.ToLower(req.Query))
	if sources := m.getCachedSources(cacheKey); sources != nil {
		// Return cached results
		return sources, nil
	}

	// Search Cognee
	cogneeReq := &llm.SearchRequest{
		Query:       req.Query,
		DatasetName: req.DatasetName,
		Limit:       req.Limit,
	}

	resp, err := m.client.SearchMemory(cogneeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search memory in Cognee: %w", err)
	}

	// Convert to MemorySource format
	sources := m.convertToMemorySourcesFromSearch(resp)

	// Cache the results with expiration
	m.setCachedSources(cacheKey, sources)

	return sources, nil
}

// SearchMemoryWithInsights performs insight-based search using graph reasoning
func (m *MemoryService) SearchMemoryWithInsights(ctx context.Context, req *SearchRequest) ([]models.MemorySource, error) {
	if !m.enabled {
		return nil, fmt.Errorf("memory service is disabled")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("insights:%s", strings.ToLower(req.Query))
	if sources := m.getCachedSources(cacheKey); sources != nil {
		return sources, nil
	}

	// Search with insights
	insightsReq := &llm.InsightsRequest{
		Query:    req.Query,
		Datasets: []string{req.DatasetName},
		Limit:    req.Limit,
	}

	resp, err := m.client.SearchInsights(insightsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search memory insights in Cognee: %w", err)
	}

	// Convert insights to MemorySource format
	sources := m.convertInsightsToMemorySources(resp)

	// Cache the results with expiration
	m.setCachedSources(cacheKey, sources)

	return sources, nil
}

// SearchMemoryWithGraphCompletion performs LLM-powered graph completion search
func (m *MemoryService) SearchMemoryWithGraphCompletion(ctx context.Context, req *SearchRequest) ([]models.MemorySource, error) {
	if !m.enabled {
		return nil, fmt.Errorf("memory service is disabled")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("graph:%s", strings.ToLower(req.Query))
	if sources := m.getCachedSources(cacheKey); sources != nil {
		return sources, nil
	}

	// Search with graph completion
	resp, err := m.client.SearchGraphCompletion(req.Query, []string{req.DatasetName}, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search memory with graph completion in Cognee: %w", err)
	}

	// Convert to MemorySource format
	sources := m.convertToMemorySourcesFromSearch(resp)

	// Cache the results with expiration
	m.setCachedSources(cacheKey, sources)

	return sources, nil
}

// CognifyDataset processes a dataset into knowledge graphs
func (m *MemoryService) CognifyDataset(ctx context.Context, datasetNames []string) error {
	if !m.enabled {
		return fmt.Errorf("memory service is disabled")
	}

	cognifyReq := &llm.CognifyRequest{
		Datasets: datasetNames,
	}

	resp, err := m.client.Cognify(cognifyReq)
	if err != nil {
		return fmt.Errorf("failed to cognify dataset in Cognee: %w", err)
	}

	if resp.Status != "success" {
		return fmt.Errorf("cognify failed with status: %s", resp.Status)
	}

	return nil
}

// ProcessCodeForMemory processes code content through Cognee's code pipeline
func (m *MemoryService) ProcessCodeForMemory(ctx context.Context, code, language, datasetName string) error {
	if !m.enabled {
		return fmt.Errorf("memory service is disabled")
	}

	codeReq := &llm.CodePipelineRequest{
		Code:        code,
		DatasetName: datasetName,
		Language:    language,
	}

	resp, err := m.client.ProcessCodePipeline(codeReq)
	if err != nil {
		return fmt.Errorf("failed to process code in Cognee: %w", err)
	}

	if !resp.Processed {
		return fmt.Errorf("code processing failed")
	}

	return nil
}

// EnhanceCodeRequest enhances a code-related request with Cognee insights
func (m *MemoryService) EnhanceCodeRequest(ctx context.Context, req *models.LLMRequest) error {
	if !m.enabled {
		return nil
	}

	// Extract code from the request
	code := m.extractCodeFromRequest(req)
	if code == "" {
		return nil // No code to process
	}

	// Determine language
	language := m.detectLanguage(code)

	// Process code through Cognee
	err := m.ProcessCodeForMemory(ctx, code, language, "code-analysis")
	if err != nil {
		return fmt.Errorf("failed to enhance code request: %w", err)
	}

	// Add code analysis context
	if req.Memory == nil {
		req.Memory = make(map[string]string)
	}
	req.Memory["code_language"] = language
	req.Memory["code_processed"] = "true"

	return nil
}

// extractCodeFromRequest extracts code snippets from LLM request
func (m *MemoryService) extractCodeFromRequest(req *models.LLMRequest) string {
	var codeParts []string

	// Check prompt for code blocks
	if strings.Contains(req.Prompt, "```") {
		// Simple extraction - in production, use proper markdown parsing
		parts := strings.Split(req.Prompt, "```")
		for i := 1; i < len(parts); i += 2 {
			if i < len(parts) {
				codeParts = append(codeParts, parts[i])
			}
		}
	}

	// Check messages for code
	for _, msg := range req.Messages {
		if strings.Contains(msg.Content, "```") {
			parts := strings.Split(msg.Content, "```")
			for i := 1; i < len(parts); i += 2 {
				if i < len(parts) {
					codeParts = append(codeParts, parts[i])
				}
			}
		}
	}

	return strings.Join(codeParts, "\n\n")
}

// detectLanguage attempts to detect programming language from code
func (m *MemoryService) detectLanguage(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))

	// Simple language detection based on keywords
	if strings.Contains(code, "import ") || strings.Contains(code, "from ") || strings.Contains(code, "def ") {
		return "python"
	}
	if strings.Contains(code, "func ") || strings.Contains(code, "package ") {
		return "go"
	}
	if strings.Contains(code, "function ") || strings.Contains(code, "const ") || strings.Contains(code, "let ") {
		return "javascript"
	}
	if strings.Contains(code, "class ") && strings.Contains(code, "public ") {
		return "java"
	}
	if strings.Contains(code, "#include") || strings.Contains(code, "int main") {
		return "c"
	}

	return "unknown"
}

// CreateDataset creates a new Cognee dataset
func (m *MemoryService) CreateDataset(ctx context.Context, name, description string) error {
	if !m.enabled {
		return fmt.Errorf("memory service is disabled")
	}

	datasetReq := &llm.DatasetRequest{
		Name:        name,
		Description: description,
		Metadata: map[string]interface{}{
			"created_by": "helix-agent",
			"created_at": time.Now().Format(time.RFC3339),
		},
	}

	_, err := m.client.CreateDataset(datasetReq)
	if err != nil {
		return fmt.Errorf("failed to create dataset in Cognee: %w", err)
	}

	return nil
}

// ListDatasets retrieves all available datasets
func (m *MemoryService) ListDatasets(ctx context.Context) ([]string, error) {
	if !m.enabled {
		return nil, fmt.Errorf("memory service is disabled")
	}

	resp, err := m.client.ListDatasets()
	if err != nil {
		return nil, fmt.Errorf("failed to list datasets from Cognee: %w", err)
	}

	datasetNames := make([]string, len(resp.Datasets))
	for i, dataset := range resp.Datasets {
		datasetNames[i] = dataset.Name
	}

	return datasetNames, nil
}

// SwitchDataset changes the current working dataset
func (m *MemoryService) SwitchDataset(datasetName string) {
	m.dataset = datasetName
	// Clear cache when switching datasets
	m.ClearCache()
}

// GetCurrentDataset returns the current dataset name
func (m *MemoryService) GetCurrentDataset() string {
	return m.dataset
}

// EnhanceRequest enhances a request with relevant memory context
func (m *MemoryService) EnhanceRequest(ctx context.Context, req *models.LLMRequest) error {
	if !m.enabled || !req.MemoryEnhanced {
		return nil
	}

	// Search for relevant memory sources
	sources, err := m.SearchMemory(ctx, &SearchRequest{
		Query:       m.extractKeywords(req),
		DatasetName: m.dataset,
		Limit:       5, // Limit for performance
	})
	if err != nil {
		return fmt.Errorf("failed to enhance request with memory: %w", err)
	}

	// Add memory sources to request
	if req.Memory == nil {
		req.Memory = make(map[string]string)
	}

	// Add context from memory sources
	for i, source := range sources {
		if source.Content != "" {
			req.Memory[fmt.Sprintf("memory_%d", i)] = source.Content
		}
	}

	return nil
}

// GetMemorySources returns memory sources for a request
func (m *MemoryService) GetMemorySources(ctx context.Context, req *models.LLMRequest) ([]models.MemorySource, error) {
	if !m.enabled {
		return nil, fmt.Errorf("memory service is disabled")
	}

	// Search for relevant memory sources
	sources, err := m.SearchMemory(ctx, &SearchRequest{
		Query:       m.extractKeywords(req),
		DatasetName: m.dataset,
		Limit:       10,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get memory sources: %w", err)
	}

	return sources, nil
}

// CacheCleanup removes expired cache entries and returns cleanup statistics
func (m *MemoryService) CacheCleanup() *CleanupStats {
	startTime := time.Now()

	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	entriesRemoved := 0
	entriesKept := 0

	for key, entry := range m.cache {
		if time.Now().After(entry.expiresAt) {
			delete(m.cache, key)
			entriesRemoved++
		} else {
			entriesKept++
		}
	}

	stats := &CleanupStats{
		EntriesRemoved: entriesRemoved,
		EntriesKept:    entriesKept,
		TimeTaken:      time.Since(startTime),
		CleanupTime:    startTime,
	}

	m.lastCleanup = startTime
	m.lastCleanupStats = stats

	return stats
}

// cleanupRoutine runs periodic cache cleanup in the background.
// Exits when stopCh is closed via Stop().
func (m *MemoryService) cleanupRoutine() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.CacheCleanup()
		}
	}
}

// Stop stops the background cleanup goroutine gracefully and waits for it to finish.
func (m *MemoryService) Stop() {
	m.cacheMu.Lock()
	if m.stopped {
		m.cacheMu.Unlock()
		return
	}
	m.stopped = true
	close(m.stopCh)
	m.cacheMu.Unlock()
	m.wg.Wait()
}

// getCachedSources retrieves cached sources if they exist and haven't expired
func (m *MemoryService) getCachedSources(key string) []models.MemorySource {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	entry, exists := m.cache[key]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.sources
}

// setCachedSources stores sources in the cache with expiration
func (m *MemoryService) setCachedSources(key string, sources []models.MemorySource) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	now := time.Now()
	m.cache[key] = &memoryCacheEntry{
		sources:   sources,
		createdAt: now,
		expiresAt: now.Add(m.ttl),
	}
}

// GetLastCleanupStats returns the statistics from the last cleanup operation
func (m *MemoryService) GetLastCleanupStats() *CleanupStats {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	return m.lastCleanupStats
}

// SetCleanupInterval sets the interval for background cache cleanup
func (m *MemoryService) SetCleanupInterval(interval time.Duration) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.cleanupInterval = interval
}

// GetCleanupInterval returns the current cleanup interval
func (m *MemoryService) GetCleanupInterval() time.Duration {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	return m.cleanupInterval
}

// convertToMemorySources converts Cognee responses to MemorySource format
func (m *MemoryService) convertToMemorySources(resp *llm.MemoryResponse) []models.MemorySource {
	if resp == nil {
		return nil
	}

	sources := make([]models.MemorySource, 0, len(resp.GraphNodes))

	for _, node := range resp.GraphNodes {
		if content, ok := node.(string); ok && content != "" {
			sources = append(sources, models.MemorySource{
				DatasetName:    "default", // MemoryResponse doesn't have DatasetName
				Content:        content,
				RelevanceScore: 1.0, // Default relevance
				SourceType:     "cognee",
			})
		}
	}

	return sources
}

// convertToMemorySourcesFromSearch converts Cognee search responses to MemorySource format
func (m *MemoryService) convertToMemorySourcesFromSearch(resp *llm.SearchResponse) []models.MemorySource {
	if resp == nil {
		return nil
	}

	sources := make([]models.MemorySource, 0, len(resp.Results))

	for _, result := range resp.Results {
		sources = append(sources, models.MemorySource{
			DatasetName:    result.DatasetName,
			Content:        result.Content,
			RelevanceScore: result.RelevanceScore,
			SourceType:     "cognee",
		})
	}

	return sources
}

// convertInsightsToMemorySources converts Cognee insights responses to MemorySource format
func (m *MemoryService) convertInsightsToMemorySources(resp *llm.InsightsResponse) []models.MemorySource {
	if resp == nil {
		return nil
	}

	sources := make([]models.MemorySource, 0, len(resp.Insights))

	for _, insight := range resp.Insights {
		content := ""
		if c, ok := insight["content"].(string); ok {
			content = c
		} else {
			// Convert map to JSON string
			contentBytes, err := json.Marshal(insight)
			if err != nil {
				content = ""
			} else {
				content = string(contentBytes)
			}
		}

		sources = append(sources, models.MemorySource{
			DatasetName:    "insights",
			Content:        content,
			RelevanceScore: 1.0,
			SourceType:     "cognee_insights",
		})
	}

	return sources
}

// extractKeywords extracts keywords from a request for memory search
func (m *MemoryService) extractKeywords(req *models.LLMRequest) string {
	keywords := []string{}

	// Extract from prompt
	if req.Prompt != "" {
		keywords = append(keywords, strings.Fields(req.Prompt)...)
	}

	// Extract from messages
	for _, msg := range req.Messages {
		if msg.Content != "" {
			keywords = append(keywords, strings.Fields(msg.Content)...)
		}
	}

	// Return first few keywords as search query
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	return strings.Join(keywords, " ")
}

// MemoryRequest represents a request to add memory
type MemoryRequest struct {
	Content     string `json:"content"`
	DatasetName string `json:"dataset_name"`
	ContentType string `json:"content_type"`
}

// SearchRequest represents a request to search memory
type SearchRequest struct {
	Query       string `json:"query"`
	DatasetName string `json:"dataset_name"`
	Limit       int    `json:"limit"`
}

// IsEnabled returns whether memory service is enabled
func (m *MemoryService) IsEnabled() bool {
	return m.enabled
}

// ClearCache clears the memory cache
func (m *MemoryService) ClearCache() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.cache = make(map[string]*memoryCacheEntry)
}

// GetStats returns memory service statistics
func (m *MemoryService) GetStats() map[string]interface{} {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	cogneeURL := ""
	if m.client != nil {
		cogneeURL = m.client.GetBaseURL()
	}

	stats := map[string]interface{}{
		"enabled":          m.enabled,
		"cache_size":       len(m.cache),
		"dataset":          m.dataset,
		"ttl_minutes":      m.ttl.Minutes(),
		"cognee_url":       cogneeURL,
		"cleanup_interval": m.cleanupInterval.String(),
		"last_cleanup":     m.lastCleanup,
	}

	if m.lastCleanupStats != nil {
		stats["last_cleanup_stats"] = map[string]interface{}{
			"entries_removed": m.lastCleanupStats.EntriesRemoved,
			"entries_kept":    m.lastCleanupStats.EntriesKept,
			"time_taken":      m.lastCleanupStats.TimeTaken.String(),
		}
	}

	return stats
}
