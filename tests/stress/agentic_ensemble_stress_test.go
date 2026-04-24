//go:build stress

// Package stress provides stress tests for the AgenticEnsemble pipeline.
package stress

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/handlers"
	"dev.helix.agent/internal/services"
)

func init() {
	runtime.GOMAXPROCS(2)
}

// newStressAgenticRouter creates a fresh agentic handler+router for stress tests.
func newStressAgenticRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	h := handlers.NewAgenticHandler(logger)
	r := gin.New()
	r.Use(gin.Recovery())
	api := r.Group("/v1")
	handlers.RegisterAgenticRoutes(api, h)
	return r
}

// buildStressWorkflowBody returns a minimal workflow payload for stress requests.
func buildStressWorkflowBody(id int) map[string]interface{} {
	return map[string]interface{}{
		"name":        "stress-workflow",
		"description": "concurrent stress test workflow",
		"nodes": []map[string]interface{}{
			{"id": "entry", "name": "Entry", "type": "agent"},
		},
		"edges":       []map[string]interface{}{},
		"entry_point": "entry",
		"end_nodes":   []string{"entry"},
		"input": map[string]interface{}{
			"query": "stress test query number " + string(rune('A'+id%26)),
		},
	}
}

// fireWorkflowRequest fires a single POST /v1/agentic/workflows request.
func fireWorkflowRequest(r *gin.Engine, body interface{}) int {
	raw, err := json.Marshal(body)
	if err != nil {
		return http.StatusInternalServerError
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agentic/workflows",
		strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// TestAgenticEnsemble_ConcurrentRequests fires 50 simultaneous Process() calls
// and verifies no panics occur and the handler remains responsive.
func TestAgenticEnsemble_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	r := newStressAgenticRouter()

	const concurrency = 50
	var (
		wg         sync.WaitGroup
		successCnt int64
		failCnt    int64
	)

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			code := fireWorkflowRequest(r, buildStressWorkflowBody(id))
			if code == http.StatusOK {
				atomic.AddInt64(&successCnt, 1)
			} else {
				atomic.AddInt64(&failCnt, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("concurrent requests: success=%d fail=%d elapsed=%s",
		successCnt, failCnt, elapsed)

	assert.Greater(t, int(successCnt), 0, "at least one request must succeed")
	// Allow up to 10% failures under load (server-side processing errors)
	maxAllowedFails := int64(concurrency / 10)
	assert.LessOrEqual(t, failCnt, maxAllowedFails,
		"too many failures under concurrent load")
}

// TestAgenticEnsemble_AgentPoolSaturation verifies the system handles requests
// beyond the default semaphore/pool limit without deadlocking.
func TestAgenticEnsemble_AgentPoolSaturation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	cfg := services.DefaultAgenticEnsembleConfig()
	poolLimit := cfg.MaxConcurrentAgents

	// Send 3x the pool limit
	requestCount := poolLimit * 3
	r := newStressAgenticRouter()

	var wg sync.WaitGroup
	var completedCnt int64

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fireWorkflowRequest(r, buildStressWorkflowBody(id))
			atomic.AddInt64(&completedCnt, 1)
		}(i)
	}

	select {
	case <-done:
		// All requests completed — no deadlock
	case <-time.After(30 * time.Second):
		t.Fatal("pool saturation test timed out — possible deadlock")
	}

	t.Logf("pool saturation: completed=%d requested=%d poolLimit=%d",
		completedCnt, requestCount, poolLimit)
	assert.Equal(t, int64(requestCount), completedCnt,
		"all requests should complete even past pool limit")
}

// TestAgenticEnsemble_ToolContention simulates concurrent tool access by
// firing multiple parallel requests each using tool nodes, verifying no data
// races occur in the tool execution path.
func TestAgenticEnsemble_ToolContention(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	r := newStressAgenticRouter()

	buildToolWorkflow := func(id int) map[string]interface{} {
		return map[string]interface{}{
			"name":        "tool-contention-workflow",
			"description": "concurrent tool node stress test",
			"nodes": []map[string]interface{}{
				{"id": "plan", "name": "Planner", "type": "agent"},
				{"id": "search", "name": "Search", "type": "tool"},
				{"id": "synthesize", "name": "Synthesizer", "type": "agent"},
			},
			"edges": []map[string]interface{}{
				{"from": "plan", "to": "search"},
				{"from": "search", "to": "synthesize"},
			},
			"entry_point": "plan",
			"end_nodes":   []string{"synthesize"},
			"input": map[string]interface{}{
				"query": "concurrent tool call",
			},
		}
	}

	const parallelism = 20
	var (
		wg         sync.WaitGroup
		successCnt int64
	)

	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			code := fireWorkflowRequest(r, buildToolWorkflow(id))
			if code == http.StatusOK {
				atomic.AddInt64(&successCnt, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("tool contention: success=%d parallel=%d", successCnt, parallelism)
	assert.Greater(t, int(successCnt), 0,
		"at least one request should succeed under tool contention")
}
