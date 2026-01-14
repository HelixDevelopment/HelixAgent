package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

// Package-level metrics (registered once)
var (
	fcvMetricsOnce            sync.Once
	fcvValidationResultGauge  prometheus.Gauge
	fcvDiversityScoreGauge    prometheus.Gauge
	fcvValidationAlertsTotal  prometheus.Counter
)

func initFCVMetrics() {
	fcvMetricsOnce.Do(func() {
		fcvValidationResultGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "helixagent_fallback_chain_valid",
				Help: "Whether fallback chain validation passed (1=valid, 0=invalid)",
			},
		)

		fcvDiversityScoreGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "helixagent_fallback_chain_diversity_score",
				Help: "Diversity score of fallback chain (0-100)",
			},
		)

		fcvValidationAlertsTotal = promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "helixagent_fallback_chain_alerts_total",
				Help: "Total number of fallback chain validation alerts",
			},
		)
	})
}

// FallbackChainValidator validates that fallback chains have proper diversity
type FallbackChainValidator struct {
	mu               sync.RWMutex
	logger           *logrus.Logger
	debateTeamConfig *DebateTeamConfig
	listeners        []FallbackChainAlertListener

	// Validation results cache
	lastValidation  *FallbackChainValidationResult
	lastValidatedAt time.Time
}

// FallbackChainAlertListener is called when validation alerts occur
type FallbackChainAlertListener func(alert FallbackChainAlert)

// FallbackChainAlert represents a validation alert
type FallbackChainAlert struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Position  int       `json:"position,omitempty"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"` // warning, critical
}

// FallbackChainValidationResult represents the validation result
type FallbackChainValidationResult struct {
	Valid             bool                        `json:"valid"`
	DiversityScore    float64                     `json:"diversity_score"`
	Issues            []FallbackChainIssue        `json:"issues,omitempty"`
	Positions         []FallbackChainPositionInfo `json:"positions"`
	UniqueProviders   int                         `json:"unique_providers"`
	HasOAuthPrimaries bool                        `json:"has_oauth_primaries"`
	HasAPIFallbacks   bool                        `json:"has_api_fallbacks"`
	ValidatedAt       time.Time                   `json:"validated_at"`
}

// FallbackChainIssue represents a validation issue
type FallbackChainIssue struct {
	Severity    string `json:"severity"`
	Position    int    `json:"position,omitempty"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// FallbackChainPositionInfo represents info about a debate position
type FallbackChainPositionInfo struct {
	Position        int      `json:"position"`
	PositionName    string   `json:"position_name"`
	PrimaryProvider string   `json:"primary_provider"`
	PrimaryModel    string   `json:"primary_model"`
	PrimaryIsOAuth  bool     `json:"primary_is_oauth"`
	Fallbacks       []string `json:"fallbacks"`
	HasDiversity    bool     `json:"has_diversity"`
}

// NewFallbackChainValidator creates a new validator
func NewFallbackChainValidator(logger *logrus.Logger, debateTeamConfig *DebateTeamConfig) *FallbackChainValidator {
	// Initialize package-level metrics (idempotent)
	initFCVMetrics()

	return &FallbackChainValidator{
		logger:           logger,
		debateTeamConfig: debateTeamConfig,
		listeners:        make([]FallbackChainAlertListener, 0),
	}
}

// AddAlertListener adds a listener for alerts
func (fcv *FallbackChainValidator) AddAlertListener(listener FallbackChainAlertListener) {
	fcv.mu.Lock()
	defer fcv.mu.Unlock()
	fcv.listeners = append(fcv.listeners, listener)
}

// Validate performs the fallback chain validation
func (fcv *FallbackChainValidator) Validate() *FallbackChainValidationResult {
	fcv.mu.Lock()
	defer fcv.mu.Unlock()

	result := &FallbackChainValidationResult{
		Valid:       true,
		Issues:      make([]FallbackChainIssue, 0),
		Positions:   make([]FallbackChainPositionInfo, 0),
		ValidatedAt: time.Now(),
	}

	if fcv.debateTeamConfig == nil {
		result.Valid = false
		result.Issues = append(result.Issues, FallbackChainIssue{
			Severity:    "critical",
			Description: "DebateTeamConfig is not initialized",
			Suggestion:  "Ensure DebateTeamConfig is properly initialized at startup",
		})
		fcv.lastValidation = result
		fcv.lastValidatedAt = time.Now()
		fcvValidationResultGauge.Set(0)
		return result
	}

	// Get verified LLMs from the debate team config
	verifiedLLMs := fcv.debateTeamConfig.GetVerifiedLLMs()
	if len(verifiedLLMs) == 0 {
		result.Valid = false
		result.Issues = append(result.Issues, FallbackChainIssue{
			Severity:    "critical",
			Description: "No verified LLMs available",
			Suggestion:  "Check API keys and provider availability",
		})
		fcv.lastValidation = result
		fcv.lastValidatedAt = time.Now()
		fcvValidationResultGauge.Set(0)
		return result
	}

	// Track unique providers and provider types
	uniqueProviders := make(map[string]bool)
	oauthProviders := make(map[string]bool)
	apiKeyProviders := make(map[string]bool)
	freeProviders := make(map[string]bool)

	for _, llm := range verifiedLLMs {
		uniqueProviders[llm.ProviderName] = true
		if llm.IsOAuth {
			oauthProviders[llm.ProviderName] = true
		} else if isFreeProvider(llm.ProviderName) {
			freeProviders[llm.ProviderName] = true
		} else {
			apiKeyProviders[llm.ProviderName] = true
		}
	}

	result.UniqueProviders = len(uniqueProviders)
	result.HasOAuthPrimaries = len(oauthProviders) > 0
	result.HasAPIFallbacks = len(apiKeyProviders) > 0

	// Validate each position using team members
	positionNames := []string{"Analyst", "Proposer", "Critic", "Synthesis", "Mediator"}
	positions := []DebateTeamPosition{PositionAnalyst, PositionProposer, PositionCritic, PositionSynthesis, PositionMediator}

	for i, pos := range positions {
		posInfo := FallbackChainPositionInfo{
			Position:     i + 1,
			PositionName: positionNames[i],
			Fallbacks:    make([]string, 0),
		}

		// Get team member for this position
		member := fcv.debateTeamConfig.GetTeamMember(pos)
		if member != nil {
			posInfo.PrimaryProvider = member.ProviderName
			posInfo.PrimaryModel = member.ModelName
			posInfo.PrimaryIsOAuth = member.IsOAuth

			// Get fallbacks (linked list iteration)
			fb := member.Fallback
			for fb != nil {
				posInfo.Fallbacks = append(posInfo.Fallbacks, fmt.Sprintf("%s/%s", fb.ProviderName, fb.ModelName))
				fb = fb.Fallback
			}

			// Check diversity for this position
			posInfo.HasDiversity = fcv.checkMemberDiversity(member)
			if !posInfo.HasDiversity && member.IsOAuth {
				result.Issues = append(result.Issues, FallbackChainIssue{
					Severity:    "warning",
					Position:    i + 1,
					Description: fmt.Sprintf("Position %d (%s) OAuth primary lacks non-OAuth fallbacks", i+1, positionNames[i]),
					Suggestion:  "Ensure OAuth primaries have non-OAuth fallbacks",
				})
			}

			// Critical: OAuth primary must have non-OAuth fallbacks
			if member.IsOAuth {
				hasNonOAuthFallback := false
				fb := member.Fallback
				for fb != nil {
					if !fb.IsOAuth && !isFreeProvider(fb.ProviderName) {
						hasNonOAuthFallback = true
						break
					}
					fb = fb.Fallback
				}
				if !hasNonOAuthFallback && member.Fallback != nil {
					result.Issues = append(result.Issues, FallbackChainIssue{
						Severity:    "critical",
						Position:    i + 1,
						Description: fmt.Sprintf("Position %d (%s) OAuth primary has NO non-OAuth fallbacks", i+1, positionNames[i]),
						Suggestion:  "Add API key providers (Cerebras, Mistral) as fallbacks",
					})
					result.Valid = false
				}
			}
		} else {
			result.Issues = append(result.Issues, FallbackChainIssue{
				Severity:    "critical",
				Position:    i + 1,
				Description: fmt.Sprintf("Position %d (%s) has no primary provider", i+1, positionNames[i]),
				Suggestion:  "Ensure at least one verified provider is available",
			})
			result.Valid = false
		}

		result.Positions = append(result.Positions, posInfo)
	}

	// Calculate diversity score (0-100)
	result.DiversityScore = fcv.calculateDiversityScore(result)
	fcvDiversityScoreGauge.Set(result.DiversityScore)

	// Final validation checks
	if !result.HasAPIFallbacks && len(apiKeyProviders) == 0 {
		result.Issues = append(result.Issues, FallbackChainIssue{
			Severity:    "warning",
			Description: "No API key providers in fallback chain",
			Suggestion:  "Add Cerebras, Mistral, DeepSeek, or Gemini as fallbacks",
		})
	}

	if result.UniqueProviders < 3 {
		result.Issues = append(result.Issues, FallbackChainIssue{
			Severity:    "warning",
			Description: fmt.Sprintf("Low provider diversity: only %d unique providers", result.UniqueProviders),
			Suggestion:  "Add more providers to improve resilience",
		})
	}

	// Update metrics and cache
	if result.Valid {
		fcvValidationResultGauge.Set(1)
	} else {
		fcvValidationResultGauge.Set(0)
		fcv.sendAlert(FallbackChainAlert{
			Type:      "validation_failed",
			Message:   fmt.Sprintf("Fallback chain validation failed with %d issues", len(result.Issues)),
			Timestamp: time.Now(),
			Severity:  "critical",
		})
	}

	fcv.lastValidation = result
	fcv.lastValidatedAt = time.Now()

	fcv.logger.WithFields(logrus.Fields{
		"valid":            result.Valid,
		"diversity_score":  result.DiversityScore,
		"unique_providers": result.UniqueProviders,
		"issues_count":     len(result.Issues),
	}).Info("Fallback chain validation completed")

	return result
}

// checkMemberDiversity checks if a team member has diverse fallbacks
func (fcv *FallbackChainValidator) checkMemberDiversity(member *DebateTeamMember) bool {
	if member == nil {
		return false
	}

	// If primary is OAuth, we need at least one non-OAuth fallback
	if member.IsOAuth {
		fb := member.Fallback
		for fb != nil {
			if !fb.IsOAuth && !isFreeProvider(fb.ProviderName) {
				return true
			}
			fb = fb.Fallback
		}
		return false
	}

	// For non-OAuth primaries, just need at least one fallback
	return member.Fallback != nil
}

// calculateDiversityScore calculates a diversity score from 0-100
func (fcv *FallbackChainValidator) calculateDiversityScore(result *FallbackChainValidationResult) float64 {
	score := 0.0

	// Points for unique providers (max 40 points)
	providerScore := float64(result.UniqueProviders) * 8.0
	if providerScore > 40 {
		providerScore = 40
	}
	score += providerScore

	// Points for having both OAuth and API key providers (20 points each)
	if result.HasOAuthPrimaries {
		score += 20
	}
	if result.HasAPIFallbacks {
		score += 20
	}

	// Points for position diversity (4 points per diverse position, max 20)
	diversePositions := 0
	for _, pos := range result.Positions {
		if pos.HasDiversity {
			diversePositions++
		}
	}
	score += float64(diversePositions) * 4.0

	// Deduct points for critical issues
	for _, issue := range result.Issues {
		if issue.Severity == "critical" {
			score -= 15
		} else if issue.Severity == "warning" {
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// sendAlert sends an alert to all listeners
func (fcv *FallbackChainValidator) sendAlert(alert FallbackChainAlert) {
	fcvValidationAlertsTotal.Inc()

	listeners := fcv.listeners

	for _, listener := range listeners {
		go listener(alert)
	}

	fcv.logger.WithFields(logrus.Fields{
		"type":     alert.Type,
		"message":  alert.Message,
		"severity": alert.Severity,
	}).Error("Fallback chain validation alert triggered")
}

// GetLastValidation returns the last validation result
func (fcv *FallbackChainValidator) GetLastValidation() *FallbackChainValidationResult {
	fcv.mu.RLock()
	defer fcv.mu.RUnlock()
	return fcv.lastValidation
}

// GetStatus returns the current validation status
func (fcv *FallbackChainValidator) GetStatus() FallbackChainStatus {
	fcv.mu.RLock()
	defer fcv.mu.RUnlock()

	if fcv.lastValidation == nil {
		return FallbackChainStatus{
			Validated: false,
			Message:   "Validation not yet performed",
		}
	}

	return FallbackChainStatus{
		Validated:       true,
		Valid:           fcv.lastValidation.Valid,
		DiversityScore:  fcv.lastValidation.DiversityScore,
		UniqueProviders: fcv.lastValidation.UniqueProviders,
		IssuesCount:     len(fcv.lastValidation.Issues),
		ValidatedAt:     fcv.lastValidatedAt,
	}
}

// FallbackChainStatus represents the summary status
type FallbackChainStatus struct {
	Validated       bool      `json:"validated"`
	Valid           bool      `json:"valid,omitempty"`
	DiversityScore  float64   `json:"diversity_score,omitempty"`
	UniqueProviders int       `json:"unique_providers,omitempty"`
	IssuesCount     int       `json:"issues_count,omitempty"`
	ValidatedAt     time.Time `json:"validated_at,omitempty"`
	Message         string    `json:"message,omitempty"`
}

// isFreeProvider returns true if the provider is a free/unreliable provider
func isFreeProvider(providerName string) bool {
	freeProviders := map[string]bool{
		"zen":        true,
		"openrouter": true, // OpenRouter free models
	}
	return freeProviders[providerName]
}

// ValidateOnStartup performs validation at startup and returns any critical issues
func (fcv *FallbackChainValidator) ValidateOnStartup() error {
	result := fcv.Validate()

	if !result.Valid {
		criticalCount := 0
		for _, issue := range result.Issues {
			if issue.Severity == "critical" {
				criticalCount++
				fcv.logger.WithFields(logrus.Fields{
					"position":    issue.Position,
					"description": issue.Description,
					"suggestion":  issue.Suggestion,
				}).Error("Critical fallback chain issue detected")
			}
		}

		return fmt.Errorf("fallback chain validation failed: %d critical issues found", criticalCount)
	}

	fcv.logger.WithFields(logrus.Fields{
		"diversity_score":  result.DiversityScore,
		"unique_providers": result.UniqueProviders,
	}).Info("Fallback chain validation passed")

	return nil
}
