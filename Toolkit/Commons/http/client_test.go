package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit/common/ratelimit"
)

func TestNewClient(t *testing.T) {
	config := ClientConfig{
		BaseURL:     "https://api.example.com",
		Timeout:     30 * time.Second,
		MaxRetries:  5,
		BaseBackoff: 2 * time.Second,
		RateLimit: &ratelimit.TokenBucketConfig{
			Capacity:   10,
			RefillRate: 1,
		},
	}

	client := NewClient(config)

	if client.baseURL != "https://api.example.com" {
		t.Errorf("Expected baseURL 'https://api.example.com', got %s", client.baseURL)
	}

	if client.maxRetries != 5 {
		t.Errorf("Expected maxRetries 5, got %d", client.maxRetries)
	}

	if client.baseBackoff != 2*time.Second {
		t.Errorf("Expected baseBackoff 2s, got %v", client.baseBackoff)
	}

	if client.rateLimiter == nil {
		t.Error("Expected rate limiter to be set")
	}
}

func TestNewClient_Defaults(t *testing.T) {
	config := ClientConfig{
		BaseURL: "https://api.example.com",
		Timeout: 30 * time.Second,
	}

	client := NewClient(config)

	if client.maxRetries != 3 {
		t.Errorf("Expected default maxRetries 3, got %d", client.maxRetries)
	}

	if client.baseBackoff != time.Second {
		t.Errorf("Expected default baseBackoff 1s, got %v", client.baseBackoff)
	}

	if client.rateLimiter != nil {
		t.Error("Expected no rate limiter by default")
	}
}

func TestClient_SetAuth(t *testing.T) {
	client := NewClient(ClientConfig{BaseURL: "https://api.example.com"})

	client.SetAuth("Authorization", "Bearer token123")

	if client.authHeader != "Authorization" {
		t.Errorf("Expected authHeader 'Authorization', got %s", client.authHeader)
	}

	if client.authValue != "Bearer token123" {
		t.Errorf("Expected authValue 'Bearer token123', got %s", client.authValue)
	}
}

func TestClient_AddRequestInterceptor(t *testing.T) {
	client := NewClient(ClientConfig{BaseURL: "https://api.example.com"})

	interceptor := func(req *http.Request) error {
		req.Header.Set("X-Test", "test")
		return nil
	}

	client.AddRequestInterceptor(interceptor)

	if len(client.requestInterceptors) != 1 {
		t.Errorf("Expected 1 request interceptor, got %d", len(client.requestInterceptors))
	}
}

func TestClient_AddResponseInterceptor(t *testing.T) {
	client := NewClient(ClientConfig{BaseURL: "https://api.example.com"})

	interceptor := func(resp *http.Response) error {
		return nil
	}

	client.AddResponseInterceptor(interceptor)

	if len(client.responseInterceptors) != 1 {
		t.Errorf("Expected 1 response interceptor, got %d", len(client.responseInterceptors))
	}
}

func TestClient_DoRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/test" {
			t.Errorf("Expected path '/test', got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	var result map[string]string
	err := client.DoRequest(context.Background(), "GET", "/test", nil, &result)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result["message"] != "success" {
		t.Errorf("Expected message 'success', got %s", result["message"])
	}
}

func TestClient_DoRequest_WithPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received": true}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	payload := map[string]string{"key": "value"}
	var result map[string]bool
	err := client.DoRequest(context.Background(), "POST", "/test", payload, &result)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result["received"] {
		t.Error("Expected received to be true")
	}
}

func TestClient_DoRequest_WithAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer token123" {
			t.Errorf("Expected Authorization 'Bearer token123', got %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	client.SetAuth("Authorization", "Bearer token123")

	var result map[string]bool
	err := client.DoRequest(context.Background(), "GET", "/test", nil, &result)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestClient_DoRequest_RequestInterceptor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "custom-value" {
			t.Errorf("Expected X-Custom 'custom-value', got %s", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	client.AddRequestInterceptor(func(req *http.Request) error {
		req.Header.Set("X-Custom", "custom-value")
		return nil
	})

	var result map[string]bool
	err := client.DoRequest(context.Background(), "GET", "/test", nil, &result)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestClient_DoRequest_ResponseInterceptor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	interceptorCalled := false
	client.AddResponseInterceptor(func(resp *http.Response) error {
		interceptorCalled = true
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		return nil
	})

	var result map[string]bool
	err := client.DoRequest(context.Background(), "GET", "/test", nil, &result)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !interceptorCalled {
		t.Error("Expected response interceptor to be called")
	}
}

func TestClient_DoRequest_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	var result map[string]string
	err := client.DoRequest(context.Background(), "GET", "/test", nil, &result)

	if err == nil {
		t.Error("Expected error for bad request")
	}

	if result != nil {
		t.Error("Expected result to be nil for error response")
	}
}

func TestClient_DoRequest_ServerError_Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`server error`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:     server.URL,
		Timeout:     5 * time.Second,
		MaxRetries:  2,
		BaseBackoff: 10 * time.Millisecond, // Short backoff for test
	})

	var result map[string]bool
	err := client.DoRequest(context.Background(), "GET", "/test", nil, &result)

	if err != nil {
		t.Fatalf("Expected no error after retry, got %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls (initial + 1 retry), got %d", callCount)
	}

	if !result["success"] {
		t.Error("Expected success to be true")
	}
}

func TestClient_DoRequest_RateLimit_Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`rate limited`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:     server.URL,
		Timeout:     5 * time.Second,
		MaxRetries:  2,
		BaseBackoff: 10 * time.Millisecond, // Short backoff for test
	})

	var result map[string]bool
	err := client.DoRequest(context.Background(), "GET", "/test", nil, &result)

	if err != nil {
		t.Fatalf("Expected no error after retry, got %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls (initial + 1 retry), got %d", callCount)
	}
}

func TestClient_DoRequest_MaxRetriesExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`server error`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:     server.URL,
		Timeout:     5 * time.Second,
		MaxRetries:  2,
		BaseBackoff: 10 * time.Millisecond,
	})

	var result map[string]string
	err := client.DoRequest(context.Background(), "GET", "/test", nil, &result)

	if err == nil {
		t.Error("Expected error when max retries exceeded")
	}
}

func TestClient_DoRequest_InvalidPayload(t *testing.T) {
	client := NewClient(ClientConfig{
		BaseURL: "https://api.example.com",
		Timeout: 5 * time.Second,
	})

	// Payload that cannot be marshaled
	payload := make(chan int) // channels cannot be marshaled to JSON

	err := client.DoRequest(context.Background(), "POST", "/test", payload, nil)

	if err == nil {
		t.Error("Expected error for invalid payload")
	}
}

func TestClient_DoRequest_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	var result map[string]string
	err := client.DoRequest(context.Background(), "GET", "/test", nil, &result)

	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
}
