// Package catalog builds the unified exposure catalog (SP2 P2.1/P2.2): one root
// list that joins, as uniformly-named selectable targets,
//
//  1. the AI-debate ensemble (aggregate + named presets),
//  2. HelixLLM promoted to a first-class root entry (when enabled),
//  3. every discovered provider individually, and
//  4. each provider's WORKING (Verified==true) models.
//
// The catalog is a pure JOIN over three already-existing runtime sources
// (the provider registry, the LLMsVerifier discovery service, and the wired
// ensemble strategies). It NEVER fabricates a "working" list: when the
// verifier is absent/disabled the per-model entries are honestly empty
// (CONST-036/037, §11.4 anti-bluff). No source is removed (§11.4.122).
//
// Naming scheme (analysis-C, confirmed against existing conventions):
//
//	ensemble                       — the aggregate AI-debate ensemble
//	ensemble/<preset>              — one per wired voting strategy
//	helixllm                       — HelixLLM root (only when enabled)
//	helixllm/<model>               — a HelixLLM model (only when enabled)
//	<provider>                     — one per discovered provider
//	<provider>/<model_id>          — one per Verified==true model
//
// Already-namespaced ids ("openrouter/x-ai/grok-4") are preserved verbatim.
package catalog

import (
	"sort"
	"strings"
)

// Kind classifies a catalog entry.
type Kind string

const (
	// KindEnsemble is the aggregate AI-debate ensemble (and its presets).
	KindEnsemble Kind = "ensemble"
	// KindProvider is a single discovered provider (a selectable root).
	KindProvider Kind = "provider"
	// KindModel is a single working (verified) model under a provider,
	// or a HelixLLM model.
	KindModel Kind = "model"
)

// NameEnsemble is the canonical root name of the aggregate ensemble.
const NameEnsemble = "ensemble"

// NameHelixLLM is the canonical root name of the promoted HelixLLM provider.
const NameHelixLLM = "helixllm"

// Entry is one uniformly-named, individually-addressable selection target.
type Entry struct {
	// Name is the selector a user passes (e.g. "ensemble",
	// "ensemble/majority_vote", "helixllm", "anthropic/claude-3-sonnet",
	// "openrouter/x-ai/grok-4"). It is the join key.
	Name string `json:"name"`
	// Kind is ensemble | provider | model.
	Kind Kind `json:"kind"`
	// Provider is the owning provider name for provider/model entries
	// (empty for the aggregate "ensemble" entry).
	Provider string `json:"provider,omitempty"`
	// Model is the bare model id for KindModel entries.
	Model string `json:"model,omitempty"`
	// Verified is true only for model entries the verifier confirmed working.
	Verified bool `json:"verified"`
	// OverallScore is the verifier score for a verified model (0 otherwise).
	OverallScore float64 `json:"overall_score,omitempty"`
	// Enabled reports whether the underlying provider is enabled/registered.
	Enabled bool `json:"enabled"`
}

// ProviderInfo is the minimal projection of a registered provider that the
// catalog needs. It decouples the catalog from the concrete
// services.ProviderRegistry (CONST-051 — the catalog is reusable/testable
// without booting the registry).
type ProviderInfo struct {
	Name    string
	Enabled bool
	// SupportedModels is the provider's self-declared model list
	// (GetCapabilities().SupportedModels). Used ONLY as a fallback display
	// hint, never promoted to "verified".
	SupportedModels []string
}

// VerifiedModel is the minimal projection of a verifier DiscoveredModel that
// the catalog needs. The catalog includes a model entry ONLY when the
// verifier reports it Verified==true.
type VerifiedModel struct {
	Provider     string
	ModelID      string
	Verified     bool
	OverallScore float64
}

// ProviderSource yields the set of discovered providers and their enabled
// state. Implemented by an adapter over services.ProviderRegistry.
type ProviderSource interface {
	Providers() []ProviderInfo
}

// VerifiedModelSource yields the verifier's discovered models. It MAY be nil
// (verifier disabled / not wired): in that case the catalog is honestly empty
// of model entries rather than fabricating a working list.
type VerifiedModelSource interface {
	VerifiedModels() []VerifiedModel
}

// CatalogService joins the runtime sources into the unified catalog.
type CatalogService struct {
	providers       ProviderSource
	verified        VerifiedModelSource // nilable
	ensemblePresets []string
	helixLLMEnabled bool
	helixLLMModels  []string
}

// Options configures a CatalogService.
type Options struct {
	// Providers is the source of discovered providers (required).
	Providers ProviderSource
	// Verified is the verifier discovery source. MAY be nil → honest-empty
	// model section (no fabricated "working" models).
	Verified VerifiedModelSource
	// EnsemblePresets are the wired voting-strategy names
	// (e.g. confidence_weighted, majority_vote, quality_weighted).
	EnsemblePresets []string
	// HelixLLMEnabled promotes HelixLLM to a first-class root entry when true.
	HelixLLMEnabled bool
	// HelixLLMModels are the HelixLLM model ids to expose as helixllm/<model>
	// (only emitted when HelixLLMEnabled).
	HelixLLMModels []string
}

// New constructs a CatalogService.
func New(opts Options) *CatalogService {
	return &CatalogService{
		providers:       opts.Providers,
		verified:        opts.Verified,
		ensemblePresets: opts.EnsemblePresets,
		helixLLMEnabled: opts.HelixLLMEnabled,
		helixLLMModels:  opts.HelixLLMModels,
	}
}

// Build assembles the unified catalog. The result is deterministically ordered
// (§11.4.50): ensemble first, then its presets, then helixllm (+models) when
// enabled, then providers and their verified models, all sorted by Name.
func (s *CatalogService) Build() []Entry {
	entries := make([]Entry, 0, 32)

	// (a) the aggregate ensemble + presets.
	entries = append(entries, Entry{
		Name:    NameEnsemble,
		Kind:    KindEnsemble,
		Enabled: true,
	})
	for _, p := range dedupSorted(normalizeAll(s.ensemblePresets)) {
		if p == "" {
			continue
		}
		entries = append(entries, Entry{
			Name:    NameEnsemble + "/" + p,
			Kind:    KindEnsemble,
			Enabled: true,
		})
	}

	// (b) HelixLLM promoted to a first-class root — only when enabled
	// (never fabricated when disabled, §11.4 anti-bluff).
	if s.helixLLMEnabled {
		entries = append(entries, Entry{
			Name:     NameHelixLLM,
			Kind:     KindProvider,
			Provider: NameHelixLLM,
			Enabled:  true,
		})
		for _, m := range dedupSorted(normalizeAll(s.helixLLMModels)) {
			if m == "" {
				continue
			}
			entries = append(entries, Entry{
				Name:     NameHelixLLM + "/" + m,
				Kind:     KindModel,
				Provider: NameHelixLLM,
				Model:    m,
				Enabled:  true,
			})
		}
	}

	// (c) one entry per discovered provider (excluding helixllm, already
	// promoted above to avoid a duplicate root).
	var provInfos []ProviderInfo
	if s.providers != nil {
		provInfos = s.providers.Providers()
	}
	enabledByProvider := make(map[string]bool, len(provInfos))
	for _, pi := range provInfos {
		name := normalize(pi.Name)
		if name == "" {
			continue
		}
		enabledByProvider[name] = pi.Enabled
		if name == NameHelixLLM {
			// helixllm root is governed by the dedicated promotion above.
			continue
		}
		entries = append(entries, Entry{
			Name:     name,
			Kind:     KindProvider,
			Provider: name,
			Enabled:  pi.Enabled,
		})
	}

	// (d) one <provider>/<model_id> entry per Verified==true model.
	// Honest-empty when the verifier source is absent.
	if s.verified != nil {
		for _, vm := range s.verified.VerifiedModels() {
			if !vm.Verified {
				continue
			}
			prov := normalize(vm.Provider)
			model := strings.TrimSpace(vm.ModelID)
			if prov == "" || model == "" {
				continue
			}
			entries = append(entries, Entry{
				Name:         prov + "/" + model,
				Kind:         KindModel,
				Provider:     prov,
				Model:        model,
				Verified:     true,
				OverallScore: vm.OverallScore,
				Enabled:      enabledByProvider[prov],
			})
		}
	}

	sortEntries(entries)
	return entries
}

// sortEntries orders entries deterministically: the aggregate "ensemble" root
// first, then everything else by Name (case-folded).
func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		ai := entries[i].Name == NameEnsemble
		aj := entries[j].Name == NameEnsemble
		if ai != aj {
			return ai // the bare "ensemble" root sorts first
		}
		return entries[i].Name < entries[j].Name
	})
}

// normalize lower-cases and trims a provider/preset/model token so the
// catalog namespace is uniformly lowercase (matches the existing convention:
// every registered provider name is already lowercase). It preserves any
// embedded "/" (so "x-ai/grok-4" stays intact).
func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func normalizeAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, normalize(s))
	}
	return out
}

func dedupSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
