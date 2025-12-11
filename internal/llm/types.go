package llm

// ProviderCapabilities describes capabilities exposed by an LLM provider.
type ProviderCapabilities struct {
	SupportedModels         []string          `json:"supported_models"`
	SupportedFeatures       []string          `json:"supported_features"`
	SupportedRequestTypes   []string          `json:"supported_request_types"`
	SupportsStreaming       bool              `json:"supports_streaming"`
	SupportsFunctionCalling bool              `json:"supports_function_calling"`
	SupportsVision          bool              `json:"supports_vision"`
	SupportsTools           bool              `json:"supports_tools"`
	SupportsSearch          bool              `json:"supports_search"`
	SupportsReasoning       bool              `json:"supports_reasoning"`
	SupportsCodeCompletion  bool              `json:"supports_code_completion"`
	SupportsCodeAnalysis    bool              `json:"supports_code_analysis"`
	SupportsRefactoring     bool              `json:"supports_refactoring"`
	Limits                  ModelLimits       `json:"limits"`
	Metadata                map[string]string `json:"metadata"`
}

// ModelLimits defines operational limits of an LLM model.
type ModelLimits struct {
	MaxTokens             int `json:"max_tokens"`
	MaxInputLength        int `json:"max_input_length"`
	MaxOutputLength       int `json:"max_output_length"`
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
}
