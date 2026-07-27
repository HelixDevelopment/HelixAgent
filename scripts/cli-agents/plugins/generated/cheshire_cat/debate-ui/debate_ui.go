// AI Debate UI Plugin
// Provides visualization for AI Debate responses

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var PluginName = "debate-ui"
var PluginVersion = "1.0.0"

// DebatePhase represents a phase in the debate
type DebatePhase struct {
	Name       string  `json:"name"`
	Icon       string  `json:"icon"`
	Status     string  `json:"status"`
	Confidence float64 `json:"confidence"`
}

// DebateResponse represents a formatted debate response
type DebateResponse struct {
	Topic             string        `json:"topic"`
	Phases            []DebatePhase `json:"phases"`
	CurrentPhase      int           `json:"current_phase"`
	FinalResponse     string        `json:"final_response"`
	OverallConfidence float64       `json:"overall_confidence"`
	Participants      []string      `json:"participants"`
}

// FormatDebateProgress formats the debate progress for display
func FormatDebateProgress(response *DebateResponse) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("╔══════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    AI DEBATE IN PROGRESS                      ║\n")
	sb.WriteString("╠══════════════════════════════════════════════════════════════╣\n")

	for i, phase := range response.Phases {
		status := "⏳"
		if i < response.CurrentPhase {
			status = "✅"
		} else if i == response.CurrentPhase {
			status = "🔄"
		}

		line := fmt.Sprintf("║ %s %s %-40s %s ║\n",
			status, phase.Icon, phase.Name,
			formatConfidence(phase.Confidence))
		sb.WriteString(line)
	}

	sb.WriteString("╠══════════════════════════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║ Participants: %-46s ║\n",
		strings.Join(response.Participants[:min(3, len(response.Participants))], ", ")))
	sb.WriteString(fmt.Sprintf("║ Overall Confidence: %-40s ║\n",
		formatConfidence(response.OverallConfidence)))
	sb.WriteString("╚══════════════════════════════════════════════════════════════╝\n")

	return sb.String()
}

func formatConfidence(conf float64) string {
	bars := int(conf * 10)
	return fmt.Sprintf("[%s%s] %.0f%%",
		strings.Repeat("█", bars),
		strings.Repeat("░", 10-bars),
		conf*100)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DefaultPhases returns the default debate phases
func DefaultPhases() []DebatePhase {
	return []DebatePhase{
		{Name: "Initial Response", Icon: "🔍", Status: "pending", Confidence: 0},
		{Name: "Validation", Icon: "✓", Status: "pending", Confidence: 0},
		{Name: "Polish & Improve", Icon: "✨", Status: "pending", Confidence: 0},
		{Name: "Final Conclusion", Icon: "📜", Status: "pending", Confidence: 0},
	}
}

func Init() error {
	fmt.Println("[debate-ui] Plugin initialized")
	return nil
}

func Shutdown() error {
	fmt.Println("[debate-ui] Plugin shutdown")
	return nil
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			fmt.Printf("%s v%s\n", PluginName, PluginVersion)
			return
		case "--demo":
			demo := &DebateResponse{
				Topic:             "AI Ethics",
				Phases:            DefaultPhases(),
				CurrentPhase:      2,
				Participants:      []string{"Claude", "Gemini", "DeepSeek"},
				OverallConfidence: 0.87,
			}
			demo.Phases[0].Status = "complete"
			demo.Phases[0].Confidence = 0.85
			demo.Phases[1].Status = "complete"
			demo.Phases[1].Confidence = 0.90
			fmt.Println(FormatDebateProgress(demo))
			return
		}
	}

	// Process stdin JSON
	var response DebateResponse
	if err := json.NewDecoder(os.Stdin).Decode(&response); err != nil {
		fmt.Println("Usage: debate-ui [--version|--demo] or pipe JSON")
		return
	}
	fmt.Println(FormatDebateProgress(&response))
}
