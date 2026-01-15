package handlers

import (
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/services"
	"github.com/stretchr/testify/assert"
)

// TestFormatRequestIndicator tests the request indicator formatting
func TestFormatRequestIndicator(t *testing.T) {
	t.Run("Formats request indicator with position and provider", func(t *testing.T) {
		indicator := FormatRequestIndicator(
			services.PositionAnalyst,
			services.RoleAnalyst,
			"DeepSeek",
			"deepseek-chat",
		)

		// Should contain position indicator
		assert.Contains(t, indicator, "[A: Analyst]")
		// Should contain request arrow
		assert.Contains(t, indicator, "<---")
		// Should contain provider info
		assert.Contains(t, indicator, "DeepSeek")
		assert.Contains(t, indicator, "deepseek-chat")
	})

	t.Run("Handles all debate positions", func(t *testing.T) {
		positions := []struct {
			pos  services.DebateTeamPosition
			role services.DebateRole
			code string
			name string
		}{
			{services.PositionAnalyst, services.RoleAnalyst, "A", "Analyst"},
			{services.PositionProposer, services.RoleProposer, "P", "Proposer"},
			{services.PositionCritic, services.RoleCritic, "C", "Critic"},
			{services.PositionSynthesis, services.RoleSynthesis, "S", "Synthesis"},
			{services.PositionMediator, services.RoleMediator, "M", "Mediator"},
		}

		for _, p := range positions {
			indicator := FormatRequestIndicator(p.pos, p.role, "TestProvider", "test-model")
			assert.Contains(t, indicator, p.code+": "+p.name, "Position %d should have correct indicator", p.pos)
		}
	})
}

// TestFormatResponseIndicator tests the response indicator formatting
func TestFormatResponseIndicator(t *testing.T) {
	t.Run("Formats response with milliseconds timing", func(t *testing.T) {
		indicator := FormatResponseIndicator(
			services.PositionAnalyst,
			services.RoleAnalyst,
			450*time.Millisecond,
		)

		// Should contain position indicator
		assert.Contains(t, indicator, "[A: Analyst]")
		// Should contain response arrow
		assert.Contains(t, indicator, "--->")
		// Should contain timing in ms
		assert.Contains(t, indicator, "450 ms")
	})

	t.Run("Formats response with seconds timing", func(t *testing.T) {
		indicator := FormatResponseIndicator(
			services.PositionMediator,
			services.RoleMediator,
			1500*time.Millisecond,
		)

		// Should contain timing in seconds
		assert.Contains(t, indicator, "1.5 s")
	})
}

// TestFormatFallbackIndicator tests the fallback chain visualization
func TestFormatFallbackIndicator(t *testing.T) {
	t.Run("Shows fallback provider in chain", func(t *testing.T) {
		indicator := FormatFallbackIndicator(
			services.PositionMediator,
			services.RoleMediator,
			"Claude",
			"claude-sonnet-4.5",
			650*time.Millisecond,
		)

		// Should contain position indicator
		assert.Contains(t, indicator, "[M: Mediator]")
		// Should contain fallback indicator
		assert.Contains(t, indicator, "Fallback:")
		assert.Contains(t, indicator, "Claude")
		// Should contain response arrow
		assert.Contains(t, indicator, "--->")
		// Should contain timing
		assert.Contains(t, indicator, "650 ms")
	})
}

// TestFormatPhaseContent tests the phase content formatting
func TestFormatPhaseContent(t *testing.T) {
	t.Run("Wraps content with dim ANSI codes", func(t *testing.T) {
		content := "This is debate phase content"
		formatted := FormatPhaseContent(content)

		// Should start with dim code and end with reset
		assert.True(t, strings.HasPrefix(formatted, ANSIDim))
		assert.True(t, strings.HasSuffix(formatted, ANSIReset))
		// Should contain original content
		assert.Contains(t, formatted, content)
	})
}

// TestFormatFinalResponse tests the final response formatting
func TestFormatFinalResponse(t *testing.T) {
	t.Run("Wraps content with bright white ANSI codes", func(t *testing.T) {
		content := "This is the final consensus response"
		formatted := FormatFinalResponse(content)

		// Should start with bright white and end with reset
		assert.True(t, strings.HasPrefix(formatted, ANSIBrightWhite))
		assert.True(t, strings.HasSuffix(formatted, ANSIReset))
		// Should contain original content
		assert.Contains(t, formatted, content)
	})
}

// TestFormatConsensusHeader tests the consensus header formatting
func TestFormatConsensusHeader(t *testing.T) {
	t.Run("Contains consensus text and formatting", func(t *testing.T) {
		header := FormatConsensusHeader()

		// Should contain consensus text
		assert.Contains(t, header, "CONSENSUS REACHED")
		// Should contain emoji
		assert.Contains(t, header, "📜")
		// Should contain box characters
		assert.Contains(t, header, "═")
	})
}

// TestFormatDebateTeamIntroduction tests the team introduction formatting
func TestFormatDebateTeamIntroduction(t *testing.T) {
	t.Run("Contains topic and header", func(t *testing.T) {
		members := []*services.DebateTeamMember{
			{
				Position:     services.PositionAnalyst,
				Role:         services.RoleAnalyst,
				ProviderName: "DeepSeek",
				ModelName:    "deepseek-chat",
			},
		}

		intro := FormatDebateTeamIntroduction("Test Topic", members)

		// Should contain header
		assert.Contains(t, intro, "HELIXAGENT AI DEBATE ENSEMBLE")
		// Should contain topic
		assert.Contains(t, intro, "Test Topic")
		// Should contain DRAMATIS PERSONAE
		assert.Contains(t, intro, "DRAMATIS PERSONAE")
		// Should contain member info
		assert.Contains(t, intro, "DeepSeek")
	})

	t.Run("Truncates long topics", func(t *testing.T) {
		longTopic := strings.Repeat("x", 100)
		intro := FormatDebateTeamIntroduction(longTopic, nil)

		// Should truncate and add ...
		assert.Contains(t, intro, "...")
		// Should not contain full topic
		assert.NotContains(t, intro, longTopic)
	})
}

// TestStripANSI tests the ANSI stripping function
func TestStripANSI(t *testing.T) {
	t.Run("Removes ANSI codes from string", func(t *testing.T) {
		colored := ANSIRed + "Red text" + ANSIReset
		stripped := StripANSI(colored)

		assert.Equal(t, "Red text", stripped)
		assert.NotContains(t, stripped, "\033[")
	})

	t.Run("Removes all common ANSI codes", func(t *testing.T) {
		colored := ANSIBold + ANSICyan + "Bold Cyan" + ANSIReset +
			ANSIDim + "Dim" + ANSIReset +
			ANSIBrightWhite + "Bright" + ANSIReset

		stripped := StripANSI(colored)

		assert.Equal(t, "Bold CyanDimBright", stripped)
	})
}

// TestDebatePositionResponse tests the enhanced response struct
func TestDebatePositionResponse(t *testing.T) {
	t.Run("Contains all tracking fields", func(t *testing.T) {
		resp := &DebatePositionResponse{
			Content:         "Test content",
			Position:        services.PositionAnalyst,
			ResponseTime:    500 * time.Millisecond,
			PrimaryProvider: "DeepSeek",
			PrimaryModel:    "deepseek-chat",
			ActualProvider:  "Claude",
			ActualModel:     "claude-sonnet-4.5",
			UsedFallback:    true,
			FallbackChain: []FallbackAttempt{
				{
					Provider:   "DeepSeek",
					Model:      "deepseek-chat",
					Success:    false,
					Error:      "timeout",
					Duration:   200 * time.Millisecond,
					AttemptNum: 1,
				},
				{
					Provider:   "Claude",
					Model:      "claude-sonnet-4.5",
					Success:    true,
					Duration:   300 * time.Millisecond,
					AttemptNum: 2,
				},
			},
			Timestamp: time.Now(),
		}

		assert.Equal(t, "Test content", resp.Content)
		assert.True(t, resp.UsedFallback)
		assert.Len(t, resp.FallbackChain, 2)
		assert.Equal(t, "DeepSeek", resp.PrimaryProvider)
		assert.Equal(t, "Claude", resp.ActualProvider)
	})
}

// TestFormatDuration tests the duration formatting helper
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "0 ms"},
		{100 * time.Millisecond, "100 ms"},
		{999 * time.Millisecond, "999 ms"},
		{1000 * time.Millisecond, "1.0 s"},
		{1500 * time.Millisecond, "1.5 s"},
		{2000 * time.Millisecond, "2.0 s"},
		{10500 * time.Millisecond, "10.5 s"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		assert.Equal(t, tt.expected, result, "formatDuration(%v) should return %s", tt.duration, tt.expected)
	}
}

// TestRoleColors verifies all roles have colors
func TestRoleColors(t *testing.T) {
	roles := []services.DebateRole{
		services.RoleAnalyst,
		services.RoleProposer,
		services.RoleCritic,
		services.RoleSynthesis,
		services.RoleMediator,
	}

	for _, role := range roles {
		color := getRoleColor(role)
		assert.NotEmpty(t, color, "Role %s should have a color", role)
		assert.Contains(t, color, "\033[", "Color should be an ANSI escape code")
	}
}

// TestFormatFallbackChainIndicator tests the full fallback chain visualization
func TestFormatFallbackChainIndicator(t *testing.T) {
	t.Run("Single failed attempt with successful fallback", func(t *testing.T) {
		chain := []FallbackAttempt{
			{
				Provider:   "DeepSeek",
				Model:      "deepseek-chat",
				Success:    false,
				Error:      "rate limit exceeded",
				Duration:   10 * time.Millisecond,
				AttemptNum: 1,
			},
			{
				Provider:   "Claude",
				Model:      "claude-sonnet-4.5",
				Success:    true,
				Duration:   100 * time.Millisecond,
				AttemptNum: 2,
			},
		}

		indicator := FormatFallbackChainIndicator(
			services.PositionAnalyst,
			services.RoleAnalyst,
			chain,
			110*time.Millisecond,
		)

		// Should contain position indicator
		assert.Contains(t, indicator, "[A: Analyst]")
		// Should contain first failure timing
		assert.Contains(t, indicator, "10 ms")
		// Should contain fallback indicator with error
		assert.Contains(t, indicator, "Fallback")
		assert.Contains(t, indicator, "rate limit exceeded")
		assert.Contains(t, indicator, "DeepSeek")
		// Should contain successful timing
		assert.Contains(t, indicator, "100 ms")
	})

	t.Run("Multiple chained fallbacks", func(t *testing.T) {
		chain := []FallbackAttempt{
			{
				Provider:   "DeepSeek",
				Model:      "deepseek-chat",
				Success:    false,
				Error:      "rate limit",
				Duration:   10 * time.Millisecond,
				AttemptNum: 1,
			},
			{
				Provider:   "Gemini",
				Model:      "gemini-pro",
				Success:    false,
				Error:      "timeout",
				Duration:   5000 * time.Millisecond,
				AttemptNum: 2,
			},
			{
				Provider:   "Claude",
				Model:      "claude-sonnet-4.5",
				Success:    true,
				Duration:   100 * time.Millisecond,
				AttemptNum: 3,
			},
		}

		indicator := FormatFallbackChainIndicator(
			services.PositionMediator,
			services.RoleMediator,
			chain,
			5110*time.Millisecond,
		)

		// Should contain position indicator
		assert.Contains(t, indicator, "[M: Mediator]")
		// Should contain both failed providers
		assert.Contains(t, indicator, "DeepSeek")
		assert.Contains(t, indicator, "Gemini")
		// Should contain all timings
		assert.Contains(t, indicator, "10 ms")
		assert.Contains(t, indicator, "5.0 s")
		assert.Contains(t, indicator, "100 ms")
	})

	t.Run("Empty chain", func(t *testing.T) {
		indicator := FormatFallbackChainIndicator(
			services.PositionAnalyst,
			services.RoleAnalyst,
			[]FallbackAttempt{},
			0,
		)

		// Should still contain position indicator
		assert.Contains(t, indicator, "[A: Analyst]")
	})
}

// TestFormatFallbackChainWithContent tests fallback chain with response content
func TestFormatFallbackChainWithContent(t *testing.T) {
	t.Run("Formats fallback chain with content", func(t *testing.T) {
		chain := []FallbackAttempt{
			{
				Provider:   "DeepSeek",
				Model:      "deepseek-chat",
				Success:    false,
				Error:      "rate limit exceeded",
				Duration:   10 * time.Millisecond,
				AttemptNum: 1,
			},
			{
				Provider:   "Claude",
				Model:      "claude-sonnet-4.5",
				Success:    true,
				Duration:   100 * time.Millisecond,
				AttemptNum: 2,
			},
		}

		result := FormatFallbackChainWithContent(
			services.PositionAnalyst,
			services.RoleAnalyst,
			chain,
			"Response content here",
		)

		// Should contain position indicator
		assert.Contains(t, result, "[A: Analyst]")
		// Should contain fallback info
		assert.Contains(t, result, "Fallback")
		assert.Contains(t, result, "Rate limit reached")
		// Should contain content
		assert.Contains(t, result, "Response content here")
	})

	t.Run("Direct success without fallback", func(t *testing.T) {
		chain := []FallbackAttempt{
			{
				Provider:   "Claude",
				Model:      "claude-sonnet-4.5",
				Success:    true,
				Duration:   100 * time.Millisecond,
				AttemptNum: 1,
			},
		}

		result := FormatFallbackChainWithContent(
			services.PositionAnalyst,
			services.RoleAnalyst,
			chain,
			"Direct response",
		)

		// Should not contain Fallback
		assert.NotContains(t, result, "Fallback")
		// Should contain response arrow
		assert.Contains(t, result, "--->")
		// Should contain timing
		assert.Contains(t, result, "100 ms")
		// Should contain content
		assert.Contains(t, result, "Direct response")
	})

	t.Run("Multiple chained fallbacks with content", func(t *testing.T) {
		chain := []FallbackAttempt{
			{
				Provider:   "DeepSeek",
				Model:      "deepseek-chat",
				Success:    false,
				Error:      "rate limit",
				Duration:   10 * time.Millisecond,
				AttemptNum: 1,
			},
			{
				Provider:   "Gemini",
				Model:      "gemini-pro",
				Success:    false,
				Error:      "service unavailable",
				Duration:   50 * time.Millisecond,
				AttemptNum: 2,
			},
			{
				Provider:   "Claude",
				Model:      "claude-sonnet-4.5",
				Success:    true,
				Duration:   100 * time.Millisecond,
				AttemptNum: 3,
			},
		}

		result := FormatFallbackChainWithContent(
			services.PositionMediator,
			services.RoleMediator,
			chain,
			"Final response after multiple fallbacks",
		)

		// Should contain position indicator
		assert.Contains(t, result, "[M: Mediator]")
		// Should contain both fallback reasons
		assert.Contains(t, result, "Rate limit reached")
		assert.Contains(t, result, "Service unavailable")
		// Should contain content
		assert.Contains(t, result, "Final response after multiple fallbacks")
	})
}

// TestFormatFallbackReason tests the fallback reason formatting
func TestFormatFallbackReason(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Rate limit", "rate limit exceeded", "Rate limit reached"},
		{"Rate limit uppercase", "RATE LIMIT", "Rate limit reached"},
		{"Timeout", "request timeout", "Timeout"},
		{"Connection error", "connection refused", "Connection error"},
		{"Unavailable", "service unavailable", "Service unavailable"},
		{"Auth error", "authentication failed", "Auth error"},
		{"Quota", "quota exceeded", "Quota exceeded"},
		{"Overloaded", "service overloaded", "Service overloaded"},
		{"Empty error", "", "Provider error"},
		{"Short error", "unknown", "unknown"},
		{"Long error truncation", "this is a very long error message that should be truncated", "this is a very long error m..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFallbackReason(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTimingColorIsDarker verifies timing uses darker shade
func TestTimingColorIsDarker(t *testing.T) {
	t.Run("Response indicator uses dim timing color", func(t *testing.T) {
		indicator := FormatResponseIndicator(
			services.PositionAnalyst,
			services.RoleAnalyst,
			100*time.Millisecond,
		)

		// Should contain dim + bright black for timing (darker shade)
		assert.Contains(t, indicator, ANSIDim+ANSIBrightBlack)
	})

	t.Run("Fallback indicator uses dim timing color", func(t *testing.T) {
		indicator := FormatFallbackIndicator(
			services.PositionAnalyst,
			services.RoleAnalyst,
			"Claude",
			"claude-sonnet-4.5",
			100*time.Millisecond,
		)

		// Should contain dim + bright black for timing (darker shade)
		assert.Contains(t, indicator, ANSIDim+ANSIBrightBlack)
	})
}

// TestFallbackAttemptStruct tests the FallbackAttempt struct
func TestFallbackAttemptStruct(t *testing.T) {
	t.Run("Contains all required fields", func(t *testing.T) {
		attempt := FallbackAttempt{
			Provider:   "DeepSeek",
			Model:      "deepseek-chat",
			Success:    false,
			Error:      "rate limit exceeded",
			Duration:   100 * time.Millisecond,
			AttemptNum: 1,
		}

		assert.Equal(t, "DeepSeek", attempt.Provider)
		assert.Equal(t, "deepseek-chat", attempt.Model)
		assert.False(t, attempt.Success)
		assert.Equal(t, "rate limit exceeded", attempt.Error)
		assert.Equal(t, 100*time.Millisecond, attempt.Duration)
		assert.Equal(t, 1, attempt.AttemptNum)
	})
}

// TestComplexFallbackScenarios tests complex real-world fallback scenarios
func TestComplexFallbackScenarios(t *testing.T) {
	t.Run("All providers fail except last", func(t *testing.T) {
		chain := []FallbackAttempt{
			{Provider: "DeepSeek", Success: false, Error: "rate limit", Duration: 10 * time.Millisecond, AttemptNum: 1},
			{Provider: "Gemini", Success: false, Error: "quota exceeded", Duration: 20 * time.Millisecond, AttemptNum: 2},
			{Provider: "Mistral", Success: false, Error: "timeout", Duration: 5000 * time.Millisecond, AttemptNum: 3},
			{Provider: "Claude", Success: true, Duration: 100 * time.Millisecond, AttemptNum: 4},
		}

		result := FormatFallbackChainWithContent(
			services.PositionAnalyst,
			services.RoleAnalyst,
			chain,
			"Finally got a response",
		)

		// Should show all fallback reasons
		assert.Contains(t, result, "Rate limit reached")
		assert.Contains(t, result, "Quota exceeded")
		assert.Contains(t, result, "Timeout")
		assert.Contains(t, result, "Finally got a response")
	})

	t.Run("First provider succeeds immediately", func(t *testing.T) {
		chain := []FallbackAttempt{
			{Provider: "DeepSeek", Success: true, Duration: 50 * time.Millisecond, AttemptNum: 1},
		}

		result := FormatFallbackChainWithContent(
			services.PositionAnalyst,
			services.RoleAnalyst,
			chain,
			"Quick response",
		)

		// Should not show any fallback info
		assert.NotContains(t, result, "Fallback")
		assert.Contains(t, result, "Quick response")
	})
}
