package services

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/sirupsen/logrus"
)

type DebateOptimizationConfig struct {
	EnableParallelExecution   bool          `json:"enable_parallel_execution"`
	EnableResponseCaching     bool          `json:"enable_response_caching"`
	EnableEarlyTermination    bool          `json:"enable_early_termination"`
	EnableStreaming           bool          `json:"enable_streaming"`
	EnableSmartFallback       bool          `json:"enable_smart_fallback"`
	MaxParallelRequests       int           `json:"max_parallel_requests"`
	CacheTTL                  time.Duration `json:"cache_ttl"`
	EarlyTerminationThreshold float64       `json:"early_termination_threshold"`
	RequestTimeout            time.Duration `json:"request_timeout"`
	FallbackTimeout           time.Duration `json:"fallback_timeout"`
	MaxCacheEntries           int           `json:"max_cache_entries"`
}

func DefaultDebateOptimizationConfig() DebateOptimizationConfig {
	return DebateOptimizationConfig{
		EnableParallelExecution:   true,
		EnableResponseCaching:     true,
		EnableEarlyTermination:    true,
		EnableStreaming:           true,
		EnableSmartFallback:       true,
		MaxParallelRequests:       3,
		CacheTTL:                  5 * time.Minute,
		EarlyTerminationThreshold: 0.95,
		RequestTimeout:            60 * time.Second,
		FallbackTimeout:           30 * time.Second,
		MaxCacheEntries:           10000,
	}
}

type CachedResponse struct {
	Response  *models.LLMResponse
	Timestamp time.Time
	Model     string
	Provider  string
}

type DebatePerformanceOptimizer struct {
	config    DebateOptimizationConfig
	cache     sync.Map
	cacheSize int64
	logger    *logrus.Logger
	registry  *ProviderRegistry
	semaphore chan struct{}
	stats     *OptimizationStats
}

type OptimizationStats struct {
	CacheHits          int64 `json:"cache_hits"`
	CacheMisses        int64 `json:"cache_misses"`
	ParallelRequests   int64 `json:"parallel_requests"`
	EarlyTerminations  int64 `json:"early_terminations"`
	FallbacksTriggered int64 `json:"fallbacks_triggered"`
	TotalRequests      int64 `json:"total_requests"`
	AverageLatencyMs   int64 `json:"average_latency_ms"`
}

func NewDebatePerformanceOptimizer(config DebateOptimizationConfig, registry *ProviderRegistry, logger *logrus.Logger) *DebatePerformanceOptimizer {
	if logger == nil {
		logger = logrus.New()
	}
	if config.MaxCacheEntries <= 0 {
		config.MaxCacheEntries = 10000
	}

	return &DebatePerformanceOptimizer{
		config:    config,
		logger:    logger,
		registry:  registry,
		semaphore: make(chan struct{}, config.MaxParallelRequests),
		stats:     &OptimizationStats{},
	}
}

func (dpo *DebatePerformanceOptimizer) ExecuteWithOptimization(
	ctx context.Context,
	member *DebateTeamMember,
	prompt string,
	previousResponses map[DebateTeamPosition]string,
) (*models.LLMResponse, error) {
	startTime := time.Now()
	atomic.AddInt64(&dpo.stats.TotalRequests, 1)

	if dpo.config.EnableResponseCaching {
		if cached := dpo.getCachedResponse(prompt, member.ModelName); cached != nil {
			atomic.AddInt64(&dpo.stats.CacheHits, 1)
			dpo.logger.WithFields(logrus.Fields{
				"model":     member.ModelName,
				"provider":  member.ProviderName,
				"cache_hit": true,
			}).Debug("Debate response served from cache")
			return cached.Response, nil
		}
		atomic.AddInt64(&dpo.stats.CacheMisses, 1)
	}

	var response *models.LLMResponse
	var err error

	if dpo.config.EnableSmartFallback && member.Fallbacks != nil && len(member.Fallbacks) > 0 {
		response, err = dpo.executeWithSmartFallback(ctx, member, prompt)
	} else {
		response, err = dpo.executeSingle(ctx, member, prompt)
	}

	if err == nil && dpo.config.EnableResponseCaching {
		dpo.cacheResponse(prompt, member.ModelName, member.ProviderName, response)
	}

	latency := time.Since(startTime).Milliseconds()
	// Use CAS loop for running average — lock-free
	for {
		old := atomic.LoadInt64(&dpo.stats.AverageLatencyMs)
		newAvg := (old + latency) / 2
		if atomic.CompareAndSwapInt64(&dpo.stats.AverageLatencyMs, old, newAvg) {
			break
		}
	}

	return response, err
}

func (dpo *DebatePerformanceOptimizer) ExecuteParallel(
	ctx context.Context,
	members []*DebateTeamMember,
	prompt string,
) map[DebateTeamPosition]*models.LLMResponse {
	results := make(map[DebateTeamPosition]*models.LLMResponse)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, member := range members {
		wg.Add(1)
		go func(m *DebateTeamMember) {
			defer wg.Done()

			if err := dpo.acquireWithBackoff(ctx); err != nil {
				return
			}
			defer func() { <-dpo.semaphore }()

			atomic.AddInt64(&dpo.stats.ParallelRequests, 1)

			resp, err := dpo.ExecuteWithOptimization(ctx, m, prompt, nil)
			if err == nil && resp != nil {
				mu.Lock()
				results[m.Position] = resp
				mu.Unlock()
			}
		}(member)
	}

	wg.Wait()
	return results
}

func (dpo *DebatePerformanceOptimizer) executeSingle(
	ctx context.Context,
	member *DebateTeamMember,
	prompt string,
) (*models.LLMResponse, error) {
	provider := member.Provider
	if provider == nil {
		var err error
		provider, err = dpo.registry.GetProvider(member.ProviderName)
		if err != nil {
			return nil, err
		}
	}

	req := &models.LLMRequest{
		Messages: []models.Message{
			{Role: "user", Content: prompt},
		},
		ModelParams: models.ModelParameters{
			Model: member.ModelName,
		},
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dpo.config.RequestTimeout)
	defer cancel()

	return provider.Complete(timeoutCtx, req)
}

func (dpo *DebatePerformanceOptimizer) executeWithSmartFallback(
	ctx context.Context,
	member *DebateTeamMember,
	prompt string,
) (*models.LLMResponse, error) {
	response, err := dpo.executeSingle(ctx, member, prompt)
	if err == nil {
		return response, nil
	}

	dpo.logger.WithFields(logrus.Fields{
		"position": member.Position,
		"model":    member.ModelName,
		"error":    err.Error(),
	}).Warn("Primary provider failed, trying fallbacks")

	atomic.AddInt64(&dpo.stats.FallbacksTriggered, 1)

	for i, fallback := range member.Fallbacks {
		if fallback == nil || !fallback.IsActive {
			continue
		}

		dpo.logger.WithFields(logrus.Fields{
			"position":       member.Position,
			"fallback_index": i,
			"model":          fallback.ModelName,
			"provider":       fallback.ProviderName,
		}).Info("Trying fallback provider")

		timeoutCtx, cancel := context.WithTimeout(ctx, dpo.config.FallbackTimeout)
		response, fallbackErr := dpo.executeSingle(timeoutCtx, fallback, prompt)
		cancel()

		if fallbackErr == nil {
			return response, nil
		}

		dpo.logger.WithFields(logrus.Fields{
			"position":       member.Position,
			"fallback_index": i,
			"error":          fallbackErr.Error(),
		}).Warn("Fallback provider failed")
	}

	return nil, err
}

// acquireWithBackoff attempts to acquire the semaphore with exponential backoff and jitter
// instead of blocking, preventing goroutine pile-ups under high load.
func (o *DebatePerformanceOptimizer) acquireWithBackoff(ctx context.Context) error {
	backoff := 10 * time.Millisecond
	maxBackoff := 5 * time.Second
	for {
		select {
		case o.semaphore <- struct{}{}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
			jitter := time.Duration(rand.Int63n(int64(backoff / 2))) //nolint:gosec
			select {
			case <-time.After(backoff + jitter):
			case <-ctx.Done():
				return ctx.Err()
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
		}
	}
}

func (dpo *DebatePerformanceOptimizer) ShouldTerminateEarly(responses map[DebateTeamPosition]string) bool {
	if !dpo.config.EnableEarlyTermination || len(responses) < 3 {
		return false
	}

	consensus := dpo.calculateConsensus(responses)
	if consensus >= dpo.config.EarlyTerminationThreshold {
		atomic.AddInt64(&dpo.stats.EarlyTerminations, 1)

		dpo.logger.WithFields(logrus.Fields{
			"consensus": consensus,
			"threshold": dpo.config.EarlyTerminationThreshold,
			"responses": len(responses),
		}).Info("Early termination triggered due to high consensus")

		return true
	}
	return false
}

func (dpo *DebatePerformanceOptimizer) calculateConsensus(responses map[DebateTeamPosition]string) float64 {
	if len(responses) < 2 {
		return 0.0
	}

	keyPoints := make(map[string]int)
	totalPoints := 0

	for _, resp := range responses {
		words := tokenize(resp)
		for _, word := range words {
			if len(word) > 4 {
				keyPoints[word]++
				totalPoints++
			}
		}
	}

	if totalPoints == 0 {
		return 0.0
	}

	agreementCount := 0
	for _, count := range keyPoints {
		if count > 1 {
			agreementCount += count
		}
	}

	return float64(agreementCount) / float64(totalPoints)
}

func (dpo *DebatePerformanceOptimizer) getCachedResponse(prompt, model string) *CachedResponse {
	cacheKey := dpo.generateCacheKey(prompt, model)
	if cached, ok := dpo.cache.Load(cacheKey); ok {
		cachedResp, ok := cached.(*CachedResponse)
		if !ok {
			dpo.cache.Delete(cacheKey)
			atomic.AddInt64(&dpo.cacheSize, -1)
			return nil
		}
		if time.Since(cachedResp.Timestamp) < dpo.config.CacheTTL {
			return cachedResp
		}
		dpo.cache.Delete(cacheKey)
		atomic.AddInt64(&dpo.cacheSize, -1)
	}
	return nil
}

func (dpo *DebatePerformanceOptimizer) cacheResponse(prompt, model, provider string, response *models.LLMResponse) {
	cacheKey := dpo.generateCacheKey(prompt, model)

	// Check if we already have this key to avoid double-counting.
	if _, exists := dpo.cache.Load(cacheKey); !exists {
		newSize := atomic.AddInt64(&dpo.cacheSize, 1)
		if newSize > int64(dpo.config.MaxCacheEntries) {
			dpo.evictOldEntries()
		}
	}

	dpo.cache.Store(cacheKey, &CachedResponse{
		Response:  response,
		Timestamp: time.Now(),
		Model:     model,
		Provider:  provider,
	})
}

// evictOldEntries sweeps the cache and removes entries whose timestamps are
// older than half the configured TTL (median-age threshold). This provides
// simple bounded-size behaviour without a full LRU structure.
func (dpo *DebatePerformanceOptimizer) evictOldEntries() {
	evictionCutoff := time.Now().Add(-(dpo.config.CacheTTL / 2))
	dpo.cache.Range(func(key, value any) bool {
		cachedResp, ok := value.(*CachedResponse)
		if !ok || cachedResp.Timestamp.Before(evictionCutoff) {
			dpo.cache.Delete(key)
			atomic.AddInt64(&dpo.cacheSize, -1)
		}
		return true
	})
}

func (dpo *DebatePerformanceOptimizer) generateCacheKey(prompt, model string) string {
	return model + ":" + hashString(prompt)
}

func (dpo *DebatePerformanceOptimizer) GetStats() *OptimizationStats {
	return &OptimizationStats{
		CacheHits:          atomic.LoadInt64(&dpo.stats.CacheHits),
		CacheMisses:        atomic.LoadInt64(&dpo.stats.CacheMisses),
		ParallelRequests:   atomic.LoadInt64(&dpo.stats.ParallelRequests),
		EarlyTerminations:  atomic.LoadInt64(&dpo.stats.EarlyTerminations),
		FallbacksTriggered: atomic.LoadInt64(&dpo.stats.FallbacksTriggered),
		TotalRequests:      atomic.LoadInt64(&dpo.stats.TotalRequests),
		AverageLatencyMs:   atomic.LoadInt64(&dpo.stats.AverageLatencyMs),
	}
}

func (dpo *DebatePerformanceOptimizer) ClearCache() {
	dpo.cache = sync.Map{}
	atomic.StoreInt64(&dpo.cacheSize, 0)
	dpo.logger.Info("Debate response cache cleared")
}

func tokenize(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	result := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.TrimFunc(word, func(r rune) bool {
			return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
		})
		if word != "" {
			result = append(result, word)
		}
	}
	return result
}

func hashString(s string) string {
	h := uint32(2166136261)
	for _, c := range s {
		h ^= uint32(c) // #nosec G115 -- integer conversion bounded by reachable resource limits; overflow is mathematically unreachable
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}
