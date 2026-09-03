package helixllm

// HealthResponse represents the health check response from HelixLLM
type HealthResponse struct {
	Status    string          `json:"status"`
	Version   string          `json:"version"`
	Mode      string          `json:"mode"`
	Uptime    string          `json:"uptime"`
	Services  map[string]bool `json:"services"`
	Timestamp int64           `json:"timestamp"`
}

// ChatCompletionRequest represents a chat completion request
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// EmbeddingRequest represents an embedding request
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse represents an embedding response
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

// EmbeddingData represents a single embedding
type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// KnowledgeIngestRequest represents a knowledge ingestion request
type KnowledgeIngestRequest struct {
	Collection string                   `json:"collection"`
	Documents  []string                 `json:"documents"`
	Metadata   []map[string]interface{} `json:"metadata,omitempty"`
}

// KnowledgeIngestResponse represents a knowledge ingestion response
type KnowledgeIngestResponse struct {
	ID         string `json:"id"`
	Collection string `json:"collection"`
	ChunkCount int    `json:"chunk_count"`
	Status     string `json:"status"`
	DurationMs int    `json:"duration_ms"`
}

// KnowledgeQueryRequest represents a knowledge query request
type KnowledgeQueryRequest struct {
	Collection string `json:"collection"`
	Query      string `json:"query"`
	TopK       int    `json:"top_k,omitempty"`
}

// KnowledgeQueryResponse represents a knowledge query response
type KnowledgeQueryResponse struct {
	Results []KnowledgeResult `json:"results"`
	Query   string            `json:"query"`
	TopK    int               `json:"top_k"`
}

// KnowledgeResult represents a single knowledge result
type KnowledgeResult struct {
	Content    string                 `json:"content"`
	Score      float64                `json:"score"`
	Metadata   map[string]interface{} `json:"metadata"`
	Collection string                 `json:"collection"`
}

// AgentChatRequest represents an agent chat request
type AgentChatRequest struct {
	SessionID string    `json:"session_id,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []string  `json:"tools,omitempty"`
	UseRAG    bool      `json:"use_rag,omitempty"`
}

// AgentChatResponse represents an agent chat response
type AgentChatResponse struct {
	SessionID string     `json:"session_id"`
	Response  string     `json:"response"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Sources   []string   `json:"sources,omitempty"`
}

// ToolCall represents a tool invocation
type ToolCall struct {
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments"`
	Result    string                 `json:"result"`
}

// ModelsResponse represents the models list response
type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// ModelInfo represents a single model's information as GET /v1/models reports
// it.
//
// The first five fields are the OpenAI-compatible shape. The four below them
// are the model-listing contract's additions, which let a consumer name the
// serving host and tell a served model from a withheld one. They are all
// omitempty and every one of them is optional on the wire: a serving layer
// that publishes only the OpenAI shape decodes cleanly here, and the resulting
// zero values mean "not reported" — which is never read as a serving claim.
type ModelInfo struct {
	ID           string          `json:"id"`
	Object       string          `json:"object"`
	Created      int64           `json:"created"`
	OwnedBy      string          `json:"owned_by"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`

	// ModelIdentity is the human-readable `helixllm/<host>/<model>[:<variant>]`
	// identity VALUE. ID remains the identifier; this is never used as one.
	ModelIdentity string `json:"model_identity,omitempty"`
	// Host is the machine serving the model.
	Host string `json:"host,omitempty"`
	// Availability is the reported serving state ("serving" | "withheld").
	// Empty means the serving layer reported nothing.
	Availability string `json:"availability,omitempty"`
	// WithheldReason is why a withheld model is not being served, as one of the
	// recorded machine keys (catalog.WithheldReason). Anything outside that
	// closed set is discarded rather than carried, because a consumer acting on
	// a reason the contract cannot give a remedy for is acting on nothing.
	WithheldReason string `json:"withheld_reason,omitempty"`
}

// ServiceStatus represents the status of HelixLLM services
type ServiceStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Healthy bool   `json:"healthy"`
	Uptime  string `json:"uptime,omitempty"`
	Image   string `json:"image,omitempty"`
	Ports   []Port `json:"ports,omitempty"`
}

// Port represents a port mapping
type Port struct {
	Host      string `json:"host"`
	Container string `json:"container"`
	Protocol  string `json:"protocol"`
}
