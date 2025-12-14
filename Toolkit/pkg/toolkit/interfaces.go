package toolkit

import (
	"context"
	"fmt"
)

// Provider represents an AI model provider interface
type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
	Rerank(ctx context.Context, req RerankRequest) (RerankResponse, error)
	DiscoverModels(ctx context.Context) ([]ModelInfo, error)
	ValidateConfig(config map[string]interface{}) error
}

// Agent represents an AI agent interface
type Agent interface {
	Name() string
	Execute(ctx context.Context, task string, config interface{}) (string, error)
	ValidateConfig(config interface{}) error
	Capabilities() []string
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Messages         []Message          `json:"messages"`
	Model            string             `json:"model,omitempty"`
	Temperature      float64            `json:"temperature,omitempty"`
	MaxTokens        int                `json:"max_tokens,omitempty"`
	Stream           bool               `json:"stream,omitempty"`
	TopP             float64            `json:"top_p,omitempty"`
	TopK             int                `json:"top_k,omitempty"`
	Stop             []string           `json:"stop,omitempty"`
	PresencePenalty  float64            `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64            `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64 `json:"logit_bias,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// EmbeddingRequest represents an embedding request
type EmbeddingRequest struct {
	Input          []string `json:"input"`
	Model          string   `json:"model,omitempty"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	User           string   `json:"user,omitempty"`
	Dimensions     int      `json:"dimensions,omitempty"`
}

// EmbeddingResponse represents an embedding response
type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

// EmbeddingData alias for Embedding for backward compatibility
type EmbeddingData = Embedding

// RerankRequest represents a reranking request
type RerankRequest struct {
	Query      string   `json:"query"`
	Documents  []string `json:"documents"`
	Model      string   `json:"model,omitempty"`
	TopN       int      `json:"top_n,omitempty"`
	ReturnDocs bool     `json:"return_documents,omitempty"`
}

// RerankResponse represents a reranking response
type RerankResponse struct {
	Object  string         `json:"object"`
	Data    []RerankData   `json:"data"`
	Results []RerankResult `json:"results,omitempty"`
	Model   string         `json:"model"`
}

// RerankResult represents a reranking result
type RerankResult struct {
	Index    int     `json:"index"`
	Score    float64 `json:"score"`
	Document string  `json:"document,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatMessage alias for Message for backward compatibility
type ChatMessage = Message

// ChatChoice represents a chat completion choice
type ChatChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Choice represents a chat completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Embedding represents an embedding vector
type Embedding struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// RerankData represents reranking result data
type RerankData struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// ModelCategory represents the category of an AI model
type ModelCategory string

const (
	CategoryChat       ModelCategory = "chat"
	CategoryEmbedding  ModelCategory = "embedding"
	CategoryRerank     ModelCategory = "rerank"
	CategoryMultimodal ModelCategory = "multimodal"
	CategoryImage      ModelCategory = "image"
	CategoryAudio      ModelCategory = "audio"
	CategoryVideo      ModelCategory = "video"
)

// ModelInfo represents information about an AI model
type ModelInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name,omitempty"`
	Object       string            `json:"object"`
	Category     ModelCategory     `json:"category,omitempty"`
	Capabilities ModelCapabilities `json:"capabilities,omitempty"`
	Provider     string            `json:"provider,omitempty"`
	Description  string            `json:"description,omitempty"`
	Created      int64             `json:"created"`
	OwnedBy      string            `json:"owned_by"`
}

// ModelCapabilities represents the capabilities of an AI model
type ModelCapabilities struct {
	SupportsChat      bool `json:"supports_chat"`
	SupportsEmbedding bool `json:"supports_embedding"`
	SupportsRerank    bool `json:"supports_rerank"`
	SupportsAudio     bool `json:"supports_audio"`
	SupportsVideo     bool `json:"supports_video"`
	SupportsVision    bool `json:"supports_vision"`
	FunctionCalling   bool `json:"function_calling"`
	ContextWindow     int  `json:"context_window"`
	MaxTokens         int  `json:"max_tokens"`
}

// ProviderFactory is a function that creates a provider from configuration
type ProviderFactory func(config map[string]interface{}) (Provider, error)

// Global provider registry
var globalProviderRegistry = NewProviderFactoryRegistry()

// RegisterProviderFactory registers a provider factory globally
func RegisterProviderFactory(name string, factory ProviderFactory) {
	globalProviderRegistry.Register(name, factory)
}

// CreateProvider creates a provider using the global registry
func CreateProvider(name string, config map[string]interface{}) (Provider, error) {
	return globalProviderRegistry.Create(name, config)
}

// ListProviders returns a list of registered provider names
func ListProviders() []string {
	return globalProviderRegistry.ListProviders()
}

// ProviderFactoryRegistry manages provider factories
type ProviderFactoryRegistry struct {
	factories map[string]ProviderFactory
}

// NewProviderFactoryRegistry creates a new provider factory registry
func NewProviderFactoryRegistry() *ProviderFactoryRegistry {
	return &ProviderFactoryRegistry{
		factories: make(map[string]ProviderFactory),
	}
}

// Register registers a provider factory
func (r *ProviderFactoryRegistry) Register(name string, factory ProviderFactory) error {
	r.factories[name] = factory
	return nil
}

// Create creates a provider using the registered factory
func (r *ProviderFactoryRegistry) Create(name string, config map[string]interface{}) (Provider, error) {
	factory, exists := r.factories[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not registered", name)
	}
	return factory(config)
}

// ListProviders returns a list of registered provider names
func (r *ProviderFactoryRegistry) ListProviders() []string {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}
