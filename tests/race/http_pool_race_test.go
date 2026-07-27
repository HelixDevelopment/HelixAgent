// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"fmt"
	"sync"
	"testing"

	internalhttp "dev.helix.agent/internal/http"
)

// TestHTTPClientPool_ConcurrentGetClient tests concurrent GetClient calls on
// the HTTPClientPool. The pool uses sync.RWMutex with double-checked locking —
// this test drives both the read-path (existing host) and write-path (new host)
// concurrently.
func TestHTTPClientPool_ConcurrentGetClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode") // SKIP-OK: #short-mode
	}

	pool := internalhttp.NewHTTPClientPool(internalhttp.DefaultPoolConfig())

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				// Mix shared hosts (contention on read path) and unique hosts
				// (contention on write path).
				var host string
				if j%3 == 0 {
					host = "https://api.example.com"
				} else {
					host = fmt.Sprintf("https://host-%d.example.com", id)
				}
				client := pool.GetClient(host)
				_ = client
			}
		}(i)
	}

	wg.Wait()
}

// TestHTTPClientPool_ConcurrentGetClientForURL tests the URL-parsing variant
// alongside GetClient to exercise all lock paths simultaneously.
func TestHTTPClientPool_ConcurrentGetClientForURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode") // SKIP-OK: #short-mode
	}

	pool := internalhttp.NewHTTPClientPool(internalhttp.DefaultPoolConfig())

	var wg sync.WaitGroup
	const goroutines = 15

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				urlStr := fmt.Sprintf("https://api-%d.llm-provider.com/v1/completions", id%4)
				client, err := pool.GetClientForURL(urlStr)
				if err == nil {
					_ = client
				}
			}
		}(i)
	}

	wg.Wait()
}
