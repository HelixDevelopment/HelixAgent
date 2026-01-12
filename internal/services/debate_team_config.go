package services

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
	"dev.helix.agent/internal/llm"
)

// TotalDebatePositions is the total number of positions in the AI debate team
const TotalDebatePositions = 5

// FallbacksPerPosition is the number of fallbacks per position
const FallbacksPerPosition = 2

// TotalDebateLLMs is the total number of LLMs used (positions * (1 primary + fallbacks))
const TotalDebateLLMs = TotalDebatePositions * (1 + FallbacksPerPosition) // 15 LLMs

// DebateTeamPosition represents a position in the AI debate team
type DebateTeamPosition int

const (
	PositionAnalyst   DebateTeamPosition = 1 // Primary analyst
	PositionProposer  DebateTeamPosition = 2 // Primary proposer
	PositionCritic    DebateTeamPosition = 3 // Primary critic
	PositionSynthesis DebateTeamPosition = 4 // Synthesis expert
	PositionMediator  DebateTeamPosition = 5 // Mediator/consensus
)

// DebateRole represents the role a team member plays
type DebateRole string

const (
	RoleAnalyst   DebateRole = "analyst"
	RoleProposer  DebateRole = "proposer"
	RoleCritic    DebateRole = "critic"
	RoleSynthesis DebateRole = "synthesis"
	RoleMediator  DebateRole = "mediator"
)

// ClaudeModels defines the available Claude models (OAuth2 provider)
var ClaudeModels = struct {
	Sonnet       string // High quality
	Opus         string // Highest quality
	Haiku        string // Fast, efficient
	SonnetLatest string // Latest Sonnet version
	OpusLatest   string // Latest Opus version
}{
	Sonnet:       "claude-3-sonnet-20240229",
	Opus:         "claude-3-opus-20240229",
	Haiku:        "claude-3-haiku-20240307",
	SonnetLatest: "claude-3-5-sonnet-20241022",
	OpusLatest:   "claude-3-opus-20240229",
}

// QwenModels defines the available Qwen models (OAuth2 provider)
var QwenModels = struct {
	Turbo string // Fast, efficient model
	Plus  string // Balanced model
	Max   string // Most capable model
	Coder string // Code-focused model
	Long  string // Long context model
}{
	Turbo: "qwen-turbo",
	Plus:  "qwen-plus",
	Max:   "qwen-max",
	Coder: "qwen-coder-turbo",
	Long:  "qwen-long",
}

// LLMsVerifierModels defines LLMsVerifier-scored models
var LLMsVerifierModels = struct {
	DeepSeek string // High code quality
	Gemini   string // Strong synthesis
	Mistral  string // Good mediator
	Groq     string // Fast inference
	Cerebras string // Fast inference
}{
	DeepSeek: "deepseek-chat",
	Gemini:   "gemini-2.0-flash",
	Mistral:  "mistral-large-latest",
	Groq:     "llama-3.1-70b-versatile",
	Cerebras: "llama-3.3-70b",
}

// VerifiedLLM represents a verified LLM from LLMsVerifier
type VerifiedLLM struct {
	ProviderName string
	ModelName    string
	Score        float64
	Provider     llm.LLMProvider
	IsOAuth      bool // True if from OAuth2 provider (Claude/Qwen)
	Verified     bool
}

// DebateTeamMember represents a member of the AI debate team
type DebateTeamMember struct {
	Position     DebateTeamPosition `json:"position"`
	Role         DebateRole         `json:"role"`
	ProviderName string             `json:"provider_name"`
	ModelName    string             `json:"model_name"`
	Provider     llm.LLMProvider    `json:"-"`
	Fallback     *DebateTeamMember  `json:"fallback,omitempty"`
	Score        float64            `json:"score"`
	IsActive     bool               `json:"is_active"`
	IsOAuth      bool               `json:"is_oauth"`
}

// DebateTeamConfig manages the AI debate team configuration
type DebateTeamConfig struct {
	mu               sync.RWMutex
	members          map[DebateTeamPosition]*DebateTeamMember
	verifiedLLMs     []*VerifiedLLM // All verified LLMs sorted by score
	providerRegistry *ProviderRegistry
	discovery        *ProviderDiscovery
	logger           *logrus.Logger
}

// NewDebateTeamConfig creates a new debate team configuration
func NewDebateTeamConfig(
	providerRegistry *ProviderRegistry,
	discovery *ProviderDiscovery,
	logger *logrus.Logger,
) *DebateTeamConfig {
	if logger == nil {
		logger = logrus.New()
	}
	config := &DebateTeamConfig{
		members:          make(map[DebateTeamPosition]*DebateTeamMember),
		verifiedLLMs:     make([]*VerifiedLLM, 0),
		providerRegistry: providerRegistry,
		discovery:        discovery,
		logger:           logger,
	}
	return config
}

// InitializeTeam sets up the debate team using:
// 1. OAuth2 providers (Claude, Qwen) if available and verified by LLMsVerifier
// 2. LLMsVerifier-scored providers for remaining positions (best scores used)
// 3. Same LLM can be used in multiple instances if needed
// Total: 15 LLMs (5 positions × 3 LLMs each: 1 primary + 2 fallbacks)
func (dtc *DebateTeamConfig) InitializeTeam(ctx context.Context) error {
	dtc.mu.Lock()
	defer dtc.mu.Unlock()

	dtc.logger.Info("Initializing AI Debate Team (15 LLMs total)...")
	dtc.logger.Info("Strategy: OAuth2 providers (if verified) + LLMsVerifier best-scored providers")

	// Step 1: Verify all providers and collect verified LLMs
	dtc.collectVerifiedLLMs(ctx)

	// Step 2: Sort verified LLMs by score (highest first)
	sort.Slice(dtc.verifiedLLMs, func(i, j int) bool {
		// Prioritize OAuth providers, then by score
		if dtc.verifiedLLMs[i].IsOAuth != dtc.verifiedLLMs[j].IsOAuth {
			return dtc.verifiedLLMs[i].IsOAuth
		}
		return dtc.verifiedLLMs[i].Score > dtc.verifiedLLMs[j].Score
	})

	dtc.logger.WithField("verified_count", len(dtc.verifiedLLMs)).Info("Collected verified LLMs")

	// Step 3: Assign primary positions (5 positions)
	dtc.assignPrimaryPositions()

	// Step 4: Assign fallbacks (2 per position = 10 more slots)
	dtc.assignAllFallbacks()

	// Step 5: Log final team composition
	dtc.logTeamComposition()

	dtc.logger.WithFields(logrus.Fields{
		"total_positions": TotalDebatePositions,
		"total_llms":      TotalDebateLLMs,
		"assigned":        len(dtc.members),
	}).Info("AI Debate Team initialized")

	return nil
}

// collectVerifiedLLMs gathers all verified LLMs from OAuth2 and LLMsVerifier
func (dtc *DebateTeamConfig) collectVerifiedLLMs(ctx context.Context) {
	dtc.verifiedLLMs = make([]*VerifiedLLM, 0)

	// Verify providers if discovery is available
	if dtc.discovery != nil {
		dtc.discovery.VerifyAllProviders(ctx)
	}

	// Collect OAuth2 Claude models (if verified)
	dtc.collectClaudeModels()

	// Collect OAuth2 Qwen models (if verified)
	dtc.collectQwenModels()

	// Collect LLMsVerifier-scored providers
	dtc.collectLLMsVerifierProviders()

	dtc.logger.WithFields(logrus.Fields{
		"total_verified": len(dtc.verifiedLLMs),
		"oauth_count":    dtc.countOAuthLLMs(),
	}).Info("Verified LLMs collected")
}

// collectClaudeModels collects Claude models if the provider is verified
func (dtc *DebateTeamConfig) collectClaudeModels() {
	provider := dtc.getVerifiedProvider("claude", "claude-oauth")
	if provider == nil {
		dtc.logger.Debug("Claude provider not available or not verified")
		return
	}

	// Add all Claude models
	claudeModels := []struct {
		Name  string
		Score float64
	}{
		{ClaudeModels.SonnetLatest, 9.5},
		{ClaudeModels.Opus, 9.5},
		{ClaudeModels.Haiku, 8.5},
	}

	for _, m := range claudeModels {
		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "claude",
			ModelName:    m.Name,
			Score:        m.Score,
			Provider:     provider,
			IsOAuth:      true,
			Verified:     true,
		})
	}

	dtc.logger.WithField("models", len(claudeModels)).Info("Added Claude OAuth2 models")
}

// collectQwenModels collects Qwen models if the provider is verified
func (dtc *DebateTeamConfig) collectQwenModels() {
	provider := dtc.getVerifiedProvider("qwen", "qwen-oauth")
	if provider == nil {
		dtc.logger.Debug("Qwen provider not available or not verified")
		return
	}

	// Add all Qwen models
	qwenModels := []struct {
		Name  string
		Score float64
	}{
		{QwenModels.Max, 8.0},
		{QwenModels.Plus, 7.8},
		{QwenModels.Turbo, 7.5},
		{QwenModels.Coder, 7.5},
		{QwenModels.Long, 7.5},
	}

	for _, m := range qwenModels {
		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "qwen",
			ModelName:    m.Name,
			Score:        m.Score,
			Provider:     provider,
			IsOAuth:      true,
			Verified:     true,
		})
	}

	dtc.logger.WithField("models", len(qwenModels)).Info("Added Qwen OAuth2 models")
}

// collectLLMsVerifierProviders collects providers verified by LLMsVerifier
func (dtc *DebateTeamConfig) collectLLMsVerifierProviders() {
	if dtc.discovery == nil {
		return
	}

	// Get best providers from discovery (already verified and scored)
	bestProviders := dtc.discovery.GetBestProviders(20)

	for _, p := range bestProviders {
		// Skip if already added as OAuth provider
		if p.Type == "claude" || p.Type == "qwen" {
			continue
		}

		if p.Verified && p.Provider != nil {
			dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
				ProviderName: p.Name,
				ModelName:    p.DefaultModel,
				Score:        p.Score,
				Provider:     p.Provider,
				IsOAuth:      false,
				Verified:     true,
			})
		}
	}

	dtc.logger.WithField("llmsverifier_count", len(bestProviders)).Debug("Collected LLMsVerifier providers")
}

// getVerifiedProvider tries to get a verified provider by name(s)
func (dtc *DebateTeamConfig) getVerifiedProvider(names ...string) llm.LLMProvider {
	for _, name := range names {
		// Try registry first
		if dtc.providerRegistry != nil {
			if p, err := dtc.providerRegistry.GetProvider(name); err == nil && p != nil {
				return p
			}
		}
		// Try discovery
		if dtc.discovery != nil {
			if discovered := dtc.discovery.GetProviderByName(name); discovered != nil {
				if discovered.Verified && discovered.Provider != nil {
					return discovered.Provider
				}
			}
		}
	}
	return nil
}

// countOAuthLLMs counts the number of OAuth2 LLMs in the verified list
func (dtc *DebateTeamConfig) countOAuthLLMs() int {
	count := 0
	for _, llm := range dtc.verifiedLLMs {
		if llm.IsOAuth {
			count++
		}
	}
	return count
}

// assignPrimaryPositions assigns the best 5 LLMs to primary positions
func (dtc *DebateTeamConfig) assignPrimaryPositions() {
	roles := []struct {
		Position DebateTeamPosition
		Role     DebateRole
	}{
		{PositionAnalyst, RoleAnalyst},
		{PositionProposer, RoleProposer},
		{PositionCritic, RoleCritic},
		{PositionSynthesis, RoleSynthesis},
		{PositionMediator, RoleMediator},
	}

	usedIdx := 0
	for _, r := range roles {
		var llmToUse *VerifiedLLM

		// Find next available LLM (can reuse if needed)
		if usedIdx < len(dtc.verifiedLLMs) {
			llmToUse = dtc.verifiedLLMs[usedIdx]
			usedIdx++
		} else if len(dtc.verifiedLLMs) > 0 {
			// Reuse best available LLM if we've exhausted the list
			llmToUse = dtc.verifiedLLMs[0]
			dtc.logger.WithField("position", r.Position).Debug("Reusing LLM for position (not enough unique LLMs)")
		}

		if llmToUse != nil {
			member := &DebateTeamMember{
				Position:     r.Position,
				Role:         r.Role,
				ProviderName: llmToUse.ProviderName,
				ModelName:    llmToUse.ModelName,
				Provider:     llmToUse.Provider,
				Score:        llmToUse.Score,
				IsActive:     true,
				IsOAuth:      llmToUse.IsOAuth,
			}
			dtc.members[r.Position] = member

			dtc.logger.WithFields(logrus.Fields{
				"position": r.Position,
				"role":     r.Role,
				"provider": llmToUse.ProviderName,
				"model":    llmToUse.ModelName,
				"score":    llmToUse.Score,
				"oauth":    llmToUse.IsOAuth,
			}).Info("Assigned primary position")
		} else {
			dtc.logger.WithField("position", r.Position).Warn("No LLM available for position")
		}
	}
}

// assignAllFallbacks assigns 2 fallbacks to each position
func (dtc *DebateTeamConfig) assignAllFallbacks() {
	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		if member == nil {
			continue
		}

		// Get fallback LLMs (different from primary if possible)
		fallbacks := dtc.getFallbackLLMs(member.ProviderName, member.ModelName, FallbacksPerPosition)

		// Chain fallbacks
		current := member
		for _, fb := range fallbacks {
			fallbackMember := &DebateTeamMember{
				Position:     pos,
				Role:         member.Role,
				ProviderName: fb.ProviderName,
				ModelName:    fb.ModelName,
				Provider:     fb.Provider,
				Score:        fb.Score,
				IsActive:     false,
				IsOAuth:      fb.IsOAuth,
			}
			current.Fallback = fallbackMember
			current = fallbackMember
		}

		dtc.logger.WithFields(logrus.Fields{
			"position":       pos,
			"fallback_count": len(fallbacks),
		}).Debug("Assigned fallbacks")
	}
}

// getFallbackLLMs returns fallback LLMs different from the primary
// IMPORTANT: For OAuth primaries, prioritize non-OAuth fallbacks to ensure
// fallback chain works when OAuth tokens are incompatible with public APIs
func (dtc *DebateTeamConfig) getFallbackLLMs(primaryProvider, primaryModel string, count int) []*VerifiedLLM {
	fallbacks := make([]*VerifiedLLM, 0, count)

	// Check if primary is OAuth
	primaryIsOAuth := false
	for _, llm := range dtc.verifiedLLMs {
		if llm.ProviderName == primaryProvider && llm.ModelName == primaryModel {
			primaryIsOAuth = llm.IsOAuth
			break
		}
	}

	// First pass: For OAuth primaries, prioritize non-OAuth providers as fallbacks
	// This ensures fallback works when OAuth tokens fail with public APIs
	if primaryIsOAuth {
		for _, llm := range dtc.verifiedLLMs {
			if len(fallbacks) >= count {
				break
			}
			// Prioritize non-OAuth providers for OAuth primaries
			if !llm.IsOAuth && (llm.ProviderName != primaryProvider || llm.ModelName != primaryModel) {
				fallbacks = append(fallbacks, llm)
				dtc.logger.WithFields(logrus.Fields{
					"primary_provider": primaryProvider,
					"fallback_provider": llm.ProviderName,
					"fallback_model":   llm.ModelName,
					"reason":           "non-oauth fallback for oauth primary",
				}).Debug("Selected non-OAuth fallback for OAuth primary")
			}
		}
	}

	// Second pass: If still need more fallbacks, add different provider/model
	for _, llm := range dtc.verifiedLLMs {
		if len(fallbacks) >= count {
			break
		}
		// Skip if already added
		alreadyUsed := false
		for _, fb := range fallbacks {
			if fb == llm {
				alreadyUsed = true
				break
			}
		}
		if alreadyUsed {
			continue
		}
		// Prefer different provider/model
		if llm.ProviderName != primaryProvider || llm.ModelName != primaryModel {
			fallbacks = append(fallbacks, llm)
		}
	}

	// Third pass: If still not enough, allow reuse (last resort)
	for i := 0; len(fallbacks) < count && i < len(dtc.verifiedLLMs); i++ {
		alreadyUsed := false
		for _, fb := range fallbacks {
			if fb == dtc.verifiedLLMs[i] {
				alreadyUsed = true
				break
			}
		}
		if !alreadyUsed {
			fallbacks = append(fallbacks, dtc.verifiedLLMs[i])
		}
	}

	return fallbacks
}

// logTeamComposition logs the final team composition
func (dtc *DebateTeamConfig) logTeamComposition() {
	dtc.logger.Info("=== AI Debate Team Composition (15 LLMs) ===")
	totalLLMs := 0

	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		if member != nil {
			totalLLMs++
			fields := logrus.Fields{
				"position": pos,
				"role":     member.Role,
				"provider": member.ProviderName,
				"model":    member.ModelName,
				"score":    member.Score,
				"oauth":    member.IsOAuth,
			}

			// Count fallbacks
			fallbackCount := 0
			fb := member.Fallback
			for fb != nil {
				fallbackCount++
				totalLLMs++
				fb = fb.Fallback
			}
			fields["fallbacks"] = fallbackCount

			dtc.logger.WithFields(fields).Info("Position assigned")
		} else {
			dtc.logger.WithField("position", pos).Warn("Position unassigned")
		}
	}

	dtc.logger.WithField("total_llms_used", totalLLMs).Info("Team composition complete")
}

// GetTeamMember returns the team member at the specified position
func (dtc *DebateTeamConfig) GetTeamMember(position DebateTeamPosition) *DebateTeamMember {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()
	return dtc.members[position]
}

// GetActiveMembers returns all active team members
func (dtc *DebateTeamConfig) GetActiveMembers() []*DebateTeamMember {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()

	members := make([]*DebateTeamMember, 0, len(dtc.members))
	for _, member := range dtc.members {
		if member != nil && member.IsActive {
			members = append(members, member)
		}
	}
	return members
}

// GetAllLLMs returns all 15 LLMs used in the debate team
func (dtc *DebateTeamConfig) GetAllLLMs() []*DebateTeamMember {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()

	allLLMs := make([]*DebateTeamMember, 0, TotalDebateLLMs)

	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		for member != nil {
			allLLMs = append(allLLMs, member)
			member = member.Fallback
		}
	}

	return allLLMs
}

// GetVerifiedLLMs returns the list of verified LLMs used for team formation
func (dtc *DebateTeamConfig) GetVerifiedLLMs() []*VerifiedLLM {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()
	return dtc.verifiedLLMs
}

// GetTeamSummary returns a summary of the debate team configuration
func (dtc *DebateTeamConfig) GetTeamSummary() map[string]interface{} {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()

	positions := make([]map[string]interface{}, 0, TotalDebatePositions)
	totalLLMs := 0
	oauthCount := 0
	verifierCount := 0

	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		if member == nil {
			positions = append(positions, map[string]interface{}{
				"position": pos,
				"status":   "unassigned",
			})
			continue
		}

		totalLLMs++
		if member.IsOAuth {
			oauthCount++
		} else {
			verifierCount++
		}

		posInfo := map[string]interface{}{
			"position":  pos,
			"role":      member.Role,
			"provider":  member.ProviderName,
			"model":     member.ModelName,
			"score":     member.Score,
			"is_active": member.IsActive,
			"is_oauth":  member.IsOAuth,
		}

		if member.Fallback != nil {
			fallbacks := []map[string]interface{}{}
			fb := member.Fallback
			for fb != nil {
				totalLLMs++
				if fb.IsOAuth {
					oauthCount++
				} else {
					verifierCount++
				}
				fallbacks = append(fallbacks, map[string]interface{}{
					"provider": fb.ProviderName,
					"model":    fb.ModelName,
					"score":    fb.Score,
					"is_oauth": fb.IsOAuth,
				})
				fb = fb.Fallback
			}
			posInfo["fallback_chain"] = fallbacks
		}

		positions = append(positions, posInfo)
	}

	return map[string]interface{}{
		"team_name":             "HelixAgent AI Debate Team",
		"total_positions":       TotalDebatePositions,
		"total_llms":            totalLLMs,
		"expected_llms":         TotalDebateLLMs,
		"oauth_llms":            oauthCount,
		"llmsverifier_llms":     verifierCount,
		"active_positions":      len(dtc.GetActiveMembers()),
		"positions":             positions,
		"verified_llms_count":   len(dtc.verifiedLLMs),
		"claude_models": map[string]string{
			"sonnet_latest": ClaudeModels.SonnetLatest,
			"opus":          ClaudeModels.Opus,
			"haiku":         ClaudeModels.Haiku,
		},
		"qwen_models": map[string]string{
			"turbo": QwenModels.Turbo,
			"plus":  QwenModels.Plus,
			"max":   QwenModels.Max,
			"coder": QwenModels.Coder,
			"long":  QwenModels.Long,
		},
		"llmsverifier_models": map[string]string{
			"deepseek": LLMsVerifierModels.DeepSeek,
			"gemini":   LLMsVerifierModels.Gemini,
			"mistral":  LLMsVerifierModels.Mistral,
			"groq":     LLMsVerifierModels.Groq,
			"cerebras": LLMsVerifierModels.Cerebras,
		},
	}
}

// ActivateFallback activates the fallback for a position when the primary fails
func (dtc *DebateTeamConfig) ActivateFallback(position DebateTeamPosition) (*DebateTeamMember, error) {
	dtc.mu.Lock()
	defer dtc.mu.Unlock()

	member := dtc.members[position]
	if member == nil {
		return nil, fmt.Errorf("no member at position %d", position)
	}

	if member.Fallback == nil {
		return nil, fmt.Errorf("no fallback available for position %d", position)
	}

	// Deactivate current member
	member.IsActive = false

	// Activate fallback
	fallback := member.Fallback
	fallback.IsActive = true
	dtc.members[position] = fallback

	dtc.logger.WithFields(logrus.Fields{
		"position":     position,
		"old_provider": member.ProviderName,
		"new_provider": fallback.ProviderName,
		"new_model":    fallback.ModelName,
	}).Info("Activated fallback for debate position")

	return fallback, nil
}

// GetProviderForPosition returns the appropriate provider for a debate position
func (dtc *DebateTeamConfig) GetProviderForPosition(position DebateTeamPosition) (llm.LLMProvider, string, error) {
	member := dtc.GetTeamMember(position)
	if member == nil {
		return nil, "", fmt.Errorf("no member assigned to position %d", position)
	}

	if member.Provider == nil {
		// Try to activate fallback
		fallback, err := dtc.ActivateFallback(position)
		if err != nil {
			return nil, "", fmt.Errorf("provider unavailable and no fallback: %w", err)
		}
		return fallback.Provider, fallback.ModelName, nil
	}

	return member.Provider, member.ModelName, nil
}

// CountTotalLLMs returns the total number of LLMs in the team (including fallbacks)
func (dtc *DebateTeamConfig) CountTotalLLMs() int {
	return len(dtc.GetAllLLMs())
}

// IsFullyPopulated returns true if all 15 LLM slots are filled
func (dtc *DebateTeamConfig) IsFullyPopulated() bool {
	return dtc.CountTotalLLMs() >= TotalDebateLLMs
}
