//go:build performance
// +build performance

// Package performance contains benchmark and load tests for critical components.
// This file provides comprehensive benchmarks covering: lazy provider
// initialization, cache operations, health check endpoint, router dispatch,
// circuit breaker decisions, intent cache lookups, and JSON serialization.
package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"dev.helix.agent/internal/cache"
	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/router"
	"dev.helix.agent/internal/services"
)

// =============================================================================
// 1. Lazy provider initialization vs cached access
// =============================================================================

// BenchmarkLazyProviderFirstAccess benchmarks the first Get() call on a
// LazyProvider — the cold path that runs the factory and sync.Once.Do.
// Each sub-benchmark runs with a fresh LazyProvider so every iteration
// truly exercises the initialization code path.
func BenchmarkLazyProviderFirstAccess(b *testing.B) {
	cfg := &llm.LazyProviderConfig{
		InitTimeout:   5 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create a fresh provider so sync.Once has not fired yet.
		lp := llm.NewLazyProvider("bench", func() (llm.LLMProvider, error) {
			return &benchMockProvider{response: &models.LLMResponse{Content: "ok"}}, nil
		}, cfg)
		b.StartTimer()

		_, _ = lp.Get()
	}
}

// BenchmarkLazyProviderCachedAccess benchmarks repeated Get() calls after
// the first initialization — the hot path where sync.Once is a no-op and
// only an RLock + pointer copy are performed.
func BenchmarkLazyProviderCachedAccess(b *testing.B) {
	cfg := &llm.LazyProviderConfig{
		InitTimeout:   5 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    0,
	}
	lp := llm.NewLazyProvider("bench-cached", func() (llm.LLMProvider, error) {
		return &benchMockProvider{response: &models.LLMResponse{Content: "ok"}}, nil
	}, cfg)
	// Warm up: force initialization before timing.
	_, _ = lp.Get()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = lp.Get()
		}
	})
}

// =============================================================================
// 2. Cache operations (TieredCache L1 — no Redis required)
// =============================================================================

// BenchmarkCacheSet benchmarks in-memory (L1) cache write operations.
func BenchmarkCacheSet(b *testing.B) {
	tc := cache.NewTieredCache(nil, &cache.TieredCacheConfig{
		L1MaxSize: 100_000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	})
	defer tc.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench:set:%d", i%1000)
			tc.Set(ctx, key, i, time.Minute, "bench")
			i++
		}
	})
}

// BenchmarkCacheGet benchmarks in-memory (L1) cache read operations
// (cache-hit path).
func BenchmarkCacheGet(b *testing.B) {
	tc := cache.NewTieredCache(nil, &cache.TieredCacheConfig{
		L1MaxSize: 100_000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	})
	defer tc.Close()

	ctx := context.Background()

	// Pre-populate so reads hit the cache.
	const keys = 1000
	for i := 0; i < keys; i++ {
		tc.Set(ctx, fmt.Sprintf("bench:get:%d", i), i, time.Minute, "bench")
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var dest int
		i := 0
		for pb.Next() {
			tc.Get(ctx, fmt.Sprintf("bench:get:%d", i%keys), &dest)
			i++
		}
	})
}

// BenchmarkCacheGetMiss benchmarks in-memory cache reads for absent keys
// (cache-miss path, no L2 fallback).
func BenchmarkCacheGetMiss(b *testing.B) {
	tc := cache.NewTieredCache(nil, &cache.TieredCacheConfig{
		L1MaxSize: 100_000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	})
	defer tc.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var dest int
		i := 0
		for pb.Next() {
			// Keys that were never set — always a miss.
			tc.Get(ctx, fmt.Sprintf("bench:miss:%d", i), &dest)
			i++
		}
	})
}

// =============================================================================
// 3. Health check endpoint response time
// =============================================================================

// benchHealthRouter returns a minimal Gin engine with a /health route that
// matches the shape of HelixAgent's real health handler.
func benchHealthRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UnixMilli(),
		})
	})
	return r
}

// BenchmarkHealthCheckEndpoint measures round-trip latency for GET /health
// via httptest.Server (includes HTTP stack serialization overhead).
func BenchmarkHealthCheckEndpoint(b *testing.B) {
	srv := httptest.NewServer(benchHealthRouter())
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(srv.URL + "/health")
			if err == nil {
				resp.Body.Close()
			}
		}
	})
}

// =============================================================================
// 4. Router path matching (Gin dispatch)
// =============================================================================

// BenchmarkRouterPathMatch benchmarks Gin's route dispatch via httptest.
// Uses ServeHTTP directly (no network) to isolate routing overhead.
func BenchmarkRouterPathMatch(b *testing.B) {
	r := gin.New()
	r.GET("/v1/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/v1/models", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// =============================================================================
// 5. Circuit breaker admission decision
// =============================================================================

// BenchmarkCircuitBreakerBeforeRequest benchmarks the admission decision
// made by a closed circuit breaker for every inbound request.
// The exported IsOpen + GetState pair mirrors the internal beforeRequest logic.
func BenchmarkCircuitBreakerBeforeRequest(b *testing.B) {
	cb := llm.NewDefaultCircuitBreaker(
		"bench-cb",
		&benchMockProvider{response: &models.LLMResponse{Content: "ok"}},
	)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cb.IsOpen()
			_ = cb.GetState()
		}
	})
}

// =============================================================================
// 6. Intent cache lookup
// =============================================================================

// BenchmarkIntentCacheLookup benchmarks the full IntentClassifier.ClassifyIntent
// call for a short, unambiguous message — the hot path used by the
// intent-based router to gate every user request.
func BenchmarkIntentCacheLookup(b *testing.B) {
	ic := services.NewIntentClassifier()

	messages := []string{
		"yes, please proceed",
		"no, cancel that",
		"what does this function do?",
		"implement the feature now",
		"ok go ahead",
	}
	n := len(messages)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = ic.ClassifyIntent(messages[i%n], i%2 == 0)
			i++
		}
	})
}

// =============================================================================
// 7. JSON response serialization
// =============================================================================

// benchChatResponse is a typical OpenAI-compatible chat completion response.
type benchChatResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []benchChoiceItem `json:"choices"`
	Usage   benchUsage        `json:"usage"`
}

type benchChoiceItem struct {
	Index        int          `json:"index"`
	Message      benchMessage `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

type benchMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type benchUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// BenchmarkResponseSerialization benchmarks json.Marshal of a realistic
// chat-completion response payload — the final step in every API handler.
func BenchmarkResponseSerialization(b *testing.B) {
	resp := &benchChatResponse{
		ID:      "chatcmpl-benchmark-001",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "helixagent-debate",
		Choices: []benchChoiceItem{
			{
				Index: 0,
				Message: benchMessage{
					Role:    "assistant",
					Content: "The lazy initialization pattern defers expensive setup until the resource is first accessed, reducing startup latency and memory footprint for unused services.",
				},
				FinishReason: "stop",
			},
		},
		Usage: benchUsage{
			PromptTokens:     24,
			CompletionTokens: 36,
			TotalTokens:      60,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = json.Marshal(resp)
		}
	})
}

// =============================================================================
// 8. LazyServiceRegistry dispatch
// =============================================================================

// BenchmarkLazyServiceRegistryGet benchmarks a Get() call on an already-
// initialized LazyServiceRegistry entry — the path taken by every router
// handler that resolves a service via the registry.
func BenchmarkLazyServiceRegistryGet(b *testing.B) {
	reg := router.NewLazyServiceRegistry()

	var initCount int64
	svc := router.NewLazyService(func() (interface{}, error) {
		atomic.AddInt64(&initCount, 1)
		return "my-service", nil
	})
	reg.Register("bench-svc", svc)

	// Warm up: trigger initialization.
	_, _ = reg.Get("bench-svc")

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = reg.Get("bench-svc")
		}
	})
}

// =============================================================================
// 9. Lazy provider registry concurrent access
// =============================================================================

// BenchmarkLazyProviderRegistryGetProvider benchmarks concurrent GetProvider
// calls on a pre-initialized registry of 20 providers.
func BenchmarkLazyProviderRegistryGetProvider(b *testing.B) {
	cfg := &llm.LazyProviderConfig{
		InitTimeout:   5 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    0,
	}
	reg := llm.NewLazyProviderRegistry(cfg, nil)

	const numProviders = 20
	for i := 0; i < numProviders; i++ {
		name := fmt.Sprintf("bench-p%02d", i)
		reg.Register(name, func() (llm.LLMProvider, error) {
			return &benchMockProvider{response: nil}, nil
		})
	}

	// Warm: initialize all providers before timing.
	ctx := context.Background()
	_ = reg.PreloadAll(ctx)

	names := reg.List()
	n := len(names)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = reg.GetProvider(names[i%n])
			i++
		}
	})
}

// =============================================================================
// 10. Cache + LRU eviction under pressure
// =============================================================================

// BenchmarkCacheEvictionPressure benchmarks cache write performance when the
// L1 cache is at capacity and every write triggers an LRU eviction.
func BenchmarkCacheEvictionPressure(b *testing.B) {
	const maxSize = 100
	tc := cache.NewTieredCache(nil, &cache.TieredCacheConfig{
		L1MaxSize: maxSize,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	})
	defer tc.Close()

	ctx := context.Background()
	var counter int64

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Unique key per iteration so every write is a new entry and
			// triggers eviction once the cache is full.
			n := atomic.AddInt64(&counter, 1)
			tc.Set(ctx, fmt.Sprintf("evict:%d", n), n, time.Minute, "bench")
		}
	})
}

// =============================================================================
// 11. Concurrent sync.Once (hot path — after initialization)
// =============================================================================

// BenchmarkSyncOnceHotPath benchmarks the hot path of sync.Once.Do when
// the factory has already fired — measures atomic load overhead only.
func BenchmarkSyncOnceHotPath(b *testing.B) {
	var once sync.Once
	var result int

	// Prime once before timing.
	once.Do(func() { result = 42 })

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			once.Do(func() { result = 99 }) // no-op after first call
			_ = result
		}
	})
}
