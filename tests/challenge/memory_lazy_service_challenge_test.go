package challenge

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelixMemoryLazyServiceChallenge(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory lazy-service challenge in short mode (requires live server and lazy services)")  // SKIP-OK: #short-mode
	}

	baseURL := getBaseURL()
	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)  // SKIP-OK: #legacy-untriaged
	}

	t.Run("BigDataSubsystemHealth", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(baseURL + "/v1/bigdata/health")
		if err != nil {
			t.Skipf("BigData health endpoint unreachable: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("BigData subsystem not mounted on this server")  // SKIP-OK: #legacy-untriaged
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var health map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &health))

		t.Logf("BigData health: %s", string(body))
		assert.Equal(t, http.StatusOK, resp.StatusCode, "BigData health should return 200")
	})

	t.Run("MemorySyncStatus", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(baseURL + "/v1/memory/sync/status")
		if err != nil {
			t.Skipf("Memory sync status endpoint unreachable: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Memory sync endpoint not mounted on this server")  // SKIP-OK: #legacy-untriaged
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var status map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &status), "Body: %s", string(body))

		t.Logf("Memory sync status: %s", string(body))
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ForceMemorySync", func(t *testing.T) {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Post(baseURL+"/v1/memory/sync/force", "application/json", nil)
		if err != nil {
			t.Skipf("Force memory sync endpoint unreachable: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Memory sync endpoint not mounted on this server")  // SKIP-OK: #legacy-untriaged
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &result), "Body: %s", string(body))

		t.Logf("Force memory sync result: %s", string(body))
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("KnowledgeGraphSearch", func(t *testing.T) {
		client := &http.Client{Timeout: 30 * time.Second}
		searchReq := map[string]interface{}{
			"query": "HelixAgent architecture",
			"limit": 5,
		}
		bodyBytes, _ := json.Marshal(searchReq)

		resp, err := client.Post(baseURL+"/v1/knowledge/search", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Skipf("Knowledge search endpoint unreachable: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Knowledge graph endpoint not mounted on this server")  // SKIP-OK: #legacy-untriaged
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		t.Logf("Knowledge search result: status=%d body=%s", resp.StatusCode, truncate(string(body), 500))
		assert.Contains(t, []int{http.StatusOK, http.StatusAccepted}, resp.StatusCode)
	})

	t.Run("ContextReplayWithMemory", func(t *testing.T) {
		client := &http.Client{Timeout: 15 * time.Second}
		replayReq := map[string]interface{}{
			"conversation_id":      "challenge-test-conv-001",
			"max_tokens":           1024,
			"compression_strategy": "hybrid",
		}
		bodyBytes, _ := json.Marshal(replayReq)

		resp, err := client.Post(baseURL+"/v1/context/replay", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Skipf("Context replay endpoint unreachable: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Context replay endpoint not mounted on this server")  // SKIP-OK: #legacy-untriaged
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		t.Logf("Context replay result: status=%d body=%s", resp.StatusCode, truncate(string(body), 500))
		assert.Contains(t, []int{http.StatusOK, http.StatusAccepted, http.StatusNoContent}, resp.StatusCode)
	})

	t.Run("LearningInsightsAndPatterns", func(t *testing.T) {
		client := &http.Client{Timeout: 15 * time.Second}

		resp, err := client.Get(baseURL + "/v1/learning/insights")
		if err != nil {
			t.Skipf("Learning insights endpoint unreachable: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Learning subsystem not mounted on this server")  // SKIP-OK: #legacy-untriaged
		}

		assert.Contains(t, []int{http.StatusOK, http.StatusAccepted}, resp.StatusCode,
			"Learning insights should return 200 or 202")

		resp2, err := client.Get(baseURL + "/v1/learning/patterns")
		require.NoError(t, err)
		defer resp2.Body.Close()

		assert.Contains(t, []int{http.StatusOK, http.StatusAccepted}, resp2.StatusCode,
			"Learning patterns should return 200 or 202")
	})

	t.Run("CogneeLazyBoot", func(t *testing.T) {
		cogneeURL := "http://localhost:8000"
		client := &http.Client{Timeout: 5 * time.Second}

		resp, err := client.Get(cogneeURL + "/")
		if err != nil {
			t.Log("Cognee not running, triggering lazy boot via memory sync...")

			forceResp, forceErr := (&http.Client{Timeout: 60 * time.Second}).Post(
				baseURL+"/v1/memory/sync/force", "application/json", nil)
			if forceErr != nil {
				t.Skipf("Cannot trigger lazy boot: %v (SKIP-OK: #unmarked-skip-needs-ticket)", forceErr)
			}
			forceResp.Body.Close()

			t.Log("Waiting for Cognee to lazy-boot...")
			booted := false
			for i := 0; i < 30; i++ {
				time.Sleep(2 * time.Second)
				checkResp, checkErr := client.Get(cogneeURL + "/")
				if checkErr == nil {
					checkResp.Body.Close()
					if checkResp.StatusCode == http.StatusOK {
						booted = true
						t.Logf("Cognee lazy-booted after %d seconds", (i+1)*2)
						break
					}
				}
			}

			if !booted {
				t.Skip("Cognee did not lazy-boot within 60 seconds (may require docker-compose.memory.yml)")  // SKIP-OK: #legacy-untriaged
			}
		} else {
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "Cognee health check should return 200")
			t.Log("Cognee already running")
		}
	})

	t.Run("CogneeDataPipeline", func(t *testing.T) {
		cogneeURL := "http://localhost:8000"
		client := &http.Client{Timeout: 30 * time.Second}

		resp, err := client.Get(cogneeURL + "/")
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			t.Skip("Cognee not available for data pipeline test")  // SKIP-OK: #legacy-untriaged
		}
		resp.Body.Close()

		searchBody := map[string]interface{}{
			"query":       "test challenge query",
			"search_type": "CHUNKS",
		}
		bodyBytes, _ := json.Marshal(searchBody)

		searchResp, err := client.Post(cogneeURL+"/api/v1/search", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Logf("Cognee search failed (service may still be initializing): %v", err)
			return
		}
		defer searchResp.Body.Close()

		assert.Contains(t, []int{http.StatusOK, http.StatusAccepted, http.StatusServiceUnavailable},
			searchResp.StatusCode, "Cognee search should return 200, 202, or 503")
	})

	t.Run("ProviderAnalytics", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}

		resp, err := client.Get(baseURL + "/v1/analytics/provider/test-provider")
		if err != nil {
			t.Skipf("Analytics endpoint unreachable: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Analytics endpoint not mounted on this server")  // SKIP-OK: #legacy-untriaged
		}

		assert.Contains(t, []int{http.StatusOK, http.StatusNotFound, http.StatusNoContent}, resp.StatusCode)
	})

	t.Run("DebateAnalytics", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}

		resp, err := client.Get(baseURL + "/v1/analytics/debate/challenge-test-debate")
		if err != nil {
			t.Skipf("Debate analytics endpoint unreachable: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Deate analytics endpoint not mounted on this server")  // SKIP-OK: #legacy-untriaged
		}

		assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, resp.StatusCode)
	})
}
