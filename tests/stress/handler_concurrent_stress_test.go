package stress

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/handlers"
)

// TestStress_AgentHandler_ConcurrentListAndGet verifies that the AgentHandler
// handles concurrent ListAgents, GetAgent, and ListAgentsByProtocol requests
// without panics, races, or partial reads. Uses the real AgentHandler struct
// with the production agents registry.
func TestStress_AgentHandler_ConcurrentListAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)

	handler := handlers.NewAgentHandler()
	router := gin.New()
	router.GET("/v1/agents", handler.ListAgents)
	router.GET("/v1/agents/:name", handler.GetAgent)
	router.GET("/v1/agents/protocol/:protocol", handler.ListAgentsByProtocol)
	router.GET("/v1/agents/tool/:tool", handler.ListAgentsByTool)

	const (
		listWorkers     = 40
		getWorkers      = 30
		protocolWorkers = 20
		toolWorkers     = 10
	)

	var wg sync.WaitGroup
	var successCount, failCount, panicCount int64

	start := make(chan struct{})

	// Concurrent ListAgents
	for i := 0; i < listWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 10; j++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/v1/agents", nil)
				router.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}()
	}

	// Concurrent GetAgent for known and unknown agents
	agentNames := []string{
		"opencode", "crush", "kilo-code", "helix-code",
		"aider", "goose", "nonexistent-agent-xyz",
	}
	for i := 0; i < getWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 10; j++ {
				name := agentNames[(id+j)%len(agentNames)]
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/v1/agents/"+name, nil)
				router.ServeHTTP(w, req)
				// Both 200 and 404 are valid — agent may or may not exist
				if w.Code == http.StatusOK || w.Code == http.StatusNotFound {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}(i)
	}

	// Concurrent ListAgentsByProtocol
	protocols := []string{"openai", "mcp", "acp", "lsp", "unknown-proto"}
	for i := 0; i < protocolWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 10; j++ {
				proto := protocols[(id+j)%len(protocols)]
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/v1/agents/protocol/"+proto, nil)
				router.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}(i)
	}

	// Concurrent ListAgentsByTool
	tools := []string{"code-review", "semantic-search", "memory-recall", "lsp-diagnostics"}
	for i := 0; i < toolWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 10; j++ {
				tool := tools[(id+j)%len(tools)]
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/v1/agents/tool/"+tool, nil)
				router.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: AgentHandler concurrent stress test timed out")
	}

	assert.Zero(t, panicCount, "no panics should occur under concurrent AgentHandler load")
	assert.Zero(t, failCount, "all requests should return valid status codes")

	totalExpected := int64(listWorkers+getWorkers+protocolWorkers+toolWorkers) * 10
	total := successCount + failCount
	assert.Equal(t, totalExpected, total, "all requests should be accounted for")

	t.Logf("AgentHandler concurrent: success=%d, fail=%d, panics=%d, total=%d",
		successCount, failCount, panicCount, total)
}

// TestStress_AgentHandler_RouterInitConcurrency verifies that multiple
// concurrent router constructions sharing the same AgentHandler do not
// cause data races. Simulates hot-reload or multi-instance scenarios.
func TestStress_AgentHandler_RouterInitConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)

	const goroutineCount = 50
	var wg sync.WaitGroup
	var panicCount, requestCount int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			// Each goroutine creates its own handler and router
			h := handlers.NewAgentHandler()
			r := gin.New()
			r.GET("/v1/agents", h.ListAgents)

			// Process 5 requests per goroutine
			for j := 0; j < 5; j++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/v1/agents", nil)
				r.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					atomic.AddInt64(&requestCount, 1)
				}
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: router init concurrency timed out")
	}

	assert.Zero(t, panicCount, "no panics during concurrent router creation")
	assert.Equal(t, int64(goroutineCount*5), requestCount,
		"all requests should be processed")

	t.Logf("Router init concurrency: total_requests=%d, panics=%d",
		requestCount, panicCount)
}

// TestStress_HandlerChain_MultiEndpointConcurrent stress-tests a chain of
// multiple handler endpoints called concurrently with high request rates.
// Validates that Gin's router can handle concurrent dispatch across different
// route patterns without race conditions.
func TestStress_HandlerChain_MultiEndpointConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)

	handler := handlers.NewAgentHandler()
	router := gin.New()
	router.GET("/v1/agents", handler.ListAgents)
	router.GET("/v1/agents/:name", handler.GetAgent)

	// Add non-handler routes to simulate a full API surface
	var requestMetric int64
	router.GET("/v1/health", func(c *gin.Context) {
		atomic.AddInt64(&requestMetric, 1)
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	router.GET("/v1/status", func(c *gin.Context) {
		atomic.AddInt64(&requestMetric, 1)
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"requests": requestMetric,
		})
	})

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/agents"},
		{"GET", "/v1/agents/opencode"},
		{"GET", "/v1/agents/nonexistent"},
		{"GET", "/v1/health"},
		{"GET", "/v1/status"},
	}

	const goroutineCount = 100
	var wg sync.WaitGroup
	var totalRequests, panicCount int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 20; j++ {
				ep := endpoints[(id+j)%len(endpoints)]
				w := httptest.NewRecorder()
				req := httptest.NewRequest(ep.method, ep.path, nil)
				router.ServeHTTP(w, req)
				atomic.AddInt64(&totalRequests, 1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: handler chain multi-endpoint stress timed out")
	}

	assert.Zero(t, panicCount, "no panics in handler chain under load")
	assert.Equal(t, int64(goroutineCount*20), totalRequests,
		"all requests should complete")

	t.Logf("Handler chain multi-endpoint: total=%d, panics=%d",
		totalRequests, panicCount)
}
