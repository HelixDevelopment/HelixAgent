package handlers

import (
	"fmt"
	"strings"
	"time"

	"dev.helix.agent/internal/services"
)

// ============================================================================
// ANSI Color Codes for Terminal Visualization
// ============================================================================

// ANSI escape codes for terminal colors
const (
	// Reset
	ANSIReset = "\033[0m"

	// Regular Colors
	ANSIBlack   = "\033[30m"
	ANSIRed     = "\033[31m"
	ANSIGreen   = "\033[32m"
	ANSIYellow  = "\033[33m"
	ANSIBlue    = "\033[34m"
	ANSIMagenta = "\033[35m"
	ANSICyan    = "\033[36m"
	ANSIWhite   = "\033[37m"

	// Bright/Bold Colors
	ANSIBrightBlack   = "\033[90m"
	ANSIBrightRed     = "\033[91m"
	ANSIBrightGreen   = "\033[92m"
	ANSIBrightYellow  = "\033[93m"
	ANSIBrightBlue    = "\033[94m"
	ANSIBrightMagenta = "\033[95m"
	ANSIBrightCyan    = "\033[96m"
	ANSIBrightWhite   = "\033[97m"

	// Dim/Gray for debate phase content
	ANSIDim      = "\033[2m"
	ANSIBold     = "\033[1m"
	ANSIItalic   = "\033[3m"
	ANSIUnderline = "\033[4m"

	// Background Colors
	ANSIBgBlack   = "\033[40m"
	ANSIBgRed     = "\033[41m"
	ANSIBgGreen   = "\033[42m"
	ANSIBgYellow  = "\033[43m"
	ANSIBgBlue    = "\033[44m"
	ANSIBgMagenta = "\033[45m"
	ANSIBgCyan    = "\033[46m"
	ANSIBgWhite   = "\033[47m"
)

// Role colors - each debate role has a distinct color
var RoleColors = map[services.DebateRole]string{
	services.RoleAnalyst:   ANSICyan,
	services.RoleProposer:  ANSIGreen,
	services.RoleCritic:    ANSIYellow,
	services.RoleSynthesis: ANSIMagenta,
	services.RoleMediator:  ANSIBlue,
}

// Phase indicators with colors
var PhaseIndicators = map[services.ValidationPhase]struct {
	Icon  string
	Color string
}{
	services.PhaseInitialResponse: {"🔍", ANSICyan},
	services.PhaseValidation:      {"✓", ANSIGreen},
	services.PhasePolishImprove:   {"✨", ANSIYellow},
	services.PhaseFinalConclusion: {"📜", ANSIBrightWhite},
}

// ============================================================================
// Enhanced Debate Response with Timing and Fallback Chain
// ============================================================================

// DebatePositionResponse represents a response from a debate position with metadata
type DebatePositionResponse struct {
	Content         string                      `json:"content"`
	ToolCalls       []StreamingToolCall         `json:"tool_calls,omitempty"`
	Position        services.DebateTeamPosition `json:"position"`
	ResponseTime    time.Duration               `json:"response_time"`
	PrimaryProvider string                      `json:"primary_provider"`
	PrimaryModel    string                      `json:"primary_model"`
	ActualProvider  string                      `json:"actual_provider"`
	ActualModel     string                      `json:"actual_model"`
	UsedFallback    bool                        `json:"used_fallback"`
	FallbackChain   []FallbackAttempt           `json:"fallback_chain,omitempty"`
	Timestamp       time.Time                   `json:"timestamp"`
}

// FallbackAttempt records a single fallback attempt
type FallbackAttempt struct {
	Provider   string        `json:"provider"`
	Model      string        `json:"model"`
	Success    bool          `json:"success"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	AttemptNum int           `json:"attempt_num"`
}

// ============================================================================
// Visualization Formatting Functions
// ============================================================================

// FormatRequestIndicator formats the request indicator showing request being sent to LLM
// Example: [A: Analyst] <--- Request sent to DeepSeek (deepseek-chat)
func FormatRequestIndicator(position services.DebateTeamPosition, role services.DebateRole, provider, model string) string {
	roleColor := getRoleColor(role)
	avatar := getPositionAvatar(position)
	roleName := getRoleName(role)

	return fmt.Sprintf("%s%s[%s: %s]%s %s<---%s Request sent to %s%s%s (%s)\n",
		ANSIDim,                    // Dim for phase content
		roleColor,                  // Role color
		avatar,                     // [A], [P], [C], [S], [M]
		roleName,                   // Analyst, Proposer, etc.
		ANSIReset,
		ANSIBrightBlack,           // Gray arrow
		ANSIReset,
		ANSIBold,                   // Bold provider name
		provider,
		ANSIReset,
		model,
	)
}

// FormatResponseIndicator formats the response indicator showing LLM response received
// Example: [A: Analyst] ---> (450 ms) Response content...
func FormatResponseIndicator(position services.DebateTeamPosition, role services.DebateRole, responseTime time.Duration) string {
	roleColor := getRoleColor(role)
	avatar := getPositionAvatar(position)
	roleName := getRoleName(role)
	timeStr := formatDuration(responseTime)

	return fmt.Sprintf("%s%s[%s: %s]%s %s--->%s %s(%s)%s ",
		ANSIDim,                    // Dim for phase content
		roleColor,                  // Role color
		avatar,                     // [A], [P], [C], [S], [M]
		roleName,                   // Analyst, Proposer, etc.
		ANSIReset,
		ANSIBrightBlack,           // Gray arrow
		ANSIReset,
		ANSIDim+ANSIBrightBlack,   // Darker shade for timing
		timeStr,                    // e.g., "450 ms" or "1.2 s"
		ANSIReset,
	)
}

// FormatFallbackIndicator formats the fallback chain indicator
// Example: [A: Analyst] ---> [Fallback: Sonnet 4.5] ---> (650 ms) Response...
func FormatFallbackIndicator(position services.DebateTeamPosition, role services.DebateRole, fallbackProvider, fallbackModel string, responseTime time.Duration) string {
	roleColor := getRoleColor(role)
	avatar := getPositionAvatar(position)
	roleName := getRoleName(role)
	timeStr := formatDuration(responseTime)

	return fmt.Sprintf("%s%s[%s: %s]%s %s--->%s [%sFallback: %s%s] %s--->%s %s(%s)%s ",
		ANSIDim,                    // Dim for phase content
		roleColor,                  // Role color
		avatar,                     // [A], [P], [C], [S], [M]
		roleName,                   // Analyst, Proposer, etc.
		ANSIReset,
		ANSIBrightBlack,           // Gray arrow
		ANSIReset,
		ANSIYellow,                // Yellow for fallback
		fallbackProvider,
		ANSIReset,
		ANSIBrightBlack,           // Gray arrow
		ANSIReset,
		ANSIDim+ANSIBrightBlack,   // Darker shade for timing
		timeStr,
		ANSIReset,
	)
}

// FormatFallbackChainIndicator formats a complete fallback chain with reasons and timings
// Example: [A: Sonnet 4.5] <--- (10 ms) [Fallback, Rate limit reached: DeepSeek] (100 ms) Response...
// Supports multiple chained fallbacks with individual timing for each attempt
func FormatFallbackChainIndicator(position services.DebateTeamPosition, role services.DebateRole, chain []FallbackAttempt, totalTime time.Duration) string {
	roleColor := getRoleColor(role)
	avatar := getPositionAvatar(position)
	roleName := getRoleName(role)

	var sb strings.Builder

	// Start with role indicator
	sb.WriteString(fmt.Sprintf("%s%s[%s: %s]%s ",
		ANSIDim, roleColor, avatar, roleName, ANSIReset))

	// Format each fallback attempt in the chain
	for i, attempt := range chain {
		if i == 0 && !attempt.Success {
			// First failed attempt (original provider)
			sb.WriteString(fmt.Sprintf("%s<---%s %s(%s)%s ",
				ANSIBrightBlack, ANSIReset,
				ANSIDim+ANSIBrightBlack, formatDuration(attempt.Duration), ANSIReset))
			sb.WriteString(fmt.Sprintf("[%sFallback, %s: %s%s] ",
				ANSIYellow, attempt.Error, attempt.Provider, ANSIReset))
		} else if attempt.Success {
			// Successful fallback
			sb.WriteString(fmt.Sprintf("%s(%s)%s ",
				ANSIDim+ANSIBrightBlack, formatDuration(attempt.Duration), ANSIReset))
		} else {
			// Failed fallback in chain
			sb.WriteString(fmt.Sprintf("%s(%s)%s ",
				ANSIDim+ANSIBrightBlack, formatDuration(attempt.Duration), ANSIReset))
			sb.WriteString(fmt.Sprintf("[%sFallback, %s: %s%s] ",
				ANSIYellow, attempt.Error, attempt.Provider, ANSIReset))
		}
	}

	return sb.String()
}

// FormatFallbackChainWithContent formats a complete fallback chain with the response content
// Example: [A: Sonnet 4.5] <--- (10 ms) [Fallback, Rate limit reached: DeepSeek] (100 ms) Response content...
func FormatFallbackChainWithContent(position services.DebateTeamPosition, role services.DebateRole, chain []FallbackAttempt, content string) string {
	roleColor := getRoleColor(role)
	avatar := getPositionAvatar(position)
	roleName := getRoleName(role)

	var sb strings.Builder

	// Start with role indicator and primary provider
	var primaryTime time.Duration
	if len(chain) > 0 {
		primaryTime = chain[0].Duration
	}

	sb.WriteString(fmt.Sprintf("%s%s[%s: %s]%s ",
		ANSIDim, roleColor, avatar, roleName, ANSIReset))

	// Check if we had any fallbacks
	hadFallback := false
	for _, attempt := range chain {
		if !attempt.Success && attempt.AttemptNum > 0 {
			hadFallback = true
			break
		}
	}

	if hadFallback {
		// Format with fallback chain
		for i, attempt := range chain {
			if !attempt.Success {
				// Failed attempt
				if i == 0 {
					// Original failed
					sb.WriteString(fmt.Sprintf("%s<---%s %s(%s)%s ",
						ANSIBrightBlack, ANSIReset,
						ANSIDim+ANSIBrightBlack, formatDuration(attempt.Duration), ANSIReset))
					sb.WriteString(fmt.Sprintf("[%sFallback, %s: %s%s] ",
						ANSIYellow, formatFallbackReason(attempt.Error), attempt.Provider, ANSIReset))
				} else {
					// Chained fallback failed
					sb.WriteString(fmt.Sprintf("%s(%s)%s ",
						ANSIDim+ANSIBrightBlack, formatDuration(attempt.Duration), ANSIReset))
					sb.WriteString(fmt.Sprintf("[%sFallback, %s: %s%s] ",
						ANSIYellow, formatFallbackReason(attempt.Error), attempt.Provider, ANSIReset))
				}
			} else {
				// Successful attempt (final fallback that worked)
				sb.WriteString(fmt.Sprintf("%s(%s)%s ",
					ANSIDim+ANSIBrightBlack, formatDuration(attempt.Duration), ANSIReset))
			}
		}
	} else if len(chain) > 0 && chain[0].Success {
		// No fallback, direct success
		sb.WriteString(fmt.Sprintf("%s--->%s %s(%s)%s ",
			ANSIBrightBlack, ANSIReset,
			ANSIDim+ANSIBrightBlack, formatDuration(primaryTime), ANSIReset))
	}

	// Add content (dimmed for non-final phases)
	sb.WriteString(content)

	return sb.String()
}

// formatFallbackReason formats the fallback reason for display
func formatFallbackReason(errorMsg string) string {
	// Convert common error messages to user-friendly reasons
	switch {
	case strings.Contains(strings.ToLower(errorMsg), "rate limit"):
		return "Rate limit reached"
	case strings.Contains(strings.ToLower(errorMsg), "timeout"):
		return "Timeout"
	case strings.Contains(strings.ToLower(errorMsg), "connection"):
		return "Connection error"
	case strings.Contains(strings.ToLower(errorMsg), "unavailable"):
		return "Service unavailable"
	case strings.Contains(strings.ToLower(errorMsg), "auth"):
		return "Auth error"
	case strings.Contains(strings.ToLower(errorMsg), "quota"):
		return "Quota exceeded"
	case strings.Contains(strings.ToLower(errorMsg), "overloaded"):
		return "Service overloaded"
	case errorMsg == "":
		return "Provider error"
	default:
		// Truncate long error messages
		if len(errorMsg) > 30 {
			return errorMsg[:27] + "..."
		}
		return errorMsg
	}
}

// FormatPhaseHeader formats a phase header with color
func FormatPhaseHeader(phase services.ValidationPhase, phaseNum int) string {
	indicator, ok := PhaseIndicators[phase]
	if !ok {
		indicator.Icon = "▸"
		indicator.Color = ANSIWhite
	}

	phaseName := getPhaseDisplayName(phase)

	return fmt.Sprintf("\n%s%s══════════════════════════════════════════════════════════════════════════════%s\n"+
		"%s%s %s PHASE %d: %s %s%s\n"+
		"%s══════════════════════════════════════════════════════════════════════════════%s\n\n",
		ANSIDim, ANSIBrightBlack, ANSIReset,
		indicator.Color, ANSIBold, indicator.Icon, phaseNum, phaseName, indicator.Icon, ANSIReset,
		ANSIDim+ANSIBrightBlack, ANSIReset,
	)
}

// FormatPhaseContent formats debate phase content in dim/gray
func FormatPhaseContent(content string) string {
	return fmt.Sprintf("%s%s%s", ANSIDim, content, ANSIReset)
}

// FormatFinalResponse formats the final consensus response in bright white (no dimming)
func FormatFinalResponse(content string) string {
	return fmt.Sprintf("%s%s%s", ANSIBrightWhite, content, ANSIReset)
}

// FormatConsensusHeader formats the consensus section header
func FormatConsensusHeader() string {
	return fmt.Sprintf("\n%s%s═══════════════════════════════════════════════════════════════════════════════%s\n"+
		"%s%s                              📜 CONSENSUS REACHED 📜                              %s\n"+
		"%s═══════════════════════════════════════════════════════════════════════════════%s\n"+
		"%s  The AI Debate Ensemble has synthesized the following response:%s\n"+
		"%s───────────────────────────────────────────────────────────────────────────────%s\n\n",
		ANSIBrightWhite, ANSIBold, ANSIReset,
		ANSIBrightYellow, ANSIBold, ANSIReset,
		ANSIBrightWhite+ANSIBold, ANSIReset,
		ANSIDim, ANSIReset,
		ANSIDim+ANSIBrightBlack, ANSIReset,
	)
}

// FormatDebateTeamIntroduction formats the debate team introduction with colors
func FormatDebateTeamIntroduction(topic string, members []*services.DebateTeamMember) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("\n%s%s╔══════════════════════════════════════════════════════════════════════════════╗%s\n",
		ANSIBrightCyan, ANSIBold, ANSIReset))
	sb.WriteString(fmt.Sprintf("%s%s║                      🎭 HELIXAGENT AI DEBATE ENSEMBLE 🎭                      ║%s\n",
		ANSIBrightCyan, ANSIBold, ANSIReset))
	sb.WriteString(fmt.Sprintf("%s%s╠══════════════════════════════════════════════════════════════════════════════╣%s\n",
		ANSIBrightCyan, ANSIBold, ANSIReset))
	sb.WriteString(fmt.Sprintf("%s%s║  Five AI minds deliberate to synthesize the optimal response.                ║%s\n",
		ANSIDim, ANSIWhite, ANSIReset))
	sb.WriteString(fmt.Sprintf("%s%s╚══════════════════════════════════════════════════════════════════════════════╝%s\n\n",
		ANSIBrightCyan, ANSIBold, ANSIReset))

	// Topic
	topicDisplay := topic
	if len(topicDisplay) > 70 {
		topicDisplay = topicDisplay[:70] + "..."
	}
	sb.WriteString(fmt.Sprintf("%s📋 TOPIC:%s %s\n\n", ANSIBold, ANSIReset, topicDisplay))

	// Cast of characters
	sb.WriteString(fmt.Sprintf("%s%s═══════════════════════════════════════════════════════════════════════════════%s\n",
		ANSIDim, ANSIBrightBlack, ANSIReset))
	sb.WriteString(fmt.Sprintf("%s%s                              DRAMATIS PERSONAE%s\n",
		ANSIBold, ANSIWhite, ANSIReset))
	sb.WriteString(fmt.Sprintf("%s%s═══════════════════════════════════════════════════════════════════════════════%s\n\n",
		ANSIDim, ANSIBrightBlack, ANSIReset))

	for _, member := range members {
		if member == nil {
			continue
		}
		roleColor := getRoleColor(member.Role)
		avatar := getPositionAvatar(member.Position)
		roleName := getRoleName(member.Role)

		sb.WriteString(fmt.Sprintf("  %s%s%s  %-15s%s │ %s%s%s (%s)\n",
			roleColor, ANSIBold, avatar, roleName, ANSIReset,
			ANSIBold, member.ModelName, ANSIReset, member.ProviderName))

		// Show fallback if present
		if member.Fallback != nil {
			sb.WriteString(fmt.Sprintf("      %s└─ Fallback: %s (%s)%s\n",
				ANSIDim, member.Fallback.ModelName, member.Fallback.ProviderName, ANSIReset))
		}
	}

	sb.WriteString(fmt.Sprintf("\n%s%s═══════════════════════════════════════════════════════════════════════════════%s\n",
		ANSIDim, ANSIBrightBlack, ANSIReset))
	sb.WriteString(fmt.Sprintf("%s%s                               THE DELIBERATION%s\n",
		ANSIBold, ANSIWhite, ANSIReset))
	sb.WriteString(fmt.Sprintf("%s%s═══════════════════════════════════════════════════════════════════════════════%s\n\n",
		ANSIDim, ANSIBrightBlack, ANSIReset))

	return sb.String()
}

// ============================================================================
// Helper Functions
// ============================================================================

func getRoleColor(role services.DebateRole) string {
	if color, ok := RoleColors[role]; ok {
		return color
	}
	return ANSIWhite
}

func getPositionAvatar(position services.DebateTeamPosition) string {
	avatars := map[services.DebateTeamPosition]string{
		services.PositionAnalyst:   "A",
		services.PositionProposer:  "P",
		services.PositionCritic:    "C",
		services.PositionSynthesis: "S",
		services.PositionMediator:  "M",
	}
	if avatar, ok := avatars[position]; ok {
		return avatar
	}
	return "?"
}

func getRoleName(role services.DebateRole) string {
	names := map[services.DebateRole]string{
		services.RoleAnalyst:   "Analyst",
		services.RoleProposer:  "Proposer",
		services.RoleCritic:    "Critic",
		services.RoleSynthesis: "Synthesis",
		services.RoleMediator:  "Mediator",
	}
	if name, ok := names[role]; ok {
		return name
	}
	return string(role)
}

func getPhaseDisplayName(phase services.ValidationPhase) string {
	names := map[services.ValidationPhase]string{
		services.PhaseInitialResponse: "INITIAL RESPONSE",
		services.PhaseValidation:      "VALIDATION",
		services.PhasePolishImprove:   "POLISH & IMPROVE",
		services.PhaseFinalConclusion: "FINAL CONCLUSION",
	}
	if name, ok := names[phase]; ok {
		return name
	}
	return string(phase)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1f s", d.Seconds())
}

// StripANSI removes ANSI escape codes from a string (for non-terminal output)
func StripANSI(s string) string {
	// Simple replacement of common ANSI codes
	result := s
	codes := []string{
		ANSIReset, ANSIBlack, ANSIRed, ANSIGreen, ANSIYellow, ANSIBlue, ANSIMagenta, ANSICyan, ANSIWhite,
		ANSIBrightBlack, ANSIBrightRed, ANSIBrightGreen, ANSIBrightYellow, ANSIBrightBlue,
		ANSIBrightMagenta, ANSIBrightCyan, ANSIBrightWhite, ANSIDim, ANSIBold, ANSIItalic, ANSIUnderline,
		ANSIBgBlack, ANSIBgRed, ANSIBgGreen, ANSIBgYellow, ANSIBgBlue, ANSIBgMagenta, ANSIBgCyan, ANSIBgWhite,
	}
	for _, code := range codes {
		result = strings.ReplaceAll(result, code, "")
	}
	return result
}
