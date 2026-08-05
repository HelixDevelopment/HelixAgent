package services

// protocol_security_hxc221_red_test.go is the §11.4.115 RED-polarity guard for
// HXC-221: ProtocolSecurity.CreateAPIKey minted an API key from a CLOCK READING
// when the cryptographic entropy source failed.
//
// The pre-fix defect (internal/services/protocol_security.go, generateID):
//
//	b := make([]byte, 16)
//	if _, err := rand.Read(b); err != nil {
//	        // Fallback only if crypto/rand fails (extremely rare)
//	        return fmt.Sprintf("%d", time.Now().UnixNano())
//	}
//	return hex.EncodeToString(b)
//
// The happy path is correct. The FALLBACK is the defect: on entropy failure the
// function silently returned a value derived from the wall clock, and the
// caller could not tell. The result was stored in s.apiKeys and subsequently
// matched against the `Authorization: Bearer` header by ExtractAPIKeyFromHeader
// + ValidateAccess — i.e. a real credential on a real authentication path, not
// an internal identifier. A UnixNano-derived key is guessable: an attacker who
// knows the approximate issue time enumerates a narrow integer range.
//
// CreateAPIKey already returned (*APIKey, error). The ability to refuse safely
// existed and was simply unused. The fix is to fail closed — propagate the
// error and mint nothing.
//
// # Polarity switch (§11.4.115) — one source, two roles
//
//	RED_MODE=1 : reproduce the defect. Asserts that a failing entropy source
//	             still yields a key, that the key is clock-derived, and that
//	             ValidateAccess ACCEPTS it. PASSES only on the broken artifact;
//	             it MUST fail once the fix lands (that is the proof the guard
//	             is not blind).
//	RED_MODE=0 : (default) standing GREEN regression guard — a failing entropy
//	             source yields an ERROR, no key is minted, the key store is not
//	             polluted, and a clock-shaped key is rejected by ValidateAccess.
//
// # Honest boundary (§11.4.6) — why the entropy source is injected
//
// On this toolchain (go1.26.4) crypto/rand.Read CANNOT return a non-nil error.
// Per $(go env GOROOT)/src/crypto/rand/rand.go, on a failing Reader it calls
// runtime.fatal ("crypto/rand: failed to read random data (see
// https://go.dev/issue/66821)") — an IRRECOVERABLE crash, not a returned error.
// The pre-fix fallback branch was therefore unreachable dead code via
// rand.Read on Go >= 1.24, and swapping crypto/rand.Reader in-process cannot
// demonstrate the defect: it aborts the test binary instead.
//
// So the guard injects the entropy source through ProtocolSecurity.entropy
// (defaulting to crypto/rand.Reader in NewProtocolSecurity) and the production
// code reads it with io.ReadFull. That change is what makes the failure both
// REACHABLE and RECOVERABLE: instead of the process dying on entropy loss, key
// generation refuses and reports. The defect being guarded is the fail-OPEN
// fallback, and it is reproduced here against the genuine pre-fix fallback
// logic driven through the real CreateAPIKey -> ValidateAccess path.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redMode reports whether the guard runs in defect-reproduction polarity.
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// errEntropyExhausted is the injected entropy failure.
var errEntropyExhausted = errors.New("entropy source unavailable")

// failingEntropy always fails, modelling a dead CSPRNG.
type failingEntropy struct{}

func (failingEntropy) Read(p []byte) (int, error) { return 0, errEntropyExhausted }

// shortEntropy yields n bytes on its first read and then reports EOF,
// modelling a truncated/partial entropy read. io.ReadFull converts the
// premature EOF into io.ErrUnexpectedEOF when some bytes were delivered.
//
// The receiver MUST be a pointer: the reader is stateful (it exhausts after
// the first read), and a value receiver would mutate a copy, leaving it
// returning n bytes forever until io.ReadFull happened to fill the buffer.
type shortEntropy struct{ n int }

func (s *shortEntropy) Read(p []byte) (int, error) {
	if s.n <= 0 {
		return 0, io.EOF
	}
	n := s.n
	if n > len(p) {
		n = len(p)
	}
	for i := 0; i < n; i++ {
		p[i] = 0xAB
	}
	s.n = 0
	return n, nil
}

// clockShapedKey matches the pre-fix fallback output: "sk-" + decimal UnixNano.
var clockShapedKey = regexp.MustCompile(`^sk-[0-9]{15,20}$`)

// properKey matches a correct key: "sk-" + 16 random bytes, hex-encoded.
var properKey = regexp.MustCompile(`^sk-[0-9a-f]{32}$`)

// TestHXC221_EntropyFailureMustNotMintClockDerivedAPIKey is BOTH the
// bug-catcher (RED_MODE=1) and the standing regression guard (RED_MODE=0).
func TestHXC221_EntropyFailureMustNotMintClockDerivedAPIKey(t *testing.T) {
	security := NewProtocolSecurity(newSecurityTestLogger())
	security.entropy = failingEntropy{}

	apiKey, err := security.CreateAPIKey("victim-key", "system", []string{"*"})

	if redMode() {
		// ---- RED: assert the defect is PRESENT on the pre-fix artifact ----
		require.NoError(t, err,
			"RED: pre-fix CreateAPIKey swallowed entropy failure and returned no error")
		require.NotNil(t, apiKey, "RED: pre-fix CreateAPIKey still minted a key")

		assert.Regexp(t, clockShapedKey, apiKey.Key,
			"RED: minted key is derived from the wall clock, not from entropy")

		// The clock-derived key is not merely returned — it is ACCEPTED as a
		// credential on the real authentication path.
		bearer := ExtractAPIKeyFromHeader("Bearer " + apiKey.Key)
		require.Equal(t, apiKey.Key, bearer)
		assert.NoError(t,
			security.ValidateAccess(context.Background(), ProtocolAccessRequest{
				APIKey: bearer, Protocol: "mcp", Action: "execute",
			}),
			"RED: the clock-derived key authenticates for mcp:execute")

		// Demonstrate guessability: the key body is a plain integer an
		// attacker can enumerate over a narrow time window.
		body := strings.TrimPrefix(apiKey.Key, "sk-")
		nanos, convErr := strconv.ParseInt(body, 10, 64)
		require.NoError(t, convErr,
			"RED: key body parses as a decimal timestamp -> trivially enumerable")
		assert.Positive(t, nanos)
		t.Logf("RED reproduced HXC-221: entropy failed, key=%q (UnixNano=%d) minted AND accepted",
			apiKey.Key, nanos)
		return
	}

	// ---- GREEN: assert the defect is ABSENT on the fixed artifact ----
	require.Error(t, err,
		"GREEN: entropy failure MUST surface as an error, never a silent fallback")
	assert.Nil(t, apiKey, "GREEN: no APIKey may be returned when entropy fails")
	assert.ErrorIs(t, err, errEntropyExhausted,
		"GREEN: the underlying entropy error must be wrapped, not discarded")

	// No credential may have been persisted by the failed attempt.
	assert.Empty(t, security.ListAPIKeys(),
		"GREEN: a failed generation must not pollute the key store")

	// A clock-shaped key must not authenticate.
	forged := fmt.Sprintf("sk-%d", int64(1)<<60)
	assert.Error(t,
		security.ValidateAccess(context.Background(), ProtocolAccessRequest{
			APIKey: forged, Protocol: "mcp", Action: "execute",
		}),
		"GREEN: a clock-shaped key must not authenticate")
}

// TestHXC221_ExtendedCaseSet fans the fix out across the entropy case-space
// (§11.4.146 STEP 3). These cases assert the FIXED contract and are skipped in
// RED polarity, where the contract under test does not yet exist.
func TestHXC221_ExtendedCaseSet(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: #HXC-221 extended cases assert the post-fix contract; RED polarity asserts the pre-fix defect")
	}

	t.Run("happy path yields a proper 16-byte hex key", func(t *testing.T) {
		security := NewProtocolSecurity(newSecurityTestLogger())
		apiKey, err := security.CreateAPIKey("k", "o", []string{"mcp:read"})
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		assert.Regexp(t, properKey, apiKey.Key)
		assert.NotRegexp(t, clockShapedKey, apiKey.Key)
	})

	t.Run("keys are unique across many generations", func(t *testing.T) {
		security := NewProtocolSecurity(newSecurityTestLogger())
		seen := make(map[string]struct{}, 256)
		for i := 0; i < 256; i++ {
			apiKey, err := security.CreateAPIKey("k", "o", []string{"mcp:read"})
			require.NoError(t, err)
			_, dup := seen[apiKey.Key]
			require.False(t, dup, "duplicate key generated: %q", apiKey.Key)
			seen[apiKey.Key] = struct{}{}
		}
		assert.Len(t, seen, 256)
	})

	t.Run("partial entropy read fails closed", func(t *testing.T) {
		security := NewProtocolSecurity(newSecurityTestLogger())
		security.entropy = &shortEntropy{n: 4} // fewer than the 16 bytes required
		apiKey, err := security.CreateAPIKey("k", "o", []string{"*"})
		require.Error(t, err, "a truncated entropy read must not yield a key")
		assert.Nil(t, apiKey)
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.Empty(t, security.ListAPIKeys())
	})

	t.Run("empty entropy source fails closed", func(t *testing.T) {
		security := NewProtocolSecurity(newSecurityTestLogger())
		security.entropy = &shortEntropy{n: 0}
		apiKey, err := security.CreateAPIKey("k", "o", []string{"*"})
		require.Error(t, err)
		assert.Nil(t, apiKey)
		assert.Empty(t, security.ListAPIKeys())
	})

	t.Run("nil entropy defaults to crypto/rand, never to a weak source", func(t *testing.T) {
		// A zero-value ProtocolSecurity must not silently weaken generation.
		security := NewProtocolSecurity(newSecurityTestLogger())
		security.entropy = nil
		apiKey, err := security.CreateAPIKey("k", "o", []string{"mcp:read"})
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		assert.Regexp(t, properKey, apiKey.Key)
	})

	t.Run("explicit key path is unaffected by entropy state", func(t *testing.T) {
		// CreateAPIKeyWithValue takes a caller-supplied key and must keep
		// working even when the entropy source is dead.
		security := NewProtocolSecurity(newSecurityTestLogger())
		security.entropy = failingEntropy{}
		apiKey, err := security.CreateAPIKeyWithValue("env-key", "system", "sk-from-env", []string{"*"})
		require.NoError(t, err)
		require.NotNil(t, apiKey)
		assert.Equal(t, "sk-from-env", apiKey.Key)
	})

	t.Run("InitializeDefaultSecurity fails closed on entropy failure", func(t *testing.T) {
		t.Setenv("HELIXAGENT_API_KEY", "") // force the auto-generated admin path
		security := NewProtocolSecurity(newSecurityTestLogger())
		security.entropy = failingEntropy{}
		err := security.InitializeDefaultSecurity()
		require.Error(t, err, "default security setup must not proceed without entropy")
		assert.Empty(t, security.ListAPIKeys(),
			"no default credential may exist when entropy failed")
	})

	t.Run("concurrent generation stays unique and error-free", func(t *testing.T) {
		security := NewProtocolSecurity(newSecurityTestLogger())
		const workers = 16
		keys := make(chan string, workers)
		errs := make(chan error, workers)
		for i := 0; i < workers; i++ {
			go func() {
				apiKey, err := security.CreateAPIKey("k", "o", []string{"mcp:read"})
				if err != nil {
					errs <- err
					return
				}
				keys <- apiKey.Key
			}()
		}
		seen := make(map[string]struct{}, workers)
		for i := 0; i < workers; i++ {
			select {
			case err := <-errs:
				t.Fatalf("concurrent CreateAPIKey failed: %v", err)
			case k := <-keys:
				_, dup := seen[k]
				require.False(t, dup, "duplicate key under concurrency: %q", k)
				seen[k] = struct{}{}
			}
		}
		assert.Len(t, seen, workers)
	})
}
