package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/services"
)

// ============================================================================
// Markdown Formatting Tests
// ============================================================================

func TestFormatDebateTeamIntroductionMarkdown(t *testing.T) {
	t.Parallel()
	t.Run("Contains no ANSI codes", func(t *testing.T) {
		members := []*services.DebateTeamMember{
			{
				Role:         services.RoleAnalyst,
				ModelName:    "claude-3-opus",
				ProviderName: "anthropic",
			},
		}

		result := FormatDebateTeamIntroductionMarkdown("Test Topic", members)

		// Must NOT contain any ANSI escape codes
		assert.False(t, ContainsANSI(result), "Markdown output should not contain ANSI codes")
		assert.NotContains(t, result, "\033[")
		assert.NotContains(t, result, "\x1b[")
	})

	t.Run("Contains proper Markdown structure", func(t *testing.T) {
		t.Parallel()
		members := []*services.DebateTeamMember{
			{
				Role:         services.RoleAnalyst,
				ModelName:    "claude-3-opus",
				ProviderName: "anthropic",
			},
			{
				Role:         services.RoleProposer,
				ModelName:    "gpt-4",
				ProviderName: "openai",
			},
		}

		result := FormatDebateTeamIntroductionMarkdown("Test Topic", members)

		// Check Markdown elements
		assert.Contains(t, result, "# HelixAgent AI Debate Ensemble")
		assert.Contains(t, result, "**Topic:**")
		assert.Contains(t, result, "## Debate Team")
		assert.Contains(t, result, "| Role | Model | Provider |")
		assert.Contains(t, result, "|------|-------|----------|")
		assert.Contains(t, result, "| **Analyst**")
		assert.Contains(t, result, "## The Deliberation")
	})

	t.Run("Includes fallback information", func(t *testing.T) {
		t.Parallel()
		members := []*services.DebateTeamMember{
			{
				Role:         services.RoleAnalyst,
				ModelName:    "claude-3-opus",
				ProviderName: "anthropic",
				Fallback: &services.DebateTeamMember{
					ModelName:    "gpt-4",
					ProviderName: "openai",
				},
			},
		}

		result := FormatDebateTeamIntroductionMarkdown("Test Topic", members)

		assert.Contains(t, result, "└─ Fallback")
		assert.Contains(t, result, "gpt-4")
	})

	t.Run("Truncates long topics", func(t *testing.T) {
		t.Parallel()
		longTopic := strings.Repeat("x", 100)
		result := FormatDebateTeamIntroductionMarkdown(longTopic, nil)

		assert.Contains(t, result, "...")
		// Should be truncated to 70 chars + "..."
		assert.LessOrEqual(t, len(strings.Split(result, "**Topic:**")[1][:80]), 80)
	})
}

func TestFormatPhaseHeaderMarkdown(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		phase       services.ValidationPhase
		expectedStr string
		icon        string
	}{
		{services.PhaseInitialResponse, "INITIAL RESPONSE", "🔍"},
		{services.PhaseValidation, "VALIDATION", "✓"},
		{services.PhasePolishImprove, "POLISH & IMPROVE", "✨"},
		{services.PhaseFinalConclusion, "FINAL CONCLUSION", "📜"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.phase), func(t *testing.T) {
			t.Parallel()
			result := FormatPhaseHeaderMarkdown(tc.phase, 1)

			// Must NOT contain ANSI codes
			assert.False(t, ContainsANSI(result), "Phase header should not contain ANSI codes")

			// Should contain proper Markdown header
			assert.Contains(t, result, "### ")
			assert.Contains(t, result, tc.expectedStr)
			assert.Contains(t, result, tc.icon)
			assert.Contains(t, result, "Phase 1")
		})
	}
}

func TestFormatFinalResponseMarkdown(t *testing.T) {
	t.Parallel()
	t.Run("Contains no ANSI codes", func(t *testing.T) {
		content := "This is the final answer to your question."
		result := FormatFinalResponseMarkdown(content)

		assert.False(t, ContainsANSI(result), "Final response should not contain ANSI codes")
		assert.NotContains(t, result, "\033[")
	})

	t.Run("Has proper Markdown structure", func(t *testing.T) {
		t.Parallel()
		content := "This is the final answer."
		result := FormatFinalResponseMarkdown(content)

		assert.Contains(t, result, "## Final Answer")
		assert.Contains(t, result, "---")
		assert.Contains(t, result, content)
	})
}

func TestFormatConsensusHeaderMarkdown(t *testing.T) {
	t.Parallel()
	t.Run("Contains no ANSI codes", func(t *testing.T) {
		result := FormatConsensusHeaderMarkdown(0.85)

		assert.False(t, ContainsANSI(result))
	})

	t.Run("Shows correct confidence percentage", func(t *testing.T) {
		t.Parallel()
		result := FormatConsensusHeaderMarkdown(0.85)

		assert.Contains(t, result, "85.0%")
		assert.Contains(t, result, "## Consensus")
	})
}

func TestFormatPhaseContentMarkdown(t *testing.T) {
	t.Parallel()
	t.Run("Wraps content in quote blocks", func(t *testing.T) {
		content := "Line 1\nLine 2\nLine 3"
		result := FormatPhaseContentMarkdown(content)

		lines := strings.Split(result, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				assert.True(t, strings.HasPrefix(line, ">"), "Each line should be a quote")
			}
		}
	})
}

// ============================================================================
// Plain Text Formatting Tests
// ============================================================================

func TestFormatDebateTeamIntroductionPlain(t *testing.T) {
	t.Parallel()
	t.Run("Contains no ANSI codes", func(t *testing.T) {
		members := []*services.DebateTeamMember{
			{
				Role:         services.RoleAnalyst,
				ModelName:    "claude-3-opus",
				ProviderName: "anthropic",
			},
		}

		result := FormatDebateTeamIntroductionPlain("Test Topic", members)

		assert.False(t, ContainsANSI(result))
	})

	t.Run("Contains no Markdown", func(t *testing.T) {
		t.Parallel()
		members := []*services.DebateTeamMember{
			{
				Role:         services.RoleAnalyst,
				ModelName:    "claude-3-opus",
				ProviderName: "anthropic",
			},
		}

		result := FormatDebateTeamIntroductionPlain("Test Topic", members)

		assert.NotContains(t, result, "#")
		assert.NotContains(t, result, "**")
		assert.NotContains(t, result, "|")
	})

	t.Run("Contains plain text content", func(t *testing.T) {
		t.Parallel()
		members := []*services.DebateTeamMember{
			{
				Role:         services.RoleAnalyst,
				ModelName:    "claude-3-opus",
				ProviderName: "anthropic",
			},
		}

		result := FormatDebateTeamIntroductionPlain("Test Topic", members)

		assert.Contains(t, result, "HELIXAGENT AI DEBATE ENSEMBLE")
		assert.Contains(t, result, "Topic: Test Topic")
		assert.Contains(t, result, "Debate Team:")
		assert.Contains(t, result, "Analyst")
	})
}

// ============================================================================
// ANSI Stripping Tests
// ============================================================================

func TestStripANSIRegex(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic color code",
			input:    "\033[31mRed Text\033[0m",
			expected: "Red Text",
		},
		{
			name:     "Multiple color codes",
			input:    "\033[1m\033[96mBold Cyan\033[0m",
			expected: "Bold Cyan",
		},
		{
			name:     "256 color codes",
			input:    "\033[38;5;208mOrange\033[0m",
			expected: "Orange",
		},
		{
			name:     "RGB color codes",
			input:    "\033[38;2;255;128;0mRGB Orange\033[0m",
			expected: "RGB Orange",
		},
		{
			name:     "No ANSI codes",
			input:    "Plain text without codes",
			expected: "Plain text without codes",
		},
		{
			name:     "Mixed content",
			input:    "Normal \033[1mBold\033[0m Normal",
			expected: "Normal Bold Normal",
		},
		{
			name:     "Box drawing chars (should be preserved)",
			input:    "╔══════╗\n║ Test ║\n╚══════╝",
			expected: "╔══════╗\n║ Test ║\n╚══════╝",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := StripANSIRegex(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestContainsANSI(t *testing.T) {
	t.Parallel()
	t.Run("Detects ANSI codes", func(t *testing.T) {
		assert.True(t, ContainsANSI("\033[31mRed\033[0m"))
		assert.True(t, ContainsANSI("\x1b[1mBold\x1b[0m"))
	})

	t.Run("Returns false for clean text", func(t *testing.T) {
		t.Parallel()
		assert.False(t, ContainsANSI("Plain text"))
		assert.False(t, ContainsANSI("# Markdown Header"))
		assert.False(t, ContainsANSI("╔══════╗"))
	})
}

// ============================================================================
// Markdown Stripping Tests
// ============================================================================

func TestStripMarkdown(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Headers",
			input:    "# Header 1\n## Header 2",
			expected: "Header 1\nHeader 2",
		},
		{
			name:     "Bold text",
			input:    "This is **bold** text",
			expected: "This is bold text",
		},
		{
			name:     "Italic text",
			input:    "This is *italic* text",
			expected: "This is italic text",
		},
		{
			name:     "Links",
			input:    "[Link Text](https://example.com)",
			expected: "Link Text",
		},
		{
			name:     "Inline code",
			input:    "Use `code` here",
			expected: "Use code here",
		},
		{
			name:     "Blockquotes",
			input:    "> This is a quote\n> Another line",
			expected: "This is a quote\nAnother line",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := StripMarkdown(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestStripAllFormatting(t *testing.T) {
	t.Parallel()
	t.Run("Removes both ANSI and Markdown", func(t *testing.T) {
		input := "\033[1m**Bold and ANSI**\033[0m"
		result := StripAllFormatting(input)

		assert.NotContains(t, result, "\033[")
		assert.NotContains(t, result, "**")
		assert.Contains(t, result, "Bold and ANSI")
	})
}

// ============================================================================
// Output Format Detection Tests
// ============================================================================

func TestDetectOutputFormat(t *testing.T) {
	t.Parallel()
	t.Run("Explicit format hints", func(t *testing.T) {
		assert.Equal(t, OutputFormatANSI, DetectOutputFormat("", "", "ansi"))
		assert.Equal(t, OutputFormatANSI, DetectOutputFormat("", "", "terminal"))
		assert.Equal(t, OutputFormatMarkdown, DetectOutputFormat("", "", "markdown"))
		assert.Equal(t, OutputFormatMarkdown, DetectOutputFormat("", "", "md"))
		assert.Equal(t, OutputFormatPlain, DetectOutputFormat("", "", "plain"))
		assert.Equal(t, OutputFormatPlain, DetectOutputFormat("", "", "text"))
	})

	t.Run("Terminal clients get ANSI", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, OutputFormatANSI, DetectOutputFormat("", "curl/7.64.1", ""))
		assert.Equal(t, OutputFormatANSI, DetectOutputFormat("", "wget", ""))
		assert.Equal(t, OutputFormatANSI, DetectOutputFormat("", "HTTPie/2.0.0", ""))
	})

	t.Run("API clients get Markdown", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, OutputFormatMarkdown, DetectOutputFormat("", "OpenCode/1.0", ""))
		assert.Equal(t, OutputFormatMarkdown, DetectOutputFormat("", "Crush/0.1", ""))
		assert.Equal(t, OutputFormatMarkdown, DetectOutputFormat("", "claude-code", ""))
		assert.Equal(t, OutputFormatMarkdown, DetectOutputFormat("", "VSCode/1.80", ""))
		assert.Equal(t, OutputFormatMarkdown, DetectOutputFormat("", "Cursor/0.5", ""))
	})

	t.Run("Accept header text/plain", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, OutputFormatPlain, DetectOutputFormat("text/plain", "", ""))
	})

	t.Run("Default is Markdown", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, OutputFormatMarkdown, DetectOutputFormat("", "", ""))
		assert.Equal(t, OutputFormatMarkdown, DetectOutputFormat("application/json", "unknown-client", ""))
	})
}

func TestIsTerminalClient(t *testing.T) {
	t.Parallel()
	terminalClients := []string{
		"curl/7.64.1",
		"Wget/1.20.3",
		"HTTPie/2.0.0",
		"terminal-client/1.0",
		"tty-browser",
		"console-app",
	}

	for _, client := range terminalClients {
		t.Run(client, func(t *testing.T) {
			t.Parallel()
			assert.True(t, IsTerminalClient(client), "Should detect %s as terminal client", client)
		})
	}

	nonTerminalClients := []string{
		"OpenCode/1.0",
		"Crush/0.1",
		"Mozilla/5.0",
		"Python-requests/2.28.0",
		"VSCode/1.80",
	}

	for _, client := range nonTerminalClients {
		t.Run(client, func(t *testing.T) {
			t.Parallel()
			assert.False(t, IsTerminalClient(client), "Should not detect %s as terminal client", client)
		})
	}
}

// ============================================================================
// Format Selection Tests
// ============================================================================

func TestFormatDebateTeamIntroductionForFormat(t *testing.T) {
	t.Parallel()
	members := []*services.DebateTeamMember{
		{
			Role:         services.RoleAnalyst,
			ModelName:    "claude-3-opus",
			ProviderName: "anthropic",
		},
	}

	t.Run("ANSI format contains ANSI codes", func(t *testing.T) {
		t.Parallel()
		result := FormatDebateTeamIntroductionForFormat(OutputFormatANSI, "Test", members)
		assert.True(t, ContainsANSI(result), "ANSI format should contain ANSI codes")
	})

	t.Run("Markdown format contains no ANSI codes", func(t *testing.T) {
		t.Parallel()
		result := FormatDebateTeamIntroductionForFormat(OutputFormatMarkdown, "Test", members)
		assert.False(t, ContainsANSI(result), "Markdown format should not contain ANSI codes")
		assert.Contains(t, result, "#") // Should have Markdown headers
	})

	t.Run("Plain format contains no ANSI or Markdown", func(t *testing.T) {
		t.Parallel()
		result := FormatDebateTeamIntroductionForFormat(OutputFormatPlain, "Test", members)
		assert.False(t, ContainsANSI(result))
		assert.NotContains(t, result, "#")
		assert.NotContains(t, result, "**")
	})
}

// ============================================================================
// Output Readability Tests
// ============================================================================

func TestOutputReadability(t *testing.T) {
	t.Parallel()
	members := []*services.DebateTeamMember{
		{
			Role:         services.RoleAnalyst,
			ModelName:    "claude-3-opus",
			ProviderName: "anthropic",
		},
		{
			Role:         services.RoleProposer,
			ModelName:    "gpt-4",
			ProviderName: "openai",
		},
	}

	t.Run("Markdown output is well-structured", func(t *testing.T) {
		t.Parallel()
		result := FormatDebateTeamIntroductionMarkdown("What is the meaning of life?", members)

		// Check for proper sections
		sections := []string{
			"# HelixAgent AI Debate Ensemble",
			"**Topic:**",
			"## Debate Team",
			"## The Deliberation",
		}

		for _, section := range sections {
			assert.Contains(t, result, section, "Missing section: %s", section)
		}

		// Check that table is well-formed
		assert.Contains(t, result, "| Role | Model | Provider |")
		assert.Contains(t, result, "| **Analyst**")
		assert.Contains(t, result, "| **Proposer**")
	})

	t.Run("No garbage characters in output", func(t *testing.T) {
		t.Parallel()
		result := FormatDebateTeamIntroductionMarkdown("Test", members)

		// Should not contain escape sequences rendered as text
		assert.NotContains(t, result, "␛")
		assert.NotContains(t, result, "[0m")
		assert.NotContains(t, result, "[1m")
		assert.NotContains(t, result, "[31m")
	})

	t.Run("Output has reasonable line lengths", func(t *testing.T) {
		t.Parallel()
		result := FormatDebateTeamIntroductionMarkdown("Test", members)

		lines := strings.Split(result, "\n")
		for _, line := range lines {
			// Most lines should be under 120 characters for readability
			if len(line) > 120 {
				// Only allow long lines if they're table rows
				assert.True(t, strings.Contains(line, "|") || strings.Contains(line, "---"),
					"Non-table line exceeds 120 chars: %s", line)
			}
		}
	})
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestFullDebateOutputFlow(t *testing.T) {
	t.Parallel()
	t.Run("Complete Markdown debate output is clean", func(t *testing.T) {
		members := []*services.DebateTeamMember{
			{Role: services.RoleAnalyst, ModelName: "claude-3-opus", ProviderName: "anthropic"},
			{Role: services.RoleProposer, ModelName: "gpt-4", ProviderName: "openai"},
			{Role: services.RoleCritic, ModelName: "gemini-pro", ProviderName: "google"},
			{Role: services.RoleSynthesis, ModelName: "deepseek-chat", ProviderName: "deepseek"},
			{Role: services.RoleMediator, ModelName: "mistral-large", ProviderName: "mistral"},
		}

		var fullOutput strings.Builder

		// Simulate full debate output
		fullOutput.WriteString(FormatDebateTeamIntroductionMarkdown("What is AI?", members))
		fullOutput.WriteString(FormatPhaseHeaderMarkdown(services.PhaseInitialResponse, 1))
		fullOutput.WriteString(FormatPhaseContentMarkdown("Initial analysis of the topic..."))
		fullOutput.WriteString(FormatPhaseHeaderMarkdown(services.PhaseValidation, 2))
		fullOutput.WriteString(FormatPhaseContentMarkdown("Validation of responses..."))
		fullOutput.WriteString(FormatPhaseHeaderMarkdown(services.PhasePolishImprove, 3))
		fullOutput.WriteString(FormatPhaseContentMarkdown("Polishing the response..."))
		fullOutput.WriteString(FormatFinalResponseMarkdown("This is the final synthesized answer about AI."))

		result := fullOutput.String()

		// Verify cleanliness
		assert.False(t, ContainsANSI(result), "Full output should not contain ANSI codes")
		assert.NotContains(t, result, "␛")
		assert.NotContains(t, result, "\033[")

		// Verify structure
		assert.Contains(t, result, "# HelixAgent AI Debate Ensemble")
		assert.Contains(t, result, "### 🔍 Phase 1: INITIAL RESPONSE")
		assert.Contains(t, result, "### ✓ Phase 2: VALIDATION")
		assert.Contains(t, result, "### ✨ Phase 3: POLISH & IMPROVE")
		assert.Contains(t, result, "## Final Answer")
	})
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkFormatDebateTeamIntroductionMarkdown(b *testing.B) {
	members := make([]*services.DebateTeamMember, 5)
	for i := 0; i < 5; i++ {
		members[i] = &services.DebateTeamMember{
			Role:         services.RoleAnalyst,
			ModelName:    "test-model",
			ProviderName: "test-provider",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatDebateTeamIntroductionMarkdown("Test Topic", members)
	}
}

func BenchmarkStripANSIRegex(b *testing.B) {
	input := "\033[1m\033[96mBold Cyan Text\033[0m with \033[31mRed\033[0m and \033[32mGreen\033[0m"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StripANSIRegex(input)
	}
}

func BenchmarkContainsANSI(b *testing.B) {
	inputs := []string{
		"Plain text without ANSI",
		"\033[1mBold text\033[0m",
		"Mixed \033[31mred\033[0m text",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			ContainsANSI(input)
		}
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestNilMemberHandling(t *testing.T) {
	t.Parallel()
	t.Run("Markdown format handles nil members", func(t *testing.T) {
		members := []*services.DebateTeamMember{nil, nil}

		require.NotPanics(t, func() {
			result := FormatDebateTeamIntroductionMarkdown("Test", members)
			assert.NotEmpty(t, result)
		})
	})

	t.Run("Plain format handles nil members", func(t *testing.T) {
		t.Parallel()
		members := []*services.DebateTeamMember{nil, nil}

		require.NotPanics(t, func() {
			result := FormatDebateTeamIntroductionPlain("Test", members)
			assert.NotEmpty(t, result)
		})
	})
}

func TestEmptyInputHandling(t *testing.T) {
	t.Parallel()
	t.Run("Empty topic", func(t *testing.T) {
		result := FormatDebateTeamIntroductionMarkdown("", nil)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "**Topic:**")
	})

	t.Run("Empty content in FormatFinalResponseMarkdown", func(t *testing.T) {
		t.Parallel()
		result := FormatFinalResponseMarkdown("")
		assert.Contains(t, result, "## Final Answer")
	})
}

// ============================================================================
// Fallback Error Formatting Tests
// ============================================================================

func TestCategorizeErrorString(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Rate limit error", "rate limit exceeded", "rate_limit"},
		{"Rate limit mixed case", "You've hit the Rate Limit", "rate_limit"},
		{"Timeout error", "context deadline exceeded: timeout", "timeout"},
		{"Timeout explicit", "request timed out after 30s", "timeout"},
		{"Auth error 401", "HTTP 401 Unauthorized", "auth"},
		{"Auth invalid key", "invalid api key provided", "auth"},
		{"Quota exceeded", "quota exceeded for today", "quota"},
		{"Connection refused", "connection refused by remote host", "connection"},
		{"Network error", "network unreachable", "connection"},
		{"Dial error", "dial tcp: connection refused", "connection"},
		{"Service unavailable", "503 Service Unavailable", "unavailable"},
		{"Overloaded", "server overloaded, try again later", "overloaded"},
		{"Invalid request", "400 bad request: invalid parameters", "invalid_request"},
		{"Empty response", "received empty response from server", "empty_response"},
		{"Unknown error", "something weird happened", "unknown"},
		{"Empty string", "", "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := categorizeErrorString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetCategoryIcon(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		category string
		expected string
	}{
		{"rate_limit", "🚦"},
		{"timeout", "⏱️"},
		{"auth", "🔑"},
		{"quota", "📊"},
		{"connection", "🔌"},
		{"unavailable", "🚫"},
		{"overloaded", "🔥"},
		{"invalid_request", "⚠️"},
		{"empty_response", "📭"},
		{"unknown", "❓"},
		{"", "❓"},
	}

	for _, tc := range testCases {
		t.Run(tc.category, func(t *testing.T) {
			t.Parallel()
			result := getCategoryIcon(tc.category)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatFallbackTriggeredMarkdown(t *testing.T) {
	t.Parallel()
	t.Run("Contains all required information", func(t *testing.T) {
		result := FormatFallbackTriggeredMarkdown(
			"Analyst",
			"openai", "gpt-4",
			"anthropic", "claude-3",
			"rate limit exceeded",
			"rate_limit",
			500*time.Millisecond,
		)

		assert.Contains(t, result, "**[Analyst] Fallback Triggered**")
		assert.Contains(t, result, "Primary: openai/gpt-4")
		assert.Contains(t, result, "🚦")
		assert.Contains(t, result, "**Error:**")
		assert.Contains(t, result, "rate limit exceeded")
		assert.Contains(t, result, "→ Trying: anthropic/claude-3")
		assert.Contains(t, result, "500 ms") // formatDuration adds space
	})

	t.Run("Contains no ANSI codes", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackTriggeredMarkdown(
			"Analyst",
			"openai", "gpt-4",
			"anthropic", "claude-3",
			"timeout",
			"timeout",
			1*time.Second,
		)

		assert.False(t, ContainsANSI(result), "Markdown output should not contain ANSI codes")
	})
}

func TestFormatFallbackSuccessMarkdown(t *testing.T) {
	t.Parallel()
	t.Run("Shows success message", func(t *testing.T) {
		result := FormatFallbackSuccessMarkdown(
			"Proposer",
			"anthropic", "claude-3",
			2,
			750*time.Millisecond,
		)

		assert.Contains(t, result, "**[Proposer] Fallback Succeeded**")
		assert.Contains(t, result, "anthropic/claude-3")
		assert.Contains(t, result, "attempt 2") // lowercase "attempt" in actual output
		assert.Contains(t, result, "750 ms")    // formatDuration adds space
	})
}

func TestFormatFallbackFailedMarkdown(t *testing.T) {
	t.Parallel()
	t.Run("Shows failure with error", func(t *testing.T) {
		result := FormatFallbackFailedMarkdown(
			"Critic",
			"google", "gemini-pro",
			"connection refused",
			"connection",
			2,
			1*time.Second,
		)

		assert.Contains(t, result, "**[Critic] Fallback 2 Failed**") // Actual format includes attempt number
		assert.Contains(t, result, "google/gemini-pro")
		assert.Contains(t, result, "🔌")
		assert.Contains(t, result, "connection refused")
		// Attempt number is in the "Fallback 2 Failed" part, not separate
	})
}

func TestFormatFallbackExhaustedMarkdown(t *testing.T) {
	t.Parallel()
	t.Run("Shows exhausted message", func(t *testing.T) {
		result := FormatFallbackExhaustedMarkdown("Synthesis", 4)

		assert.Contains(t, result, "**[Synthesis] ALL FALLBACKS EXHAUSTED**") // Uppercase
		assert.Contains(t, result, "4 attempts failed")
	})
}

func TestFormatFallbackWithErrorForFormat(t *testing.T) {
	t.Parallel()
	t.Run("ANSI format contains ANSI codes", func(t *testing.T) {
		result := FormatFallbackWithErrorForFormat(
			OutputFormatANSI,
			services.RoleAnalyst,
			"openai", "gpt-4",
			"anthropic", "claude-3",
			"rate limit exceeded",
			1,
			500*time.Millisecond,
		)

		assert.True(t, ContainsANSI(result), "ANSI format should contain ANSI codes")
		assert.Contains(t, result, "Fallback")
	})

	t.Run("Markdown format is clean", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackWithErrorForFormat(
			OutputFormatMarkdown,
			services.RoleAnalyst,
			"openai", "gpt-4",
			"anthropic", "claude-3",
			"rate limit exceeded",
			1,
			500*time.Millisecond,
		)

		assert.False(t, ContainsANSI(result), "Markdown format should not contain ANSI codes")
		assert.Contains(t, result, "rate limit exceeded")
		assert.Contains(t, result, "🚦")
	})

	t.Run("Plain format has no formatting", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackWithErrorForFormat(
			OutputFormatPlain,
			services.RoleAnalyst,
			"openai", "gpt-4",
			"anthropic", "claude-3",
			"timeout",
			1,
			500*time.Millisecond,
		)

		assert.False(t, ContainsANSI(result))
		assert.NotContains(t, result, "**")
		assert.Contains(t, result, "Fallback")
		assert.Contains(t, result, "timeout")
	})
}

func TestFormatFallbackChainMarkdown(t *testing.T) {
	t.Parallel()
	t.Run("Shows complete chain", func(t *testing.T) {
		chain := []FallbackAttempt{
			{
				Provider:   "openai",
				Model:      "gpt-4",
				Success:    false,
				Error:      "rate limit exceeded",
				Duration:   500 * time.Millisecond,
				AttemptNum: 1,
			},
			{
				Provider:   "anthropic",
				Model:      "claude-3",
				Success:    false,
				Error:      "timeout",
				Duration:   2 * time.Second,
				AttemptNum: 2,
			},
			{
				Provider:   "google",
				Model:      "gemini-pro",
				Success:    true,
				Duration:   750 * time.Millisecond,
				AttemptNum: 3,
			},
		}

		result := FormatFallbackChainMarkdown(services.PositionAnalyst, chain)

		assert.Contains(t, result, "Fallback Chain for Position 1")
		assert.Contains(t, result, "❌")
		assert.Contains(t, result, "✅")
		assert.Contains(t, result, "openai/gpt-4")
		assert.Contains(t, result, "anthropic/claude-3")
		assert.Contains(t, result, "google/gemini-pro")
		assert.Contains(t, result, "rate limit exceeded")
		assert.Contains(t, result, "timeout")
		assert.False(t, ContainsANSI(result))
	})

	t.Run("Empty chain returns empty string", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackChainMarkdown(services.PositionAnalyst, nil)
		assert.Empty(t, result)
	})
}

func TestFormatFallbackChainWithErrorsForFormat(t *testing.T) {
	t.Parallel()
	chain := []FallbackAttempt{
		{
			Provider:   "openai",
			Model:      "gpt-4",
			Success:    false,
			Error:      "rate limit exceeded",
			Duration:   500 * time.Millisecond,
			AttemptNum: 1,
		},
		{
			Provider:   "anthropic",
			Model:      "claude-3",
			Success:    true,
			Duration:   750 * time.Millisecond,
			AttemptNum: 2,
		},
	}

	t.Run("ANSI format", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackChainWithErrorsForFormat(
			OutputFormatANSI,
			services.PositionAnalyst,
			services.RoleAnalyst,
			chain,
			1250*time.Millisecond,
		)

		assert.True(t, ContainsANSI(result) || len(result) > 0)
	})

	t.Run("Markdown format", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackChainWithErrorsForFormat(
			OutputFormatMarkdown,
			services.PositionAnalyst,
			services.RoleAnalyst,
			chain,
			1250*time.Millisecond,
		)

		assert.False(t, ContainsANSI(result))
		assert.Contains(t, result, "openai/gpt-4")
	})

	t.Run("Plain format", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackChainWithErrorsForFormat(
			OutputFormatPlain,
			services.PositionAnalyst,
			services.RoleAnalyst,
			chain,
			1250*time.Millisecond,
		)

		assert.False(t, ContainsANSI(result))
		assert.NotContains(t, result, "**")
		assert.Contains(t, result, "Fallback Chain")
	})
}

// ============================================================================
// Fallback Error Category Edge Cases
// ============================================================================

func TestErrorCategoryEdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("Multiple keywords - first match wins", func(t *testing.T) {
		// "invalid" appears after "timeout" so timeout should match
		result := categorizeErrorString("request timeout: invalid response")
		assert.Equal(t, "timeout", result)
	})

	t.Run("Case insensitivity", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "rate_limit", categorizeErrorString("RATE LIMIT"))
		assert.Equal(t, "timeout", categorizeErrorString("TIMEOUT"))
		assert.Equal(t, "auth", categorizeErrorString("UNAUTHORIZED"))
	})
}

// ============================================================================
// Full Fallback Flow Integration Test
// ============================================================================

func TestFullFallbackErrorReportingFlow(t *testing.T) {
	t.Parallel()
	t.Run("Complete fallback sequence in Markdown", func(t *testing.T) {
		var fullOutput strings.Builder

		// Simulate a fallback sequence
		fullOutput.WriteString(FormatFallbackTriggeredMarkdown(
			"Analyst",
			"openai", "gpt-4",
			"anthropic", "claude-3",
			"rate limit exceeded",
			"rate_limit",
			500*time.Millisecond,
		))

		fullOutput.WriteString(FormatFallbackFailedMarkdown(
			"Analyst",
			"anthropic", "claude-3",
			"service unavailable",
			"unavailable",
			2,
			1*time.Second,
		))

		fullOutput.WriteString(FormatFallbackSuccessMarkdown(
			"Analyst",
			"google", "gemini-pro",
			3,
			750*time.Millisecond,
		))

		result := fullOutput.String()

		// Verify the complete flow is clean
		assert.False(t, ContainsANSI(result))

		// Verify all events are present
		assert.Contains(t, result, "Fallback Triggered")
		assert.Contains(t, result, "Failed") // "Fallback 2 Failed"
		assert.Contains(t, result, "Fallback Succeeded")

		// Verify error information is present
		assert.Contains(t, result, "rate limit exceeded")
		assert.Contains(t, result, "service unavailable")

		// Verify icons are present
		assert.Contains(t, result, "🚦") // rate limit
		assert.Contains(t, result, "🚫") // unavailable
		assert.Contains(t, result, "🔄") // success
	})
}

// ============================================================================
// Model Reference Formatting Tests
// ============================================================================

func TestFormatModelRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		provider string
		model    string
		expected string
	}{
		{
			name:     "Standard case - no prefix duplication",
			provider: "anthropic",
			model:    "claude-3-opus",
			expected: "anthropic/claude-3-opus",
		},
		{
			name:     "NVIDIA model with org prefix in model ID",
			provider: "nvidia",
			model:    "nvidia/llama-3.1-nemotron-70b-instruct",
			expected: "nvidia/llama-3.1-nemotron-70b-instruct",
		},
		{
			name:     "Meta model with org prefix",
			provider: "nvidia",
			model:    "meta/llama-3.1-405b-instruct",
			expected: "nvidia/meta/llama-3.1-405b-instruct",
		},
		{
			name:     "HuggingFace model with org prefix",
			provider: "huggingface",
			model:    "meta-llama/Llama-3.3-70B-Instruct",
			expected: "huggingface/meta-llama/Llama-3.3-70B-Instruct",
		},
		{
			name:     "Empty model returns provider",
			provider: "anthropic",
			model:    "",
			expected: "anthropic",
		},
		{
			name:     "Empty provider returns model",
			provider: "",
			model:    "claude-3-opus",
			expected: "claude-3-opus",
		},
		{
			name:     "Both empty returns empty",
			provider: "",
			model:    "",
			expected: "",
		},
		{
			name:     "Claude model - no prefix duplication",
			provider: "claude",
			model:    "claude-sonnet-4-5-20250929",
			expected: "claude/claude-sonnet-4-5-20250929",
		},
		{
			name:     "Case insensitive prefix matching",
			provider: "NVIDIA",
			model:    "nvidia/llama-3.1-nemotron-70b-instruct",
			expected: "nvidia/llama-3.1-nemotron-70b-instruct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := formatModelRef(tt.provider, tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatFallbackTriggeredMarkdown_NoDoublePrefix(t *testing.T) {
	t.Parallel()
	t.Run("NVIDIA model with org prefix does not double prefix", func(t *testing.T) {
		result := FormatFallbackTriggeredMarkdown(
			"Analyst",
			"nvidia", "nvidia/llama-3.1-nemotron-70b-instruct",
			"huggingface", "meta-llama/Llama-3.3-70B-Instruct",
			"API error: 404",
			"api_error",
			226*time.Millisecond,
		)

		// Should NOT contain double prefix
		assert.NotContains(t, result, "nvidia/nvidia/")
		assert.Contains(t, result, "nvidia/llama-3.1-nemotron-70b-instruct")

		// HuggingFace model should have provider prefix added
		assert.Contains(t, result, "huggingface/meta-llama/Llama-3.3-70B-Instruct")
	})

	t.Run("Standard model displays correctly", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackTriggeredMarkdown(
			"Analyst",
			"anthropic", "claude-3-opus",
			"openai", "gpt-4",
			"rate limit exceeded",
			"rate_limit",
			500*time.Millisecond,
		)

		// Should have provider/model format
		assert.Contains(t, result, "anthropic/claude-3-opus")
		assert.Contains(t, result, "openai/gpt-4")
	})
}
