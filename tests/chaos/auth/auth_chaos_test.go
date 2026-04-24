// Package auth provides chaos tests for the authentication system.
package auth

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

// TestAuthChaos_InvalidTokenFlood floods the server with invalid JWT tokens.
func TestAuthChaos_InvalidTokenFlood(t *testing.T) {
	testutil.RequireServer(t)

	invalidTokens := []string{
		"",
		"Bearer ",
		"Bearer invalid.token.here",
		"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.invalid",
		"Bearer " + randStr(256),
		"NotBearer validlooking",
		"Bearer eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiIxMjM0NTY3ODkwIn0.",
		fmt.Sprintf("Bearer %s.%s.%s", randStr(36), randStr(48), randStr(43)),
		"Token " + randStr(32),
		"Basic dXNlcjpwYXNzd29yZA==",
	}

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var rejectedCount, errorCount int64
	done := make(chan struct{})

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					token := invalidTokens[rand.Intn(len(invalidTokens))]
					body := map[string]interface{}{
						"model":    "helixagent",
						"messages": []map[string]string{{"role": "user", "content": "test"}},
					}
					data, _ := json.Marshal(body)

					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					if token != "" {
						req.Header.Set("Authorization", token)
					}

					resp, err := client.Do(req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					resp.Body.Close()
					if resp.StatusCode == 401 || resp.StatusCode == 403 {
						atomic.AddInt64(&rejectedCount, 1)
					}
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Invalid token flood: %d rejected (401/403), %d connection errors", rejectedCount, errorCount)
	assert.True(t, checkAvailable(baseURL), "Server must survive auth token flood")
}

// TestAuthChaos_ExpiredAndMutatedTokens tests handling of expired/mutated tokens.
func TestAuthChaos_ExpiredAndMutatedTokens(t *testing.T) {
	testutil.RequireServer(t)

	// Expired JWT (exp=0 means 1970-01-01) - well-structured but invalid
	expiredToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0IiwiZXhwIjowfQ." +
		"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var processedCount int64
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
					body := map[string]interface{}{
						"model":    "helixagent",
						"messages": []map[string]string{{"role": "user", "content": "test"}},
					}
					data, _ := json.Marshal(body)

					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer "+expiredToken)

					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
						atomic.AddInt64(&processedCount, 1)
					}
				}
			}
		}()
	}

	time.Sleep(8 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Expired/mutated tokens: %d requests processed", processedCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle expired token chaos")
}

// TestAuthChaos_ConcurrentAuthAttempts tests concurrent authentication requests.
func TestAuthChaos_ConcurrentAuthAttempts(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var successCount, failCount int64

	// Try concurrent login attempts
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := map[string]interface{}{
				"username": fmt.Sprintf("user%d", idx),
				"password": randStr(16),
			}
			data, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", baseURL+"/v1/auth/login", bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				atomic.AddInt64(&failCount, 1)
				return
			}
			resp.Body.Close()
			atomic.AddInt64(&successCount, 1)
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent auth attempts: %d got responses, %d connection errors", successCount, failCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive concurrent auth chaos")
}

// TestAuthChaos_HeaderInjection tests header injection and manipulation.
func TestAuthChaos_HeaderInjection(t *testing.T) {
	testutil.RequireServer(t)

	maliciousHeaders := map[string]string{
		"X-Forwarded-For": "127.0.0.1, 192.168.1.1, 10.0.0.1",
		"X-Real-IP":       "127.0.0.1",
		"X-Auth-User":     "admin",
		"X-User-ID":       "1",
		"X-Admin":         "true",
		"X-Bypass-Auth":   "1",
	}

	client := &http.Client{Timeout: 5 * time.Second}
	var processedCount int64
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := map[string]interface{}{
				"model":    "helixagent",
				"messages": []map[string]string{{"role": "user", "content": "test"}},
			}
			data, _ := json.Marshal(body)

			req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json")
			// Add malicious headers without auth
			for k, v := range maliciousHeaders {
				req.Header.Set(k, v)
			}

			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				atomic.AddInt64(&processedCount, 1)
			}
		}()
	}

	wg.Wait()

	t.Logf("Header injection: %d requests processed", processedCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle header injection gracefully")
}

func randStr(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
