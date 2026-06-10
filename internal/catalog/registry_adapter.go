package catalog

import (
	"time"

	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/verifier"
)

// verifiedModelTTL is the CONST-037 freshness window: a model may be surfaced
// to users only if its verifier VerifiedAt is within this window. Every model
// displayed to users MUST be verified by LLMsVerifier within 24h.
const verifiedModelTTL = 24 * time.Hour

// registryProviderSource adapts *services.ProviderRegistry to ProviderSource.
// It reads the live provider list + each provider's enabled state + its
// self-declared SupportedModels (display hint only — never promoted to
// "verified").
type registryProviderSource struct {
	reg *services.ProviderRegistry
}

// NewRegistryProviderSource builds a ProviderSource over the runtime registry.
func NewRegistryProviderSource(reg *services.ProviderRegistry) ProviderSource {
	return &registryProviderSource{reg: reg}
}

func (s *registryProviderSource) Providers() []ProviderInfo {
	if s == nil || s.reg == nil {
		return nil
	}
	names := s.reg.ListProviders()
	out := make([]ProviderInfo, 0, len(names))
	for _, name := range names {
		pi := ProviderInfo{Name: name, Enabled: true}
		if cfg, err := s.reg.GetProviderConfig(name); err == nil && cfg != nil {
			pi.Enabled = cfg.Enabled
		}
		if prov, err := s.reg.GetProvider(name); err == nil && prov != nil {
			if caps := prov.GetCapabilities(); caps != nil {
				pi.SupportedModels = caps.SupportedModels
			}
		}
		out = append(out, pi)
	}
	return out
}

// discoveryVerifiedSource adapts *verifier.ModelDiscoveryService to
// VerifiedModelSource. When the discovery service is nil (verifier disabled /
// not wired) the catalog stays honestly empty of model entries — callers pass
// a nil VerifiedModelSource in that case.
type discoveryVerifiedSource struct {
	disc *verifier.ModelDiscoveryService
}

// NewDiscoveryVerifiedSource builds a VerifiedModelSource over the verifier
// discovery service. Returns nil when disc is nil so the catalog is
// honest-empty (no fabricated working list).
func NewDiscoveryVerifiedSource(disc *verifier.ModelDiscoveryService) VerifiedModelSource {
	if disc == nil {
		return nil
	}
	return &discoveryVerifiedSource{disc: disc}
}

func (s *discoveryVerifiedSource) VerifiedModels() []VerifiedModel {
	if s == nil || s.disc == nil {
		return nil
	}
	discovered := s.disc.GetDiscoveredModels()
	out := make([]VerifiedModel, 0, len(discovered))
	for _, m := range discovered {
		if m == nil {
			continue
		}
		out = append(out, VerifiedModel{
			Provider:     m.Provider,
			ModelID:      m.ModelID,
			Verified:     m.Verified,
			OverallScore: m.OverallScore,
		})
	}
	return out
}

// startupVerifierSource adapts *verifier.StartupVerifier to VerifiedModelSource.
// This is the REAL wiring used at runtime (CONST-036 — LLMsVerifier is the
// single source of truth): it reads the already-populated verified providers
// (GetVerifiedProviders → UnifiedProvider.Models) and surfaces only the models
// the verifier marked Verified==true AND verified within the CONST-037 24h
// window. When the StartupVerifier is nil (verifier disabled / not wired) the
// constructor returns a nil source so the catalog stays honest-empty.
type startupVerifierSource struct {
	sv *verifier.StartupVerifier
}

// NewStartupVerifierSource builds a VerifiedModelSource over the StartupVerifier.
// Returns nil when sv is nil so the catalog is honest-empty (no fabricated
// working list, §11.4 anti-bluff).
func NewStartupVerifierSource(sv *verifier.StartupVerifier) VerifiedModelSource {
	if sv == nil {
		return nil
	}
	return &startupVerifierSource{sv: sv}
}

func (s *startupVerifierSource) VerifiedModels() []VerifiedModel {
	if s == nil || s.sv == nil {
		return nil
	}
	return verifiedModelsFromProviders(s.sv.GetVerifiedProviders(), time.Now(), verifiedModelTTL)
}

// verifiedModelsFromProviders projects the verifier's verified providers into
// the catalog's VerifiedModel set, applying BOTH constitutional gates:
//
//   - CONST-036: only models the verifier marked Verified==true are surfaced
//     (NO hardcoded/fabricated working list).
//   - CONST-037: only models whose VerifiedAt is within ttl of now are surfaced
//     (every displayed model verified within 24h). A zero VerifiedAt cannot
//     prove freshness and is treated as stale (§11.4.6 no-guessing).
//
// It is a pure function (now + ttl injected) so the gates are deterministically
// unit-testable without booting a real verifier (§11.4.50).
func verifiedModelsFromProviders(providers []*verifier.UnifiedProvider, now time.Time, ttl time.Duration) []VerifiedModel {
	out := make([]VerifiedModel, 0)
	for _, p := range providers {
		if p == nil {
			continue
		}
		for _, m := range p.Models {
			// CONST-036: verifier must mark the model working.
			if !m.Verified {
				continue
			}
			// CONST-037: must be verified within the 24h window. A zero
			// timestamp is NOT proof of recency → excluded.
			if m.VerifiedAt.IsZero() || now.Sub(m.VerifiedAt) > ttl {
				continue
			}
			provider := m.Provider
			if provider == "" {
				provider = p.Name
			}
			out = append(out, VerifiedModel{
				Provider:     provider,
				ModelID:      m.ID,
				Verified:     true,
				OverallScore: m.Score,
			})
		}
	}
	return out
}
