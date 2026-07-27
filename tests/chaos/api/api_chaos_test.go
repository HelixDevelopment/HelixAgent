// Package api provides chaos tests for the main HelixAgent API endpoints.
//
// Every suite here drives a REAL, running HelixAgent server (resolved and
// identity-verified by requireHelixAgent — see chaos_support_test.go) with an
// adversarial request storm and asserts the server degrades gracefully: it keeps
// answering, it refuses malformed input cleanly and structurally, it never
// returns a crash-shaped 5xx, and it is still healthy afterwards.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completionBody builds a well-formed OpenAI-compatible chat request. The model
// name is deliberately one the server does not serve, so the request exercises
// the full middleware + validation + provider-dispatch path and terminates in a
// deterministic, structured refusal instead of a live (slow, billable) LLM call.
func completionBody(content string) []byte {
	data, err := json.Marshal(map[string]any{
		"model":    "helixagent",
		"messages": []map[string]string{{"role": "user", "content": content}},
	})
	if err != nil {
		panic(err) // marshalling a literal map cannot fail
	}
	return data
}

// statusIn reports whether every observed status is in the allowed set.
func statusIn(byStatus map[int]int64, allowed ...int) (bool, string) {
	permitted := make(map[int]bool, len(allowed))
	for _, s := range allowed {
		permitted[s] = true
	}
	var offenders []string
	for status, count := range byStatus {
		if !permitted[status] {
			offenders = append(offenders, fmt.Sprintf("%d(x%d)", status, count))
		}
	}
	sortStrings(offenders)
	return len(offenders) == 0, strings.Join(offenders, " ")
}

// TestAPIChaos_CompletionEndpoint hammers the completion endpoint with
// concurrent requests and asserts every one is answered with a structured
// response — never a dropped connection, never a crash-shaped 5xx.
func TestAPIChaos_CompletionEndpoint(t *testing.T) {
	base := requireHelixAgent(t)

	const workers = 20
	client := newChaosClient(workers, 10*time.Second)
	defer client.CloseIdleConnections()

	tally := newTally()
	var wg sync.WaitGroup
	done := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}

				req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions",
					bytes.NewReader(completionBody("hello")))
				if err != nil {
					tally.recordError(err)
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-chaos-key")

				resp, err := client.Do(req)
				if err != nil {
					tally.recordError(err)
					continue
				}
				tally.recordResponse(resp.StatusCode, readClose(resp), true)
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	total, transportErr, nonJSON, byStatus, firstErr, firstNonJSON := tally.snapshot()
	t.Logf("Completion endpoint storm: %d responses [%s], %d transport errors",
		total, formatStatuses(byStatus), transportErr)

	require.Greater(t, total, int64(0), "storm produced no responses at all")
	assert.Zero(t, transportErr,
		"every request must reach the server and get a response; first transport error: %s", firstErr)
	assert.Zero(t, nonJSON,
		"every response must carry a complete JSON body (no truncated/dropped responses); first: %s", firstNonJSON)
	assert.Zero(t, serverErrors(byStatus),
		"server must refuse cleanly, never with a crash-shaped 5xx; statuses: %s", formatStatuses(byStatus))
	assert.NoError(t, serverHealth(base), "server must remain healthy after the completion storm")
}

// TestAPIChaos_LargePayloads sends increasingly large payloads and asserts each
// one is answered structurally rather than dropped.
func TestAPIChaos_LargePayloads(t *testing.T) {
	base := requireHelixAgent(t)

	client := newChaosClient(4, 30*time.Second)
	defer client.CloseIdleConnections()

	// All sizes stay below the router's default 10 MiB body limit, so none of
	// them may be rejected with 413.
	sizes := []int{1024, 10 * 1024, 100 * 1024, 1024 * 1024}

	for _, size := range sizes {
		size := size
		t.Run(fmt.Sprintf("%dB", size), func(t *testing.T) {
			payload := completionBody(strings.Repeat("x", size))

			req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(payload))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-chaos-key")

			resp, err := client.Do(req)
			require.NoError(t, err,
				"server must answer a %d-byte payload instead of dropping the connection", size)
			body := readClose(resp)

			t.Logf("large payload %d bytes -> status %d", size, resp.StatusCode)

			assert.NotEqual(t, http.StatusRequestEntityTooLarge, resp.StatusCode,
				"%d bytes is below the 10 MiB body limit and must not be rejected as too large", size)
			assert.True(t, resp.StatusCode < 500 || resp.StatusCode == http.StatusServiceUnavailable,
				"server must refuse cleanly, got status %d; body=%s", resp.StatusCode, truncate(body, 200))
			assert.True(t, json.Valid(bytes.TrimSpace(body)),
				"response to a %d-byte payload must be a complete JSON document; got %s",
				size, truncate(body, 200))
		})
	}

	assert.NoError(t, serverHealth(base), "server must remain healthy after large-payload chaos")
}

// mixedShape is one adversarial request shape plus the set of statuses the
// server is allowed to answer it with.
type mixedShape struct {
	name        string
	build       func(base string) (*http.Request, error)
	allowed     []int
	requireJSON bool
}

// TestAPIChaos_ConcurrentMixedRequests storms the API with a mix of valid and
// malformed requests and asserts each shape is classified correctly under load.
func TestAPIChaos_ConcurrentMixedRequests(t *testing.T) {
	base := requireHelixAgent(t)

	shapes := []mixedShape{
		{
			name: "valid",
			build: func(base string) (*http.Request, error) {
				req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions",
					bytes.NewReader(completionBody("test")))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-chaos-key")
				return req, nil
			},
			// 404: the requested model is not served (structured model_not_found).
			// 429/503: documented graceful shedding under contention.
			allowed:     []int{http.StatusOK, http.StatusNotFound, http.StatusTooManyRequests, http.StatusServiceUnavailable},
			requireJSON: true,
		},
		{
			name: "invalid-json",
			build: func(base string) (*http.Request, error) {
				req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions",
					bytes.NewReader([]byte(`{bad json`)))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", "application/json")
				return req, nil
			},
			allowed:     []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable},
			requireJSON: true,
		},
		{
			name: "no-auth-incomplete-body",
			build: func(base string) (*http.Request, error) {
				data, err := json.Marshal(map[string]any{"model": "helixagent"})
				if err != nil {
					return nil, err
				}
				req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(data))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", "application/json")
				return req, nil
			},
			allowed: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
				http.StatusTooManyRequests, http.StatusServiceUnavailable},
			requireJSON: true,
		},
		{
			name: "wrong-method",
			build: func(base string) (*http.Request, error) {
				return http.NewRequest(http.MethodDelete, base+"/v1/chat/completions", nil)
			},
			allowed: []int{http.StatusNotFound, http.StatusMethodNotAllowed,
				http.StatusTooManyRequests, http.StatusServiceUnavailable},
			// gin's built-in no-route handler answers in plain text.
			requireJSON: false,
		},
	}

	tallies := make([]*statusTally, len(shapes))
	for i := range tallies {
		tallies[i] = newTally()
	}

	const workers = 30
	client := newChaosClient(workers, 15*time.Second)
	defer client.CloseIdleConnections()

	var wg sync.WaitGroup
	done := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for {
				select {
				case <-done:
					return
				default:
				}

				idx := rng.Intn(len(shapes))
				req, err := shapes[idx].build(base)
				if err != nil {
					tallies[idx].recordError(err)
					continue
				}
				resp, err := client.Do(req)
				if err != nil {
					tallies[idx].recordError(err)
					continue
				}
				tallies[idx].recordResponse(resp.StatusCode, readClose(resp), shapes[idx].requireJSON)
			}
		}(int64(i) + 1)
	}

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	var grandTotal int64
	for i, shape := range shapes {
		total, transportErr, nonJSON, byStatus, firstErr, firstNonJSON := tallies[i].snapshot()
		grandTotal += total
		t.Logf("shape %-24s %6d responses [%s], %d transport errors",
			shape.name, total, formatStatuses(byStatus), transportErr)

		assert.Greater(t, total, int64(0), "shape %s produced no responses", shape.name)
		assert.Zero(t, transportErr,
			"shape %s: every request must be answered; first transport error: %s", shape.name, firstErr)
		ok, offenders := statusIn(byStatus, shape.allowed...)
		assert.True(t, ok,
			"shape %s produced unexpected statuses %s (allowed %v)", shape.name, offenders, shape.allowed)
		if shape.requireJSON {
			assert.Zero(t, nonJSON,
				"shape %s: every response must carry a complete JSON body; first: %s", shape.name, firstNonJSON)
		}
	}

	require.Greater(t, grandTotal, int64(0), "mixed storm produced no responses at all")
	assert.NoError(t, serverHealth(base), "server must remain healthy after mixed-request chaos")
}

// TestAPIChaos_ModelListChaos rapid-fires model-list requests and asserts every
// answer is a complete, correct catalogue — not merely "something came back".
func TestAPIChaos_ModelListChaos(t *testing.T) {
	base := requireHelixAgent(t)

	const workers = 50
	client := newChaosClient(workers, 10*time.Second)
	defer client.CloseIdleConnections()

	tally := newTally()
	badCatalogue := &firstError{}
	var wg sync.WaitGroup
	done := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}

				resp, err := client.Get(base + "/v1/models")
				if err != nil {
					tally.recordError(err)
					continue
				}
				status := resp.StatusCode
				body := readClose(resp)
				tally.recordResponse(status, body, status == http.StatusOK)

				if status == http.StatusOK {
					if _, err := parseModelList(body); err != nil {
						badCatalogue.record(err.Error())
					}
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	total, transportErr, nonJSON, byStatus, firstErr, firstNonJSON := tally.snapshot()
	okCount := successes(byStatus)
	badCount, firstBad := badCatalogue.snapshot()
	t.Logf("Model list storm: %d responses [%s], %d transport errors, %d malformed catalogues",
		total, formatStatuses(byStatus), transportErr, badCount)

	require.Greater(t, total, int64(0), "storm produced no responses at all")
	assert.Greater(t, okCount, int64(0), "the model list endpoint must serve requests under load")
	assert.Zero(t, transportErr,
		"every model-list request must be answered; first transport error: %s", firstErr)
	assert.Zero(t, nonJSON,
		"every 200 response must carry a complete JSON body; first: %s", firstNonJSON)
	ok, offenders := statusIn(byStatus, http.StatusOK, http.StatusTooManyRequests, http.StatusServiceUnavailable)
	assert.True(t, ok, "unexpected statuses under model-list storm: %s", offenders)
	assert.Zero(t, badCount,
		"every 200 response must be a complete model catalogue, not a partial one; first defect: %s",
		firstBad)
	assert.NoError(t, serverHealth(base), "server must remain healthy after model-list chaos")
}

// TestAPIChaos_ContextCancellation cancels requests mid-flight and asserts the
// only errors observed are the deliberate cancellations — never a refused or
// dropped connection — and that the server keeps serving throughout.
func TestAPIChaos_ContextCancellation(t *testing.T) {
	base := requireHelixAgent(t)

	const workers = 20
	client := newChaosClient(workers, 10*time.Second)
	defer client.CloseIdleConnections()

	var cancelled, completed int64
	unexpectedErrs := newTally()
	responses := newTally()

	var wg sync.WaitGroup
	done := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for {
				select {
				case <-done:
					return
				default:
				}

				timeout := time.Duration(rng.Intn(200)+10) * time.Millisecond
				ctx, cancel := context.WithTimeout(context.Background(), timeout)

				req, err := http.NewRequestWithContext(ctx, http.MethodPost,
					base+"/v1/chat/completions",
					bytes.NewReader(completionBody(fmt.Sprintf("test %d", rng.Int()))))
				if err != nil {
					cancel()
					unexpectedErrs.recordError(err)
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-chaos-key")

				resp, err := client.Do(req)
				if err != nil {
					cancel()
					// A deliberate mid-flight cancellation is the chaos being
					// injected. Anything else (connection refused, EOF,
					// address-unavailable) means the request never reached a
					// healthy server and must not be silently absorbed.
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						atomic.AddInt64(&cancelled, 1)
					} else {
						unexpectedErrs.recordError(err)
					}
					continue
				}
				status := resp.StatusCode
				body := readClose(resp)
				cancel()
				atomic.AddInt64(&completed, 1)
				responses.recordResponse(status, body, true)
			}
		}(int64(i) + 101)
	}

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	_, unexpected, _, _, firstUnexpected, _ := unexpectedErrs.snapshot()
	total, _, nonJSON, byStatus, _, firstNonJSON := responses.snapshot()

	t.Logf("Context cancellation storm: %d completed [%s], %d deliberately cancelled, %d unexpected errors",
		completed, formatStatuses(byStatus), cancelled, unexpected)

	require.Greater(t, completed, int64(0),
		"the server must keep completing requests while others are cancelled mid-flight")
	assert.Zero(t, unexpected,
		"the only permitted failures are the injected cancellations; first unexpected error: %s", firstUnexpected)
	assert.Zero(t, nonJSON,
		"completed responses must carry complete JSON bodies; first: %s", firstNonJSON)
	assert.Zero(t, serverErrors(byStatus),
		"mid-flight cancellation must not push the server into a crash-shaped 5xx; statuses: %s",
		formatStatuses(byStatus))
	assert.Equal(t, total, completed, "response bookkeeping must account for every completed request")

	// The server must still serve a normal request correctly after the storm.
	assert.NoError(t, serverHealth(base), "server must remain healthy after cancellation chaos")

	probe := newChaosClient(2, 15*time.Second)
	defer probe.CloseIdleConnections()
	status, body, err := getWithBody(probe, base+"/v1/models")
	require.NoError(t, err, "server must still serve requests after cancellation chaos")
	assert.Equal(t, http.StatusOK, status, "model list must still be served; body=%s", truncate(body, 200))
	_, parseErr := parseModelList(body)
	assert.NoError(t, parseErr, "model list must still be complete after cancellation chaos")
}
