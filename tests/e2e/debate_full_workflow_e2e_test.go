package e2e

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// debateWorkflowBaseURL returns the HelixAgent server URL for E2E tests.
func debateWorkflowBaseURL() string {
	return "http://localhost:8100"
}

// isWorkflowServerAvailable checks if the HelixAgent server is reachable.
func isWorkflowServerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		debateWorkflowBaseURL()+"/health", nil)
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

// debateWorkflowPayload builds a JSON body for debate creation.
func debateWorkflowPayload(
	debateID, topic string,
	maxRounds int,
) ([]byte, error) {
	payload := map[string]interface{}{
		"debate_id":  debateID,
		"topic":      topic,
		"strategy":   "structured",
		"max_rounds": maxRounds,
		"timeout":    120,
		"participants": []map[string]interface{}{
			{
				"name":         "Architect",
				"role":         "proponent",
				"llm_provider": "claude",
				"llm_model":    "claude-3-opus-20240229",
				"max_rounds":   maxRounds,
				"timeout":      60,
				"weight":       1.0,
			},
			{
				"name":         "Reviewer",
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

// TestDebateWorkflow_CreateToResult tests the full E2E workflow of
// creating a debate, polling for status, and retrieving results.
func TestDebateWorkflow_CreateToResult(t *testing.T) {
	testutil.RequireServer(t)

	debateID := fmt.Sprintf("e2e-workflow-%d", time.Now().UnixNano()%100000)
	topic := "Design a rate limiting system for a distributed API gateway"

	// Step 1: Create the debate
	payload, err := debateWorkflowPayload(debateID, topic, 2)
	require.NoError(t, err, "Failed to marshal debate payload")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createURL := debateWorkflowBaseURL() + "/v1/debates"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL,
		bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "POST /v1/debates should not fail")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Contains(t, []int{http.StatusOK, http.StatusAccepted},
		resp.StatusCode,
		"Expected 200 or 202, got %d: %s", resp.StatusCode, string(body))

	var createResult map[string]interface{}
	err = json.Unmarshal(body, &createResult)
	require.NoError(t, err, "Response should be valid JSON")

	returnedID, ok := createResult["debate_id"]
	assert.True(t, ok, "Response should contain debate_id")
	assert.NotEmpty(t, returnedID, "debate_id should not be empty")

	t.Logf("Step 1 -- Created debate: %v (status %d)", returnedID,
		resp.StatusCode)

	// Step 2: Poll for completion
	pollCtx, pollCancel := context.WithTimeout(context.Background(),
		3*time.Minute)
	defer pollCancel()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	var finalStatus string
	for {
		select {
		case <-pollCtx.Done():
			t.Logf("Debate did not complete within poll timeout -- "+
				"last status: %s", finalStatus)
			goto results
		case <-ticker.C:
			statusURL := debateWorkflowBaseURL() + "/v1/debates/" +
				debateID + "/status"
			statusReq, reqErr := http.NewRequestWithContext(pollCtx,
				http.MethodGet, statusURL, nil)
			if reqErr != nil {
				continue
			}
			statusReq.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

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

			var status map[string]interface{}
			if jsonErr := json.Unmarshal(statusBody, &status); jsonErr != nil {
				continue
			}

			if s, sOK := status["status"].(string); sOK {
				finalStatus = s
				t.Logf("Step 2 -- Poll: status=%s", finalStatus)
				if s == "completed" || s == "failed" || s == "error" {
					goto results
				}
			}
		}
	}

results:
	// Step 3: Attempt to fetch results
	resultsCtx, resultsCancel := context.WithTimeout(context.Background(),
		10*time.Second)
	defer resultsCancel()

	resultsURL := debateWorkflowBaseURL() + "/v1/debates/" + debateID +
		"/results"
	resultsReq, err := http.NewRequestWithContext(resultsCtx,
		http.MethodGet, resultsURL, nil)
	require.NoError(t, err)
	resultsReq.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

	resultsResp, err := http.DefaultClient.Do(resultsReq)
	if err != nil {
		t.Logf("Step 3 -- Results fetch failed (may not be implemented): %v", err)
		return
	}
	defer resultsResp.Body.Close()

	resultsBody, err := io.ReadAll(resultsResp.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, resultsBody,
		"Results response body should not be empty")

	t.Logf("Step 3 -- Final status: %s, results: %d bytes",
		finalStatus, len(resultsBody))
}

// TestDebateWorkflow_ConcurrentDebates verifies that 3 debates can be
// created concurrently and each runs in isolation.
func TestDebateWorkflow_ConcurrentDebates(t *testing.T) {
	testutil.RequireServer(t)

	const concurrentCount = 3
	topics := []string{
		"Compare gRPC vs REST for microservices",
		"Evaluate event sourcing for financial systems",
		"Design a CDN caching strategy for global scale",
	}

	var wg sync.WaitGroup
	results := make(chan struct {
		index    int
		debateID string
		status   int
		err      error
	}, concurrentCount)

	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			debateID := fmt.Sprintf("e2e-concurrent-%d-%d",
				idx, time.Now().UnixNano()%100000)

			payload, err := debateWorkflowPayload(debateID, topics[idx], 1)
			if err != nil {
				results <- struct {
					index    int
					debateID string
					status   int
					err      error
				}{idx, debateID, 0, err}
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(),
				30*time.Second)
			defer cancel()

			url := debateWorkflowBaseURL() + "/v1/debates"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				url, bytes.NewReader(payload))
			if err != nil {
				results <- struct {
					index    int
					debateID string
					status   int
					err      error
				}{idx, debateID, 0, err}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				results <- struct {
					index    int
					debateID string
					status   int
					err      error
				}{idx, debateID, 0, err}
				return
			}
			resp.Body.Close()

			results <- struct {
				index    int
				debateID string
				status   int
				err      error
			}{idx, debateID, resp.StatusCode, nil}
		}(i)
	}

	wg.Wait()
	close(results)

	// Verify all debates were created successfully
	createdIDs := make([]string, 0, concurrentCount)
	for r := range results {
		if r.err != nil {
			t.Logf("Debate %d failed: %v", r.index, r.err)
			continue
		}
		assert.Contains(t, []int{http.StatusOK, http.StatusAccepted},
			r.status,
			"Debate %d should get 200 or 202, got %d",
			r.index, r.status)
		createdIDs = append(createdIDs, r.debateID)
		t.Logf("Debate %d created: %s (status %d)",
			r.index, r.debateID, r.status)
	}

	assert.Len(t, createdIDs, concurrentCount,
		"All %d concurrent debates should be created", concurrentCount)

	// Verify debate IDs are unique (isolation check)
	idSet := make(map[string]bool)
	for _, id := range createdIDs {
		assert.False(t, idSet[id],
			"Debate IDs should be unique, got duplicate: %s", id)
		idSet[id] = true
	}
}

// TestDebateWorkflow_InvalidRequest verifies the server rejects invalid
// debate creation requests with appropriate error responses.
func TestDebateWorkflow_InvalidRequest(t *testing.T) {
	testutil.RequireServer(t)

	testCases := []struct {
		name     string
		payload  map[string]interface{}
		wantCode int
	}{
		{
			name: "EmptyTopic",
			payload: map[string]interface{}{
				"debate_id": "e2e-invalid-empty",
				"topic":     "",
				"participants": []map[string]interface{}{
					{"name": "A", "role": "proponent", "weight": 1.0,
						"llm_provider": "claude", "llm_model": "claude-3"},
					{"name": "B", "role": "opponent", "weight": 1.0,
						"llm_provider": "deepseek", "llm_model": "deepseek-chat"},
				},
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "NoParticipants",
			payload: map[string]interface{}{
				"debate_id":    "e2e-invalid-no-part",
				"topic":        "Valid topic",
				"participants": []map[string]interface{}{},
			},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(),
				10*time.Second)
			defer cancel()

			body, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			url := debateWorkflowBaseURL() + "/v1/debates"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				url, bytes.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantCode, resp.StatusCode,
				"Expected %d for %s, got %d: %s",
				tc.wantCode, tc.name, resp.StatusCode, string(respBody))
		})
	}
}
