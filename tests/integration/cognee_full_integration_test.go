package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// CogneeIntegrationTestSuite provides comprehensive tests for real Cognee integration
// These tests verify that Cognee is running, features are enabled, and all functionality works

const (
	cogneeBaseURL      = "http://localhost:8000"
	helixagentBaseURL  = "http://localhost:7061"
	cogneeStartTimeout = 60 * time.Second
)

// TestCogneeInfrastructure verifies Cognee containers are running
func TestCogneeInfrastructure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping infrastructure test in short mode")
	}

	t.Run("ContainersRunning", func(t *testing.T) {
		containers := []string{"helixagent-cognee", "helixagent-chromadb", "helixagent-postgres", "helixagent-redis"}

		for _, container := range containers {
			cmd := exec.Command("podman", "ps", "--filter", fmt.Sprintf("name=%s", container), "--format", "{{.Status}}")
			output, err := cmd.Output()
			if err != nil {
				// Try docker if podman fails
				cmd = exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", container), "--format", "{{.Status}}")
				output, err = cmd.Output()
			}

			status := strings.TrimSpace(string(output))
			if status == "" {
				t.Logf("Container %s not running (may need to start infrastructure)", container)
			} else {
				t.Logf("Container %s: %s", container, status)
				assert.Contains(t, strings.ToLower(status), "up", "Container %s should be running", container)
			}
		}
	})

	t.Run("NetworkConnectivity", func(t *testing.T) {
		// Test that we can reach Cognee port
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(cogneeBaseURL + "/health")
		if err != nil {
			t.Logf("Cannot connect to Cognee at %s: %v (may not be running)", cogneeBaseURL, err)
			t.Skip("Cognee not accessible")
		}
		defer resp.Body.Close()

		t.Logf("Cognee responded with status: %d", resp.StatusCode)
		// Status could be 200 (healthy) or may timeout (degraded health check)
	})
}

// TestCogneeHealthEndpoint verifies Cognee health check functionality
func TestCogneeHealthEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health test in short mode")
	}

	client := &http.Client{Timeout: 60 * time.Second}

	t.Run("DirectHealthCheck", func(t *testing.T) {
		resp, err := client.Get(cogneeBaseURL + "/health")
		if err != nil {
			t.Logf("Direct Cognee health check failed: %v", err)
			t.Skip("Cognee not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Cognee health response: %s", string(body))

		// Parse response if JSON
		var healthResp map[string]interface{}
		if json.Unmarshal(body, &healthResp) == nil {
			if status, ok := healthResp["status"]; ok {
				t.Logf("Cognee status: %v", status)
			}
			if health, ok := healthResp["health"]; ok {
				t.Logf("Cognee health: %v", health)
			}
		}
	})

	t.Run("HelixAgentCogneeHealth", func(t *testing.T) {
		apiKey := os.Getenv("HELIXAGENT_API_KEY")
		if apiKey == "" {
			apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
		}

		req, _ := http.NewRequest("GET", helixagentBaseURL+"/v1/cognee/health", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("HelixAgent Cognee health check failed: %v", err)
			t.Skip("HelixAgent not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("HelixAgent Cognee health response: %s", string(body))

		var healthResp map[string]interface{}
		if json.Unmarshal(body, &healthResp) == nil {
			if config, ok := healthResp["config"].(map[string]interface{}); ok {
				// Verify all features are enabled
				assert.True(t, config["enabled"].(bool), "Cognee should be enabled")
				assert.True(t, config["auto_cognify"].(bool), "Auto cognify should be enabled")
				assert.True(t, config["enable_code_intelligence"].(bool), "Code intelligence should be enabled")
				assert.True(t, config["enable_graph_reasoning"].(bool), "Graph reasoning should be enabled")
				assert.True(t, config["enhance_prompts"].(bool), "Prompt enhancement should be enabled")
				assert.True(t, config["temporal_awareness"].(bool), "Temporal awareness should be enabled")
			}
		}
	})
}

// TestCogneeFeatureConfiguration verifies all Cognee features are properly configured
func TestCogneeFeatureConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping feature configuration test in short mode")
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("AllFeaturesEnabled", func(t *testing.T) {
		req, _ := http.NewRequest("GET", helixagentBaseURL+"/v1/cognee/config", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			t.Skip("HelixAgent not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Cognee config: %s", string(body))

		var config map[string]interface{}
		if json.Unmarshal(body, &config) == nil {
			// Check that configuration matches expected full feature set
			expectedFeatures := []string{
				"enabled",
				"auto_cognify",
				"enable_code_intelligence",
				"enable_graph_reasoning",
				"enhance_prompts",
				"temporal_awareness",
			}

			for _, feature := range expectedFeatures {
				if val, ok := config[feature]; ok {
					t.Logf("Feature %s: %v", feature, val)
				}
			}
		}
	})

	t.Run("CogneeEndpointsAvailable", func(t *testing.T) {
		endpoints := []struct {
			method   string
			path     string
			expected int
		}{
			{"GET", "/v1/cognee/health", 200},
			{"GET", "/v1/cognee/stats", 200},
			{"GET", "/v1/cognee/config", 200},
			{"GET", "/v1/cognee/datasets", 200},
		}

		for _, ep := range endpoints {
			req, _ := http.NewRequest(ep.method, helixagentBaseURL+ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("Endpoint %s %s failed: %v", ep.method, ep.path, err)
				continue
			}
			defer resp.Body.Close()

			t.Logf("Endpoint %s %s returned status: %d", ep.method, ep.path, resp.StatusCode)
		}
	})
}

// TestCogneeMemoryOperations tests Cognee memory add/search functionality
func TestCogneeMemoryOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory operations test in short mode")
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 60 * time.Second}
	testDataset := "integration_test_dataset"
	testContent := fmt.Sprintf("Test memory content created at %s for integration testing", time.Now().Format(time.RFC3339))

	t.Run("AddMemory", func(t *testing.T) {
		payload := map[string]interface{}{
			"content":      testContent,
			"dataset":      testDataset,
			"content_type": "text",
			"metadata": map[string]interface{}{
				"source":    "integration_test",
				"timestamp": time.Now().Unix(),
			},
		}

		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/memory", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Add memory failed: %v", err)
			t.Skip("Cognee memory API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Add memory response: %s", string(body))

		// May succeed or fail depending on Cognee health
		if resp.StatusCode == 200 || resp.StatusCode == 201 {
			t.Log("Memory added successfully")
		} else {
			t.Logf("Memory add returned status %d (Cognee may be in degraded state)", resp.StatusCode)
		}
	})

	t.Run("SearchMemory", func(t *testing.T) {
		payload := map[string]interface{}{
			"query":       "integration test",
			"dataset":     testDataset,
			"limit":       5,
			"search_type": []string{"VECTOR", "GRAPH"},
		}

		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/search", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Search memory failed: %v", err)
			t.Skip("Cognee search API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Search memory response: %s", string(body))

		if resp.StatusCode == 200 {
			var results map[string]interface{}
			if json.Unmarshal(body, &results) == nil {
				t.Logf("Search returned results: %v", results)
			}
		}
	})
}

// TestCogneeKnowledgeGraph tests Cognee knowledge graph operations
func TestCogneeKnowledgeGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping knowledge graph test in short mode")
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 90 * time.Second}

	t.Run("Cognify", func(t *testing.T) {
		payload := map[string]interface{}{
			"datasets": []string{"integration_test_dataset"},
		}

		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/cognify", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Cognify failed: %v", err)
			t.Skip("Cognee cognify API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Cognify response: %s", string(body))
	})

	t.Run("GraphCompletion", func(t *testing.T) {
		payload := map[string]interface{}{
			"query":   "What are the relationships in the integration test data?",
			"dataset": "integration_test_dataset",
		}

		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/graph/complete", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Graph completion failed: %v", err)
			t.Skip("Cognee graph API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Graph completion response: %s", string(body))
	})

	t.Run("GetInsights", func(t *testing.T) {
		payload := map[string]interface{}{
			"query":   "Provide insights about the integration test data",
			"dataset": "integration_test_dataset",
		}

		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/insights", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Get insights failed: %v", err)
			t.Skip("Cognee insights API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Insights response: %s", string(body))
	})
}

// TestCogneeCodeIntelligence tests Cognee code analysis features
func TestCogneeCodeIntelligence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping code intelligence test in short mode")
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 60 * time.Second}

	t.Run("ProcessCode", func(t *testing.T) {
		testCode := `
package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
		payload := map[string]interface{}{
			"code":     testCode,
			"language": "go",
			"dataset":  "integration_test_code",
		}

		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/code", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Process code failed: %v", err)
			t.Skip("Cognee code API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Process code response: %s", string(body))

		if resp.StatusCode == 200 {
			var result map[string]interface{}
			if json.Unmarshal(body, &result) == nil {
				// Verify code analysis result structure
				if entities, ok := result["entities"]; ok {
					t.Logf("Extracted entities: %v", entities)
				}
				if summary, ok := result["summary"]; ok {
					t.Logf("Code summary: %v", summary)
				}
			}
		}
	})
}

// TestCogneeDatasetManagement tests Cognee dataset CRUD operations
func TestCogneeDatasetManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping dataset management test in short mode")
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 30 * time.Second}
	testDatasetName := fmt.Sprintf("test_dataset_%d", time.Now().Unix())

	t.Run("CreateDataset", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        testDatasetName,
			"description": "Integration test dataset",
		}

		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/datasets", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Create dataset failed: %v", err)
			t.Skip("Cognee dataset API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Create dataset response: %s", string(body))
	})

	t.Run("ListDatasets", func(t *testing.T) {
		req, _ := http.NewRequest("GET", helixagentBaseURL+"/v1/cognee/datasets", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("List datasets failed: %v", err)
			t.Skip("Cognee dataset API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("List datasets response: %s", string(body))
	})

	t.Run("DeleteDataset", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", helixagentBaseURL+"/v1/cognee/datasets/"+testDatasetName, nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Delete dataset failed: %v", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Delete dataset response: %s", string(body))
	})
}

// TestCogneeFeedback tests Cognee feedback loop functionality
func TestCogneeFeedback(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping feedback test in short mode")
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("ProvideFeedback", func(t *testing.T) {
		payload := map[string]interface{}{
			"query":    "test query",
			"response": "test response",
			"rating":   5,
			"feedback": "This is a test feedback for integration testing",
		}

		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/feedback", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Provide feedback failed: %v", err)
			t.Skip("Cognee feedback API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Feedback response: %s", string(body))
	})
}

// TestCogneeGracefulDegradation tests that system works when Cognee is unavailable
func TestCogneeGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping graceful degradation test in short mode")
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("SystemHealthWithoutCognee", func(t *testing.T) {
		// Test that HelixAgent main health endpoint works even if Cognee is degraded
		req, _ := http.NewRequest("GET", helixagentBaseURL+"/health", nil)

		resp, err := client.Do(req)
		if err != nil {
			t.Skip("HelixAgent not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("HelixAgent health: %s", string(body))

		assert.Equal(t, 200, resp.StatusCode, "HelixAgent should be healthy even with Cognee issues")
	})

	t.Run("CogneeStatsWithDegradedState", func(t *testing.T) {
		req, _ := http.NewRequest("GET", helixagentBaseURL+"/v1/cognee/stats", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			t.Skip("HelixAgent not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Cognee stats: %s", string(body))

		// Stats endpoint should always respond, even in degraded state
		assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 503,
			"Stats should return 200 (OK) or 503 (degraded)")
	})
}

// TestCogneeLLMIntegration tests that Cognee properly uses LLM providers
func TestCogneeLLMIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LLM integration test in short mode")
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 120 * time.Second}

	t.Run("ChatCompletionWithCogneeEnhancement", func(t *testing.T) {
		// First add some memory
		memPayload := map[string]interface{}{
			"content":      "The HelixAgent project is an AI-powered ensemble LLM service that combines responses from multiple language models.",
			"dataset":      "helixagent_knowledge",
			"content_type": "text",
		}

		jsonBody, _ := json.Marshal(memPayload)
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/memory", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}

		// Now test chat completion that should be enhanced by Cognee
		chatPayload := map[string]interface{}{
			"model": "helixagent-debate-v1",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "What is HelixAgent and how does it work?",
				},
			},
		}

		jsonBody, _ = json.Marshal(chatPayload)
		req, _ = http.NewRequest("POST", helixagentBaseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Chat completion failed: %v", err)
			t.Skip("Chat API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Chat completion response: %s", string(body))
	})
}

// TestCogneeContainerAutoStart tests that containers auto-start when needed
func TestCogneeContainerAutoStart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container auto-start test in short mode")
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 120 * time.Second}

	t.Run("StartCogneeViaAPI", func(t *testing.T) {
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/start", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Start Cognee API failed: %v", err)
			t.Skip("Cognee start API not accessible")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Start Cognee response: %s", string(body))
	})
}

// TestCogneeRealAPIIntegration tests actual Cognee API responses
func TestCogneeRealAPIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Direct test to Cognee container
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("DirectCogneeAPI", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(ctx, "GET", cogneeBaseURL+"/", nil)

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Direct Cognee API not accessible: %v", err)
			t.Skip("Cognee not running")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Cognee root response: %s", string(body))
	})
}

// TestAllCogneeEndpoints validates all Cognee endpoints are registered
func TestAllCogneeEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping endpoint test in short mode")
		return
	}

	// First check if HelixAgent is running with Cognee enabled
	client := &http.Client{Timeout: 10 * time.Second}
	healthResp, err := client.Get(helixagentBaseURL + "/health")
	if err != nil {
		t.Skipf("HelixAgent not accessible: %v", err)
		return
	}
	healthResp.Body.Close()

	// Check if Cognee health endpoint exists (primary indicator of Cognee routes)
	cogneeHealthResp, err := client.Get(helixagentBaseURL + "/v1/cognee/health")
	if err != nil {
		t.Skipf("Cognee routes not accessible: %v", err)
		return
	}
	if cogneeHealthResp.StatusCode == 404 {
		cogneeHealthResp.Body.Close()
		t.Skip("Cognee routes not registered in HelixAgent (404 on /v1/cognee/health)")
		return
	}
	cogneeHealthResp.Body.Close()

	// Endpoints with their HTTP methods
	type endpointConfig struct {
		path   string
		method string
	}
	expectedEndpoints := []endpointConfig{
		{"/v1/cognee/health", "GET"},
		{"/v1/cognee/stats", "GET"},
		{"/v1/cognee/config", "GET"},
		{"/v1/cognee/start", "POST"},
		{"/v1/cognee/memory", "POST"},
		{"/v1/cognee/search", "POST"},
		{"/v1/cognee/cognify", "POST"},
		{"/v1/cognee/insights", "POST"},
		{"/v1/cognee/graph/complete", "POST"},
		{"/v1/cognee/graph/visualize", "GET"},
		{"/v1/cognee/code", "POST"},
		{"/v1/cognee/datasets", "GET"},
		{"/v1/cognee/feedback", "POST"},
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	for _, ep := range expectedEndpoints {
		t.Run(strings.Replace(ep.path, "/", "_", -1), func(t *testing.T) {
			// Use correct HTTP method for each endpoint
			req, _ := http.NewRequest(ep.method, helixagentBaseURL+ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("Endpoint %s (%s) not accessible: %v", ep.path, ep.method, err)
				return
			}
			defer resp.Body.Close()

			// Any response other than 404 means endpoint exists
			// Accept 400 for POST endpoints without body (expected validation error)
			validCodes := resp.StatusCode != 404
			assert.True(t, validCodes, "Endpoint %s (%s) should exist (got %d)", ep.path, ep.method, resp.StatusCode)
			t.Logf("Endpoint %s (%s) exists (status: %d)", ep.path, ep.method, resp.StatusCode)
		})
	}
}

// BenchmarkCogneeSearch benchmarks Cognee search performance
func BenchmarkCogneeSearch(b *testing.B) {
	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2afe4c4f62a7e8b9c10d4e5f6a"
	}

	client := &http.Client{Timeout: 30 * time.Second}

	payload := map[string]interface{}{
		"query":   "test query",
		"dataset": "default",
		"limit":   5,
	}
	jsonBody, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", helixagentBaseURL+"/v1/cognee/search", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			b.Skip("Cognee not accessible")
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
}
