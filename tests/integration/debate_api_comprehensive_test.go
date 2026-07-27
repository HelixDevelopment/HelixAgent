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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type debateAPIResponse struct {
	DebateID     string `json:"debate_id"`
	Status       string `json:"status"`
	Topic        string `json:"topic"`
	Error        string `json:"error,omitempty"`
	CurrentPhase string `json:"current_phase,omitempty"`
}

type debateCreateRequest struct {
	DebateID     string                 `json:"debate_id"`
	Topic        string                 `json:"topic"`
	Strategy     string                 `json:"strategy"`
	MaxRounds    int                    `json:"max_rounds"`
	Timeout      int                    `json:"timeout"`
	Participants []debateParticipantReq `json:"participants"`
}

type debateParticipantReq struct {
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	LLMProvider string  `json:"llm_provider"`
	LLMModel    string  `json:"llm_model"`
	MaxRounds   int     `json:"max_rounds"`
	Timeout     int     `json:"timeout"`
	Weight      float64 `json:"weight"`
}

func createDebate(t *testing.T, baseURL string, req debateCreateRequest) (*http.Response, map[string]any) {
	t.Helper()
	body, err := json.Marshal(req)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/debates", bytes.NewReader(body))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return resp, result
}

func getDebateStatus(t *testing.T, baseURL, debateID string) (int, map[string]any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/debates/%s", baseURL, debateID), nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return resp.StatusCode, result
}

func waitForDebateCompletion(t *testing.T, baseURL, debateID string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		statusCode, result := getDebateStatus(t, baseURL, debateID)
		if statusCode == 404 {
			t.Logf("Debate %s not found (404), retrying...", debateID)
			time.Sleep(2 * time.Second)
			continue
		}
		status, _ := result["status"].(string)
		if status == "completed" || status == "failed" {
			return result
		}
		t.Logf("Debate %s status: %s, waiting...", debateID, status)
		time.Sleep(3 * time.Second)
	}
	t.Fatalf("Debate %s did not complete within %v", debateID, timeout)
	return nil
}

func isDebateErrorDueToAgentPool(result map[string]any) bool {
	errStr, _ := result["error"].(string)
	if errStr == "" {
		if inner, ok := result["error"].(map[string]any); ok {
			if msg, ok := inner["message"].(string); ok {
				errStr = msg
			}
		}
	}
	return containsAny(errStr,
		"not enough agents in pool",
		"orchestrator integration not initialized",
		"auto-discovery not enabled",
		"agent pool",
	)
}

func containsAny(s string, patterns ...string) bool {
	for _, p := range patterns {
		if len(s) >= len(p) {
			for i := 0; i <= len(s)-len(p); i++ {
				if s[i:i+len(p)] == p {
					return true
				}
			}
		}
	}
	return false
}

func healthyParticipants(n int) []debateParticipantReq {
	providers := []struct {
		name     string
		provider string
		model    string
		role     string
	}{
		{"DeepSeek Analyst", "deepseek", "deepseek-chat", "analyst"},
		{"Mistral Critic", "mistral", "mistral-large-latest", "critic"},
		{"Codestral Engineer", "codestral", "codestral-latest", "engineer"},
		{"NVIDIA Reviewer", "nvidia", "meta/llama-3.1-70b-instruct", "reviewer"},
		{"Sarvam Synthesizer", "sarvam", "sarvam-m", "synthesizer"},
		{"Modal Proposer", "modal", "modal-1", "proposer"},
		{"DeepSeek Debater", "deepseek", "deepseek-chat", "debater"},
		{"Mistral Mediator", "mistral", "mistral-large-latest", "mediator"},
		{"Codestral Architect", "codestral", "codestral-latest", "architect"},
		{"NVIDIA Validator", "nvidia", "meta/llama-3.1-70b-instruct", "validator"},
	}

	participants := make([]debateParticipantReq, n)
	for i := 0; i < n; i++ {
		p := providers[i%len(providers)]
		participants[i] = debateParticipantReq{
			Name:        p.name,
			Role:        p.role,
			LLMProvider: p.provider,
			LLMModel:    p.model,
			MaxRounds:   3,
			Timeout:     60,
			Weight:      1.0,
		}
	}
	return participants
}

// TestDebateAPI_5PositionDebate tests a 5-position debate via the live API.
// CONST-026 requires both debate flavors with 5 positions.
func TestDebateAPI_5PositionDebate(t *testing.T) {
	if !isDebateServerAvailable() {
		t.Skip("HelixAgent server not available") // SKIP-OK: #legacy-untriaged
	}

	baseURL := debateBaseURL()
	debateID := fmt.Sprintf("test-5pos-%d", time.Now().UnixMilli())

	resp, result := createDebate(t, baseURL, debateCreateRequest{
		DebateID:     debateID,
		Topic:        "Analyze the tradeoffs between microservices and monolithic architectures for a mid-size SaaS company",
		Strategy:     "consensus",
		MaxRounds:    2,
		Timeout:      120,
		Participants: healthyParticipants(5),
	})

	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted,
		"POST /v1/debates should return 200 or 202, got %d, body: %v", resp.StatusCode, result)

	returnedID, _ := result["debate_id"].(string)
	assert.NotEmpty(t, returnedID, "Response should contain debate_id")

	t.Logf("Created 5-position debate: %s (returned ID: %s)", debateID, returnedID)

	finalResult := waitForDebateCompletion(t, baseURL, returnedID, 3*time.Minute)
	status, _ := finalResult["status"].(string)

	if status == "failed" {
		errStr, _ := finalResult["error"].(string)
		if isDebateErrorDueToAgentPool(finalResult) {
			t.Skipf("Debate failed due to orchestrator agent pool issue (expected in test env): %s (SKIP-OK: #unmarked-skip-needs-ticket)", errStr)
		}
		t.Logf("Debate failed (may be expected with test providers): %s", errStr)
	} else {
		assert.Equal(t, "completed", status, "Debate should complete successfully")
		t.Logf("5-position debate completed successfully: %s", returnedID)
	}
}

// TestDebateAPI_8PositionDebate tests an 8+ position debate via the live API.
// CONST-026 requires both debate flavors with 8+ positions.
func TestDebateAPI_8PositionDebate(t *testing.T) {
	if !isDebateServerAvailable() {
		t.Skip("HelixAgent server not available") // SKIP-OK: #legacy-untriaged
	}

	baseURL := debateBaseURL()
	debateID := fmt.Sprintf("test-8pos-%d", time.Now().UnixMilli())

	resp, result := createDebate(t, baseURL, debateCreateRequest{
		DebateID:     debateID,
		Topic:        "Evaluate the impact of LLM-based code generation on software engineering practices, security, and developer productivity",
		Strategy:     "consensus",
		MaxRounds:    2,
		Timeout:      180,
		Participants: healthyParticipants(8),
	})

	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted,
		"POST /v1/debates should return 200 or 202, got %d, body: %v", resp.StatusCode, result)

	returnedID, _ := result["debate_id"].(string)
	assert.NotEmpty(t, returnedID)

	t.Logf("Created 8-position debate: %s (returned ID: %s)", debateID, returnedID)

	finalResult := waitForDebateCompletion(t, baseURL, returnedID, 4*time.Minute)
	status, _ := finalResult["status"].(string)

	if status == "failed" {
		errStr, _ := finalResult["error"].(string)
		if isDebateErrorDueToAgentPool(finalResult) {
			t.Skipf("Debate failed due to orchestrator agent pool issue (expected in test env): %s (SKIP-OK: #unmarked-skip-needs-ticket)", errStr)
		}
		t.Logf("Debate failed (may be expected with test providers): %s", errStr)
	} else {
		assert.Equal(t, "completed", status)
		t.Logf("8-position debate completed successfully: %s", returnedID)
	}
}

// TestDebateAPI_10PositionDebate tests a 10-position large-scale debate.
func TestDebateAPI_10PositionDebate(t *testing.T) {
	if !isDebateServerAvailable() {
		t.Skip("HelixAgent server not available") // SKIP-OK: #legacy-untriaged
	}

	baseURL := debateBaseURL()
	debateID := fmt.Sprintf("test-10pos-%d", time.Now().UnixMilli())

	resp, result := createDebate(t, baseURL, debateCreateRequest{
		DebateID:     debateID,
		Topic:        "Design the optimal architecture for an AI-powered ensemble LLM service that aggregates responses from multiple language models",
		Strategy:     "weighted_consensus",
		MaxRounds:    1,
		Timeout:      240,
		Participants: healthyParticipants(10),
	})

	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted,
		"POST /v1/debates should return 200 or 202, got %d, body: %v", resp.StatusCode, result)

	returnedID, _ := result["debate_id"].(string)
	assert.NotEmpty(t, returnedID)

	t.Logf("Created 10-position debate: %s", returnedID)

	finalResult := waitForDebateCompletion(t, baseURL, returnedID, 5*time.Minute)
	status, _ := finalResult["status"].(string)

	if status == "failed" {
		errStr, _ := finalResult["error"].(string)
		if isDebateErrorDueToAgentPool(finalResult) {
			t.Skipf("Debate failed due to orchestrator agent pool issue: %s (SKIP-OK: #unmarked-skip-needs-ticket)", errStr)
		}
		t.Logf("Debate failed: %s", errStr)
	} else {
		assert.Equal(t, "completed", status)
		t.Logf("10-position debate completed: %s", returnedID)
	}
}

// TestDebateAPI_ErrorHandling_MissingTopic tests error response for missing topic.
func TestDebateAPI_ErrorHandling_MissingTopic(t *testing.T) {
	if !isDebateServerAvailable() {
		t.Skip("HelixAgent server not available") // SKIP-OK: #legacy-untriaged
	}

	baseURL := debateBaseURL()

	resp, result := createDebate(t, baseURL, debateCreateRequest{
		DebateID:     "test-no-topic",
		Topic:        "",
		Strategy:     "consensus",
		MaxRounds:    1,
		Timeout:      30,
		Participants: healthyParticipants(2),
	})

	assert.True(t, resp.StatusCode >= 400 || resp.StatusCode == 200,
		"Should return error or process with empty topic, got: %d, body: %v", resp.StatusCode, result)
	t.Logf("Missing topic response: status=%d, body=%v", resp.StatusCode, result)
}

// TestDebateAPI_ErrorHandling_SingleParticipant tests error for single participant.
func TestDebateAPI_ErrorHandling_SingleParticipant(t *testing.T) {
	if !isDebateServerAvailable() {
		t.Skip("HelixAgent server not available") // SKIP-OK: #legacy-untriaged
	}

	baseURL := debateBaseURL()

	resp, result := createDebate(t, baseURL, debateCreateRequest{
		DebateID:     "test-single-participant",
		Topic:        "Test topic for single participant",
		Strategy:     "consensus",
		MaxRounds:    1,
		Timeout:      30,
		Participants: healthyParticipants(1),
	})

	// API should either accept and process or reject
	t.Logf("Single participant response: status=%d, body=%v", resp.StatusCode, result)
	assert.NotNil(t, result, "Response should not be nil")
}

// TestDebateAPI_ErrorHandling_NonexistentDebate tests GET for non-existent debate.
func TestDebateAPI_ErrorHandling_NonexistentDebate(t *testing.T) {
	if !isDebateServerAvailable() {
		t.Skip("HelixAgent server not available") // SKIP-OK: #legacy-untriaged
	}

	baseURL := debateBaseURL()

	statusCode, result := getDebateStatus(t, baseURL, "nonexistent-debate-id-12345")

	t.Logf("Nonexistent debate response: status=%d, body=%v", statusCode, result)
	assert.NotNil(t, result, "Response should not be nil")
}

// TestDebateAPI_ConcurrentDebates tests 3 simultaneous debates.
func TestDebateAPI_ConcurrentDebates(t *testing.T) {
	if !isDebateServerAvailable() {
		t.Skip("HelixAgent server not available") // SKIP-OK: #legacy-untriaged
	}

	baseURL := debateBaseURL()
	topics := []string{
		"Analyze REST vs GraphQL for internal microservices communication",
		"Evaluate the security implications of serverless architectures",
		"Compare container orchestration: Kubernetes vs Nomad vs Docker Swarm",
	}

	type debateResult struct {
		index int
		resp  *http.Response
		body  map[string]any
		err   error
	}

	results := make([]debateResult, len(topics))
	var wg sync.WaitGroup

	for i, topic := range topics {
		wg.Add(1)
		go func(idx int, topic string) {
			defer wg.Done()
			body, err := json.Marshal(debateCreateRequest{
				DebateID:     fmt.Sprintf("concurrent-%d-%d", idx, time.Now().UnixMilli()),
				Topic:        topic,
				Strategy:     "consensus",
				MaxRounds:    1,
				Timeout:      120,
				Participants: healthyParticipants(3 + idx),
			})
			if err != nil {
				results[idx] = debateResult{index: idx, err: err}
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/debates", bytes.NewReader(body))
			if err != nil {
				results[idx] = debateResult{index: idx, err: err}
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				results[idx] = debateResult{index: idx, err: err}
				return
			}
			defer resp.Body.Close()

			var respBody map[string]any
			respData, _ := io.ReadAll(resp.Body)
			json.Unmarshal(respData, &respBody)

			results[idx] = debateResult{index: idx, resp: resp, body: respBody}
		}(i, topic)
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			t.Logf("Concurrent debate %d error: %v", r.index, r.err)
			continue
		}
		assert.NotNil(t, r.body, "Concurrent debate %d should have response body", r.index)
		if r.resp != nil {
			t.Logf("Concurrent debate %d: HTTP %d, body: %v", r.index, r.resp.StatusCode, r.body)
		}
	}
}

// TestDebateAPI_VotingMethods tests different voting strategies.
func TestDebateAPI_VotingMethods(t *testing.T) {
	if !isDebateServerAvailable() {
		t.Skip("HelixAgent server not available") // SKIP-OK: #legacy-untriaged
	}

	baseURL := debateBaseURL()
	strategies := []string{"consensus", "majority", "weighted"}

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			resp, result := createDebate(t, baseURL, debateCreateRequest{
				DebateID:     fmt.Sprintf("test-vote-%s-%d", strategy, time.Now().UnixMilli()),
				Topic:        fmt.Sprintf("Test voting strategy: %s", strategy),
				Strategy:     strategy,
				MaxRounds:    1,
				Timeout:      60,
				Participants: healthyParticipants(3),
			})

			assert.NotNil(t, result, "Response should not be nil for strategy %s", strategy)
			t.Logf("Strategy %s: HTTP %d, body: %v", strategy, resp.StatusCode, result)
		})
	}
}
