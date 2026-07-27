// Package auth provides chaos tests for the authentication system.
//
// The suites drive a REAL, running HelixAgent server (resolved and
// identity-verified by requireHelixAgent — see chaos_support_test.go).
//
// They deliberately storm BOTH kinds of endpoint:
//
//   - a genuinely protected route (protectedPath), where the security invariant
//     is that no forged / expired / malformed credential and no injected header
//     may ever produce a 2xx. Flooding only the OpenAI-compatible completion
//     route — which the router intentionally exempts from authentication — can
//     never detect an auth bypass, so on its own it is not an auth test at all.
//   - the unauthenticated completion route, where the invariant is that garbage
//     credentials must be shrugged off cleanly rather than destabilising the
//     server.
package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// protectedPath is a route behind the authentication middleware. Requests
// without a valid credential must be rejected with 401/403.
const protectedPath = "/v1/sessions"

// openPath is the OpenAI-compatible completion route, which the router
// deliberately exempts from authentication.
const openPath = "/v1/chat/completions"

// Statuses an authenticated route may answer an invalid credential with:
// a rejection, or documented graceful shedding under contention.
var rejectionStatuses = []int{
	http.StatusUnauthorized,
	http.StatusForbidden,
	http.StatusTooManyRequests,
	http.StatusServiceUnavailable,
}

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

// rejections counts responses that actually exercised the auth decision.
func rejections(byStatus map[int]int64) int64 {
	return byStatus[http.StatusUnauthorized] + byStatus[http.StatusForbidden]
}

// invalidTokens is the catalogue of credentials the server must never accept.
func invalidTokens() []string {
	return []string{
		"",
		"Bearer ",
		"Bearer invalid.token.here",
		"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.invalid",
		"Bearer " + randStr(256),
		"NotBearer validlooking",
		// alg=none: the classic JWT signature-stripping bypass.
		"Bearer eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiIxMjM0NTY3ODkwIn0.",
		fmt.Sprintf("Bearer %s.%s.%s", randStr(36), randStr(48), randStr(43)),
		"Token " + randStr(32),
		"Basic dXNlcjpwYXNzd29yZA==",
	}
}

// TestAuthChaos_InvalidTokenFlood floods both a protected and an unauthenticated
// route with invalid credentials.
//
// Security invariant on the protected route: not one forged credential may yield
// a 2xx, and the auth decision must demonstrably run (at least one 401/403).
func TestAuthChaos_InvalidTokenFlood(t *testing.T) {
	base := requireHelixAgent(t)

	tokens := invalidTokens()

	const workers = 30
	client := newChaosClient(workers, 10*time.Second)
	defer client.CloseIdleConnections()

	protectedTally := newTally()
	openTally := newTally()

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

				token := tokens[rng.Intn(len(tokens))]

				// Protected route — must reject.
				req, err := http.NewRequest(http.MethodGet, base+protectedPath, nil)
				if err != nil {
					protectedTally.recordError(err)
				} else {
					if token != "" {
						req.Header.Set("Authorization", token)
					}
					if resp, err := client.Do(req); err != nil {
						protectedTally.recordError(err)
					} else {
						protectedTally.recordResponse(resp.StatusCode, readClose(resp), true)
					}
				}

				// Unauthenticated route — must stay stable.
				openReq, err := http.NewRequest(http.MethodPost, base+openPath,
					bytes.NewReader(completionBody("test")))
				if err != nil {
					openTally.recordError(err)
					continue
				}
				openReq.Header.Set("Content-Type", "application/json")
				if token != "" {
					openReq.Header.Set("Authorization", token)
				}
				if resp, err := client.Do(openReq); err != nil {
					openTally.recordError(err)
				} else {
					openTally.recordResponse(resp.StatusCode, readClose(resp), true)
				}
			}
		}(int64(i) + 1)
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	pTotal, pErr, pNonJSON, pStatuses, pFirstErr, pFirstNonJSON := protectedTally.snapshot()
	oTotal, oErr, oNonJSON, oStatuses, oFirstErr, oFirstNonJSON := openTally.snapshot()

	t.Logf("invalid-token flood: protected %s -> %d responses [%s], %d transport errors",
		protectedPath, pTotal, formatStatuses(pStatuses), pErr)
	t.Logf("invalid-token flood: open      %s -> %d responses [%s], %d transport errors",
		openPath, oTotal, formatStatuses(oStatuses), oErr)

	require.Greater(t, pTotal, int64(0), "protected-route flood produced no responses at all")
	assert.Zero(t, pErr, "every protected request must be answered; first transport error: %s", pFirstErr)
	assert.Zero(t, successes(pStatuses),
		"AUTH BYPASS: a forged credential produced a 2xx on %s; statuses: %s",
		protectedPath, formatStatuses(pStatuses))
	assert.Greater(t, rejections(pStatuses), int64(0),
		"the auth decision must actually run — expected 401/403 rejections, got %s",
		formatStatuses(pStatuses))
	ok, offenders := statusIn(pStatuses, rejectionStatuses...)
	assert.True(t, ok, "protected route answered invalid credentials with unexpected statuses: %s", offenders)
	assert.Zero(t, pNonJSON,
		"rejections must carry a complete JSON body; first: %s", pFirstNonJSON)

	require.Greater(t, oTotal, int64(0), "open-route flood produced no responses at all")
	assert.Zero(t, oErr, "every open-route request must be answered; first transport error: %s", oFirstErr)
	assert.Zero(t, serverErrors(oStatuses),
		"garbage credentials must not push the open route into a crash-shaped 5xx; statuses: %s",
		formatStatuses(oStatuses))
	assert.Zero(t, oNonJSON,
		"open-route responses must carry complete JSON bodies; first: %s", oFirstNonJSON)

	assert.NoError(t, serverHealth(base), "server must survive the auth token flood")
}

// TestAuthChaos_ExpiredAndMutatedTokens storms a protected route with
// well-structured but invalid JWTs: expired, signature-mutated, and alg=none.
func TestAuthChaos_ExpiredAndMutatedTokens(t *testing.T) {
	base := requireHelixAgent(t)

	// exp=0 => 1970-01-01, i.e. long expired but structurally valid.
	expiredToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0IiwiZXhwIjowfQ." +
		"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	// alg=none with no signature at all.
	algNoneToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiIxMjM0NTY3ODkwIn0."
	// Valid header/claims, corrupted signature.
	mutatedToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0IiwiZXhwIjo5OTk5OTk5OTk5fQ." +
		randStr(43)

	variants := []string{expiredToken, algNoneToken, mutatedToken}

	const workers = 20
	client := newChaosClient(workers, 10*time.Second)
	defer client.CloseIdleConnections()

	tally := newTally()
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

				req, err := http.NewRequest(http.MethodGet, base+protectedPath, nil)
				if err != nil {
					tally.recordError(err)
					continue
				}
				req.Header.Set("Authorization", "Bearer "+variants[rng.Intn(len(variants))])

				resp, err := client.Do(req)
				if err != nil {
					tally.recordError(err)
					continue
				}
				tally.recordResponse(resp.StatusCode, readClose(resp), true)
			}
		}(int64(i) + 11)
	}

	time.Sleep(8 * time.Second)
	close(done)
	wg.Wait()

	total, transportErr, nonJSON, byStatus, firstErr, firstNonJSON := tally.snapshot()
	t.Logf("expired/mutated/alg-none tokens: %d responses [%s], %d transport errors",
		total, formatStatuses(byStatus), transportErr)

	require.Greater(t, total, int64(0), "storm produced no responses at all")
	assert.Zero(t, transportErr, "every request must be answered; first transport error: %s", firstErr)
	assert.Zero(t, successes(byStatus),
		"AUTH BYPASS: an expired / mutated / alg=none token produced a 2xx on %s; statuses: %s",
		protectedPath, formatStatuses(byStatus))
	assert.Greater(t, rejections(byStatus), int64(0),
		"expected 401/403 rejections, got %s", formatStatuses(byStatus))
	ok, offenders := statusIn(byStatus, rejectionStatuses...)
	assert.True(t, ok, "unexpected statuses for invalid JWTs: %s", offenders)
	assert.Zero(t, nonJSON, "rejections must carry a complete JSON body; first: %s", firstNonJSON)
	assert.NoError(t, serverHealth(base), "server must survive expired-token chaos")
}

// TestAuthChaos_ConcurrentAuthAttempts fires concurrent logins with random
// credentials and asserts none of them is ever granted.
func TestAuthChaos_ConcurrentAuthAttempts(t *testing.T) {
	base := requireHelixAgent(t)

	const attempts = 50
	client := newChaosClient(attempts, 15*time.Second)
	defer client.CloseIdleConnections()

	tally := newTally()
	var wg sync.WaitGroup

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			payload, err := json.Marshal(map[string]any{
				"username": fmt.Sprintf("chaos-user-%d", idx),
				"password": randStr(16),
			})
			if err != nil {
				tally.recordError(err)
				return
			}
			req, err := http.NewRequest(http.MethodPost, base+"/v1/auth/login", bytes.NewReader(payload))
			if err != nil {
				tally.recordError(err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				tally.recordError(err)
				return
			}
			tally.recordResponse(resp.StatusCode, readClose(resp), true)
		}(i)
	}

	wg.Wait()

	total, transportErr, nonJSON, byStatus, firstErr, firstNonJSON := tally.snapshot()
	t.Logf("concurrent login attempts: %d responses [%s], %d transport errors",
		total, formatStatuses(byStatus), transportErr)

	assert.EqualValues(t, attempts, total+transportErr, "every attempt must be accounted for")
	assert.Zero(t, transportErr, "every login attempt must be answered; first transport error: %s", firstErr)
	assert.Zero(t, successes(byStatus),
		"AUTH BYPASS: a random credential was accepted; statuses: %s", formatStatuses(byStatus))
	ok, offenders := statusIn(byStatus,
		http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusTooManyRequests, http.StatusServiceUnavailable)
	assert.True(t, ok, "login answered random credentials with unexpected statuses: %s", offenders)
	assert.Zero(t, nonJSON, "login responses must carry a complete JSON body; first: %s", firstNonJSON)
	assert.NoError(t, serverHealth(base), "server must survive concurrent auth chaos")
}

// TestAuthChaos_HeaderInjection asserts that trust-me headers grant nothing.
func TestAuthChaos_HeaderInjection(t *testing.T) {
	base := requireHelixAgent(t)

	maliciousHeaders := map[string]string{
		"X-Forwarded-For": "127.0.0.1, 192.168.1.1, 10.0.0.1",
		"X-Real-IP":       "127.0.0.1",
		"X-Auth-User":     "admin",
		"X-User-ID":       "1",
		"X-Admin":         "true",
		"X-Bypass-Auth":   "1",
	}

	const workers = 20
	client := newChaosClient(workers, 15*time.Second)
	defer client.CloseIdleConnections()

	protectedTally := newTally()
	openTally := newTally()
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Privilege-escalation probe against the protected route.
			req, err := http.NewRequest(http.MethodGet, base+protectedPath, nil)
			if err != nil {
				protectedTally.recordError(err)
			} else {
				for k, v := range maliciousHeaders {
					req.Header.Set(k, v)
				}
				if resp, err := client.Do(req); err != nil {
					protectedTally.recordError(err)
				} else {
					protectedTally.recordResponse(resp.StatusCode, readClose(resp), true)
				}
			}

			// Stability probe against the unauthenticated route.
			openReq, err := http.NewRequest(http.MethodPost, base+openPath,
				bytes.NewReader(completionBody("test")))
			if err != nil {
				openTally.recordError(err)
				return
			}
			openReq.Header.Set("Content-Type", "application/json")
			for k, v := range maliciousHeaders {
				openReq.Header.Set(k, v)
			}
			if resp, err := client.Do(openReq); err != nil {
				openTally.recordError(err)
			} else {
				openTally.recordResponse(resp.StatusCode, readClose(resp), true)
			}
		}()
	}

	wg.Wait()

	pTotal, pErr, _, pStatuses, pFirstErr, _ := protectedTally.snapshot()
	oTotal, oErr, oNonJSON, oStatuses, oFirstErr, oFirstNonJSON := openTally.snapshot()

	t.Logf("header injection: protected %s -> %d responses [%s], %d transport errors",
		protectedPath, pTotal, formatStatuses(pStatuses), pErr)
	t.Logf("header injection: open      %s -> %d responses [%s], %d transport errors",
		openPath, oTotal, formatStatuses(oStatuses), oErr)

	assert.EqualValues(t, workers, pTotal+pErr, "every protected probe must be accounted for")
	assert.Zero(t, pErr, "every protected probe must be answered; first transport error: %s", pFirstErr)
	assert.Zero(t, successes(pStatuses),
		"PRIVILEGE ESCALATION: injected trust headers granted access to %s; statuses: %s",
		protectedPath, formatStatuses(pStatuses))
	assert.Greater(t, rejections(pStatuses), int64(0),
		"injected headers must be rejected with 401/403, got %s", formatStatuses(pStatuses))

	assert.Zero(t, oErr, "every open-route probe must be answered; first transport error: %s", oFirstErr)
	assert.Zero(t, serverErrors(oStatuses),
		"injected headers must not push the open route into a crash-shaped 5xx; statuses: %s",
		formatStatuses(oStatuses))
	assert.Zero(t, oNonJSON,
		"open-route responses must carry complete JSON bodies; first: %s", oFirstNonJSON)

	assert.NoError(t, serverHealth(base), "server must handle header injection gracefully")
}

func randStr(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
