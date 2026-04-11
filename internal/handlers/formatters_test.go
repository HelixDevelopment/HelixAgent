package handlers

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"dev.helix.agent/internal/services"
)

// ============================================================================
// Test Helpers
// ============================================================================

// createTestMembers creates a standard set of test members
func createTestMembers() []*services.DebateTeamMember {
	return []*services.DebateTeamMember{
		{
			Position:     services.PositionAnalyst,
			Role:         services.RoleAnalyst,
			ModelName:    "claude-opus-4-5",
			ProviderName: "anthropic",
			Fallback: &services.DebateTeamMember{
				ModelName:    "gpt-4",
				ProviderName: "openai",
			},
		},
		{
			Position:     services.PositionProposer,
			Role:         services.RoleProposer,
			ModelName:    "gemini-2.0-flash",
			ProviderName: "google",
		},
		{
			Position:     services.PositionCritic,
			Role:         services.RoleCritic,
			ModelName:    "deepseek-chat",
			ProviderName: "deepseek",
		},
		{
			Position:     services.PositionSynthesis,
			Role:         services.RoleSynthesis,
			ModelName:    "mistral-large",
			ProviderName: "mistral",
		},
		{
			Position:     services.PositionMediator,
			Role:         services.RoleMediator,
			ModelName:    "qwen-max",
			ProviderName: "qwen",
		},
	}
}

// ============================================================================
// JSON Formatter Tests
// ============================================================================

func TestJSONFormatter_Name(t *testing.T) {
	t.Parallel()
	f := NewJSONFormatter()
	assert.Equal(t, "json", f.Name())
}

func TestJSONFormatter_ContentType(t *testing.T) {
	t.Parallel()
	f := NewJSONFormatter()
	assert.Equal(t, "application/json", f.ContentType())
}

func TestJSONFormatter_FormatDebateTeamIntroduction(t *testing.T) {
	t.Parallel()
	f := NewJSONFormatter()
	members := createTestMembers()

	t.Run("Produces valid JSON", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		var intro JSONDebateIntroduction
		err := json.Unmarshal([]byte(result), &intro)
		require.NoError(t, err)

		assert.Equal(t, "HelixAgent AI Debate Ensemble", intro.Title)
		assert.Equal(t, "What is AI?", intro.Topic)
		assert.Len(t, intro.Team, 5)
		assert.NotEmpty(t, intro.Timestamp)
	})

	t.Run("Contains team members with correct structure", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)

		var intro JSONDebateIntroduction
		err := json.Unmarshal([]byte(result), &intro)
		require.NoError(t, err)

		// Check first member with fallback
		assert.Equal(t, "analyst", intro.Team[0].Role)
		assert.Equal(t, "claude-opus-4-5", intro.Team[0].Model)
		assert.Equal(t, "anthropic", intro.Team[0].Provider)
		assert.NotNil(t, intro.Team[0].Fallback)
		assert.Equal(t, "gpt-4", intro.Team[0].Fallback.Model)
	})

	t.Run("Handles nil members", func(t *testing.T) {
		t.Parallel()
		membersWithNil := []*services.DebateTeamMember{nil, members[0], nil}
		result := f.FormatDebateTeamIntroduction("Test", membersWithNil)

		var intro JSONDebateIntroduction
		err := json.Unmarshal([]byte(result), &intro)
		require.NoError(t, err)
		assert.Len(t, intro.Team, 1)
	})
}

func TestJSONFormatter_FormatPhaseHeader(t *testing.T) {
	t.Parallel()
	f := NewJSONFormatter()

	testCases := []struct {
		phase    services.ValidationPhase
		phaseNum int
	}{
		{services.PhaseInitialResponse, 1},
		{services.PhaseValidation, 2},
		{services.PhasePolishImprove, 3},
		{services.PhaseFinalConclusion, 4},
	}

	for _, tc := range testCases {
		t.Run(string(tc.phase), func(t *testing.T) {
			t.Parallel()
			result := f.FormatPhaseHeader(tc.phase, tc.phaseNum)

			var header JSONPhaseHeader
			err := json.Unmarshal([]byte(result), &header)
			require.NoError(t, err)

			assert.Equal(t, string(tc.phase), header.Phase)
			assert.Equal(t, tc.phaseNum, header.PhaseNum)
			assert.NotEmpty(t, header.Icon)
			assert.NotEmpty(t, header.Timestamp)
		})
	}
}

func TestJSONFormatter_FormatPhaseContent(t *testing.T) {
	t.Parallel()
	f := NewJSONFormatter()

	t.Run("Produces valid JSON", func(t *testing.T) {
		t.Parallel()
		content := "This is the phase content with multiple lines\nand special characters: \"quotes\" and {braces}"
		result := f.FormatPhaseContent(content)

		var pc JSONPhaseContent
		err := json.Unmarshal([]byte(result), &pc)
		require.NoError(t, err)

		assert.Equal(t, content, pc.Content)
		assert.NotEmpty(t, pc.Timestamp)
	})
}

func TestJSONFormatter_FormatFinalResponse(t *testing.T) {
	t.Parallel()
	f := NewJSONFormatter()

	t.Run("Produces valid JSON", func(t *testing.T) {
		t.Parallel()
		content := "This is the final answer."
		result := f.FormatFinalResponse(content)

		var resp JSONFinalResponse
		err := json.Unmarshal([]byte(result), &resp)
		require.NoError(t, err)

		assert.Equal(t, "final_response", resp.Type)
		assert.Equal(t, content, resp.Content)
		assert.NotEmpty(t, resp.Timestamp)
	})
}

func TestJSONFormatter_FormatFallbackIndicator(t *testing.T) {
	t.Parallel()
	f := NewJSONFormatter()

	t.Run("Produces valid JSON with all fields", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFallbackIndicator(
			services.RoleAnalyst,
			"openai", "gpt-4",
			"anthropic", "claude-3",
			"rate limit exceeded",
			500*time.Millisecond,
		)

		var indicator JSONFallbackIndicator
		err := json.Unmarshal([]byte(result), &indicator)
		require.NoError(t, err)

		assert.Equal(t, "fallback", indicator.Type)
		assert.Equal(t, "analyst", indicator.Role)
		assert.Equal(t, "openai", indicator.FromProvider)
		assert.Equal(t, "gpt-4", indicator.FromModel)
		assert.Equal(t, "anthropic", indicator.ToProvider)
		assert.Equal(t, "claude-3", indicator.ToModel)
		assert.Equal(t, "rate limit exceeded", indicator.Reason)
		assert.Contains(t, indicator.Duration, "ms")
	})
}

// ============================================================================
// YAML Formatter Tests
// ============================================================================

func TestYAMLFormatter_Name(t *testing.T) {
	t.Parallel()
	f := NewYAMLFormatter()
	assert.Equal(t, "yaml", f.Name())
}

func TestYAMLFormatter_ContentType(t *testing.T) {
	t.Parallel()
	f := NewYAMLFormatter()
	assert.Equal(t, "application/x-yaml", f.ContentType())
}

func TestYAMLFormatter_FormatDebateTeamIntroduction(t *testing.T) {
	t.Parallel()
	f := NewYAMLFormatter()
	members := createTestMembers()

	t.Run("Produces valid YAML", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		// Remove YAML document separator for parsing
		result = strings.TrimPrefix(result, "---\n")

		var intro YAMLDebateIntroduction
		err := yaml.Unmarshal([]byte(result), &intro)
		require.NoError(t, err)

		assert.Equal(t, "HelixAgent AI Debate Ensemble", intro.Title)
		assert.Equal(t, "What is AI?", intro.Topic)
		assert.Len(t, intro.Team, 5)
	})

	t.Run("Starts with YAML document separator", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)
		assert.True(t, strings.HasPrefix(result, "---\n"))
	})

	t.Run("Contains fallback information", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)
		assert.Contains(t, result, "fallback:")
		assert.Contains(t, result, "gpt-4")
	})
}

func TestYAMLFormatter_FormatPhaseHeader(t *testing.T) {
	t.Parallel()
	f := NewYAMLFormatter()

	t.Run("Produces valid YAML", func(t *testing.T) {
		t.Parallel()
		result := f.FormatPhaseHeader(services.PhaseInitialResponse, 1)
		result = strings.TrimPrefix(result, "---\n")

		var header YAMLPhaseHeader
		err := yaml.Unmarshal([]byte(result), &header)
		require.NoError(t, err)

		assert.Equal(t, string(services.PhaseInitialResponse), header.Phase)
		assert.Equal(t, 1, header.PhaseNum)
	})
}

func TestYAMLFormatter_FormatPhaseContent(t *testing.T) {
	t.Parallel()
	f := NewYAMLFormatter()

	t.Run("Produces valid YAML", func(t *testing.T) {
		t.Parallel()
		content := "Multiline content\nwith multiple lines"
		result := f.FormatPhaseContent(content)

		var pc YAMLPhaseContent
		err := yaml.Unmarshal([]byte(result), &pc)
		require.NoError(t, err)

		assert.Equal(t, content, pc.Content)
	})
}

func TestYAMLFormatter_FormatFinalResponse(t *testing.T) {
	t.Parallel()
	f := NewYAMLFormatter()

	t.Run("Produces valid YAML", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFinalResponse("Final answer here")
		result = strings.TrimPrefix(result, "---\n")

		var resp YAMLFinalResponse
		err := yaml.Unmarshal([]byte(result), &resp)
		require.NoError(t, err)

		assert.Equal(t, "final_response", resp.Type)
		assert.Equal(t, "Final answer here", resp.Content)
	})
}

func TestYAMLFormatter_FormatFallbackIndicator(t *testing.T) {
	t.Parallel()
	f := NewYAMLFormatter()

	t.Run("Produces valid YAML", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFallbackIndicator(
			services.RoleCritic,
			"google", "gemini",
			"mistral", "mistral-large",
			"timeout",
			2*time.Second,
		)
		result = strings.TrimPrefix(result, "---\n")

		var indicator YAMLFallbackIndicator
		err := yaml.Unmarshal([]byte(result), &indicator)
		require.NoError(t, err)

		assert.Equal(t, "critic", indicator.Role)
		assert.Equal(t, "google", indicator.FromProvider)
		assert.Equal(t, "timeout", indicator.Reason)
	})
}

// ============================================================================
// HTML Formatter Tests
// ============================================================================

func TestHTMLFormatter_Name(t *testing.T) {
	t.Parallel()
	f := NewHTMLFormatter()
	assert.Equal(t, "html", f.Name())
}

func TestHTMLFormatter_ContentType(t *testing.T) {
	t.Parallel()
	f := NewHTMLFormatter()
	assert.Equal(t, "text/html", f.ContentType())
}

func TestHTMLFormatter_FormatDebateTeamIntroduction(t *testing.T) {
	t.Parallel()
	f := NewHTMLFormatter()
	members := createTestMembers()

	t.Run("Contains HTML structure", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		assert.Contains(t, result, "<style>")
		assert.Contains(t, result, "</style>")
		assert.Contains(t, result, `<div class="debate-container">`)
		assert.Contains(t, result, `<div class="debate-header">`)
		assert.Contains(t, result, "<h1>")
		assert.Contains(t, result, "</h1>")
		assert.Contains(t, result, "<table")
		assert.Contains(t, result, "</table>")
	})

	t.Run("Escapes HTML special characters", func(t *testing.T) {
		t.Parallel()
		membersWithSpecial := []*services.DebateTeamMember{
			{
				Role:         services.RoleAnalyst,
				ModelName:    "model<script>alert('xss')</script>",
				ProviderName: "provider&company",
			},
		}
		result := f.FormatDebateTeamIntroduction("<script>alert('xss')</script>", membersWithSpecial)

		assert.NotContains(t, result, "<script>")
		assert.Contains(t, result, "&lt;script&gt;")
	})

	t.Run("Contains role-specific CSS classes", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)

		assert.Contains(t, result, `class="role-analyst"`)
		assert.Contains(t, result, `class="role-proposer"`)
	})

	t.Run("Handles long topics", func(t *testing.T) {
		t.Parallel()
		longTopic := strings.Repeat("x", 150)
		result := f.FormatDebateTeamIntroduction(longTopic, members)

		assert.Contains(t, result, "...")
	})

	t.Run("Without styles", func(t *testing.T) {
		t.Parallel()
		f := &HTMLFormatter{IncludeStyles: false}
		result := f.FormatDebateTeamIntroduction("Test", members)

		assert.NotContains(t, result, "<style>")
	})
}

func TestHTMLFormatter_FormatPhaseHeader(t *testing.T) {
	t.Parallel()
	f := NewHTMLFormatter()

	t.Run("Contains proper HTML structure", func(t *testing.T) {
		t.Parallel()
		result := f.FormatPhaseHeader(services.PhaseValidation, 2)

		assert.Contains(t, result, `<div class="phase-header">`)
		assert.Contains(t, result, "<h3>")
		assert.Contains(t, result, "Phase 2")
		assert.Contains(t, result, "VALIDATION")
	})
}

func TestHTMLFormatter_FormatPhaseContent(t *testing.T) {
	t.Parallel()
	f := NewHTMLFormatter()

	t.Run("Escapes HTML in content", func(t *testing.T) {
		t.Parallel()
		content := "<p>Content with HTML</p>"
		result := f.FormatPhaseContent(content)

		assert.Contains(t, result, `<div class="phase-content">`)
		assert.Contains(t, result, "&lt;p&gt;")
	})
}

func TestHTMLFormatter_FormatFinalResponse(t *testing.T) {
	t.Parallel()
	f := NewHTMLFormatter()

	t.Run("Contains final response styling", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFinalResponse("The final answer")

		assert.Contains(t, result, `<div class="final-response">`)
		assert.Contains(t, result, "<h2>Final Answer</h2>")
		assert.Contains(t, result, "The final answer")
	})
}

func TestHTMLFormatter_FormatFallbackIndicator(t *testing.T) {
	t.Parallel()
	f := NewHTMLFormatter()

	t.Run("Contains fallback styling", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFallbackIndicator(
			services.RoleAnalyst,
			"openai", "gpt-4",
			"anthropic", "claude-3",
			"connection error",
			1*time.Second,
		)

		assert.Contains(t, result, `<div class="fallback-indicator">`)
		assert.Contains(t, result, "[Analyst]")
		assert.Contains(t, result, "openai/gpt-4")
		assert.Contains(t, result, "anthropic/claude-3")
	})
}

// ============================================================================
// XML Formatter Tests
// ============================================================================

func TestXMLFormatter_Name(t *testing.T) {
	t.Parallel()
	f := NewXMLFormatter()
	assert.Equal(t, "xml", f.Name())
}

func TestXMLFormatter_ContentType(t *testing.T) {
	t.Parallel()
	f := NewXMLFormatter()
	assert.Equal(t, "application/xml", f.ContentType())
}

func TestXMLFormatter_FormatDebateTeamIntroduction(t *testing.T) {
	t.Parallel()
	f := NewXMLFormatter()
	members := createTestMembers()

	t.Run("Produces valid XML", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		var intro XMLDebateIntroduction
		err := xml.Unmarshal([]byte(result), &intro)
		require.NoError(t, err)

		assert.Equal(t, "HelixAgent AI Debate Ensemble", intro.Title)
		assert.Equal(t, "What is AI?", intro.Topic)
		assert.Len(t, intro.Team.Members, 5)
	})

	t.Run("Contains XML declaration", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)
		assert.True(t, strings.HasPrefix(result, "<?xml"))
	})

	t.Run("Contains position attributes", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)
		assert.Contains(t, result, `position="1"`)
		assert.Contains(t, result, `position="2"`)
	})
}

func TestXMLFormatter_FormatPhaseHeader(t *testing.T) {
	t.Parallel()
	f := NewXMLFormatter()

	t.Run("Produces valid XML", func(t *testing.T) {
		t.Parallel()
		result := f.FormatPhaseHeader(services.PhasePolishImprove, 3)

		var header XMLPhaseHeader
		err := xml.Unmarshal([]byte(result), &header)
		require.NoError(t, err)

		assert.Equal(t, string(services.PhasePolishImprove), header.Phase)
		assert.Equal(t, 3, header.PhaseNum)
	})
}

func TestXMLFormatter_FormatPhaseContent(t *testing.T) {
	t.Parallel()
	f := NewXMLFormatter()

	t.Run("Produces valid XML", func(t *testing.T) {
		t.Parallel()
		content := "Content with <special> & characters"
		result := f.FormatPhaseContent(content)

		var pc XMLPhaseContent
		err := xml.Unmarshal([]byte(result), &pc)
		require.NoError(t, err)

		assert.Equal(t, content, pc.Content)
	})
}

func TestXMLFormatter_FormatFinalResponse(t *testing.T) {
	t.Parallel()
	f := NewXMLFormatter()

	t.Run("Produces valid XML", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFinalResponse("Final answer")

		var resp XMLFinalResponse
		err := xml.Unmarshal([]byte(result), &resp)
		require.NoError(t, err)

		assert.Equal(t, "Final answer", resp.Content)
	})
}

func TestXMLFormatter_FormatFallbackIndicator(t *testing.T) {
	t.Parallel()
	f := NewXMLFormatter()

	t.Run("Produces valid XML", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFallbackIndicator(
			services.RoleMediator,
			"deepseek", "deepseek-chat",
			"qwen", "qwen-max",
			"quota exceeded",
			750*time.Millisecond,
		)

		var indicator XMLFallbackIndicator
		err := xml.Unmarshal([]byte(result), &indicator)
		require.NoError(t, err)

		assert.Equal(t, "mediator", indicator.Role)
		assert.Equal(t, "deepseek", indicator.FromProvider)
		assert.Equal(t, "qwen", indicator.ToProvider)
		assert.Equal(t, "quota exceeded", indicator.Reason)
	})
}

// ============================================================================
// CSV Formatter Tests
// ============================================================================

func TestCSVFormatter_Name(t *testing.T) {
	t.Parallel()
	f := NewCSVFormatter()
	assert.Equal(t, "csv", f.Name())
}

func TestCSVFormatter_ContentType(t *testing.T) {
	t.Parallel()
	f := NewCSVFormatter()
	assert.Equal(t, "text/csv", f.ContentType())
}

func TestCSVFormatter_FormatDebateTeamIntroduction(t *testing.T) {
	t.Parallel()
	f := NewCSVFormatter()
	members := createTestMembers()

	t.Run("Contains CSV header", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		assert.Contains(t, result, "Position,Role,Model,Provider,Fallback_Model,Fallback_Provider")
	})

	t.Run("Contains comment with topic", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		assert.Contains(t, result, "# Topic: What is AI?")
	})

	t.Run("Contains team data rows", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)

		assert.Contains(t, result, "1,analyst,claude-opus-4-5,anthropic,gpt-4,openai")
		assert.Contains(t, result, "2,proposer,gemini-2.0-flash,google,,")
	})

	t.Run("Custom delimiter", func(t *testing.T) {
		t.Parallel()
		f := &CSVFormatter{Delimiter: ';'}
		result := f.FormatDebateTeamIntroduction("Test", members)

		assert.Contains(t, result, "Position;Role;Model;Provider")
	})
}

func TestCSVFormatter_FormatPhaseHeader(t *testing.T) {
	t.Parallel()
	f := NewCSVFormatter()

	t.Run("Contains header and data row", func(t *testing.T) {
		t.Parallel()
		result := f.FormatPhaseHeader(services.PhaseValidation, 2)

		assert.Contains(t, result, "Phase_Type,Phase_Num,Icon,Timestamp")
		assert.Contains(t, result, "validation,2")
	})
}

func TestCSVFormatter_FormatPhaseContent(t *testing.T) {
	t.Parallel()
	f := NewCSVFormatter()

	t.Run("Properly escapes content", func(t *testing.T) {
		t.Parallel()
		content := `Content with "quotes" and, commas`
		result := f.FormatPhaseContent(content)

		// CSV escapes quotes by doubling them and wraps fields with special chars
		assert.Contains(t, result, `"Content with ""quotes"" and, commas"`)
	})
}

func TestCSVFormatter_FormatFinalResponse(t *testing.T) {
	t.Parallel()
	f := NewCSVFormatter()

	t.Run("Contains type indicator", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFinalResponse("Final answer")

		assert.Contains(t, result, "Type,Content,Timestamp")
		assert.Contains(t, result, "final_response,Final answer")
	})
}

func TestCSVFormatter_FormatFallbackIndicator(t *testing.T) {
	t.Parallel()
	f := NewCSVFormatter()

	t.Run("Contains all fallback fields", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFallbackIndicator(
			services.RoleSynthesis,
			"mistral", "mistral-large",
			"cerebras", "llama-3.3-70b",
			"service unavailable",
			3*time.Second,
		)

		assert.Contains(t, result, "Role,From_Provider,From_Model,To_Provider,To_Model,Reason,Duration")
		assert.Contains(t, result, "synthesis,mistral,mistral-large,cerebras,llama-3.3-70b,service unavailable")
	})
}

// ============================================================================
// RTF Formatter Tests
// ============================================================================

func TestRTFFormatter_Name(t *testing.T) {
	t.Parallel()
	f := NewRTFFormatter()
	assert.Equal(t, "rtf", f.Name())
}

func TestRTFFormatter_ContentType(t *testing.T) {
	t.Parallel()
	f := NewRTFFormatter()
	assert.Equal(t, "application/rtf", f.ContentType())
}

func TestRTFFormatter_FormatDebateTeamIntroduction(t *testing.T) {
	t.Parallel()
	f := NewRTFFormatter()
	members := createTestMembers()

	t.Run("Contains RTF header", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		assert.True(t, strings.HasPrefix(result, "{\\rtf1"))
		assert.Contains(t, result, "\\fonttbl")
		assert.Contains(t, result, "\\colortbl")
	})

	t.Run("Contains title and topic", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		assert.Contains(t, result, "HelixAgent AI Debate Ensemble")
		assert.Contains(t, result, "What is AI?")
	})

	t.Run("Contains team members", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)

		assert.Contains(t, result, "Analyst")
		assert.Contains(t, result, "claude-opus-4-5")
	})

	t.Run("Escapes RTF special characters", func(t *testing.T) {
		t.Parallel()
		membersWithSpecial := []*services.DebateTeamMember{
			{
				Role:         services.RoleAnalyst,
				ModelName:    "model{with}braces",
				ProviderName: "provider\\backslash",
			},
		}
		result := f.FormatDebateTeamIntroduction("Test", membersWithSpecial)

		assert.Contains(t, result, "model\\{with\\}braces")
		assert.Contains(t, result, "provider\\\\backslash")
	})
}

func TestRTFFormatter_FormatPhaseHeader(t *testing.T) {
	t.Parallel()
	f := NewRTFFormatter()

	t.Run("Contains RTF structure", func(t *testing.T) {
		t.Parallel()
		result := f.FormatPhaseHeader(services.PhaseInitialResponse, 1)

		assert.True(t, strings.HasPrefix(result, "{\\rtf1"))
		assert.Contains(t, result, "Phase 1")
		assert.Contains(t, result, "INITIAL RESPONSE")
	})
}

func TestRTFFormatter_FormatPhaseContent(t *testing.T) {
	t.Parallel()
	f := NewRTFFormatter()

	t.Run("Contains content", func(t *testing.T) {
		t.Parallel()
		result := f.FormatPhaseContent("Test content")

		assert.True(t, strings.HasPrefix(result, "{\\rtf1"))
		assert.Contains(t, result, "Test content")
	})
}

func TestRTFFormatter_FormatFinalResponse(t *testing.T) {
	t.Parallel()
	f := NewRTFFormatter()

	t.Run("Contains final answer heading", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFinalResponse("Final answer here")

		assert.True(t, strings.HasPrefix(result, "{\\rtf1"))
		assert.Contains(t, result, "Final Answer")
		assert.Contains(t, result, "Final answer here")
	})
}

func TestRTFFormatter_FormatFallbackIndicator(t *testing.T) {
	t.Parallel()
	f := NewRTFFormatter()

	t.Run("Contains fallback information", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFallbackIndicator(
			services.RoleProposer,
			"google", "gemini",
			"openai", "gpt-4",
			"rate limit",
			500*time.Millisecond,
		)

		assert.True(t, strings.HasPrefix(result, "{\\rtf1"))
		assert.Contains(t, result, "Fallback")
		assert.Contains(t, result, "gemini")
		assert.Contains(t, result, "gpt-4")
	})
}

// ============================================================================
// Terminal Formatter Tests
// ============================================================================

func TestTerminalFormatter_Name(t *testing.T) {
	t.Parallel()
	f := NewTerminalFormatter()
	assert.Equal(t, "terminal", f.Name())
}

func TestTerminalFormatter_ContentType(t *testing.T) {
	t.Parallel()
	f := NewTerminalFormatter()
	assert.Equal(t, "text/plain", f.ContentType())
}

func TestTerminalFormatter_FormatDebateTeamIntroduction(t *testing.T) {
	t.Parallel()
	f := NewTerminalFormatter()
	members := createTestMembers()

	t.Run("Contains ANSI codes", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		assert.True(t, ContainsANSI(result))
	})

	t.Run("Contains header elements", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		assert.Contains(t, result, "HELIXAGENT AI DEBATE ENSEMBLE")
		assert.Contains(t, result, "What is AI?")
	})

	t.Run("Contains team information", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)

		assert.Contains(t, result, "Analyst")
		assert.Contains(t, result, "claude-opus-4-5")
		assert.Contains(t, result, "anthropic")
	})

	t.Run("With 256 colors enabled", func(t *testing.T) {
		t.Parallel()
		f := &TerminalFormatter{Use256Colors: true}
		result := f.FormatDebateTeamIntroduction("Test", members)

		assert.Contains(t, result, "\033[38;5;") // 256-color prefix
	})
}

func TestTerminalFormatter_FormatPhaseHeader(t *testing.T) {
	t.Parallel()
	f := NewTerminalFormatter()

	t.Run("Contains ANSI formatting", func(t *testing.T) {
		t.Parallel()
		result := f.FormatPhaseHeader(services.PhaseValidation, 2)

		assert.True(t, ContainsANSI(result))
		assert.Contains(t, result, "Phase 2")
		assert.Contains(t, result, "VALIDATION")
	})
}

func TestTerminalFormatter_FormatPhaseContent(t *testing.T) {
	t.Parallel()
	f := NewTerminalFormatter()

	t.Run("Applies dim formatting", func(t *testing.T) {
		t.Parallel()
		result := f.FormatPhaseContent("Test content")

		assert.Contains(t, result, ANSIDim)
		assert.Contains(t, result, "Test content")
	})
}

func TestTerminalFormatter_FormatFinalResponse(t *testing.T) {
	t.Parallel()
	f := NewTerminalFormatter()

	t.Run("Contains final answer header", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFinalResponse("The answer")

		assert.True(t, ContainsANSI(result))
		assert.Contains(t, result, "FINAL ANSWER")
		assert.Contains(t, result, "The answer")
	})
}

func TestTerminalFormatter_FormatFallbackIndicator(t *testing.T) {
	t.Parallel()
	f := NewTerminalFormatter()

	t.Run("Contains colored fallback info", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFallbackIndicator(
			services.RoleCritic,
			"deepseek", "deepseek-chat",
			"mistral", "mistral-large",
			"timeout",
			1500*time.Millisecond,
		)

		assert.True(t, ContainsANSI(result))
		assert.Contains(t, result, "Fallback")
		assert.Contains(t, result, "deepseek-chat")
		assert.Contains(t, result, "mistral-large")
	})
}

// ============================================================================
// Compact Formatter Tests
// ============================================================================

func TestCompactFormatter_Name(t *testing.T) {
	t.Parallel()
	f := NewCompactFormatter()
	assert.Equal(t, "compact", f.Name())
}

func TestCompactFormatter_ContentType(t *testing.T) {
	t.Parallel()
	f := NewCompactFormatter()
	assert.Equal(t, "text/plain", f.ContentType())
}

func TestCompactFormatter_FormatDebateTeamIntroduction(t *testing.T) {
	t.Parallel()
	f := NewCompactFormatter()
	members := createTestMembers()

	t.Run("Uses compact format", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("What is AI?", members)

		assert.Contains(t, result, "DEBATE:")
		assert.Contains(t, result, "TEAM:")
		assert.NotContains(t, result, "\n\n") // No double newlines
	})

	t.Run("Contains abbreviated team info", func(t *testing.T) {
		t.Parallel()
		result := f.FormatDebateTeamIntroduction("Test", members)

		assert.Contains(t, result, "analyst=claude-opus-4-5")
		assert.Contains(t, result, "(fb:gpt-4)") // Fallback notation
	})

	t.Run("Truncates long topics", func(t *testing.T) {
		t.Parallel()
		longTopic := strings.Repeat("x", 100)
		result := f.FormatDebateTeamIntroduction(longTopic, members)

		assert.Contains(t, result, "...")
	})
}

func TestCompactFormatter_FormatPhaseHeader(t *testing.T) {
	t.Parallel()
	f := NewCompactFormatter()

	t.Run("Uses compact notation", func(t *testing.T) {
		t.Parallel()
		result := f.FormatPhaseHeader(services.PhaseInitialResponse, 1)

		assert.Equal(t, "[P1:initial_response]", result)
	})
}

func TestCompactFormatter_FormatPhaseContent(t *testing.T) {
	t.Parallel()
	f := NewCompactFormatter()

	t.Run("Removes excessive whitespace", func(t *testing.T) {
		t.Parallel()
		content := "Line 1\n\n\nLine 2  with  spaces"
		result := f.FormatPhaseContent(content)

		assert.NotContains(t, result, "\n\n")
		assert.NotContains(t, result, "  ")
	})

	t.Run("Trims content", func(t *testing.T) {
		t.Parallel()
		content := "  content with spaces  "
		result := f.FormatPhaseContent(content)

		assert.Equal(t, "content with spaces", result)
	})
}

func TestCompactFormatter_FormatFinalResponse(t *testing.T) {
	t.Parallel()
	f := NewCompactFormatter()

	t.Run("Uses compact prefix", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFinalResponse("Answer")

		assert.True(t, strings.HasPrefix(result, "[FINAL]"))
		assert.Contains(t, result, "Answer")
	})
}

func TestCompactFormatter_FormatFallbackIndicator(t *testing.T) {
	t.Parallel()
	f := NewCompactFormatter()

	t.Run("Uses compact notation", func(t *testing.T) {
		t.Parallel()
		result := f.FormatFallbackIndicator(
			services.RoleAnalyst,
			"openai", "gpt-4",
			"anthropic", "claude-3",
			"rate limit",
			500*time.Millisecond,
		)

		assert.Contains(t, result, "[FB:")
		assert.Contains(t, result, "analyst")
		assert.Contains(t, result, "gpt-4")
		assert.Contains(t, result, "claude-3")
	})
}

// ============================================================================
// Formatter Registry Tests
// ============================================================================

func TestFormatterRegistry_NewFormatterRegistry(t *testing.T) {
	t.Parallel()
	registry := NewFormatterRegistry()

	t.Run("Contains all formatters", func(t *testing.T) {
		t.Parallel()
		formats := registry.List()

		assert.Contains(t, formats, "json")
		assert.Contains(t, formats, "yaml")
		assert.Contains(t, formats, "html")
		assert.Contains(t, formats, "xml")
		assert.Contains(t, formats, "csv")
		assert.Contains(t, formats, "rtf")
		assert.Contains(t, formats, "terminal")
		assert.Contains(t, formats, "compact")
	})
}

func TestFormatterRegistry_Get(t *testing.T) {
	t.Parallel()
	registry := NewFormatterRegistry()

	t.Run("Returns correct formatter", func(t *testing.T) {
		t.Parallel()
		jsonFormatter := registry.Get(OutputFormatJSON)
		assert.NotNil(t, jsonFormatter)
		assert.Equal(t, "json", jsonFormatter.Name())

		htmlFormatter := registry.Get(OutputFormatHTML)
		assert.NotNil(t, htmlFormatter)
		assert.Equal(t, "html", htmlFormatter.Name())
	})

	t.Run("Returns nil for unknown format", func(t *testing.T) {
		t.Parallel()
		formatter := registry.Get(OutputFormat("unknown"))
		assert.Nil(t, formatter)
	})
}

func TestFormatterRegistry_GetOrDefault(t *testing.T) {
	t.Parallel()
	registry := NewFormatterRegistry()

	t.Run("Returns requested formatter when available", func(t *testing.T) {
		t.Parallel()
		formatter := registry.GetOrDefault(OutputFormatJSON, OutputFormatMarkdown)
		assert.Equal(t, "json", formatter.Name())
	})

	t.Run("Returns default when requested not available", func(t *testing.T) {
		t.Parallel()
		formatter := registry.GetOrDefault(OutputFormat("unknown"), OutputFormatJSON)
		assert.Equal(t, "json", formatter.Name())
	})
}

func TestFormatterRegistry_Register(t *testing.T) {
	t.Parallel()
	registry := NewFormatterRegistry()

	t.Run("Can register custom formatter", func(t *testing.T) {
		t.Parallel()
		customFormatter := NewCompactFormatter()
		registry.Register(OutputFormat("custom"), customFormatter)

		formatter := registry.Get(OutputFormat("custom"))
		assert.NotNil(t, formatter)
		assert.Equal(t, "compact", formatter.Name())
	})
}

// ============================================================================
// Universal Format Functions Tests
// ============================================================================

func TestFormatDebateIntroductionForFormat(t *testing.T) {
	t.Parallel()
	members := createTestMembers()

	testCases := []struct {
		format   OutputFormat
		contains string
	}{
		{OutputFormatJSON, `"title"`},
		{OutputFormatYAML, "title:"},
		{OutputFormatHTML, "<div"},
		{OutputFormatXML, "<?xml"},
		{OutputFormatCSV, "Position,Role"},
		{OutputFormatRTF, "{\\rtf1"},
		{OutputFormatTerminal, ANSIBold},
		{OutputFormatCompact, "DEBATE:"},
		{OutputFormatANSI, ANSIBrightCyan},
		{OutputFormatMarkdown, "# HelixAgent"},
		{OutputFormatPlain, "HELIXAGENT AI DEBATE"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.format), func(t *testing.T) {
			t.Parallel()
			result := FormatDebateIntroductionForFormat(tc.format, "Test", members)
			assert.Contains(t, result, tc.contains)
		})
	}
}

func TestFormatPhaseHeaderForAllFormats(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		format   OutputFormat
		contains string
	}{
		{OutputFormatJSON, `"phase"`},
		{OutputFormatYAML, "phase:"},
		{OutputFormatHTML, `class="phase-header"`},
		{OutputFormatXML, "<phase_header>"},
		{OutputFormatCSV, "Phase_Type"},
		{OutputFormatRTF, "{\\rtf1"},
		{OutputFormatTerminal, "==="},
		{OutputFormatCompact, "[P1:"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.format), func(t *testing.T) {
			t.Parallel()
			result := FormatPhaseHeaderForAllFormats(tc.format, services.PhaseInitialResponse, 1)
			assert.Contains(t, result, tc.contains)
		})
	}
}

func TestFormatFinalResponseForAllFormats(t *testing.T) {
	t.Parallel()
	content := "Final answer content"

	testCases := []struct {
		format   OutputFormat
		contains string
	}{
		{OutputFormatJSON, `"type": "final_response"`},
		{OutputFormatYAML, "type: final_response"},
		{OutputFormatHTML, `class="final-response"`},
		{OutputFormatXML, "<final_response>"},
		{OutputFormatCSV, "final_response"},
		{OutputFormatRTF, "Final Answer"},
		{OutputFormatTerminal, "FINAL ANSWER"},
		{OutputFormatCompact, "[FINAL]"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.format), func(t *testing.T) {
			t.Parallel()
			result := FormatFinalResponseForAllFormats(tc.format, content)
			assert.Contains(t, result, tc.contains)
		})
	}
}

func TestFormatFallbackIndicatorForAllFormats(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		format   OutputFormat
		contains string
	}{
		{OutputFormatJSON, `"type": "fallback"`},
		{OutputFormatYAML, "type: fallback"},
		{OutputFormatHTML, `class="fallback-indicator"`},
		{OutputFormatXML, "<fallback>"},
		{OutputFormatCSV, "From_Provider"},
		{OutputFormatRTF, "Fallback"},
		{OutputFormatTerminal, "Fallback"},
		{OutputFormatCompact, "[FB:"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.format), func(t *testing.T) {
			t.Parallel()
			result := FormatFallbackIndicatorForAllFormats(
				tc.format,
				services.RoleAnalyst,
				"openai", "gpt-4",
				"anthropic", "claude-3",
				"rate limit",
				500*time.Millisecond,
			)
			assert.Contains(t, result, tc.contains)
		})
	}
}

// ============================================================================
// Edge Cases and Error Handling Tests
// ============================================================================

func TestFormatters_EmptyInput(t *testing.T) {
	t.Parallel()
	formatters := []OutputFormatter{
		NewJSONFormatter(),
		NewYAMLFormatter(),
		NewHTMLFormatter(),
		NewXMLFormatter(),
		NewCSVFormatter(),
		NewRTFFormatter(),
		NewTerminalFormatter(),
		NewCompactFormatter(),
	}

	for _, f := range formatters {
		t.Run(f.Name()+"_EmptyTopic", func(t *testing.T) {
			t.Parallel()
			result := f.FormatDebateTeamIntroduction("", nil)
			assert.NotEmpty(t, result)
		})

		t.Run(f.Name()+"_EmptyContent", func(t *testing.T) {
			t.Parallel()
			result := f.FormatPhaseContent("")
			assert.NotPanics(t, func() { _ = result })
		})

		t.Run(f.Name()+"_EmptyFinalResponse", func(t *testing.T) {
			t.Parallel()
			result := f.FormatFinalResponse("")
			assert.NotEmpty(t, result)
		})
	}
}

func TestFormatters_NilMembers(t *testing.T) {
	t.Parallel()
	formatters := []OutputFormatter{
		NewJSONFormatter(),
		NewYAMLFormatter(),
		NewHTMLFormatter(),
		NewXMLFormatter(),
		NewCSVFormatter(),
		NewRTFFormatter(),
		NewTerminalFormatter(),
		NewCompactFormatter(),
	}

	for _, f := range formatters {
		t.Run(f.Name()+"_NilMembers", func(t *testing.T) {
			t.Parallel()
			require.NotPanics(t, func() {
				result := f.FormatDebateTeamIntroduction("Test", nil)
				assert.NotEmpty(t, result)
			})
		})

		t.Run(f.Name()+"_MixedNilMembers", func(t *testing.T) {
			t.Parallel()
			members := []*services.DebateTeamMember{
				nil,
				{
					Role:         services.RoleAnalyst,
					ModelName:    "test-model",
					ProviderName: "test-provider",
				},
				nil,
			}
			require.NotPanics(t, func() {
				result := f.FormatDebateTeamIntroduction("Test", members)
				assert.NotEmpty(t, result)
			})
		})
	}
}

func TestFormatters_SpecialCharacters(t *testing.T) {
	t.Parallel()
	specialContent := `Content with special characters: <>&"'` + "`" + `\n\t{}`

	formatters := []OutputFormatter{
		NewJSONFormatter(),
		NewYAMLFormatter(),
		NewHTMLFormatter(),
		NewXMLFormatter(),
		NewCSVFormatter(),
		NewRTFFormatter(),
		NewTerminalFormatter(),
		NewCompactFormatter(),
	}

	for _, f := range formatters {
		t.Run(f.Name()+"_SpecialChars", func(t *testing.T) {
			t.Parallel()
			require.NotPanics(t, func() {
				result := f.FormatPhaseContent(specialContent)
				assert.NotEmpty(t, result)
			})
		})
	}
}

func TestFormatters_LongContent(t *testing.T) {
	t.Parallel()
	longContent := strings.Repeat("x", 10000)

	formatters := []OutputFormatter{
		NewJSONFormatter(),
		NewYAMLFormatter(),
		NewHTMLFormatter(),
		NewXMLFormatter(),
		NewCSVFormatter(),
		NewRTFFormatter(),
		NewTerminalFormatter(),
		NewCompactFormatter(),
	}

	for _, f := range formatters {
		t.Run(f.Name()+"_LongContent", func(t *testing.T) {
			t.Parallel()
			require.NotPanics(t, func() {
				result := f.FormatPhaseContent(longContent)
				assert.NotEmpty(t, result)
			})
		})
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkJSONFormatter_FormatDebateTeamIntroduction(b *testing.B) {
	f := NewJSONFormatter()
	members := createTestMembers()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatDebateTeamIntroduction("Test topic", members)
	}
}

func BenchmarkYAMLFormatter_FormatDebateTeamIntroduction(b *testing.B) {
	f := NewYAMLFormatter()
	members := createTestMembers()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatDebateTeamIntroduction("Test topic", members)
	}
}

func BenchmarkHTMLFormatter_FormatDebateTeamIntroduction(b *testing.B) {
	f := NewHTMLFormatter()
	members := createTestMembers()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatDebateTeamIntroduction("Test topic", members)
	}
}

func BenchmarkXMLFormatter_FormatDebateTeamIntroduction(b *testing.B) {
	f := NewXMLFormatter()
	members := createTestMembers()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatDebateTeamIntroduction("Test topic", members)
	}
}

func BenchmarkCSVFormatter_FormatDebateTeamIntroduction(b *testing.B) {
	f := NewCSVFormatter()
	members := createTestMembers()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatDebateTeamIntroduction("Test topic", members)
	}
}

func BenchmarkRTFFormatter_FormatDebateTeamIntroduction(b *testing.B) {
	f := NewRTFFormatter()
	members := createTestMembers()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatDebateTeamIntroduction("Test topic", members)
	}
}

func BenchmarkTerminalFormatter_FormatDebateTeamIntroduction(b *testing.B) {
	f := NewTerminalFormatter()
	members := createTestMembers()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatDebateTeamIntroduction("Test topic", members)
	}
}

func BenchmarkCompactFormatter_FormatDebateTeamIntroduction(b *testing.B) {
	f := NewCompactFormatter()
	members := createTestMembers()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.FormatDebateTeamIntroduction("Test topic", members)
	}
}

func BenchmarkFormatterRegistry_Get(b *testing.B) {
	registry := NewFormatterRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Get(OutputFormatJSON)
		registry.Get(OutputFormatYAML)
		registry.Get(OutputFormatHTML)
		registry.Get(OutputFormatXML)
	}
}

// ============================================================================
// OutputFormatter Interface Compliance Tests
// ============================================================================

func TestOutputFormatterInterface(t *testing.T) {
	t.Parallel()
	// Ensure all formatters implement the interface
	var _ OutputFormatter = (*JSONFormatter)(nil)
	var _ OutputFormatter = (*YAMLFormatter)(nil)
	var _ OutputFormatter = (*HTMLFormatter)(nil)
	var _ OutputFormatter = (*XMLFormatter)(nil)
	var _ OutputFormatter = (*CSVFormatter)(nil)
	var _ OutputFormatter = (*RTFFormatter)(nil)
	var _ OutputFormatter = (*TerminalFormatter)(nil)
	var _ OutputFormatter = (*CompactFormatter)(nil)
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestGetPhaseIcon(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		phase    services.ValidationPhase
		expected string
	}{
		{services.PhaseInitialResponse, "?"},
		{services.PhaseValidation, "V"},
		{services.PhasePolishImprove, "*"},
		{services.PhaseFinalConclusion, "#"},
		{services.ValidationPhase("unknown"), ">"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.phase), func(t *testing.T) {
			t.Parallel()
			result := getPhaseIcon(tc.phase)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEscapeRTF(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		input    string
		expected string
	}{
		{"plain text", "plain text"},
		{"back\\slash", "back\\\\slash"},
		{"{braces}", "\\{braces\\}"},
		{"mixed\\{\\}", "mixed\\\\\\{\\\\\\}"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			result := escapeRTF(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ============================================================================
// Markdown Formatter Additional Edge Case Tests
// ============================================================================

func TestFormatDebateTeamIntroductionMarkdown_AdditionalEdgeCases(t *testing.T) {
	t.Parallel()
	members := createTestMembers()

	t.Run("Handles all nil members", func(t *testing.T) {
		t.Parallel()
		membersAllNil := []*services.DebateTeamMember{nil, nil, nil}
		result := FormatDebateTeamIntroductionMarkdown("Test", membersAllNil)
		assert.Contains(t, result, "# HelixAgent AI Debate Ensemble")
		// Should not panic
	})

	t.Run("Empty topic", func(t *testing.T) {
		t.Parallel()
		result := FormatDebateTeamIntroductionMarkdown("", members)
		assert.Contains(t, result, "**Topic:**")
	})

	t.Run("Special characters in topic", func(t *testing.T) {
		t.Parallel()
		result := FormatDebateTeamIntroductionMarkdown("Test <script>alert(1)</script>", members)
		assert.Contains(t, result, "Test <script>")
	})
}

func TestFormatFallbackChainMarkdown_AdditionalCases(t *testing.T) {
	t.Parallel()
	t.Run("Single failure chain", func(t *testing.T) {
		chain := []FallbackAttempt{
			{Provider: "openai", Model: "gpt-4", Duration: 500 * time.Millisecond, Success: false, Error: "rate limit"},
		}
		result := FormatFallbackChainMarkdown(services.PositionAnalyst, chain)
		assert.Contains(t, result, "Fallback Chain")
		assert.Contains(t, result, "rate limit")
	})

	t.Run("Chain with no errors", func(t *testing.T) {
		t.Parallel()
		chain := []FallbackAttempt{
			{Provider: "openai", Model: "gpt-4", Duration: 500 * time.Millisecond, Success: true, Error: ""},
		}
		result := FormatFallbackChainMarkdown(services.PositionAnalyst, chain)
		assert.Contains(t, result, "Fallback Chain")
	})
}

// ============================================================================
// Additional Format Functions Tests (Unique to formatters_test.go)
// ============================================================================

func TestFormatOutput_Extended(t *testing.T) {
	t.Parallel()
	t.Run("ANSI format returns as-is", func(t *testing.T) {
		input := "test content"
		result := FormatOutput(OutputFormatANSI, input)
		assert.Equal(t, input, result)
	})

	t.Run("Markdown format strips ANSI", func(t *testing.T) {
		t.Parallel()
		input := ANSIBold + "test" + ANSIReset
		result := FormatOutput(OutputFormatMarkdown, input)
		assert.NotContains(t, result, "\033")
	})

	t.Run("Plain format strips all formatting", func(t *testing.T) {
		t.Parallel()
		input := ANSIBold + "**test**" + ANSIReset
		result := FormatOutput(OutputFormatPlain, input)
		assert.NotContains(t, result, "\033")
		assert.NotContains(t, result, "**")
	})

	t.Run("Unknown format returns as-is", func(t *testing.T) {
		t.Parallel()
		input := "test content"
		result := FormatOutput(OutputFormat("unknown"), input)
		assert.Equal(t, input, result)
	})
}

func TestFormatConsensusHeaderSimpleMarkdown_Extended(t *testing.T) {
	t.Parallel()
	result := FormatConsensusHeaderSimpleMarkdown()
	assert.Contains(t, result, "## Consensus")
	assert.Contains(t, result, "---")
}

func TestFormatConsensusHeaderForFormat_Extended(t *testing.T) {
	t.Parallel()
	t.Run("ANSI format", func(t *testing.T) {
		result := FormatConsensusHeaderForFormat(OutputFormatANSI)
		assert.Contains(t, result, "CONSENSUS")
	})

	t.Run("Markdown format", func(t *testing.T) {
		t.Parallel()
		result := FormatConsensusHeaderForFormat(OutputFormatMarkdown)
		assert.Contains(t, result, "## Consensus")
	})

	t.Run("Plain format", func(t *testing.T) {
		t.Parallel()
		result := FormatConsensusHeaderForFormat(OutputFormatPlain)
		assert.Contains(t, result, "=== CONSENSUS ===")
	})

	t.Run("Unknown format uses Markdown", func(t *testing.T) {
		t.Parallel()
		result := FormatConsensusHeaderForFormat(OutputFormat("unknown"))
		assert.Contains(t, result, "## Consensus")
	})
}

func TestFormatRequestIndicatorForFormat_Extended(t *testing.T) {
	t.Parallel()
	t.Run("ANSI format", func(t *testing.T) {
		result := FormatRequestIndicatorForFormat(OutputFormatANSI, services.PositionAnalyst, services.RoleAnalyst, "anthropic", "claude-3")
		assert.True(t, ContainsANSI(result) || strings.Contains(result, "anthropic"))
	})

	t.Run("Markdown format", func(t *testing.T) {
		t.Parallel()
		result := FormatRequestIndicatorForFormat(OutputFormatMarkdown, services.PositionAnalyst, services.RoleAnalyst, "anthropic", "claude-3")
		assert.Contains(t, result, "**[Analyst]**")
	})

	t.Run("Plain format", func(t *testing.T) {
		t.Parallel()
		result := FormatRequestIndicatorForFormat(OutputFormatPlain, services.PositionAnalyst, services.RoleAnalyst, "anthropic", "claude-3")
		assert.Contains(t, result, "[Analyst]")
		assert.NotContains(t, result, "**")
	})

	t.Run("Unknown format uses Markdown", func(t *testing.T) {
		t.Parallel()
		result := FormatRequestIndicatorForFormat(OutputFormat("unknown"), services.PositionAnalyst, services.RoleAnalyst, "anthropic", "claude-3")
		assert.Contains(t, result, "**[Analyst]**")
	})
}

func TestFormatResponseIndicatorSimpleMarkdown_Extended(t *testing.T) {
	t.Parallel()
	result := FormatResponseIndicatorSimpleMarkdown(services.RoleAnalyst, 500*time.Millisecond)
	assert.Contains(t, result, "**[Analyst]**")
	assert.Contains(t, result, "Response received")
	assert.Contains(t, result, "ms")
}

func TestFormatResponseIndicatorForFormat_Extended(t *testing.T) {
	t.Parallel()
	t.Run("ANSI format", func(t *testing.T) {
		result := FormatResponseIndicatorForFormat(OutputFormatANSI, services.PositionAnalyst, services.RoleAnalyst, 500*time.Millisecond)
		assert.True(t, ContainsANSI(result) || strings.Contains(result, "Analyst"))
	})

	t.Run("Markdown format", func(t *testing.T) {
		t.Parallel()
		result := FormatResponseIndicatorForFormat(OutputFormatMarkdown, services.PositionAnalyst, services.RoleAnalyst, 500*time.Millisecond)
		assert.Contains(t, result, "**[Analyst]**")
	})

	t.Run("Plain format", func(t *testing.T) {
		t.Parallel()
		result := FormatResponseIndicatorForFormat(OutputFormatPlain, services.PositionAnalyst, services.RoleAnalyst, 500*time.Millisecond)
		assert.Contains(t, result, "[Analyst]")
	})

	t.Run("Unknown format uses Markdown", func(t *testing.T) {
		t.Parallel()
		result := FormatResponseIndicatorForFormat(OutputFormat("unknown"), services.PositionAnalyst, services.RoleAnalyst, 500*time.Millisecond)
		assert.Contains(t, result, "**[Analyst]**")
	})
}

func TestFormatFallbackIndicatorSimpleMarkdown_Extended(t *testing.T) {
	t.Parallel()
	result := FormatFallbackIndicatorSimpleMarkdown(services.RoleAnalyst, "anthropic", "claude-3", 500*time.Millisecond)
	assert.Contains(t, result, "**[Analyst]**")
	assert.Contains(t, result, "Fallback to anthropic")
}

func TestFormatPhaseContentForFormat_Extended(t *testing.T) {
	t.Parallel()
	content := "Test content"

	t.Run("ANSI format", func(t *testing.T) {
		t.Parallel()
		result := FormatPhaseContentForFormat(OutputFormatANSI, content)
		assert.True(t, ContainsANSI(result) || strings.Contains(result, content))
	})

	t.Run("Markdown format", func(t *testing.T) {
		t.Parallel()
		result := FormatPhaseContentForFormat(OutputFormatMarkdown, content)
		assert.Contains(t, result, "> ")
	})

	t.Run("Plain format", func(t *testing.T) {
		t.Parallel()
		result := FormatPhaseContentForFormat(OutputFormatPlain, content)
		assert.Equal(t, content, result)
	})

	t.Run("Unknown format uses Markdown", func(t *testing.T) {
		t.Parallel()
		result := FormatPhaseContentForFormat(OutputFormat("unknown"), content)
		assert.Contains(t, result, "> ")
	})
}

func TestFormatFallbackIndicatorForFormat_Extended(t *testing.T) {
	t.Parallel()
	t.Run("ANSI format", func(t *testing.T) {
		result := FormatFallbackIndicatorForFormat(OutputFormatANSI, services.PositionAnalyst, services.RoleAnalyst, "anthropic", "claude-3", 500*time.Millisecond)
		assert.True(t, ContainsANSI(result) || strings.Contains(result, "Fallback"))
	})

	t.Run("Markdown format", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackIndicatorForFormat(OutputFormatMarkdown, services.PositionAnalyst, services.RoleAnalyst, "anthropic", "claude-3", 500*time.Millisecond)
		assert.Contains(t, result, "**[Analyst]**")
	})

	t.Run("Plain format", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackIndicatorForFormat(OutputFormatPlain, services.PositionAnalyst, services.RoleAnalyst, "anthropic", "claude-3", 500*time.Millisecond)
		assert.Contains(t, result, "[Analyst]")
	})

	t.Run("Unknown format uses Markdown", func(t *testing.T) {
		t.Parallel()
		result := FormatFallbackIndicatorForFormat(OutputFormat("unknown"), services.PositionAnalyst, services.RoleAnalyst, "anthropic", "claude-3", 500*time.Millisecond)
		assert.Contains(t, result, "**[Analyst]**")
	})
}

// ============================================================================
// GetPhaseDisplayName Tests
// ============================================================================

func TestGetPhaseDisplayName_Formatters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		phase    services.ValidationPhase
		expected string
	}{
		{services.PhaseInitialResponse, "INITIAL RESPONSE"},
		{services.PhaseValidation, "VALIDATION"},
		{services.PhasePolishImprove, "POLISH & IMPROVE"},
		{services.PhaseFinalConclusion, "FINAL CONCLUSION"},
		{services.ValidationPhase("unknown"), "unknown"},
	}

	for _, tc := range tests {
		t.Run(string(tc.phase), func(t *testing.T) {
			t.Parallel()
			result := getPhaseDisplayName(tc.phase)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ============================================================================
// GetRoleName Tests
// ============================================================================

func TestGetRoleName_Formatters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role     services.DebateRole
		expected string
	}{
		{services.RoleAnalyst, "Analyst"},
		{services.RoleProposer, "Proposer"},
		{services.RoleCritic, "Critic"},
		{services.RoleSynthesis, "Synthesis"},
		{services.RoleMediator, "Mediator"},
		{services.DebateRole("unknown"), "unknown"},
	}

	for _, tc := range tests {
		t.Run(string(tc.role), func(t *testing.T) {
			t.Parallel()
			result := getRoleName(tc.role)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ============================================================================
// GetRoleColor Tests
// ============================================================================

func TestGetRoleColor_Formatters(t *testing.T) {
	t.Parallel()
	tests := []services.DebateRole{
		services.RoleAnalyst,
		services.RoleProposer,
		services.RoleCritic,
		services.RoleSynthesis,
		services.RoleMediator,
	}

	for _, role := range tests {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()
			result := getRoleColor(role)
			assert.True(t, ContainsANSI(result), "Role color should contain ANSI codes")
		})
	}
}

// ============================================================================
// FormatDuration Tests (Extended)
// ============================================================================

func TestFormatDuration_Formatters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{100 * time.Millisecond, "100 ms"},
		{1500 * time.Millisecond, "1.5 s"},
		{2 * time.Second, "2.0 s"},
		{500 * time.Microsecond, "0 ms"},
		{5500 * time.Millisecond, "5.5 s"},
		{10 * time.Minute, "600.0 s"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			result := formatDuration(tc.duration)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ============================================================================
// Fallback Attempt Type Tests
// ============================================================================

func TestFallbackAttempt_Formatters(t *testing.T) {
	t.Parallel()
	attempt := FallbackAttempt{
		Provider: "openai",
		Model:    "gpt-4",
		Duration: 500 * time.Millisecond,
		Success:  true,
		Error:    "",
	}

	assert.Equal(t, "openai", attempt.Provider)
	assert.Equal(t, "gpt-4", attempt.Model)
	assert.Equal(t, 500*time.Millisecond, attempt.Duration)
	assert.True(t, attempt.Success)
	assert.Empty(t, attempt.Error)
}
