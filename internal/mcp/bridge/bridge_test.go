package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	if config.Port != 9000 {
		t.Errorf("expected default port 9000, got %d", config.Port)
	}

	if config.ReadTimeout != 30*time.Second {
		t.Errorf("expected ReadTimeout 30s, got %v", config.ReadTimeout)
	}

	if config.WriteTimeout != 60*time.Second {
		t.Errorf("expected WriteTimeout 60s, got %v", config.WriteTimeout)
	}

	if config.IdleTimeout != 120*time.Second {
		t.Errorf("expected IdleTimeout 120s, got %v", config.IdleTimeout)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	// Test with nil config
	b := New(nil)
	if b == nil {
		t.Fatal("expected non-nil bridge")
	}
	if b.config.Port != 9000 {
		t.Errorf("expected default port 9000, got %d", b.config.Port)
	}

	// Test with custom config
	customConfig := &Config{
		Port:       8080,
		MCPCommand: "echo hello",
	}
	b = New(customConfig)
	if b.config.Port != 8080 {
		t.Errorf("expected port 8080, got %d", b.config.Port)
	}
}

func TestHandleRoot(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.MCPCommand = "echo test"
	b := New(config)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	b.handleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["name"] != "MCP SSE Bridge" {
		t.Errorf("expected name 'MCP SSE Bridge', got %v", response["name"])
	}

	endpoints, ok := response["endpoints"].(map[string]interface{})
	if !ok {
		t.Fatal("expected endpoints map")
	}

	expectedEndpoints := []string{"GET /", "GET /health", "GET /sse", "POST /message"}
	for _, ep := range expectedEndpoints {
		if _, exists := endpoints[ep]; !exists {
			t.Errorf("expected endpoint %s to be documented", ep)
		}
	}
}

func TestHandleMessage_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.MCPCommand = "echo test"
	b := New(config)

	req := httptest.NewRequest(http.MethodGet, "/message", nil)
	w := httptest.NewRecorder()

	b.handleMessage(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleMessage_InvalidJSON(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.MCPCommand = "echo test"
	b := New(config)

	req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	b.handleMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleHealth_NoProcess(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.MCPCommand = "echo test"
	b := New(config)
	// cmd is nil, so process is not running

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	b.handleHealth(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 when no process, got %d", w.Code)
	}
}
