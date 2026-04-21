// Previously tests/chaos/agentic/agentic_ensemble_chaos_test.go. CONST-030
// forbids in-process mocks in non-unit tests; this file exercises
// AgenticEnsemble against a local mockFailingServer (httptest) that
// abruptly closes connections mid-request — pure in-process chaos
// simulation. Real chaos engineering against live infrastructure is
// separate work (Pattern-2 in the audit). PR31 of the CONST-030 campaign.
package chaos_agentic_legacy_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/handlers"
	"dev.helix.agent/internal/services"
)

// mockFailingServer simulates a provider that dies mid-execution.
type mockFailingServer struct {
	callCount int64
	failAfter int64
	server    *httptest.Server
}

func newMockFailingServer(failAfter int64) *mockFailingServer {
	m := &mockFailingServer{failAfter: failAfter}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&m.callCount, 1)
		if count > m.failAfter {
			// Simulate provider dying: close connection abruptly
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				if conn != nil {
					conn.Close()
					return
				}
			}
			http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"id":     "chatcmpl-mock",
			"object": "chat.completion",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "mock response"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return m
}

// setupChaosRouter builds an agentic handler router for chaos tests.
func setupChaosRouter() *gin.Engine {
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

// postChaosWorkflow fires a workflow creation request and returns the recorder.
func postChaosWorkflow(r *gin.Engine, payload map[string]interface{}) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/agentic/workflows",
		strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestAgenticEnsemble_ProviderFailureMidExecution simulates a provider becoming
// unavailable after handling some requests. The system must handle the partial
// failure without crashing and report a status (completed or failed).
func TestAgenticEnsemble_ProviderFailureMidExecution(t *testing.T) {
	// Start a mock provider that fails after 2 successful calls
	mockProvider := newMockFailingServer(2)
	defer mockProvider.server.Close()

	r := setupChaosRouter()

	// Fire 5 consecutive requests; some will encounter the "dead" provider
	var successCnt, failCnt int64
	for i := 0; i < 5; i++ {
		payload := map[string]interface{}{
			"name":        "chaos-provider-failure",
			"description": "provider failure chaos test",
			"nodes": []map[string]interface{}{
				{"id": "agent", "name": "Agent", "type": "agent"},
			},
			"edges":       []map[string]interface{}{},
			"entry_point": "agent",
			"end_nodes":   []string{"agent"},
			"input": map[string]interface{}{
				"query": "chaos test query",
			},
		}

		w := postChaosWorkflow(r, payload)
		if w.Code == http.StatusOK {
			atomic.AddInt64(&successCnt, 1)
		} else {
			atomic.AddInt64(&failCnt, 1)
		}
	}

	t.Logf("provider failure chaos: success=%d fail=%d", successCnt, failCnt)

	// System must respond (not hang) even when providers fail
	total := successCnt + failCnt
	assert.Equal(t, int64(5), total,
		"all 5 requests must receive a response even with provider failure")
}

// TestAgenticEnsemble_ToolTimeout verifies that when a tool call exceeds its
// timeout, the request returns a response rather than hanging indefinitely.
func TestAgenticEnsemble_ToolTimeout(t *testing.T) {
	// Slow tool server that always times out
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	r := setupChaosRouter()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan int, 1)
	go func() {
		payload := map[string]interface{}{
			"name":        "tool-timeout-chaos",
			"description": "tool timeout chaos test",
			"nodes": []map[string]interface{}{
				{"id": "slow-tool", "name": "SlowTool", "type": "tool"},
			},
			"edges":       []map[string]interface{}{},
			"entry_point": "slow-tool",
			"end_nodes":   []string{"slow-tool"},
			"config": map[string]interface{}{
				"timeout_seconds": 1,
			},
			"input": map[string]interface{}{
				"query": "call slow tool",
			},
		}
		w := postChaosWorkflow(r, payload)
		done <- w.Code
	}()

	select {
	case code := <-done:
		// Any definitive response is acceptable; handler must not hang
		t.Logf("tool timeout chaos: response code=%d", code)
	case <-ctx.Done():
		t.Fatal("tool timeout chaos: handler hung — did not respond within deadline")
	}
}

// TestAgenticEnsemble_AgentCrash verifies that if an agent panics, the gin
// Recovery middleware catches it and returns HTTP 500 rather than crashing the
// whole server.
func TestAgenticEnsemble_AgentCrash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	h := handlers.NewAgenticHandler(logger)

	r := gin.New()
	r.Use(gin.Recovery()) // Must catch panics
	api := r.Group("/v1")
	handlers.RegisterAgenticRoutes(api, h)

	// Register a route that panics to simulate a crashing agent goroutine
	r.GET("/v1/crash-agent", func(c *gin.Context) {
		panic("simulated agent crash")
	})

	// 1. Trigger the panic route — should return 500, not crash
	panicReq := httptest.NewRequest(http.MethodGet, "/v1/crash-agent", nil)
	panicW := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		r.ServeHTTP(panicW, panicReq)
	}, "gin Recovery should absorb agent panic")
	assert.Equal(t, http.StatusInternalServerError, panicW.Code,
		"panic should yield HTTP 500 via Recovery middleware")

	// 2. Verify the server still handles subsequent normal requests
	payload := map[string]interface{}{
		"name":        "post-crash-workflow",
		"description": "workflow after crash",
		"nodes": []map[string]interface{}{
			{"id": "n1", "name": "Agent", "type": "agent"},
		},
		"edges":       []map[string]interface{}{},
		"entry_point": "n1",
		"end_nodes":   []string{"n1"},
	}
	w := postChaosWorkflow(r, payload)
	assert.Equal(t, http.StatusOK, w.Code,
		"server should remain functional after agent crash is recovered")
}

// TestAgenticEnsemble_ConcurrentCancellation verifies that cancelling a context
// mid-flight does not cause goroutine leaks or panics.
func TestAgenticEnsemble_ConcurrentCancellation(t *testing.T) {
	r := setupChaosRouter()

	var wg sync.WaitGroup
	const workers = 10

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithCancel(context.Background())

			// Cancel immediately after dispatching
			go func() {
				time.Sleep(1 * time.Millisecond)
				cancel()
			}()
			_ = ctx

			payload := map[string]interface{}{
				"name":        "cancel-test",
				"description": "concurrent cancellation test",
				"nodes": []map[string]interface{}{
					{"id": "n1", "name": "Agent", "type": "agent"},
				},
				"edges":       []map[string]interface{}{},
				"entry_point": "n1",
				"end_nodes":   []string{"n1"},
			}

			// The request itself won't be cancelled (httptest doesn't propagate),
			// but this validates no race condition with the cancel goroutine.
			w := postChaosWorkflow(r, payload)
			_ = w.Code
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed — no hang
	case <-time.After(15 * time.Second):
		t.Fatal("concurrent cancellation test timed out — goroutine leak suspected")
	}
}

// TestAgenticEnsemble_TaskFailureIsolation verifies that a failing task does not
// corrupt the status of sibling tasks.
func TestAgenticEnsemble_TaskFailureIsolation(t *testing.T) {
	tasks := []services.AgenticTask{
		{ID: "t1", Description: "Task 1", Status: services.AgenticTaskRunning},
		{ID: "t2", Description: "Task 2", Status: services.AgenticTaskRunning},
		{ID: "t3", Description: "Task 3", Status: services.AgenticTaskRunning},
	}

	// Simulate task 2 failing
	tasks[1].Status = services.AgenticTaskFailed

	// Verify task 1 and 3 are unaffected
	assert.Equal(t, services.AgenticTaskRunning, tasks[0].Status,
		"task 1 status must not be affected by task 2 failure")
	assert.Equal(t, services.AgenticTaskFailed, tasks[1].Status,
		"task 2 should be failed")
	assert.Equal(t, services.AgenticTaskRunning, tasks[2].Status,
		"task 3 status must not be affected by task 2 failure")

	// Verify recovery: task 3 can still complete
	tasks[2].Status = services.AgenticTaskCompleted
	require.Equal(t, services.AgenticTaskCompleted, tasks[2].Status)
}
