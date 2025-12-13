package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://api.example.com", "test-key")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.baseURL != "https://api.example.com" {
		t.Error("BaseURL not set correctly")
	}
	if client.apiKey != "test-key" {
		t.Error("API key not set correctly")
	}
	if client.retryCount != 3 {
		t.Error("Default retry count not set to 3")
	}
	if client.timeout != 30*time.Second {
		t.Error("Default timeout not set to 30 seconds")
	}
}

func TestClient_SetTimeout(t *testing.T) {
	client := NewClient("https://api.example.com", "")

	newTimeout := 60 * time.Second
	client.SetTimeout(newTimeout)

	if client.timeout != newTimeout {
		t.Error("Timeout not set correctly")
	}
	if client.httpClient.Timeout != newTimeout {
		t.Error("HTTP client timeout not updated")
	}
}

func TestClient_SetRetryCount(t *testing.T) {
	client := NewClient("https://api.example.com", "")

	client.SetRetryCount(5)

	if client.retryCount != 5 {
		t.Error("Retry count not set correctly")
	}
}

func TestClient_Do_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	resp, err := client.Do(context.Background(), "GET", "/", nil, nil)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	if string(body) != "success" {
		t.Errorf("Expected 'success', got '%s'", string(body))
	}
}

func TestClient_Do_RetryOnServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 4 { // Need to fail first 3 attempts (initial + 2 retries), succeed on 4th
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	client.SetRetryCount(3) // Total attempts = 4 (1 initial + 3 retries)

	resp, err := client.Do(context.Background(), "GET", "/", nil, nil)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 4 {
		t.Errorf("Expected 4 attempts, got %d", attempts)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_Do_NoRetryOnClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	client.SetRetryCount(3)

	resp, err := client.Do(context.Background(), "GET", "/", nil, nil)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestClient_Do_WithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "test-value" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	headers := map[string]string{"X-Custom": "test-value"}
	resp, err := client.Do(context.Background(), "GET", "/", nil, headers)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_Do_WithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	resp, err := client.Do(context.Background(), "GET", "/", nil, nil)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_Do_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var data map[string]string
		if err := json.Unmarshal(body, &data); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if data["test"] != "value" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	payload := map[string]string{"test": "value"}
	resp, err := client.Do(context.Background(), "POST", "/", payload, nil)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	resp, err := client.Get(context.Background(), "/", nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	resp, err := client.Post(context.Background(), "/", nil, nil)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_Put(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	resp, err := client.Put(context.Background(), "/", nil, nil)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	resp, err := client.Delete(context.Background(), "/", nil)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_DoRequest_Success(t *testing.T) {
	expectedResponse := map[string]string{"result": "success"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	var result map[string]string
	err := client.DoRequest(context.Background(), "GET", "/", nil, &result)
	if err != nil {
		t.Fatalf("DoRequest failed: %v", err)
	}

	if result["result"] != "success" {
		t.Errorf("Expected result 'success', got '%s'", result["result"])
	}
}

func TestClient_DoRequest_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	var result map[string]string
	err := client.DoRequest(context.Background(), "GET", "/", nil, &result)
	if err == nil {
		t.Fatal("Expected error for bad status code")
	}

	expectedErr := "API request failed with status 400"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestClient_Do_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Delay response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	client.SetTimeout(50 * time.Millisecond) // Short timeout

	_, err := client.Do(context.Background(), "GET", "/", nil, nil)
	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

func TestClient_Do_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	client.SetRetryCount(10) // High retry count

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Do(ctx, "GET", "/", nil, nil)
	if err == nil {
		t.Fatal("Expected context cancellation error")
	}
	if err != context.DeadlineExceeded && err != context.Canceled {
		t.Errorf("Expected context error, got %v", err)
	}
}
