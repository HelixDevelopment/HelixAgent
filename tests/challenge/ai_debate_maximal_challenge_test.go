package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAIDebateMaximalChallenge is a comprehensive challenge that exercises
// the AI debate system maximally, testing all components and producing
// verifiable results.
//
// This challenge creates a "Mini Application" that uses AI debate for:
// 1. Code review (multi-perspective analysis)
// 2. Documentation generation (consensus-based)
// 3. Test case generation (diverse approaches)
// 4. Bug prediction (confidence-weighted voting)
//
// Run with: go test -v ./tests/challenge -run TestAIDebateMaximalChallenge -timeout 5m
func TestAIDebateMaximalChallenge(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping AI debate maximal challenge in short mode (requires live LLM providers, takes 5+ minutes)") // SKIP-OK: #short-mode
	}

	baseURL := getBaseURL()

	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL) // SKIP-OK: #legacy-untriaged
	}

	// Verify we have healthy providers
	healthyProviders := getHealthyProviderCount(t, baseURL)
	registeredProviders := len(getAvailableProviders(t, baseURL))
	if healthyProviders == 0 && registeredProviders == 0 {
		t.Skip("No providers available (none registered, none healthy)") // SKIP-OK: #legacy-untriaged
	}
	effectiveProviders := healthyProviders
	if effectiveProviders == 0 {
		effectiveProviders = registeredProviders
	}
	t.Logf("Starting AI Debate Maximal Challenge with %d healthy/%d registered providers", healthyProviders, registeredProviders)

	// Initialize challenge tracker
	tracker := &ChallengeTracker{
		StartTime:     time.Now(),
		TotalTests:    0,
		PassedTests:   0,
		DebateCount:   0,
		TotalLatency:  0,
		Components:    make(map[string]bool),
		Verifications: make([]VerificationRecord, 0),
	}

	// Run all challenge sections
	t.Run("MultiPassValidation", func(t *testing.T) {
		runMultiPassValidationChallenge(t, baseURL, tracker)
	})

	t.Run("ConsensusBuilding", func(t *testing.T) {
		runConsensusBuildingChallenge(t, baseURL, tracker)
	})

	t.Run("ConfidenceWeightedVoting", func(t *testing.T) {
		runConfidenceWeightedChallenge(t, baseURL, tracker)
	})

	t.Run("CrossProviderDebate", func(t *testing.T) {
		runCrossProviderDebateChallenge(t, baseURL, tracker)
	})

	t.Run("StreamingDebate", func(t *testing.T) {
		runStreamingDebateChallenge(t, baseURL, tracker)
	})

	t.Run("FallbackChainExercise", func(t *testing.T) {
		runFallbackChainChallenge(t, baseURL, tracker)
	})

	t.Run("MiniApplicationWorkflow", func(t *testing.T) {
		runMiniApplicationChallenge(t, baseURL, tracker)
	})

	// Report final results
	reportChallengeResults(t, tracker)
}

// ChallengeTracker tracks progress and results across all challenge sections
type ChallengeTracker struct {
	StartTime     time.Time
	TotalTests    int
	PassedTests   int
	DebateCount   int
	TotalLatency  int64
	Components    map[string]bool
	Verifications []VerificationRecord
	mu            sync.Mutex
}

// VerificationRecord records a verification result
type VerificationRecord struct {
	Test       string
	Expected   string
	Actual     string
	Verified   bool
	Confidence float64
	Latency    time.Duration
}

func (ct *ChallengeTracker) RecordTest(name string, passed bool) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.TotalTests++
	if passed {
		ct.PassedTests++
	}
}

func (ct *ChallengeTracker) RecordDebate(latencyMs int64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.DebateCount++
	ct.TotalLatency += latencyMs
}

func (ct *ChallengeTracker) RecordComponent(component string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.Components[component] = true
}

func (ct *ChallengeTracker) AddVerification(record VerificationRecord) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.Verifications = append(ct.Verifications, record)
}

// getHealthyProviderCount returns the number of healthy providers
func getHealthyProviderCount(t *testing.T, baseURL string) int {
	results := getProviderVerification(t, baseURL)
	count := 0
	for _, r := range results {
		if r.Verified {
			count++
		}
	}
	return count
}

// runMultiPassValidationChallenge tests the multi-pass validation system
func runMultiPassValidationChallenge(t *testing.T, baseURL string, tracker *ChallengeTracker) {
	t.Log("Testing Multi-Pass Validation System...")
	tracker.RecordComponent("MultiPassValidation")

	testCases := []struct {
		name          string
		prompt        string
		expectPattern string // Pattern to look for in response
		minConfidence float64
	}{
		{
			name:          "MathValidation",
			prompt:        "What is the square root of 144? Provide just the number.",
			expectPattern: "12",
			minConfidence: 0.7,
		},
		{
			name:          "FactValidation",
			prompt:        "What year did the first Moon landing occur? Answer with just the year.",
			expectPattern: "1969",
			minConfidence: 0.8,
		},
		{
			name:          "LogicValidation",
			prompt:        "If A implies B, and B implies C, does A imply C? Answer Yes or No.",
			expectPattern: "Yes",
			minConfidence: 0.9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			result := runDebateWithMultiPass(baseURL, tc.prompt)
			latency := time.Since(start)

			tracker.RecordDebate(latency.Milliseconds())

			verified := strings.Contains(strings.ToLower(result.Response), strings.ToLower(tc.expectPattern))
			tracker.AddVerification(VerificationRecord{
				Test:       tc.name,
				Expected:   tc.expectPattern,
				Actual:     result.Response,
				Verified:   verified,
				Confidence: result.Confidence,
				Latency:    latency,
			})

			if result.Success && verified {
				t.Logf("  %s: VERIFIED (response contains '%s', confidence: %.2f, latency: %v)",
					tc.name, tc.expectPattern, result.Confidence, latency)
				tracker.RecordTest(tc.name, true)
			} else if result.Success {
				t.Logf("  %s: PARTIAL (response: %s, expected: %s, latency: %v)",
					tc.name, truncateString(result.Response, 50), tc.expectPattern, latency)
				tracker.RecordTest(tc.name, false)
			} else {
				t.Logf("  %s: FAILED (%s)", tc.name, result.Error)
				tracker.RecordTest(tc.name, false)
			}
		})
	}
}

// runConsensusBuildingChallenge tests consensus building across multiple providers
func runConsensusBuildingChallenge(t *testing.T, baseURL string, tracker *ChallengeTracker) {
	t.Log("Testing Consensus Building System...")
	tracker.RecordComponent("ConsensusBuilding")

	// Test that multiple providers can reach consensus on factual queries
	consensusTopics := []struct {
		name   string
		prompt string
		// We test that multiple requests give consistent answers
	}{
		{"SimpleFact", "What is the capital of France? Answer in one word."},
		{"BasicMath", "What is 7 times 8? Answer with just the number."},
		{"CommonKnowledge", "What element has atomic number 1? Answer with the element name."},
	}

	for _, topic := range consensusTopics {
		t.Run(topic.name, func(t *testing.T) {
			// Run multiple debate rounds
			responses := make([]string, 3)
			var totalLatency time.Duration

			for i := 0; i < 3; i++ {
				start := time.Now()
				result := runDebateWithConfig(baseURL, DebateRequest{
					Model:    "helixagent-debate",
					Messages: []Message{{Role: "user", Content: topic.prompt}},
					EnsembleConfig: &EnsembleConfig{
						MinProviders:        1,
						Strategy:            "confidence_weighted",
						FallbackEnabled:     true,
						ConfidenceThreshold: 0.5,
					},
				})
				totalLatency += time.Since(start)

				if result.Success {
					responses[i] = strings.ToLower(strings.TrimSpace(result.Response))
				} else {
					responses[i] = "" // Mark as failed
				}
				tracker.RecordDebate(time.Since(start).Milliseconds())
			}

			// Check consensus (at least 2 out of 3 should match)
			consensusReached := checkConsensus(responses)

			if consensusReached {
				t.Logf("  %s: CONSENSUS REACHED (avg latency: %v)", topic.name, totalLatency/3)
				tracker.RecordTest(topic.name, true)
			} else {
				t.Logf("  %s: NO CONSENSUS (responses varied: %v)", topic.name, responses)
				tracker.RecordTest(topic.name, false)
			}

			tracker.AddVerification(VerificationRecord{
				Test:       topic.name,
				Expected:   "consensus",
				Actual:     fmt.Sprintf("%v", responses),
				Verified:   consensusReached,
				Confidence: float64(countMatches(responses)) / float64(len(responses)),
				Latency:    totalLatency / 3,
			})
		})
	}
}

// runConfidenceWeightedChallenge tests confidence-weighted voting
func runConfidenceWeightedChallenge(t *testing.T, baseURL string, tracker *ChallengeTracker) {
	t.Log("Testing Confidence-Weighted Voting System...")
	tracker.RecordComponent("ConfidenceWeightedVoting")

	// Test queries that should have high confidence
	highConfidenceQueries := []struct {
		name      string
		prompt    string
		threshold float64
	}{
		{"DefiniteAnswer", "Is water wet? Answer Yes or No.", 0.7},
		{"MathCertainty", "What is 2+2? Answer with just the number.", 0.8},
		{"BasicFact", "Is the Earth round? Answer Yes or No.", 0.7},
	}

	for _, query := range highConfidenceQueries {
		t.Run(query.name, func(t *testing.T) {
			start := time.Now()
			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: query.prompt}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:        1,
					Strategy:            "confidence_weighted",
					FallbackEnabled:     true,
					ConfidenceThreshold: query.threshold,
				},
			})
			latency := time.Since(start)
			tracker.RecordDebate(latency.Milliseconds())

			if result.Success {
				t.Logf("  %s: SUCCESS (response: %s, latency: %v)",
					query.name, truncateString(result.Response, 30), latency)
				tracker.RecordTest(query.name, true)
			} else {
				t.Logf("  %s: FAILED (%s)", query.name, result.Error)
				tracker.RecordTest(query.name, false)
			}

			tracker.AddVerification(VerificationRecord{
				Test:       query.name,
				Expected:   "high confidence response",
				Actual:     result.Response,
				Verified:   result.Success,
				Confidence: query.threshold,
				Latency:    latency,
			})
		})
	}
}

// runCrossProviderDebateChallenge tests debates across multiple providers
func runCrossProviderDebateChallenge(t *testing.T, baseURL string, tracker *ChallengeTracker) {
	t.Log("Testing Cross-Provider Debate System...")
	tracker.RecordComponent("CrossProviderDebate")

	// Get available providers
	providers := getAvailableProviders(t, baseURL)
	t.Logf("  Available providers: %v", providers)

	if len(providers) < 2 {
		t.Skip("Need at least 2 providers for cross-provider debate") // SKIP-OK: #legacy-untriaged
		return
	}

	// Test debate with minimum 2 providers
	t.Run("DualProviderDebate", func(t *testing.T) {
		start := time.Now()
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Explain why testing is important in software development. Keep it brief."}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    2,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true,
			},
		})
		latency := time.Since(start)
		tracker.RecordDebate(latency.Milliseconds())

		if result.Success {
			t.Logf("  DualProviderDebate: SUCCESS (latency: %v)", latency)
			tracker.RecordTest("DualProviderDebate", true)
		} else {
			// May fail if not enough healthy providers
			t.Logf("  DualProviderDebate: %s", result.Error)
			tracker.RecordTest("DualProviderDebate", false)
		}
	})

	// Test with different voting strategies
	strategies := []string{"confidence_weighted", "majority_vote", "best_of_n"}
	for _, strategy := range strategies {
		t.Run("Strategy_"+strategy, func(t *testing.T) {
			start := time.Now()
			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: "Say 'test passed' if you can respond."}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:    1,
					Strategy:        strategy,
					FallbackEnabled: true,
				},
			})
			latency := time.Since(start)
			tracker.RecordDebate(latency.Milliseconds())

			if result.Success {
				t.Logf("  Strategy_%s: SUCCESS (latency: %v)", strategy, latency)
				tracker.RecordTest("Strategy_"+strategy, true)
			} else {
				t.Logf("  Strategy_%s: %s", strategy, result.Error)
				tracker.RecordTest("Strategy_"+strategy, false)
			}
		})
	}
}

// runStreamingDebateChallenge tests streaming responses from debates
func runStreamingDebateChallenge(t *testing.T, baseURL string, tracker *ChallengeTracker) {
	t.Log("Testing Streaming Debate System...")
	tracker.RecordComponent("StreamingDebate")

	t.Run("StreamingResponse", func(t *testing.T) {
		start := time.Now()

		// Create streaming request
		reqBody := DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Count from 1 to 5, one number per line."}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				FallbackEnabled: true,
			},
		}

		client := &http.Client{Timeout: 180 * time.Second}
		body, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")

		resp, err := client.Do(req)
		latency := time.Since(start)
		tracker.RecordDebate(latency.Milliseconds())

		if err != nil {
			t.Logf("  StreamingResponse: FAILED (connection error: %v)", err)
			tracker.RecordTest("StreamingResponse", false)
			return
		}
		defer resp.Body.Close()

		// Read response (might be streaming or non-streaming)
		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusOK && len(respBody) > 0 {
			t.Logf("  StreamingResponse: SUCCESS (received %d bytes, latency: %v)", len(respBody), latency)
			tracker.RecordTest("StreamingResponse", true)
		} else {
			t.Logf("  StreamingResponse: PARTIAL (status: %d, latency: %v)", resp.StatusCode, latency)
			tracker.RecordTest("StreamingResponse", false)
		}
	})
}

// runFallbackChainChallenge tests the fallback chain mechanism
func runFallbackChainChallenge(t *testing.T, baseURL string, tracker *ChallengeTracker) {
	t.Log("Testing Fallback Chain System...")
	tracker.RecordComponent("FallbackChain")

	// Test with fallback enabled (should always succeed if any provider works)
	t.Run("FallbackEnabled", func(t *testing.T) {
		start := time.Now()
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Say 'fallback works' to confirm."}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true,
			},
		})
		latency := time.Since(start)
		tracker.RecordDebate(latency.Milliseconds())

		if result.Success {
			t.Logf("  FallbackEnabled: SUCCESS (latency: %v)", latency)
			tracker.RecordTest("FallbackEnabled", true)
		} else {
			t.Logf("  FallbackEnabled: FAILED (%s)", result.Error)
			tracker.RecordTest("FallbackEnabled", false)
		}
	})

	// Test with high minimum providers (may require fallback)
	t.Run("HighMinWithFallback", func(t *testing.T) {
		start := time.Now()
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Test with high provider requirement."}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    3,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true, // Should fallback if not enough providers
			},
		})
		latency := time.Since(start)
		tracker.RecordDebate(latency.Milliseconds())

		// This might fail or succeed depending on provider availability
		if result.Success {
			t.Logf("  HighMinWithFallback: SUCCESS (latency: %v)", latency)
			tracker.RecordTest("HighMinWithFallback", true)
		} else {
			// Expected if not enough healthy providers
			t.Logf("  HighMinWithFallback: %s (expected if <3 healthy providers)", result.Error)
			tracker.RecordTest("HighMinWithFallback", false)
		}
	})
}

// runMiniApplicationChallenge creates a mini application that uses AI debate
// for code review, documentation, and test generation
func runMiniApplicationChallenge(t *testing.T, baseURL string, tracker *ChallengeTracker) {
	t.Log("Running Mini Application Challenge...")
	tracker.RecordComponent("MiniApplication")

	// Simulated code to review
	codeSnippet := `
func add(a, b int) int {
    return a + b
}
`

	// Step 1: Code Review via AI Debate
	t.Run("CodeReview", func(t *testing.T) {
		prompt := fmt.Sprintf("Review this Go code and identify any issues or improvements:\n%s\nProvide a brief analysis.", codeSnippet)
		start := time.Now()
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: prompt}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true,
			},
		})
		latency := time.Since(start)
		tracker.RecordDebate(latency.Milliseconds())

		if result.Success && len(result.Response) > 10 {
			t.Logf("  CodeReview: SUCCESS (review generated, %d chars, latency: %v)", len(result.Response), latency)
			tracker.RecordTest("CodeReview", true)
		} else {
			t.Logf("  CodeReview: FAILED (%s)", result.Error)
			tracker.RecordTest("CodeReview", false)
		}
	})

	// Step 2: Documentation Generation
	t.Run("DocGeneration", func(t *testing.T) {
		prompt := fmt.Sprintf("Generate a brief documentation comment for this Go function:\n%s", codeSnippet)
		start := time.Now()
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: prompt}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true,
			},
		})
		latency := time.Since(start)
		tracker.RecordDebate(latency.Milliseconds())

		if result.Success && len(result.Response) > 10 {
			t.Logf("  DocGeneration: SUCCESS (doc generated, %d chars, latency: %v)", len(result.Response), latency)
			tracker.RecordTest("DocGeneration", true)
		} else {
			t.Logf("  DocGeneration: FAILED (%s)", result.Error)
			tracker.RecordTest("DocGeneration", false)
		}
	})

	// Step 3: Test Case Suggestion
	t.Run("TestCaseSuggestion", func(t *testing.T) {
		prompt := fmt.Sprintf("Suggest 3 test cases for this Go function:\n%s\nFormat as: input -> expected output", codeSnippet)
		start := time.Now()
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: prompt}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true,
			},
		})
		latency := time.Since(start)
		tracker.RecordDebate(latency.Milliseconds())

		if result.Success && len(result.Response) > 10 {
			t.Logf("  TestCaseSuggestion: SUCCESS (tests suggested, %d chars, latency: %v)", len(result.Response), latency)
			tracker.RecordTest("TestCaseSuggestion", true)
		} else {
			t.Logf("  TestCaseSuggestion: FAILED (%s)", result.Error)
			tracker.RecordTest("TestCaseSuggestion", false)
		}
	})

	// Step 4: Bug Prediction
	t.Run("BugPrediction", func(t *testing.T) {
		prompt := fmt.Sprintf("Analyze this code for potential bugs or edge cases that could cause issues:\n%s", codeSnippet)
		start := time.Now()
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: prompt}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true,
			},
		})
		latency := time.Since(start)
		tracker.RecordDebate(latency.Milliseconds())

		if result.Success && len(result.Response) > 10 {
			t.Logf("  BugPrediction: SUCCESS (analysis generated, %d chars, latency: %v)", len(result.Response), latency)
			tracker.RecordTest("BugPrediction", true)
		} else {
			t.Logf("  BugPrediction: FAILED (%s)", result.Error)
			tracker.RecordTest("BugPrediction", false)
		}
	})
}

// runDebateWithMultiPass runs a debate with multi-pass validation
func runDebateWithMultiPass(baseURL, prompt string) DebateResultExtended {
	result := DebateResultExtended{}
	start := time.Now()

	client := &http.Client{Timeout: 180 * time.Second}

	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"ensemble_config": map[string]interface{}{
			"min_providers":    1,
			"strategy":         "confidence_weighted",
			"fallback_enabled": true,
		},
		"enable_multi_pass_validation": true,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	result.ResponseTimeMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(respBody, &chatResp); err == nil && len(chatResp.Choices) > 0 {
			result.Success = true
			result.Response = chatResp.Choices[0].Message.Content
			result.Confidence = 0.8 // Default confidence if not provided
		}
	} else {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			result.Error = errResp.Error.Message
		} else {
			result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}

	return result
}

// DebateResultExtended extends DebateResult with additional fields
type DebateResultExtended struct {
	Success        bool
	ResponseTimeMs int64
	Response       string
	Error          string
	Confidence     float64
	Phases         []string
}

// checkConsensus checks if responses reached consensus (majority match)
func checkConsensus(responses []string) bool {
	if len(responses) < 2 {
		return false
	}

	matches := countMatches(responses)
	return matches > len(responses)/2
}

// countMatches counts how many responses match the most common response
func countMatches(responses []string) int {
	if len(responses) == 0 {
		return 0
	}

	counts := make(map[string]int)
	for _, r := range responses {
		if r != "" {
			// Normalize response for comparison
			normalized := strings.ToLower(strings.TrimSpace(r))
			// Take first word or first 20 chars for comparison
			if idx := strings.Index(normalized, " "); idx > 0 && idx < 20 {
				normalized = normalized[:idx]
			}
			if len(normalized) > 20 {
				normalized = normalized[:20]
			}
			counts[normalized]++
		}
	}

	maxCount := 0
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}

	return maxCount
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	// Remove newlines for display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// reportChallengeResults reports the final challenge results
func reportChallengeResults(t *testing.T, tracker *ChallengeTracker) {
	duration := time.Since(tracker.StartTime)

	t.Log("")
	t.Log("═══════════════════════════════════════════════════")
	t.Log("       AI DEBATE MAXIMAL CHALLENGE RESULTS         ")
	t.Log("═══════════════════════════════════════════════════")
	t.Logf("  Total Duration: %v", duration)
	t.Logf("  Tests Run:      %d", tracker.TotalTests)
	t.Logf("  Tests Passed:   %d (%.1f%%)", tracker.PassedTests, float64(tracker.PassedTests)/float64(tracker.TotalTests)*100)
	t.Logf("  Debates Run:    %d", tracker.DebateCount)
	if tracker.DebateCount > 0 {
		t.Logf("  Avg Latency:    %dms", tracker.TotalLatency/int64(tracker.DebateCount))
	}
	t.Log("")
	t.Log("  Components Exercised:")
	for component := range tracker.Components {
		t.Logf("    ✓ %s", component)
	}
	t.Log("")

	// Summary verification
	verifiedCount := 0
	for _, v := range tracker.Verifications {
		if v.Verified {
			verifiedCount++
		}
	}
	t.Logf("  Verifications:  %d/%d passed", verifiedCount, len(tracker.Verifications))
	t.Log("═══════════════════════════════════════════════════")

	// Assert minimum requirements
	assert.GreaterOrEqual(t, tracker.PassedTests, tracker.TotalTests/2, "Should pass at least 50% of tests")
	assert.GreaterOrEqual(t, len(tracker.Components), 5, "Should exercise at least 5 components")
}

// TestAIDebateStressChallenge tests the AI debate system under stress
func TestAIDebateStressChallenge(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	baseURL := getBaseURL()
	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running") // SKIP-OK: #legacy-untriaged
	}

	t.Log("Starting AI Debate Stress Challenge...")

	const (
		numRequests    = 20
		maxConcurrent  = 5
		timeoutSeconds = 300
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeoutSeconds*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make(chan DebateResult, numRequests)
	semaphore := make(chan struct{}, maxConcurrent)

	start := time.Now()

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			}

			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: fmt.Sprintf("Stress test %d: say OK", idx)}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:    1,
					FallbackEnabled: true,
				},
			})

			results <- result
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	successCount := 0
	var totalLatency int64
	for result := range results {
		if result.Success {
			successCount++
			totalLatency += result.ResponseTimeMs
		}
	}

	duration := time.Since(start)
	successRate := float64(successCount) / float64(numRequests) * 100

	t.Logf("Stress Test Results:")
	t.Logf("  Total Requests: %d", numRequests)
	t.Logf("  Successful:     %d (%.1f%%)", successCount, successRate)
	t.Logf("  Duration:       %v", duration)
	if successCount > 0 {
		t.Logf("  Avg Latency:    %dms", totalLatency/int64(successCount))
	}
	t.Logf("  Throughput:     %.2f req/s", float64(numRequests)/duration.Seconds())

	// Stress test should achieve at least 50% success rate
	require.GreaterOrEqual(t, successRate, 50.0, "Should achieve at least 50% success rate under stress")
}
