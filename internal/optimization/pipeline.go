package optimization

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/optimization/outlines"
	"github.com/superagent/superagent/internal/optimization/streaming"
)

// PipelineStage represents a stage in the optimization pipeline.
type PipelineStage string

const (
	StageCacheCheck      PipelineStage = "cache_check"
	StageContextRetrieval PipelineStage = "context_retrieval"
	StageTaskDecomposition PipelineStage = "task_decomposition"
	StagePrefixWarm      PipelineStage = "prefix_warm"
	StageValidation      PipelineStage = "validation"
	StageCacheStore      PipelineStage = "cache_store"
)

// PipelineResult contains the result of running through the optimization pipeline.
type PipelineResult struct {
	// Request optimization results
	CacheHit         bool                `json:"cache_hit"`
	CachedResponse   string              `json:"cached_response,omitempty"`
	RetrievedContext []string            `json:"retrieved_context,omitempty"`
	DecomposedTasks  []string            `json:"decomposed_tasks,omitempty"`
	PrefixWarmed     bool                `json:"prefix_warmed"`
	OptimizedPrompt  string              `json:"optimized_prompt"`

	// Response optimization results
	ValidationResult *outlines.ValidationResult `json:"validation_result,omitempty"`
	StructuredData   interface{}                `json:"structured_data,omitempty"`
	Cached           bool                       `json:"cached"`

	// Metrics
	StageTimings map[PipelineStage]time.Duration `json:"stage_timings"`
	TotalTime    time.Duration                   `json:"total_time"`
	StagesRun    []PipelineStage                 `json:"stages_run"`
}

// Pipeline orchestrates the optimization stages for requests and responses.
type Pipeline struct {
	service *Service
	config  *PipelineConfig
	metrics *Metrics
	mu      sync.RWMutex
}

// PipelineConfig configures the optimization pipeline behavior.
type PipelineConfig struct {
	// Enable/disable stages
	EnableCacheCheck       bool `yaml:"enable_cache_check" json:"enable_cache_check"`
	EnableContextRetrieval bool `yaml:"enable_context_retrieval" json:"enable_context_retrieval"`
	EnableTaskDecomposition bool `yaml:"enable_task_decomposition" json:"enable_task_decomposition"`
	EnablePrefixWarm       bool `yaml:"enable_prefix_warm" json:"enable_prefix_warm"`
	EnableValidation       bool `yaml:"enable_validation" json:"enable_validation"`
	EnableCacheStore       bool `yaml:"enable_cache_store" json:"enable_cache_store"`

	// Stage timeouts
	CacheCheckTimeout       time.Duration `yaml:"cache_check_timeout" json:"cache_check_timeout"`
	ContextRetrievalTimeout time.Duration `yaml:"context_retrieval_timeout" json:"context_retrieval_timeout"`
	TaskDecompositionTimeout time.Duration `yaml:"task_decomposition_timeout" json:"task_decomposition_timeout"`

	// Thresholds
	MinPromptLengthForContext     int `yaml:"min_prompt_length_for_context" json:"min_prompt_length_for_context"`
	MinPromptLengthForDecomposition int `yaml:"min_prompt_length_for_decomposition" json:"min_prompt_length_for_decomposition"`

	// Parallelization
	ParallelStages bool `yaml:"parallel_stages" json:"parallel_stages"`
}

// DefaultPipelineConfig returns the default pipeline configuration.
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		EnableCacheCheck:       true,
		EnableContextRetrieval: true,
		EnableTaskDecomposition: true,
		EnablePrefixWarm:       true,
		EnableValidation:       true,
		EnableCacheStore:       true,

		CacheCheckTimeout:        100 * time.Millisecond,
		ContextRetrievalTimeout:  2 * time.Second,
		TaskDecompositionTimeout: 3 * time.Second,

		MinPromptLengthForContext:     50,
		MinPromptLengthForDecomposition: 100,

		ParallelStages: true,
	}
}

// NewPipeline creates a new optimization pipeline.
func NewPipeline(service *Service, config *PipelineConfig) *Pipeline {
	if config == nil {
		config = DefaultPipelineConfig()
	}
	return &Pipeline{
		service: service,
		config:  config,
		metrics: GetMetrics(),
	}
}

// OptimizeRequest runs the request through the optimization pipeline.
func (p *Pipeline) OptimizeRequest(ctx context.Context, prompt string, embedding []float64) (*PipelineResult, error) {
	startTime := time.Now()
	result := &PipelineResult{
		OptimizedPrompt: prompt,
		StageTimings:    make(map[PipelineStage]time.Duration),
		StagesRun:       []PipelineStage{},
	}

	// Stage 1: Cache Check
	if p.config.EnableCacheCheck && p.service.semanticCache != nil && len(embedding) > 0 {
		stageStart := time.Now()
		cacheCtx, cancel := context.WithTimeout(ctx, p.config.CacheCheckTimeout)
		hit, err := p.service.semanticCache.Get(cacheCtx, embedding)
		cancel()

		result.StageTimings[StageCacheCheck] = time.Since(stageStart)
		result.StagesRun = append(result.StagesRun, StageCacheCheck)

		if err == nil && hit != nil && hit.Entry != nil {
			result.CacheHit = true
			result.CachedResponse = hit.Entry.Response
			p.metrics.RecordCacheHit(time.Since(stageStart))
			result.TotalTime = time.Since(startTime)
			return result, nil
		}
		p.metrics.RecordCacheMiss(time.Since(stageStart))
	}

	// Parallel stages if enabled
	if p.config.ParallelStages {
		p.runParallelStages(ctx, prompt, result)
	} else {
		p.runSequentialStages(ctx, prompt, result)
	}

	result.TotalTime = time.Since(startTime)
	p.metrics.RecordOptimization(true, result.TotalTime)
	return result, nil
}

// runParallelStages runs independent stages in parallel.
func (p *Pipeline) runParallelStages(ctx context.Context, prompt string, result *PipelineResult) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Stage 2: Context Retrieval (parallel)
	if p.config.EnableContextRetrieval && p.service.llamaindexClient != nil &&
		len(prompt) >= p.config.MinPromptLengthForContext {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stageStart := time.Now()
			stageCtx, cancel := context.WithTimeout(ctx, p.config.ContextRetrievalTimeout)
			defer cancel()

			contexts, err := p.retrieveContext(stageCtx, prompt)
			mu.Lock()
			result.StageTimings[StageContextRetrieval] = time.Since(stageStart)
			result.StagesRun = append(result.StagesRun, StageContextRetrieval)
			if err == nil && len(contexts) > 0 {
				result.RetrievedContext = contexts
				p.metrics.ContextRetrieved.Inc()
			}
			mu.Unlock()
		}()
	}

	// Stage 3: Task Decomposition (parallel)
	if p.config.EnableTaskDecomposition && p.service.langchainClient != nil &&
		len(prompt) >= p.config.MinPromptLengthForDecomposition && isComplexTask(prompt) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stageStart := time.Now()
			stageCtx, cancel := context.WithTimeout(ctx, p.config.TaskDecompositionTimeout)
			defer cancel()

			tasks, err := p.decomposeTask(stageCtx, prompt)
			mu.Lock()
			result.StageTimings[StageTaskDecomposition] = time.Since(stageStart)
			result.StagesRun = append(result.StagesRun, StageTaskDecomposition)
			if err == nil && len(tasks) > 0 {
				result.DecomposedTasks = tasks
				p.metrics.TasksDecomposed.Inc()
			}
			mu.Unlock()
		}()
	}

	// Stage 4: Prefix Warm (parallel)
	if p.config.EnablePrefixWarm && p.service.sglangClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stageStart := time.Now()
			_, err := p.service.sglangClient.WarmPrefix(ctx, prompt[:min(500, len(prompt))])
			mu.Lock()
			result.StageTimings[StagePrefixWarm] = time.Since(stageStart)
			result.StagesRun = append(result.StagesRun, StagePrefixWarm)
			if err == nil {
				result.PrefixWarmed = true
				p.metrics.PrefixesWarmed.Inc()
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Build optimized prompt from results
	p.buildOptimizedPrompt(prompt, result)
}

// runSequentialStages runs stages sequentially.
func (p *Pipeline) runSequentialStages(ctx context.Context, prompt string, result *PipelineResult) {
	// Stage 2: Context Retrieval
	if p.config.EnableContextRetrieval && p.service.llamaindexClient != nil &&
		len(prompt) >= p.config.MinPromptLengthForContext {
		stageStart := time.Now()
		stageCtx, cancel := context.WithTimeout(ctx, p.config.ContextRetrievalTimeout)
		contexts, err := p.retrieveContext(stageCtx, prompt)
		cancel()

		result.StageTimings[StageContextRetrieval] = time.Since(stageStart)
		result.StagesRun = append(result.StagesRun, StageContextRetrieval)
		if err == nil && len(contexts) > 0 {
			result.RetrievedContext = contexts
			p.metrics.ContextRetrieved.Inc()
		}
	}

	// Stage 3: Task Decomposition
	if p.config.EnableTaskDecomposition && p.service.langchainClient != nil &&
		len(prompt) >= p.config.MinPromptLengthForDecomposition && isComplexTask(prompt) {
		stageStart := time.Now()
		stageCtx, cancel := context.WithTimeout(ctx, p.config.TaskDecompositionTimeout)
		tasks, err := p.decomposeTask(stageCtx, prompt)
		cancel()

		result.StageTimings[StageTaskDecomposition] = time.Since(stageStart)
		result.StagesRun = append(result.StagesRun, StageTaskDecomposition)
		if err == nil && len(tasks) > 0 {
			result.DecomposedTasks = tasks
			p.metrics.TasksDecomposed.Inc()
		}
	}

	// Stage 4: Prefix Warm
	if p.config.EnablePrefixWarm && p.service.sglangClient != nil {
		stageStart := time.Now()
		_, err := p.service.sglangClient.WarmPrefix(ctx, prompt[:min(500, len(prompt))])
		result.StageTimings[StagePrefixWarm] = time.Since(stageStart)
		result.StagesRun = append(result.StagesRun, StagePrefixWarm)
		if err == nil {
			result.PrefixWarmed = true
			p.metrics.PrefixesWarmed.Inc()
		}
	}

	// Build optimized prompt from results
	p.buildOptimizedPrompt(prompt, result)
}

// buildOptimizedPrompt constructs the optimized prompt from pipeline results.
func (p *Pipeline) buildOptimizedPrompt(originalPrompt string, result *PipelineResult) {
	optimized := originalPrompt

	// Add retrieved context
	if len(result.RetrievedContext) > 0 {
		contextStr := "Relevant context:\n"
		for i, ctx := range result.RetrievedContext {
			contextStr += fmt.Sprintf("[Context %d]: %s\n", i+1, ctx)
		}
		optimized = contextStr + "\nQuestion: " + originalPrompt
	}

	result.OptimizedPrompt = optimized
}

// retrieveContext retrieves relevant context from documents.
func (p *Pipeline) retrieveContext(ctx context.Context, prompt string) ([]string, error) {
	if p.service.llamaindexClient == nil {
		return nil, fmt.Errorf("llamaindex client not available")
	}

	resp, err := p.service.llamaindexClient.Query(ctx, &llamaindexQueryRequest{
		Query:     prompt,
		TopK:      5,
		UseCognee: p.service.config.LlamaIndex.UseCogneeIndex,
		Rerank:    true,
	})
	if err != nil {
		return nil, err
	}

	var contexts []string
	for _, source := range resp.Sources {
		contexts = append(contexts, source.Content)
	}
	return contexts, nil
}

// decomposeTask decomposes a complex task into subtasks.
func (p *Pipeline) decomposeTask(ctx context.Context, task string) ([]string, error) {
	if p.service.langchainClient == nil {
		return nil, fmt.Errorf("langchain client not available")
	}

	resp, err := p.service.langchainClient.Decompose(ctx, &langchainDecomposeRequest{
		Task:     task,
		MaxSteps: 5,
	})
	if err != nil {
		return nil, err
	}

	var tasks []string
	for _, subtask := range resp.Subtasks {
		tasks = append(tasks, subtask.Description)
	}
	return tasks, nil
}

// OptimizeResponse runs the response through the optimization pipeline.
func (p *Pipeline) OptimizeResponse(ctx context.Context, response string, embedding []float64, query string, schema *outlines.JSONSchema) (*PipelineResult, error) {
	startTime := time.Now()
	result := &PipelineResult{
		StageTimings: make(map[PipelineStage]time.Duration),
		StagesRun:    []PipelineStage{},
	}

	// Stage 5: Validation
	if p.config.EnableValidation && schema != nil {
		stageStart := time.Now()
		validator, err := outlines.NewSchemaValidator(schema)
		if err == nil {
			validationResult := validator.Validate(response)
			result.ValidationResult = validationResult
			if validationResult.Valid {
				result.StructuredData = validationResult.Data
			}
			p.metrics.RecordValidation(validationResult.Valid, time.Since(stageStart))
		}
		result.StageTimings[StageValidation] = time.Since(stageStart)
		result.StagesRun = append(result.StagesRun, StageValidation)
	}

	// Stage 6: Cache Store
	if p.config.EnableCacheStore && p.service.semanticCache != nil && len(embedding) > 0 {
		stageStart := time.Now()
		_, err := p.service.semanticCache.Set(ctx, query, response, embedding, nil)
		result.StageTimings[StageCacheStore] = time.Since(stageStart)
		result.StagesRun = append(result.StagesRun, StageCacheStore)
		result.Cached = err == nil
	}

	result.TotalTime = time.Since(startTime)
	p.metrics.RecordOptimization(false, result.TotalTime)
	return result, nil
}

// StreamWithPipeline wraps a stream with pipeline optimizations.
func (p *Pipeline) StreamWithPipeline(ctx context.Context, stream <-chan *streaming.StreamChunk, progress streaming.ProgressCallback) (<-chan *streaming.StreamChunk, func() *streaming.AggregatedStream) {
	p.metrics.StreamsStarted.Inc()
	startTime := time.Now()

	out, getResult := p.service.StreamEnhanced(ctx, stream, progress)

	// Wrap getResult to record metrics
	wrappedGetResult := func() *streaming.AggregatedStream {
		result := getResult()
		if result != nil {
			p.metrics.RecordStreamComplete(time.Since(startTime), result.TokenCount)
		}
		return result
	}

	return out, wrappedGetResult
}

// GetConfig returns the pipeline configuration.
func (p *Pipeline) GetConfig() *PipelineConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// SetConfig updates the pipeline configuration.
func (p *Pipeline) SetConfig(config *PipelineConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
}

// Internal request types to avoid import cycles
type llamaindexQueryRequest struct {
	Query     string
	TopK      int
	UseCognee bool
	Rerank    bool
}

type langchainDecomposeRequest struct {
	Task     string
	MaxSteps int
}
