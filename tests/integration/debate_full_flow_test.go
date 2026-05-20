package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"digital.vasic.debate/orchestrator"
	"digital.vasic.debate/topology"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isDebateServerAvailable checks whether the HelixAgent server is reachable
// by hitting the /health endpoint.
func isDebateServerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := getEnv("HELIXAGENT_URL", "http://localhost:8100") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// debateBaseURL returns the configured HelixAgent base URL.
func debateBaseURL() string {
	return getEnv("HELIXAGENT_URL", "http://localhost:8100")
}

// createDebatePayload builds a valid JSON body for POST /v1/debates.
func createDebatePayload(
	debateID, topic, strategy string,
	maxRounds int,
) ([]byte, error) {
	payload := map[string]any{
		"debate_id":  debateID,
		"topic":      topic,
		"strategy":   strategy,
		"max_rounds": maxRounds,
		"timeout":    120,
		"participants": []map[string]any{
			{
				"name":         "Alice",
				"role":         "proponent",
				"llm_provider": "claude",
				"llm_model":    "claude-3-opus-20240229",
				"max_rounds":   maxRounds,
				"timeout":      60,
				"weight":       1.0,
			},
			{
				"name":         "Bob",
				"role":         "opponent",
				"llm_provider": "deepseek",
				"llm_model":    "deepseek-chat",
				"max_rounds":   maxRounds,
				"timeout":      60,
				"weight":       1.0,
			},
		},
	}
	return json.Marshal(payload)
}

// TestDebateFullFlow_OrchestratorInit verifies that the default orchestrator
// configuration can be created with valid defaults.
//
// NOTE (round-342, HXA-002): the DebateOrchestrator submodule was rebuilt
// from scratch (DebateOrchestrator commit 196d0ea) with a slim
// OrchestratorConfig. The pre-reconstruction fields asserted by the
// original version of this test — DefaultMinConsensus, MaxAgentsPerDebate,
// EnableAgentDiversity, MinConsensusForLesson, VotingMethod,
// EnableConfidenceWeighting — were genuinely DELETED (a tree-wide search
// of dependencies/ found no surviving copy). This test now asserts only
// the fields the reconstructed OrchestratorConfig actually exposes.
func TestDebateFullFlow_OrchestratorInit(t *testing.T) {
	cfg := orchestrator.DefaultOrchestratorConfig()

	assert.Equal(t, 3, cfg.DefaultMaxRounds,
		"Default max rounds should be 3")
	assert.Equal(t, 30*time.Second, cfg.DefaultTimeout,
		"Default timeout should be 30 seconds")
	assert.Equal(t, topology.TopologyGraphMesh, cfg.DefaultTopology,
		"Default topology should be GraphMesh")
	assert.Equal(t, 2, cfg.MinAgentsPerDebate,
		"Min agents per debate should be 2")
	assert.True(t, cfg.EnableLearning,
		"Learning should be enabled by default")
	assert.True(t, cfg.EnableCrossDebateLearning,
		"Cross-debate learning should be enabled by default")
}

// TestDebateFullFlow_HealthEndpoint verifies that the HelixAgent /health
// endpoint is reachable and returns HTTP 200.
func TestDebateFullFlow_HealthEndpoint(t *testing.T) {
	testutil.RequireServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := debateBaseURL() + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err, "Failed to create health request")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Health request failed")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"Health endpoint should return 200")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read health response body")
	assert.NotEmpty(t, body, "Health response body should not be empty")
}

// TestDebateFullFlow_CreateDebate verifies that POST /v1/debates accepts a
// valid debate creation request and returns HTTP 202 (Accepted).
func TestDebateFullFlow_CreateDebate(t *testing.T) {
	testutil.RequireServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payload, err := createDebatePayload(
		"integration-create-001",
		"Should AI systems be open source?",
		"structured",
		2,
	)
	require.NoError(t, err, "Failed to marshal debate payload")

	url := debateBaseURL() + "/v1/debates"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		bytes.NewReader(payload))
	require.NoError(t, err, "Failed to create POST request")
	req.Header.Set("Content-Type", "application/json")

	// Add JWT token for authentication
	req.Header.Set("Authorization", "Bearer "+getTestAPIKey())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "POST /v1/debates request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read create response body")

	// The server should accept the debate (202) or return 200
	assert.Contains(t, []int{http.StatusOK, http.StatusAccepted},
		resp.StatusCode,
		"Expected 200 or 202, got %d: %s", resp.StatusCode, string(body))

	var result map[string]any
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "Response should be valid JSON")

	// The response should contain a debate_id
	debateID, ok := result["debate_id"]
	assert.True(t, ok, "Response should contain debate_id")
	assert.NotEmpty(t, debateID, "debate_id should not be empty")
}

// TestDebateFullFlow_DebateStatus verifies that we can retrieve the status
// of a debate after creating it via GET /v1/debates/:id/status.
func TestDebateFullFlow_DebateStatus(t *testing.T) {
	testutil.RequireServer(t)

	debateID := "integration-status-001"

	// Step 1: Create the debate
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payload, err := createDebatePayload(
		debateID,
		"Is quantum computing practical today?",
		"structured",
		2,
	)
	require.NoError(t, err)

	createURL := debateBaseURL() + "/v1/debates"
	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		createURL, bytes.NewReader(payload))
	require.NoError(t, err)
	createReq.Header.Set("Content-Type", "application/json")

	// Add JWT token for authentication
	createReq.Header.Set("Authorization", "Bearer "+getTestAPIKey())

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err, "Failed to create debate for status test")
	defer createResp.Body.Close()

	createBody, err := io.ReadAll(createResp.Body)
	require.NoError(t, err)

	if createResp.StatusCode != http.StatusOK &&
		createResp.StatusCode != http.StatusAccepted {
		t.Skipf("Could not create debate (HTTP %d): %s (SKIP-OK: #unmarked-skip-needs-ticket)",
			createResp.StatusCode, string(createBody))
	}

	// Step 2: Query status
	statusCtx, statusCancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer statusCancel()

	statusURL := debateBaseURL() + "/v1/debates/" + debateID + "/status"
	statusReq, err := http.NewRequestWithContext(statusCtx, http.MethodGet,
		statusURL, nil)
	require.NoError(t, err)
	statusReq.Header.Set("Authorization", "Bearer "+getTestAPIKey())

	statusResp, err := http.DefaultClient.Do(statusReq)
	require.NoError(t, err, "GET status request failed")
	defer statusResp.Body.Close()

	assert.Equal(t, http.StatusOK, statusResp.StatusCode,
		"Status endpoint should return 200")

	statusBody, err := io.ReadAll(statusResp.Body)
	require.NoError(t, err)

	var status map[string]any
	err = json.Unmarshal(statusBody, &status)
	require.NoError(t, err, "Status response should be valid JSON")
	assert.NotEmpty(t, status, "Status response should not be empty")
}

// TestDebateFullFlow_DebateComplete tests the full lifecycle of a debate:
// create, poll for completion, and verify the results.
func TestDebateFullFlow_DebateComplete(t *testing.T) {
	testutil.RequireServer(t)

	debateID := "integration-complete-001"

	// Step 1: Create the debate
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payload, err := createDebatePayload(
		debateID,
		"What programming language is best for systems programming?",
		"structured",
		2,
	)
	require.NoError(t, err)

	createURL := debateBaseURL() + "/v1/debates"
	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		createURL, bytes.NewReader(payload))
	require.NoError(t, err)
	createReq.Header.Set("Content-Type", "application/json")

	// Add JWT token for authentication
	createReq.Header.Set("Authorization", "Bearer "+getTestAPIKey())

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err, "Failed to create debate")
	defer createResp.Body.Close()

	createBody, err := io.ReadAll(createResp.Body)
	require.NoError(t, err)

	if createResp.StatusCode != http.StatusOK &&
		createResp.StatusCode != http.StatusAccepted {
		t.Skipf("Could not create debate (HTTP %d): %s (SKIP-OK: #unmarked-skip-needs-ticket)",
			createResp.StatusCode, string(createBody))
	}

	// Step 2: Poll until the debate completes or times out
	pollCtx, pollCancel := context.WithTimeout(
		context.Background(), 5*time.Minute)
	defer pollCancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var finalStatus string
	for {
		select {
		case <-pollCtx.Done():
			t.Fatalf("Debate %s did not complete within timeout", debateID)
		case <-ticker.C:
			statusURL := debateBaseURL() + "/v1/debates/" + debateID +
				"/status"
			statusReq, reqErr := http.NewRequestWithContext(
				pollCtx, http.MethodGet, statusURL, nil)
			require.NoError(t, reqErr)
			statusReq.Header.Set("Authorization", "Bearer "+getTestAPIKey())

			statusResp, respErr := http.DefaultClient.Do(statusReq)
			if respErr != nil {
				t.Logf("Poll error: %v", respErr)
				continue
			}

			statusBody, readErr := io.ReadAll(statusResp.Body)
			statusResp.Body.Close()
			if readErr != nil {
				continue
			}

			var status map[string]any
			if jsonErr := json.Unmarshal(statusBody, &status); jsonErr != nil {
				continue
			}

			if s, ok := status["status"].(string); ok {
				finalStatus = s
				if s == "completed" || s == "failed" || s == "error" {
					goto done
				}
			}
		}
	}
done:

	// Step 3: Verify final state
	assert.NotEmpty(t, finalStatus, "Debate should have a final status")
	t.Logf("Debate %s finished with status: %s", debateID, finalStatus)

	// Attempt to fetch results
	resultsCtx, resultsCancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer resultsCancel()

	resultsURL := debateBaseURL() + "/v1/debates/" + debateID + "/results"
	resultsReq, err := http.NewRequestWithContext(
		resultsCtx, http.MethodGet, resultsURL, nil)
	require.NoError(t, err)
	resultsReq.Header.Set("Authorization", "Bearer "+getTestAPIKey())

	resultsResp, err := http.DefaultClient.Do(resultsReq)
	require.NoError(t, err, "GET results request failed")
	defer resultsResp.Body.Close()

	resultsBody, err := io.ReadAll(resultsResp.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, resultsBody,
		"Results response body should not be empty")
}

// TestDebateFullFlow_ConcurrentDebates verifies that multiple debates can
// be created and run concurrently without interference.
func TestDebateFullFlow_ConcurrentDebates(t *testing.T) {
	testutil.RequireServer(t)

	const concurrentCount = 3
	var wg sync.WaitGroup
	errors := make(chan error, concurrentCount)
	debateIDs := make(chan string, concurrentCount)

	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			debateID := fmt.Sprintf("integration-concurrent-%03d", index)
			topic := fmt.Sprintf(
				"Concurrent debate topic #%d: best practices in software", index)

			payload, err := createDebatePayload(
				debateID, topic, "structured", 2)
			if err != nil {
				errors <- fmt.Errorf("debate %d: marshal error: %w",
					index, err)
				return
			}

			ctx, cancel := context.WithTimeout(
				context.Background(), 30*time.Second)
			defer cancel()

			url := debateBaseURL() + "/v1/debates"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				url, bytes.NewReader(payload))
			if err != nil {
				errors <- fmt.Errorf("debate %d: request error: %w",
					index, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			// Add JWT token for authentication
			req.Header.Set("Authorization", "Bearer "+getTestAPIKey())

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				errors <- fmt.Errorf("debate %d: HTTP error: %w",
					index, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK &&
				resp.StatusCode != http.StatusAccepted {
				body, _ := io.ReadAll(resp.Body)
				errors <- fmt.Errorf(
					"debate %d: unexpected status %d: %s",
					index, resp.StatusCode, string(body))
				return
			}

			debateIDs <- debateID
		}(i)
	}

	wg.Wait()
	close(errors)
	close(debateIDs)

	// Collect and assert no errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	assert.Empty(t, errs,
		"All concurrent debates should succeed, but got errors: %v", errs)

	// Verify all debate IDs were returned
	var ids []string
	for id := range debateIDs {
		ids = append(ids, id)
	}
	assert.Len(t, ids, concurrentCount,
		"Should have created %d debates", concurrentCount)

	// Verify each debate is queryable
	for _, id := range ids {
		ctx, cancel := context.WithTimeout(
			context.Background(), 10*time.Second)

		statusURL := debateBaseURL() + "/v1/debates/" + id
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			statusURL, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+getTestAPIKey())

		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode,
				"Debate %s should be queryable", id)
		}

		cancel()
	}
}

// TestDebateFullFlow_InvalidTopic tests that the server returns an error
// when creating a debate with an empty or invalid topic.
func TestDebateFullFlow_InvalidTopic(t *testing.T) {
	testutil.RequireServer(t)

	testCases := []struct {
		name     string
		payload  map[string]any
		wantCode int
	}{
		{
			name: "EmptyTopic",
			payload: map[string]any{
				"debate_id": "integration-invalid-empty",
				"topic":     "",
				"participants": []map[string]any{
					{
						"name":         "Alice",
						"role":         "proponent",
						"llm_provider": "claude",
						"llm_model":    "claude-3-opus-20240229",
						"weight":       1.0,
					},
					{
						"name":         "Bob",
						"role":         "opponent",
						"llm_provider": "deepseek",
						"llm_model":    "deepseek-chat",
						"weight":       1.0,
					},
				},
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "MissingParticipants",
			payload: map[string]any{
				"debate_id":    "integration-invalid-no-participants",
				"topic":        "A valid topic with no participants",
				"participants": []map[string]any{},
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "SingleParticipant",
			payload: map[string]any{
				"debate_id": "integration-invalid-one-participant",
				"topic":     "A valid topic with only one participant",
				"participants": []map[string]any{
					{
						"name":         "Alice",
						"role":         "proponent",
						"llm_provider": "claude",
						"llm_model":    "claude-3-opus-20240229",
						"weight":       1.0,
					},
				},
			},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				context.Background(), 10*time.Second)
			defer cancel()

			body, err := json.Marshal(tc.payload)
			require.NoError(t, err, "Failed to marshal test payload")

			url := debateBaseURL() + "/v1/debates"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				url, bytes.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Add JWT token for authentication
			req.Header.Set("Authorization", "Bearer "+getTestAPIKey())

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err, "POST request failed")
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantCode, resp.StatusCode,
				"Expected HTTP %d for %s, got %d: %s",
				tc.wantCode, tc.name, resp.StatusCode,
				string(respBody))

			// Error responses should contain an error field
			var result map[string]any
			if err := json.Unmarshal(respBody, &result); err == nil {
				_, hasError := result["error"]
				assert.True(t, hasError,
					"Error response should contain an 'error' field")
			}
		})
	}
}
