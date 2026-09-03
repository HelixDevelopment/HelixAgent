package catalog

// WiredEnsemblePresets returns the voting-strategy presets that are ACTUALLY
// wired in the ensemble service (internal/services/ensemble.go vote switch:
// confidence_weighted, majority_vote, quality_weighted). Exposing only the
// presets the code implements avoids advertising a non-existent preset
// (a §11.4 capability bluff). When the ensemble service grows a new wired
// strategy, add it here so the catalog stays truthful.
func WiredEnsemblePresets() []string {
	return []string{
		"confidence_weighted",
		"majority_vote",
		"quality_weighted",
	}
}

// NOTE: there is deliberately no DefaultHelixLLMModels() here.
//
// A fixed HelixLLM model-id list used to be fed straight into the catalog, so a
// deployment advertised `helixllm/helixllm-default` whether or not anything was
// serving it — a model id with no identity, no host and no serving report,
// presented as a selectable target (BLUFF-002, CONST-036). The catalog's
// HelixLLM section is now sourced from the serving layer's live GET /v1/models
// (see helixllm_source.go); when that listing is unavailable the section is
// honestly empty, which is the only claim the evidence supports.
//
// Options.HelixLLMModels remains as a CALLER-SUPPLIED fallback for a deployment
// that has its own list to offer; nothing in this package supplies one.
