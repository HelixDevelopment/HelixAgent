package models

import "time"

type LLMProvider struct {
	ID           string                 `json:"id" db:"id"`
	Name         string                 `json:"name" db:"name"`
	Type         string                 `json:"type" db:"type"`
	APIKey       string                 `json:"-" db:"api_key"`
	BaseURL      string                 `json:"base_url" db:"base_url"`
	Model        string                 `json:"model" db:"model"`
	Weight       float64                `json:"weight" db:"weight"`
	Enabled      bool                   `json:"enabled" db:"enabled"`
	Config       map[string]interface{} `json:"config" db:"config"`
	HealthStatus string                 `json:"health_status" db:"health_status"`
	ResponseTime int64                  `json:"response_time" db:"response_time"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
}

// LLMRequest represents a request to an LLM provider
type LLMRequest struct {
	ID             string            `json:"id" db:"id"`
	SessionID      string            `json:"session_id" db:"session_id"`
	UserID         string            `json:"user_id" db:"user_id"`
	Prompt         string            `json:"prompt" db:"prompt"`
	Messages       []Message         `json:"messages" db:"messages"`
	ModelParams    ModelParameters   `json:"model_params" db:"model_params"`
	EnsembleConfig *EnsembleConfig   `json:"ensemble_config" db:"ensemble_config"`
	MemoryEnhanced bool              `json:"memory_enhanced" db:"memory_enhanced"`
	Memory         map[string]string `json:"memory" db:"memory"`
	Status         string            `json:"status" db:"status"`
	CreatedAt      time.Time         `json:"created_at" db:"created_at"`
	StartedAt      *time.Time        `json:"started_at" db:"started_at"`
	CompletedAt    *time.Time        `json:"completed_at" db:"completed_at"`
	RequestType    string            `json:"request_type" db:"request_type"`
}

type LLMResponse struct {
	ID             string                 `json:"id" db:"id"`
	RequestID      string                 `json:"request_id" db:"request_id"`
	ProviderID     string                 `json:"provider_id" db:"provider_id"`
	ProviderName   string                 `json:"provider_name" db:"provider_name"`
	Content        string                 `json:"content" db:"content"`
	Confidence     float64                `json:"confidence" db:"confidence"`
	TokensUsed     int                    `json:"tokens_used" db:"tokens_used"`
	ResponseTime   int64                  `json:"response_time" db:"response_time"`
	FinishReason   string                 `json:"finish_reason" db:"finish_reason"`
	Metadata       map[string]interface{} `json:"metadata" db:"metadata"`
	Selected       bool                   `json:"selected" db:"selected"`
	SelectionScore float64                `json:"selection_score" db:"selection_score"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
}

type Message struct {
	Role      string                 `json:"role" db:"role"`
	Content   string                 `json:"content" db:"content"`
	Name      *string                `json:"name" db:"name"`
	ToolCalls map[string]interface{} `json:"tool_calls" db:"tool_calls"`
}

type ModelParameters struct {
	Model            string                 `json:"model" db:"model"`
	Temperature      float64                `json:"temperature" db:"temperature"`
	MaxTokens        int                    `json:"max_tokens" db:"max_tokens"`
	TopP             float64                `json:"top_p" db:"top_p"`
	StopSequences    []string               `json:"stop_sequences" db:"stop_sequences"`
	ProviderSpecific map[string]interface{} `json:"provider_specific" db:"provider_specific"`
}

type EnsembleConfig struct {
	Strategy            string   `json:"strategy" db:"strategy"`
	MinProviders        int      `json:"min_providers" db:"min_providers"`
	ConfidenceThreshold float64  `json:"confidence_threshold" db:"confidence_threshold"`
	FallbackToBest      bool     `json:"fallback_to_best" db:"fallback_to_best"`
	Timeout             int      `json:"timeout" db:"timeout"`
	PreferredProviders  []string `json:"preferred_providers" db:"preferred_providers"`
}

type UserSession struct {
	ID           string                 `json:"id" db:"id"`
	UserID       string                 `json:"user_id" db:"user_id"`
	SessionToken string                 `json:"session_token" db:"session_token"`
	Context      map[string]interface{} `json:"context" db:"context"`
	MemoryID     *string                `json:"memory_id" db:"memory_id"`
	Status       string                 `json:"status" db:"status"`
	RequestCount int                    `json:"request_count" db:"request_count"`
	LastActivity time.Time              `json:"last_activity" db:"last_activity"`
	ExpiresAt    time.Time              `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
}

type CogneeMemory struct {
	ID          string                 `json:"id" db:"id"`
	SessionID   *string                `json:"session_id" db:"session_id"`
	DatasetName string                 `json:"dataset_name" db:"dataset_name"`
	ContentType string                 `json:"content_type" db:"content_type"`
	Content     string                 `json:"content" db:"content"`
	VectorID    string                 `json:"vector_id" db:"vector_id"`
	GraphNodes  map[string]interface{} `json:"graph_nodes" db:"graph_nodes"`
	SearchKey   string                 `json:"search_key" db:"search_key"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
}

type MemorySource struct {
	DatasetName    string  `json:"dataset_name"`
	Content        string  `json:"content"`
	RelevanceScore float64 `json:"relevance_score"`
	SourceType     string  `json:"source_type"`
}

// ProviderCapabilities describes capabilities exposed by an LLM provider.
type ProviderCapabilities struct {
	SupportedModels         []string          `json:"supported_models"`
	SupportedFeatures       []string          `json:"supported_features"`
	SupportedRequestTypes   []string          `json:"supported_request_types"`
	SupportsStreaming       bool              `json:"supports_streaming"`
	SupportsFunctionCalling bool              `json:"supports_function_calling"`
	SupportsVision          bool              `json:"supports_vision"`
	Limits                  ModelLimits       `json:"limits"`
	Metadata                map[string]string `json:"metadata"`

	// LSP specific capabilities
	SupportsTools          bool `json:"supports_tools"`
	SupportsSearch         bool `json:"supports_search"`
	SupportsReasoning      bool `json:"supports_reasoning"`
	SupportsCodeCompletion bool `json:"supports_code_completion"`
	SupportsCodeAnalysis   bool `json:"supports_code_analysis"`
	SupportsRefactoring    bool `json:"supports_refactoring"`
}

// ModelLimits defines the operational limits of an LLM model.
type ModelLimits struct {
	MaxTokens             int `json:"max_tokens"`
	MaxInputLength        int `json:"max_input_length"`
	MaxOutputLength       int `json:"max_output_length"`
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
}

// LSP-related types for Language Server Protocol integration

// CodeIntelligence represents comprehensive code intelligence from LSP
type CodeIntelligence struct {
	FilePath       string            `json:"file_path"`
	Diagnostics    []*Diagnostic     `json:"diagnostics"`
	Completions    []*CompletionItem `json:"completions"`
	Hover          *HoverInfo        `json:"hover"`
	Definitions    []*Location       `json:"definitions"`
	References     []*Location       `json:"references"`
	Symbols        []*SymbolInfo     `json:"symbols"`
	SemanticTokens *SemanticTokens   `json:"semantic_tokens"`
}

// Diagnostic represents a diagnostic message from LSP
type Diagnostic struct {
	Range              Range                          `json:"range"`
	Severity           int                            `json:"severity"`
	Code               string                         `json:"code"`
	Source             string                         `json:"source"`
	Message            string                         `json:"message"`
	RelatedInformation []DiagnosticRelatedInformation `json:"related_information"`
}

// DiagnosticRelatedInformation represents related diagnostic information
type DiagnosticRelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

// CompletionItem represents a completion item from LSP
type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind"`
	Detail        string `json:"detail"`
	Documentation string `json:"documentation"`
	InsertText    string `json:"insert_text"`
}

// HoverInfo represents hover information from LSP
type HoverInfo struct {
	Content  string `json:"content"`
	Language string `json:"language"`
}

// Location represents a location in a file
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Range represents a range in a text document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a position in a text document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// SymbolInfo represents symbol information from LSP
type SymbolInfo struct {
	Name          string        `json:"name"`
	Kind          int           `json:"kind"`
	Location      Location      `json:"location"`
	ContainerName string        `json:"container_name"`
	Children      []*SymbolInfo `json:"children"`
}

// SemanticTokens represents semantic tokens from LSP
type SemanticTokens struct {
	Data []int `json:"data"`
}

// WorkspaceEdit represents a workspace edit from LSP
type WorkspaceEdit struct {
	Changes map[string][]*TextEdit `json:"changes"`
}

// TextEdit represents a text edit
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}
