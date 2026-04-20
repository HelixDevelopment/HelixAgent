// Package models provides an embedding model registry for multiple embedding providers.
package models

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
)

// EmbeddingModel defines the interface for all embedding models.
type EmbeddingModel interface {
	// Encode generates embeddings for multiple texts.
	Encode(ctx context.Context, texts []string) ([][]float32, error)

	// EncodeSingle generates an embedding for a single text.
	EncodeSingle(ctx context.Context, text string) ([]float32, error)

	// Name returns the model name.
	Name() string

	// Dimensions returns the embedding dimensions.
	Dimensions() int

	// MaxTokens returns the maximum tokens supported.
	MaxTokens() int

	// Provider returns the provider name.
	Provider() string

	// Health checks if the model is healthy.
	Health(ctx context.Context) error

	// Close releases any resources.
	Close() error
}

// EmbeddingModelConfig holds configuration for an embedding model.
type EmbeddingModelConfig struct {
	Name         string        `json:"name"`
	Provider     string        `json:"provider"` // "openai", "ollama", "sentence-transformers", "local"
	ModelID      string        `json:"model_id"`
	Dimensions   int           `json:"dimensions"`
	MaxTokens    int           `json:"max_tokens"`
	BatchSize    int           `json:"batch_size"`
	Timeout      time.Duration `json:"timeout"`
	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl"`
	BaseURL      string        `json:"base_url,omitempty"`
	APIKey       string        `json:"api_key,omitempty"`
}

// lazyModel holds a sync.Once-based deferred initializer for an embedding model.
// The factory is called at most once on first access via get().
type lazyModel struct {
	once    sync.Once
	model   EmbeddingModel
	initErr error
	factory func() (EmbeddingModel, error)
}

// get returns the lazily initialized model, invoking the factory on first call.
func (lm *lazyModel) get() (EmbeddingModel, error) {
	lm.once.Do(func() {
		lm.model, lm.initErr = lm.factory()
	})
	return lm.model, lm.initErr
}

// EmbeddingModelRegistry manages multiple embedding models.
//
// Concurrency model (CONST-029): models/lazyModels/configs → 3×
// *safe.Store. Registration serialises Put + Delete-the-other-kind
// atomically via the Store's internal lock; GetOrCreate uses Get
// followed by createModel() outside any lock, then PutIfAbsent to
// publish without double-creating.
type EmbeddingModelRegistry struct {
	models        *safe.Store[string, EmbeddingModel]
	lazyModels    *safe.Store[string, *lazyModel]
	configs       *safe.Store[string, EmbeddingModelConfig]
	defaultModel  string
	fallbackChain []string
	logger        *logrus.Logger
	cache         EmbeddingCache
}

// EmbeddingCache provides caching for embeddings.
type EmbeddingCache interface {
	Get(key string) ([]float32, bool)
	Set(key string, embedding []float32, ttl time.Duration)
	Delete(key string)
}

// RegistryConfig holds configuration for the registry.
type RegistryConfig struct {
	Logger        *logrus.Logger
	DefaultModel  string
	FallbackChain []string
	Cache         EmbeddingCache
	Configs       map[string]EmbeddingModelConfig
}

// NewEmbeddingModelRegistry creates a new embedding model registry.
func NewEmbeddingModelRegistry(config RegistryConfig) *EmbeddingModelRegistry {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	registry := &EmbeddingModelRegistry{
		models:        safe.NewStore[string, EmbeddingModel](),
		lazyModels:    safe.NewStore[string, *lazyModel](),
		configs:       safe.NewStore[string, EmbeddingModelConfig](),
		defaultModel:  config.DefaultModel,
		fallbackChain: config.FallbackChain,
		logger:        config.Logger,
		cache:         config.Cache,
	}

	// Load default configurations
	registry.loadDefaultConfigs()

	// Override with provided configs
	for name, cfg := range config.Configs {
		registry.configs.Put(name, cfg)
	}

	// Set default fallback chain if not provided
	if len(registry.fallbackChain) == 0 {
		registry.fallbackChain = []string{"openai-3-small", "bge-m3", "all-mpnet-base-v2", "local-fallback"}
	}

	// Set default model if not provided
	if registry.defaultModel == "" {
		if len(registry.fallbackChain) > 0 {
			registry.defaultModel = registry.fallbackChain[0]
		} else {
			registry.defaultModel = "local-fallback"
		}
	}

	return registry
}

// loadDefaultConfigs loads default model configurations.
func (r *EmbeddingModelRegistry) loadDefaultConfigs() {
	// OpenAI models
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		r.configs.Put("openai-ada", EmbeddingModelConfig{
			Name:       "openai-ada",
			Provider:   "openai",
			ModelID:    "text-embedding-ada-002",
			Dimensions: 1536,
			MaxTokens:  8191,
			BatchSize:  100,
			Timeout:    30 * time.Second,
			APIKey:     apiKey,
		})
		r.configs.Put("openai-3-small", EmbeddingModelConfig{
			Name:       "openai-3-small",
			Provider:   "openai",
			ModelID:    "text-embedding-3-small",
			Dimensions: 1536,
			MaxTokens:  8191,
			BatchSize:  100,
			Timeout:    30 * time.Second,
			APIKey:     apiKey,
		})
		r.configs.Put("openai-3-large", EmbeddingModelConfig{
			Name:       "openai-3-large",
			Provider:   "openai",
			ModelID:    "text-embedding-3-large",
			Dimensions: 3072,
			MaxTokens:  8191,
			BatchSize:  100,
			Timeout:    30 * time.Second,
			APIKey:     apiKey,
		})
	}

	// Ollama models
	if url := os.Getenv("OLLAMA_URL"); url != "" {
		r.configs.Put("nomic-embed-text", EmbeddingModelConfig{
			Name:       "nomic-embed-text",
			Provider:   "ollama",
			ModelID:    "nomic-embed-text",
			Dimensions: 768,
			MaxTokens:  8192,
			BatchSize:  32,
			Timeout:    60 * time.Second,
			BaseURL:    url,
		})
		r.configs.Put("bge-m3", EmbeddingModelConfig{
			Name:       "bge-m3",
			Provider:   "ollama",
			ModelID:    "bge-m3",
			Dimensions: 1024,
			MaxTokens:  8192,
			BatchSize:  32,
			Timeout:    60 * time.Second,
			BaseURL:    url,
		})
		r.configs.Put("mxbai-embed-large", EmbeddingModelConfig{
			Name:       "mxbai-embed-large",
			Provider:   "ollama",
			ModelID:    "mxbai-embed-large",
			Dimensions: 1024,
			MaxTokens:  512,
			BatchSize:  32,
			Timeout:    60 * time.Second,
			BaseURL:    url,
		})
	}

	// Sentence Transformers (if service is running)
	if url := os.Getenv("SENTENCE_TRANSFORMERS_URL"); url != "" {
		r.configs.Put("all-mpnet-base-v2", EmbeddingModelConfig{
			Name:       "all-mpnet-base-v2",
			Provider:   "sentence-transformers",
			ModelID:    "all-mpnet-base-v2",
			Dimensions: 768,
			MaxTokens:  512,
			BatchSize:  64,
			Timeout:    30 * time.Second,
			BaseURL:    url,
		})
		r.configs.Put("all-minilm-l6-v2", EmbeddingModelConfig{
			Name:       "all-minilm-l6-v2",
			Provider:   "sentence-transformers",
			ModelID:    "all-MiniLM-L6-v2",
			Dimensions: 384,
			MaxTokens:  256,
			BatchSize:  128,
			Timeout:    30 * time.Second,
			BaseURL:    url,
		})
	}

	// Local fallback (always available)
	r.configs.Put("local-fallback", EmbeddingModelConfig{
		Name:       "local-fallback",
		Provider:   "local",
		Dimensions: 1536,
		MaxTokens:  100000,
		BatchSize:  1000,
		Timeout:    1 * time.Second,
	})
}

// Register registers a new embedding model (eagerly, immediately available).
func (r *EmbeddingModelRegistry) Register(name string, model EmbeddingModel) error {
	r.models.Put(name, model)
	r.lazyModels.Delete(name) // remove any lazy registration for the same name
	r.logger.WithField("model", name).Info("Embedding model registered")
	return nil
}

// RegisterLazy registers a factory for deferred model initialization.
// The factory is called at most once, when the model is first accessed
// via Get or GetOrCreate. This avoids initialization overhead for models
// that may never be used during a session.
func (r *EmbeddingModelRegistry) RegisterLazy(name string, factory func() (EmbeddingModel, error)) {
	r.lazyModels.Put(name, &lazyModel{factory: factory})
	r.models.Delete(name) // remove any eager registration for the same name
	r.logger.WithField("model", name).Info("Embedding model registered (lazy)")
}

// Get returns an embedding model by name. It checks eagerly registered
// models first, then lazy models (triggering initialization), and finally
// falls back to config-based creation.
func (r *EmbeddingModelRegistry) Get(name string) (EmbeddingModel, error) {
	// Check eagerly registered models
	if model, exists := r.models.Get(name); exists {
		return model, nil
	}
	// Check lazily registered models
	if lm, exists := r.lazyModels.Get(name); exists {
		model, err := lm.get()
		if err != nil {
			return nil, fmt.Errorf("lazy model init failed for %s: %w", name, err)
		}
		return model, nil
	}

	return r.GetOrCreate(name)
}

// GetOrCreate gets or creates an embedding model from config.
func (r *EmbeddingModelRegistry) GetOrCreate(name string) (EmbeddingModel, error) {
	// Check already-registered eager models.
	if model, exists := r.models.Get(name); exists {
		return model, nil
	}

	// Check lazy models.
	if lm, exists := r.lazyModels.Get(name); exists {
		model, err := lm.get()
		if err != nil {
			return nil, fmt.Errorf("lazy model init failed for %s: %w", name, err)
		}
		return model, nil
	}

	config, exists := r.configs.Get(name)
	if !exists {
		return nil, fmt.Errorf("embedding model not configured: %s", name)
	}

	model, err := r.createModel(config)
	if err != nil {
		return nil, err
	}

	// PutIfAbsent guards against a race where two goroutines
	// both pass the Get check; the first publisher wins and the
	// loser's model is discarded.
	if existing, loaded := r.models.PutIfAbsent(name, model); loaded {
		return existing, nil
	}

	r.logger.WithField("model", name).Info("Embedding model created")
	return model, nil
}

// GetDefault returns the default embedding model.
func (r *EmbeddingModelRegistry) GetDefault() (EmbeddingModel, error) {
	return r.Get(r.defaultModel)
}

// createModel creates an embedding model based on configuration.
func (r *EmbeddingModelRegistry) createModel(config EmbeddingModelConfig) (EmbeddingModel, error) {
	switch config.Provider {
	case "openai":
		return NewOpenAIEmbeddingModel(config), nil
	case "ollama":
		return NewOllamaEmbeddingModel(config), nil
	case "sentence-transformers":
		return NewSentenceTransformersModel(config), nil
	case "local":
		return NewLocalHashModel(config), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", config.Provider)
	}
}

// EncodeWithFallback encodes texts using the fallback chain.
func (r *EmbeddingModelRegistry) EncodeWithFallback(ctx context.Context, texts []string) ([][]float32, string, error) {
	for _, modelName := range r.fallbackChain {
		model, err := r.GetOrCreate(modelName)
		if err != nil {
			r.logger.WithError(err).WithField("model", modelName).Debug("Failed to get model, trying next")
			continue
		}

		embeddings, err := model.Encode(ctx, texts)
		if err != nil {
			r.logger.WithError(err).WithField("model", modelName).Debug("Failed to encode, trying next")
			continue
		}

		return embeddings, modelName, nil
	}

	return nil, "", fmt.Errorf("all embedding models in fallback chain failed")
}

// EncodeSingleWithFallback encodes a single text using the fallback chain.
func (r *EmbeddingModelRegistry) EncodeSingleWithFallback(ctx context.Context, text string) ([]float32, string, error) {
	embeddings, modelName, err := r.EncodeWithFallback(ctx, []string{text})
	if err != nil {
		return nil, "", err
	}
	return embeddings[0], modelName, nil
}

// List returns all configured model names (config-based, eager, and lazy).
func (r *EmbeddingModelRegistry) List() []string {
	seen := make(map[string]struct{})
	names := make([]string, 0, r.configs.Len()+r.lazyModels.Len())
	r.configs.Range(func(name string, _ EmbeddingModelConfig) bool {
		names = append(names, name)
		seen[name] = struct{}{}
		return true
	})
	r.lazyModels.Range(func(name string, _ *lazyModel) bool {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
		return true
	})
	return names
}

// Health checks the health of all models.
func (r *EmbeddingModelRegistry) Health(ctx context.Context) map[string]error {
	results := make(map[string]error)
	for name, model := range r.models.Snapshot() {
		results[name] = model.Health(ctx)
	}
	return results
}

// Close closes all models (both eagerly and lazily initialized).
func (r *EmbeddingModelRegistry) Close() error {
	var lastErr error
	for name, model := range r.models.Snapshot() {
		if err := model.Close(); err != nil {
			r.logger.WithError(err).WithField("model", name).Warn("Error closing model")
			lastErr = err
		}
	}
	// Close any lazily initialized models
	for name, lm := range r.lazyModels.Snapshot() {
		if lm.model != nil {
			if err := lm.model.Close(); err != nil {
				r.logger.WithError(err).WithField("model", name).Warn("Error closing lazy model")
				lastErr = err
			}
		}
	}
	r.models.Clear()
	r.lazyModels.Clear()
	return lastErr
}

// =============================================================================
// OpenAI Embedding Model
// =============================================================================

// OpenAIEmbeddingModel implements EmbeddingModel for OpenAI.
type OpenAIEmbeddingModel struct {
	config     EmbeddingModelConfig
	httpClient *http.Client
}

// NewOpenAIEmbeddingModel creates a new OpenAI embedding model.
func NewOpenAIEmbeddingModel(config EmbeddingModelConfig) *OpenAIEmbeddingModel {
	return &OpenAIEmbeddingModel{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

func (m *OpenAIEmbeddingModel) Name() string     { return m.config.Name }
func (m *OpenAIEmbeddingModel) Dimensions() int  { return m.config.Dimensions }
func (m *OpenAIEmbeddingModel) MaxTokens() int   { return m.config.MaxTokens }
func (m *OpenAIEmbeddingModel) Provider() string { return "openai" }
func (m *OpenAIEmbeddingModel) Close() error     { return nil }

func (m *OpenAIEmbeddingModel) Health(ctx context.Context) error {
	_, err := m.EncodeSingle(ctx, "health check")
	return err
}

func (m *OpenAIEmbeddingModel) EncodeSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := m.Encode(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

func (m *OpenAIEmbeddingModel) Encode(ctx context.Context, texts []string) ([][]float32, error) {
	body := map[string]interface{}{
		"input": texts,
		"model": m.config.ModelID,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.config.APIKey)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("API error: status %d, failed to read error body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	embeddings := make([][]float32, len(texts))
	for _, item := range response.Data {
		embeddings[item.Index] = item.Embedding
	}

	return embeddings, nil
}

// =============================================================================
// Ollama Embedding Model
// =============================================================================

// OllamaEmbeddingModel implements EmbeddingModel for Ollama.
type OllamaEmbeddingModel struct {
	config     EmbeddingModelConfig
	httpClient *http.Client
}

// NewOllamaEmbeddingModel creates a new Ollama embedding model.
func NewOllamaEmbeddingModel(config EmbeddingModelConfig) *OllamaEmbeddingModel {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}
	return &OllamaEmbeddingModel{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

func (m *OllamaEmbeddingModel) Name() string     { return m.config.Name }
func (m *OllamaEmbeddingModel) Dimensions() int  { return m.config.Dimensions }
func (m *OllamaEmbeddingModel) MaxTokens() int   { return m.config.MaxTokens }
func (m *OllamaEmbeddingModel) Provider() string { return "ollama" }
func (m *OllamaEmbeddingModel) Close() error     { return nil }

func (m *OllamaEmbeddingModel) Health(ctx context.Context) error {
	_, err := m.EncodeSingle(ctx, "health check")
	return err
}

func (m *OllamaEmbeddingModel) EncodeSingle(ctx context.Context, text string) ([]float32, error) {
	body := map[string]interface{}{
		"model":  m.config.ModelID,
		"prompt": text,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.config.BaseURL+"/api/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("API error: status %d, failed to read error body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Embedding []float32 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Embedding, nil
}

func (m *OllamaEmbeddingModel) Encode(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := m.EncodeSingle(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to encode text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

// =============================================================================
// Sentence Transformers Model
// =============================================================================

// SentenceTransformersModel implements EmbeddingModel for Sentence Transformers.
type SentenceTransformersModel struct {
	config     EmbeddingModelConfig
	httpClient *http.Client
}

// NewSentenceTransformersModel creates a new Sentence Transformers model.
func NewSentenceTransformersModel(config EmbeddingModelConfig) *SentenceTransformersModel {
	return &SentenceTransformersModel{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

func (m *SentenceTransformersModel) Name() string     { return m.config.Name }
func (m *SentenceTransformersModel) Dimensions() int  { return m.config.Dimensions }
func (m *SentenceTransformersModel) MaxTokens() int   { return m.config.MaxTokens }
func (m *SentenceTransformersModel) Provider() string { return "sentence-transformers" }
func (m *SentenceTransformersModel) Close() error     { return nil }

func (m *SentenceTransformersModel) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", m.config.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}
	return nil
}

func (m *SentenceTransformersModel) EncodeSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := m.Encode(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

func (m *SentenceTransformersModel) Encode(ctx context.Context, texts []string) ([][]float32, error) {
	body := map[string]interface{}{
		"texts": texts,
		"model": m.config.ModelID,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.config.BaseURL+"/encode", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("API error: status %d, failed to read error body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Embeddings [][]float32 `json:"embeddings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Embeddings, nil
}

// =============================================================================
// Local Hash Model (Fallback)
// =============================================================================

// LocalHashModel implements EmbeddingModel using local hash-based embeddings.
type LocalHashModel struct {
	config EmbeddingModelConfig
}

// NewLocalHashModel creates a new local hash embedding model.
func NewLocalHashModel(config EmbeddingModelConfig) *LocalHashModel {
	if config.Dimensions == 0 {
		config.Dimensions = 1536
	}
	return &LocalHashModel{config: config}
}

func (m *LocalHashModel) Name() string     { return m.config.Name }
func (m *LocalHashModel) Dimensions() int  { return m.config.Dimensions }
func (m *LocalHashModel) MaxTokens() int   { return m.config.MaxTokens }
func (m *LocalHashModel) Provider() string { return "local" }
func (m *LocalHashModel) Close() error     { return nil }

func (m *LocalHashModel) Health(ctx context.Context) error {
	return nil // Always healthy
}

func (m *LocalHashModel) EncodeSingle(ctx context.Context, text string) ([]float32, error) {
	return m.generateHashEmbedding(text), nil
}

func (m *LocalHashModel) Encode(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embeddings[i] = m.generateHashEmbedding(text)
	}
	return embeddings, nil
}

// generateHashEmbedding generates a deterministic embedding using SHA256.
func (m *LocalHashModel) generateHashEmbedding(text string) []float32 {
	hash := sha256.Sum256([]byte(text))
	embedding := make([]float32, m.config.Dimensions)

	// Use hash bytes to seed the embedding
	for i := 0; i < m.config.Dimensions; i++ {
		// Create a deterministic value from hash
		idx := i % len(hash)
		seed := uint32(hash[idx]) + uint32(i) // #nosec G115 - byte (0-255) and dimension index fit in uint32

		// Convert to float32 in range [-1, 1]
		seedBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(seedBytes, seed)
		val := float64(binary.LittleEndian.Uint32(seedBytes)) / float64(math.MaxUint32)
		embedding[i] = float32(val*2 - 1)
	}

	// Normalize to unit length
	var norm float32
	for _, v := range embedding {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding
}
