// Package mcp provides chaos tests for the MCP (Model Context Protocol) endpoints.
package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/testutil"
)

const baseURL = "http://localhost:8100"

func checkAvailable(url string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// TestMCPChaos_InvalidToolRequests sends invalid tool call requests to the MCP endpoint.
func TestMCPChaos_InvalidToolRequests(t *testing.T) {
	testutil.RequireServer(t)

	invalidPayloads := []string{
		`{"jsonrpc": "2.0", "method": "nonexistent_tool", "params": {}, "id": 1}`,
		`{"jsonrpc": "2.0", "method": "", "params": {}, "id": 2}`,
		`{"jsonrpc": "1.0", "method": "tools/call", "params": {}, "id": 3}`,
		`{"jsonrpc": "2.0", "method": "tools/call", "params": null, "id": 4}`,
		`{invalid json}`,
		`{"jsonrpc": "2.0"}`,
		`null`,
		`[]`,
		`{"jsonrpc": "2.0", "method": "tools/call", "params": {"name": ""}, "id": 5}`,
	}

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var processedCount, errorCount int64
	done := make(chan struct{})

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					payload := invalidPayloads[rand.Intn(len(invalidPayloads))]
					req, _ := http.NewRequest("POST", baseURL+"/v1/mcp", bytes.NewBuffer([]byte(payload)))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")

					resp, err := client.Do(req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					resp.Body.Close()
					atomic.AddInt64(&processedCount, 1)
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Invalid MCP requests: %d got responses, %d connection errors", processedCount, errorCount)
	assert.True(t, checkAvailable(baseURL), "Server must survive invalid MCP request chaos")
}

// TestMCPChaos_ConcurrentToolCalls sends concurrent valid tool call requests.
func TestMCPChaos_ConcurrentToolCalls(t *testing.T) {
	testutil.RequireServer(t)

	toolNames := []string{
		"filesystem", "memory", "sequential-thinking", "everything",
		"search", "fetch", "code-review", "format",
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var wg sync.WaitGroup
	var successCount, failCount int64
	done := make(chan struct{})

	for i := 0; i < 25; i++ {
		wg.Add(1)
		id := i
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					toolName := toolNames[rand.Intn(len(toolNames))]
					payload := map[string]interface{}{
						"jsonrpc": "2.0",
						"method":  "tools/call",
						"params": map[string]interface{}{
							"name":      toolName,
							"arguments": map[string]string{"query": fmt.Sprintf("test-%d", id)},
						},
						"id": rand.Int(),
					}
					data, _ := json.Marshal(payload)

					req, _ := http.NewRequest("POST", baseURL+"/v1/mcp", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")

					resp, err := client.Do(req)
					if err != nil {
						atomic.AddInt64(&failCount, 1)
						continue
					}
					resp.Body.Close()
					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Concurrent MCP tool calls: %d responses, %d errors", successCount, failCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle concurrent MCP chaos")
}

// TestMCPChaos_MalformedJSONRPC sends malformed JSON-RPC 2.0 payloads.
func TestMCPChaos_MalformedJSONRPC(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var processedCount int64
	var wg sync.WaitGroup

	malformedPayloads := generateMalformedJSONRPC()

	for _, payload := range malformedPayloads {
		wg.Add(1)
		p := payload
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("POST", baseURL+"/v1/mcp", bytes.NewBuffer([]byte(p)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-chaos-key")

			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				atomic.AddInt64(&processedCount, 1)
			}
		}()
	}

	wg.Wait()
	t.Logf("Malformed JSON-RPC: %d requests got server responses", processedCount)
	assert.True(t, checkAvailable(baseURL), "Server must survive malformed JSON-RPC chaos")
}

// TestMCPChaos_LargeToolPayloads sends tool requests with very large argument payloads.
func TestMCPChaos_LargeToolPayloads(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 15 * time.Second}
	sizes := []int{1024, 10 * 1024, 100 * 1024}

	for _, size := range sizes {
		largeArg := make([]byte, size)
		for i := range largeArg {
			largeArg[i] = 'a'
		}

		payload := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "memory",
				"arguments": map[string]string{"content": string(largeArg)},
			},
			"id": 1,
		}
		data, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", baseURL+"/v1/mcp", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-chaos-key")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Large MCP payload (%d bytes) error: %v", size, err)
			continue
		}
		resp.Body.Close()
		t.Logf("Large MCP payload (%d bytes): status %d", size, resp.StatusCode)
	}

	assert.True(t, checkAvailable(baseURL), "Server should handle large MCP payloads")
}

func generateMalformedJSONRPC() []string {
	return []string{
		`{"jsonrpc": "2.0", "method": null, "id": 1}`,
		`{"jsonrpc": "2.0", "method": 123, "params": {}, "id": 1}`,
		`{"jsonrpc": "2.0", "method": "tools/call", "params": "string_not_object", "id": 1}`,
		`{"jsonrpc": "2.0", "method": "tools/call", "params": [], "id": 1}`,
		`{"jsonrpc": "2.0", "method": "tools/call", "params": {"name": null}, "id": 1}`,
		`{"jsonrpc": "2.0", "method": "tools/call", "id": null}`,
		`{"jsonrpc": "2.0", "method": "tools/call", "params": {}, "id": "not-a-number"}`,
		`{"jsonrpc": "2.0", "method": "tools/call", "params": {"name": true}, "id": 1}`,
		`{}`,
		`{"jsonrpc": "2.0", "method": "` + string(make([]byte, 10000)) + `", "id": 1}`,
	}
}
