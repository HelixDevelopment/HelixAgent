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

// DefaultHelixLLMModels returns the HelixLLM model ids exposed as
// helixllm/<model> when HelixLLM is enabled. Mirrors the registry default
// (provider_registry.go helixllm config, model id "helixllm-default").
func DefaultHelixLLMModels() []string {
	return []string{"helixllm-default"}
}
