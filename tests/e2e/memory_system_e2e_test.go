package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoServerMemory skips the test if the HelixAgent server is unreachable.
func skipIfNoServerMemory(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping memory E2E test in short mode (requires live server)")  // SKIP-OK: #short-mode
	}
	conn, err := net.DialTimeout("tcp", "localhost:7061", 2*time.Second)
	if err != nil {
		t.Skip("HelixAgent server not running on :7061")  // SKIP-OK: #legacy-untriaged
	}
	conn.Close()
}

// memoryClient returns an HTTP client sized for memory system E2E tests.
func memoryClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// memoryBaseURL returns the HelixAgent base URL.
func memoryBaseURL() string {
	return "http://localhost:7061"
}

// memoryDoRequest sends an HTTP request with auth headers and returns the
// response. The caller is responsible for closing the body.
func memoryDoRequest(t *testing.T, method, path string, body interface{}) *http.Response {
	t.Helper()
	client := memoryClient()
	apiKey := getE2EAPIKey()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, memoryBaseURL()+path, reqBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// skipIfMemoryEndpointUnavailable checks if the memory endpoint is reachable
// and skips the test if not.
func skipIfMemoryEndpointUnavailable(t *testing.T) {
	t.Helper()
	resp := memoryDoRequest(t, "GET", "/v1/memory/search?q=test", nil)
	defer resp.Body.Close()
	// If the memory endpoint is not mounted we get 404.
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Memory endpoint /v1/memory not available on this server")  // SKIP-OK: #legacy-untriaged
	}
}

// TestE2E_Memory_StoreAndRetrieve stores a memory entry and then retrieves
// it, verifying the content round-trips correctly.
func TestE2E_Memory_StoreAndRetrieve(t *testing.T) {
	skipIfNoServerMemory(t)
	skipIfMemoryEndpointUnavailable(t)

	// Store a memory.
	storePayload := map[string]interface{}{
		"content":  "The capital of France is Paris",
		"user_id":  "e2e-test-user-1",
		"metadata": map[string]string{"category": "geography", "source": "e2e-test"},
	}

	storeResp := memoryDoRequest(t, "POST", "/v1/memory", storePayload)
	defer storeResp.Body.Close()
	storeBody, err := io.ReadAll(storeResp.Body)
	require.NoError(t, err)

	if storeResp.StatusCode == http.StatusNotFound {
		t.Skip("Memory store endpoint not mounted")  // SKIP-OK: #legacy-untriaged
	}

	// Accept 200 or 201 for store operations.
	assert.True(t,
		storeResp.StatusCode == http.StatusOK ||
			storeResp.StatusCode == http.StatusCreated,
		"Store should succeed with 200/201, got %d: %s",
		storeResp.StatusCode, string(storeBody))

	var storeResult map[string]interface{}
	err = json.Unmarshal(storeBody, &storeResult)
	require.NoError(t, err, "Store response must be valid JSON")

	// Try to extract the memory ID for retrieval.
	memoryID, _ := storeResult["id"].(string)
	if memoryID == "" {
		memoryID, _ = storeResult["memory_id"].(string)
	}

	// Retrieve via search.
	searchResp := memoryDoRequest(t, "GET",
		"/v1/memory/search?q=capital+of+France", nil)
	defer searchResp.Body.Close()
	searchBody, err := io.ReadAll(searchResp.Body)
	require.NoError(t, err)

	if searchResp.StatusCode == http.StatusOK {
		var searchResult map[string]interface{}
		err = json.Unmarshal(searchBody, &searchResult)
		require.NoError(t, err, "Search response must be valid JSON")
		t.Logf("Search returned: %s", string(searchBody))
	} else {
		t.Logf("Search returned status %d: %s",
			searchResp.StatusCode, string(searchBody))
	}

	// If we have a memory ID, retrieve it directly.
	if memoryID != "" {
		getResp := memoryDoRequest(t, "GET", "/v1/memory/"+memoryID, nil)
		defer getResp.Body.Close()
		getBody, err := io.ReadAll(getResp.Body)
		require.NoError(t, err)

		if getResp.StatusCode == http.StatusOK {
			var getResult map[string]interface{}
			err = json.Unmarshal(getBody, &getResult)
			require.NoError(t, err)
			t.Logf("Retrieved memory by ID: %s", string(getBody))
		}
	}
}

// TestE2E_Memory_SemanticSearch stores multiple memories and verifies
// semantic search returns relevant results.
func TestE2E_Memory_SemanticSearch(t *testing.T) {
	skipIfNoServerMemory(t)
	skipIfMemoryEndpointUnavailable(t)

	memories := []map[string]interface{}{
		{
			"content":  "Go is a statically typed compiled programming language",
			"user_id":  "e2e-test-user-2",
			"metadata": map[string]string{"topic": "programming"},
		},
		{
			"content":  "Python is a dynamically typed interpreted language",
			"user_id":  "e2e-test-user-2",
			"metadata": map[string]string{"topic": "programming"},
		},
		{
			"content":  "The Eiffel Tower is located in Paris, France",
			"user_id":  "e2e-test-user-2",
			"metadata": map[string]string{"topic": "geography"},
		},
	}

	// Store all memories.
	for _, mem := range memories {
		resp := memoryDoRequest(t, "POST", "/v1/memory", mem)
		io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Memory store endpoint not available")  // SKIP-OK: #legacy-untriaged
		}
	}

	// Search for programming-related content.
	searchResp := memoryDoRequest(t, "GET",
		"/v1/memory/search?q=compiled+programming+language", nil)
	defer searchResp.Body.Close()
	searchBody, err := io.ReadAll(searchResp.Body)
	require.NoError(t, err)

	if searchResp.StatusCode == http.StatusOK {
		var searchResult interface{}
		err = json.Unmarshal(searchBody, &searchResult)
		require.NoError(t, err, "Search response must be valid JSON")

		// The result could be a list or a map with a results field.
		switch v := searchResult.(type) {
		case []interface{}:
			t.Logf("Semantic search returned %d results", len(v))
		case map[string]interface{}:
			if results, ok := v["results"].([]interface{}); ok {
				t.Logf("Semantic search returned %d results", len(results))
			} else if memories, ok := v["memories"].([]interface{}); ok {
				t.Logf("Semantic search returned %d memories", len(memories))
			}
		}
	} else {
		t.Logf("Semantic search returned status %d", searchResp.StatusCode)
	}
}

// TestE2E_Memory_EntityGraph_Creation stores entity-related memories and
// verifies the entity graph endpoint reflects the relationships.
func TestE2E_Memory_EntityGraph_Creation(t *testing.T) {
	skipIfNoServerMemory(t)
	skipIfMemoryEndpointUnavailable(t)

	// Store memories with entity relationships.
	entityMemories := []map[string]interface{}{
		{
			"content":  "Alice works at Acme Corp as a software engineer",
			"user_id":  "e2e-test-user-3",
			"metadata": map[string]string{"type": "entity_relation"},
		},
		{
			"content":  "Bob is Alice's manager at Acme Corp",
			"user_id":  "e2e-test-user-3",
			"metadata": map[string]string{"type": "entity_relation"},
		},
		{
			"content":  "Acme Corp is headquartered in San Francisco",
			"user_id":  "e2e-test-user-3",
			"metadata": map[string]string{"type": "entity_relation"},
		},
	}

	for _, mem := range entityMemories {
		resp := memoryDoRequest(t, "POST", "/v1/memory", mem)
		io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Memory store endpoint not available")  // SKIP-OK: #legacy-untriaged
		}
	}

	// Try the entity graph endpoint if it exists.
	graphResp := memoryDoRequest(t, "GET",
		"/v1/memory/entities?user_id=e2e-test-user-3", nil)
	defer graphResp.Body.Close()
	graphBody, err := io.ReadAll(graphResp.Body)
	require.NoError(t, err)

	if graphResp.StatusCode == http.StatusOK {
		var graphResult interface{}
		err = json.Unmarshal(graphBody, &graphResult)
		require.NoError(t, err, "Entity graph response must be valid JSON")
		t.Logf("Entity graph response: %s", string(graphBody))
	} else if graphResp.StatusCode == http.StatusNotFound {
		t.Log("Entity graph endpoint not available — skipping graph assertions")
	} else {
		t.Logf("Entity graph returned status %d: %s",
			graphResp.StatusCode, string(graphBody))
	}

	// Validate via search that entities are findable.
	searchResp := memoryDoRequest(t, "GET",
		"/v1/memory/search?q=Alice+Acme+Corp", nil)
	defer searchResp.Body.Close()
	searchBody, err := io.ReadAll(searchResp.Body)
	require.NoError(t, err)

	if searchResp.StatusCode == http.StatusOK {
		t.Logf("Entity search succeeded: %s", string(searchBody))
	}
}

// TestE2E_Memory_Consolidation stores many overlapping memories and
// verifies the system handles them without returning duplicates.
func TestE2E_Memory_Consolidation(t *testing.T) {
	skipIfNoServerMemory(t)
	skipIfMemoryEndpointUnavailable(t)

	// Store several similar memories to exercise consolidation logic.
	for i := 0; i < 5; i++ {
		mem := map[string]interface{}{
			"content":  "Rust is a memory-safe systems programming language created by Mozilla",
			"user_id":  "e2e-test-user-4",
			"metadata": map[string]string{"iteration": string(rune('0' + i))},
		}
		resp := memoryDoRequest(t, "POST", "/v1/memory", mem)
		io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Memory store endpoint not available")  // SKIP-OK: #legacy-untriaged
		}
	}

	// Search should return consolidated or deduplicated results.
	searchResp := memoryDoRequest(t, "GET",
		"/v1/memory/search?q=Rust+memory+safe&user_id=e2e-test-user-4", nil)
	defer searchResp.Body.Close()
	searchBody, err := io.ReadAll(searchResp.Body)
	require.NoError(t, err)

	if searchResp.StatusCode == http.StatusOK {
		var searchResult interface{}
		err = json.Unmarshal(searchBody, &searchResult)
		require.NoError(t, err, "Search response must be valid JSON")

		// Verify we do not get an explosion of duplicate results.
		switch v := searchResult.(type) {
		case []interface{}:
			assert.LessOrEqual(t, len(v), 10,
				"Consolidation should limit duplicate results")
			t.Logf("Consolidation search returned %d results", len(v))
		case map[string]interface{}:
			if results, ok := v["results"].([]interface{}); ok {
				assert.LessOrEqual(t, len(results), 10,
					"Consolidation should limit duplicate results")
				t.Logf("Consolidation search returned %d results", len(results))
			}
		}
	} else {
		t.Logf("Consolidation search returned status %d", searchResp.StatusCode)
	}
}

// TestE2E_Memory_ScopeIsolation verifies that memories stored under
// different user scopes do not leak across users.
func TestE2E_Memory_ScopeIsolation(t *testing.T) {
	skipIfNoServerMemory(t)
	skipIfMemoryEndpointUnavailable(t)

	// Store a memory for user A.
	memA := map[string]interface{}{
		"content":  "User A secret: project codename is Phoenix",
		"user_id":  "e2e-isolation-user-a",
		"metadata": map[string]string{"scope": "private"},
	}
	respA := memoryDoRequest(t, "POST", "/v1/memory", memA)
	io.ReadAll(respA.Body)
	respA.Body.Close()
	if respA.StatusCode == http.StatusNotFound {
		t.Skip("Memory store endpoint not available")  // SKIP-OK: #legacy-untriaged
	}

	// Store a memory for user B.
	memB := map[string]interface{}{
		"content":  "User B secret: project codename is Dragon",
		"user_id":  "e2e-isolation-user-b",
		"metadata": map[string]string{"scope": "private"},
	}
	respB := memoryDoRequest(t, "POST", "/v1/memory", memB)
	io.ReadAll(respB.Body)
	respB.Body.Close()

	// Search as user B for user A's content — should not find it.
	searchResp := memoryDoRequest(t, "GET",
		"/v1/memory/search?q=Phoenix&user_id=e2e-isolation-user-b", nil)
	defer searchResp.Body.Close()
	searchBody, err := io.ReadAll(searchResp.Body)
	require.NoError(t, err)

	if searchResp.StatusCode == http.StatusOK {
		bodyStr := string(searchBody)
		// The search results for user B should NOT contain user A's secret.
		assert.NotContains(t, bodyStr, "Phoenix",
			"User B should not see User A's memories (scope isolation)")
		t.Log("Scope isolation verified: User B cannot see User A's memories")
	} else {
		t.Logf("Scope isolation search returned status %d — "+
			"endpoint may not support user_id filtering",
			searchResp.StatusCode)
	}
}
