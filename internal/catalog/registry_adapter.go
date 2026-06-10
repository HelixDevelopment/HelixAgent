package catalog

import (
	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/verifier"
)

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
