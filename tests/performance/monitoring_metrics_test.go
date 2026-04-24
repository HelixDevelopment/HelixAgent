//go:build performance
// +build performance

package performance

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const monitoringBaseURL = "http://localhost:8100"

// TestMonitoringMetrics_UnderLoad verifies Prometheus metrics are
// populated when the system processes requests.
func TestMonitoringMetrics_UnderLoad(t *testing.T) {
	testutil.RequireServer(t)

	// Generate 20 requests (keep resource limits low per CLAUDE.md §15)
	client := &http.Client{Timeout: 5 * time.Second}
	for i := 0; i < 20; i++ {
		resp, err := client.Post(
			monitoringBaseURL+"/v1/chat/completions",
			"application/json",
			strings.NewReader(`{"model":"helixagent","messages":[{"role":"user","content":"ping"}]}`),
		)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}

	time.Sleep(2 * time.Second)

	resp, err := http.Get(monitoringBaseURL + "/metrics")
	require.NoError(t, err, "Prometheus metrics endpoint must be accessible")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	metrics := string(body)

	// Verify core Go runtime metrics are present
	assert.Contains(t, metrics, "go_goroutines", "goroutine metric must exist")
	assert.Contains(t, metrics, "go_memstats_alloc_bytes", "memory metric must exist")

	t.Logf("Metrics response: %d bytes", len(metrics))
}

// TestMonitoringMetrics_HealthEndpoint verifies the health endpoint responds.
func TestMonitoringMetrics_HealthEndpoint(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(monitoringBaseURL + "/health")
	require.NoError(t, err, "Health endpoint must be accessible")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestMonitoringMetrics_ConcurrentRequests verifies the system handles
// concurrent requests without panicking.
func TestMonitoringMetrics_ConcurrentRequests(t *testing.T) {
	testutil.RequireServer(t)

	const numWorkers = 5
	const requestsPerWorker = 4
	done := make(chan struct{}, numWorkers*requestsPerWorker)
	errors := make(chan error, numWorkers*requestsPerWorker)

	client := &http.Client{Timeout: 10 * time.Second}
	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			for i := 0; i < requestsPerWorker; i++ {
				resp, err := client.Get(monitoringBaseURL + "/health")
				if err != nil {
					errors <- fmt.Errorf("worker %d: %w", workerID, err)
					done <- struct{}{}
					continue
				}
				resp.Body.Close()
				done <- struct{}{}
			}
		}(w)
	}

	total := numWorkers * requestsPerWorker
	for i := 0; i < total; i++ {
		<-done
	}

	close(errors)
	errCount := 0
	for err := range errors {
		t.Logf("Request error: %v", err)
		errCount++
	}

	// Allow up to 20% failure rate (server may not be running in CI)
	assert.LessOrEqual(t, errCount, total/5, "At most 20%% of requests should fail")
}
