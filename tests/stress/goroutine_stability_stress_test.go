package stress

import (
	"fmt"
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

// TestStress_GoroutineStability_HandlerLifecycle verifies that repeatedly
// creating a handler, routing requests through it, and discarding it does not
// leak goroutines. Baseline is recorded before and after 1000 request cycles.
// Goroutine count must return to within +10 of baseline after GC.
func TestStress_GoroutineStability_HandlerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)

	// Warm up and let any initialization goroutines settle
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const cycles = 1000

	for i := 0; i < cycles; i++ {
		func() {
			h := handlers.NewAgentHandler()
			r := gin.New()
			r.GET("/v1/agents", h.ListAgents)
			r.GET("/v1/agents/:name", h.GetAgent)

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/agents", nil)
			r.ServeHTTP(w, req)

			_ = w.Code // ensure result is consumed
		}()
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	runtime.GC()

	after := runtime.NumGoroutine()
	leaked := after - baseline

	t.Logf("Handler lifecycle goroutine stability: baseline=%d, after=%d, leaked=%d",
		baseline, after, leaked)

	assert.Less(t, leaked, 10,
		"goroutine count must return to baseline (+10 margin) after 1000 handler cycles")
}

// TestStress_GoroutineStability_ConcurrentHandlerCreation verifies that
// creating many handler instances concurrently (simulating parallel request
// processing initialisation) does not produce goroutine leaks.
func TestStress_GoroutineStability_ConcurrentHandlerCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const goroutineCount = 100
	const requestsPerGoroutine = 5

	var wg sync.WaitGroup
	var processedCount atomic.Int64
	var panicCount atomic.Int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
				}
			}()
			<-start

			// Each goroutine creates and uses its own handler instance
			h := handlers.NewAgentHandler()
			r := gin.New()
			r.GET("/v1/agents", h.ListAgents)

			for j := 0; j < requestsPerGoroutine; j++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/v1/agents", nil)
				r.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					processedCount.Add(1)
				}
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent handler creation timed out")
	}

	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	runtime.GC()

	after := runtime.NumGoroutine()
	leaked := after - baseline

	assert.Zero(t, panicCount.Load(), "no panics during concurrent handler creation")
	assert.Equal(t, int64(goroutineCount*requestsPerGoroutine), processedCount.Load(),
		"all requests should be processed successfully")
	assert.Less(t, leaked, 15,
		"goroutine count should not grow after concurrent handler creation (+15 margin)")

	t.Logf("Concurrent handler creation: processed=%d, panics=%d, "+
		"baseline=%d, after=%d, leaked=%d",
		processedCount.Load(), panicCount.Load(), baseline, after, leaked)
}

// TestStress_GoroutineStability_RouterRecreation verifies that creating and
// discarding gin routers at high frequency (simulating dynamic route updates
// or test isolation patterns) does not leave goroutines behind.
func TestStress_GoroutineStability_RouterRecreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const iterations = 200

	for i := 0; i < iterations; i++ {
		func(iteration int) {
			h := handlers.NewAgentHandler()
			r := gin.New()
			r.GET("/v1/agents", h.ListAgents)
			r.GET(fmt.Sprintf("/v1/agents/%d", iteration), h.GetAgent)

			// Process a burst of 5 requests per router instance
			var innerWg sync.WaitGroup
			for j := 0; j < 5; j++ {
				innerWg.Add(1)
				go func(reqIdx int) {
					defer innerWg.Done()
					w := httptest.NewRecorder()
					req := httptest.NewRequest("GET", "/v1/agents", nil)
					r.ServeHTTP(w, req)
				}(j)
			}
			innerWg.Wait()
		}(i)
	}

	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	runtime.GC()

	after := runtime.NumGoroutine()
	leaked := after - baseline

	t.Logf("Router recreation: baseline=%d, after=%d, leaked=%d (%d iterations)",
		baseline, after, leaked, iterations)

	assert.Less(t, leaked, 15,
		"goroutine count must remain stable after %d router create/use/discard cycles",
		iterations)
}

// TestStress_GoroutineStability_MultipleHandlerTypes exercises multiple
// handler types (AgentHandler, HealthHandler-nil, inline handlers) across
// many goroutines to verify that mixed handler usage does not cause goroutine
// accumulation over time.
func TestStress_GoroutineStability_MultipleHandlerTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const goroutineCount = 50
	const cyclesPerGoroutine = 20

	var wg sync.WaitGroup
	var totalRequests atomic.Int64
	var panicCount atomic.Int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
				}
			}()
			<-start

			for c := 0; c < cyclesPerGoroutine; c++ {
				agentHandler := handlers.NewAgentHandler()

				r := gin.New()
				r.GET("/v1/agents", agentHandler.ListAgents)
				r.GET("/v1/health", func(ctx *gin.Context) {
					ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
				})
				r.GET("/v1/status", func(ctx *gin.Context) {
					ctx.JSON(http.StatusOK, gin.H{
						"goroutines": runtime.NumGoroutine(),
					})
				})

				// Alternate between different endpoints each cycle
				paths := []string{"/v1/agents", "/v1/health", "/v1/status"}
				path := paths[c%len(paths)]

				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", path, nil)
				r.ServeHTTP(w, req)
				totalRequests.Add(1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: multi-handler-type goroutine stability test timed out")
	}

	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	runtime.GC()

	after := runtime.NumGoroutine()
	leaked := after - baseline

	assert.Zero(t, panicCount.Load(), "no panics across multiple handler types")
	assert.Equal(t, int64(goroutineCount*cyclesPerGoroutine), totalRequests.Load(),
		"all requests should complete")
	assert.Less(t, leaked, 15,
		"goroutine count should be stable after multi-handler-type stress")

	t.Logf("Multi-handler-type stability: requests=%d, panics=%d, "+
		"baseline=%d, after=%d, leaked=%d",
		totalRequests.Load(), panicCount.Load(), baseline, after, leaked)
}
