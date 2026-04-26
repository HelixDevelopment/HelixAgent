package junie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsJunieTerminalAuthError covers the string matchers used to short-circuit
// health checks when Junie's CLI reports an unrecoverable subscription/auth
// failure (Finding #45).
func TestIsJunieTerminalAuthError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"jetbrains 403 explicit", "junie failed: 403 Forbidden: No active JetBrains AI subscription found.", true},
		{"jetbrains subscription banner", "Junie: 403 Forbidden: No active JetBrains AI subscription found.", true},
		{"401", "remote returned 401 Unauthorized", true},
		{"plain 403 is not enough", "remote returned 403 Forbidden", false},
		{"subscription expired", "your subscription expired", true},
		{"benign network error", "dial tcp: i/o timeout", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		got := isJunieTerminalAuthError(tc.in)
		assert.Equal(t, tc.want, got, "%s: %q", tc.name, tc.in)
	}
}

// TestJunieCLIProvider_HealthCheck_ShortCircuitsAfterTerminalError verifies
// that once markTerminalError records an unrecoverable auth failure, every
// subsequent HealthCheck returns the same error without re-invoking the CLI.
func TestJunieCLIProvider_HealthCheck_ShortCircuitsAfterTerminalError(t *testing.T) {
	t.Parallel()
	provider := NewJunieCLIProvider(JunieCLIConfig{Model: "junie-claude-sonnet-4-5"})
	provider.cliAvailable = true
	provider.cliCheckOnce.Do(func() {})

	terminal := &junieTestErr{msg: "Junie: 403 Forbidden: No active JetBrains AI subscription found."}
	provider.markTerminalError(terminal)

	got := provider.HealthCheck()
	assert.ErrorIs(t, got, terminal,
		"HealthCheck should return the recorded terminal error without re-invoking the CLI")
}

type junieTestErr struct{ msg string }

func (e *junieTestErr) Error() string { return e.msg }
