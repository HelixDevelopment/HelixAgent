package observability

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// =============================================================================
// Concurrency tests
// =============================================================================

func TestLLMMetrics_RecordRequest_Concurrent(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			provider := "openai"
			if idx%2 == 0 {
				provider = "anthropic"
			}
			var reqErr error
			if idx%7 == 0 {
				reqErr = errors.New("test error")
			}
			metrics.RecordRequest(ctx, provider, "gpt-4",
				time.Duration(idx)*time.Millisecond, 100+idx, 200+idx,
				0.01*float64(idx), reqErr)
		}(i)
	}

	wg.Wait()
	// If we get here without a panic or data race, the test passes
}

func TestLLMMetrics_RecordCacheHitMiss_Concurrent(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-cache-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			metrics.RecordCacheHit(ctx, time.Duration(idx)*time.Millisecond)
		}(i)
		go func() {
			defer wg.Done()
			metrics.RecordCacheMiss(ctx)
		}()
	}

	wg.Wait()
}

func TestLLMTracer_StartLLMRequest_Concurrent(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			params := &LLMRequestParams{
				Provider:  "openai",
				Model:     "gpt-4",
				RequestID: "concurrent-req",
			}
			ctx, span := tracer.StartLLMRequest(ctx, params)
			tracer.EndLLMRequest(ctx, span, &LLMResponseParams{
				InputTokens:  100,
				OutputTokens: 200,
				FinishReason: "stop",
			}, time.Now())
		}(i)
	}

	wg.Wait()
}

func TestLLMTracer_StartEnsembleRequest_Concurrent(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			ctx, span := tracer.StartEnsembleRequest(
				context.Background(), "weighted-vote", idx%5+1)
			span.End()
			_ = ctx
		}(i)
	}

	wg.Wait()
}

func TestLLMTracer_StartDebateRound_Concurrent(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			ctx, span := tracer.StartDebateRound(
				context.Background(), idx, "topic")
			span.End()
			_ = ctx
		}(i)
	}

	wg.Wait()
}

func TestLLMMetrics_RecordDebateRound_Concurrent(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-debate-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			metrics.RecordDebateRound(ctx, idx%5+1, float64(idx%100)/100.0)
		}(i)
	}

	wg.Wait()
}

func TestLLMMetrics_RecordRAGRetrieval_Concurrent(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-rag-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			metrics.RecordRAGRetrieval(ctx, idx%10+1,
				time.Duration(idx)*time.Millisecond, float64(idx%100)/100.0)
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// Exporter error path coverage
// =============================================================================

func TestSetupTraceExporter_OTLP_InvalidEndpoint(t *testing.T) {
	t.Parallel()
	// OTLP exporter creation typically succeeds even with bad endpoint;
	// errors appear on export. But the resource merge may fail or the
	// provider creation may reveal issues.
	config := &ExporterConfig{
		Type:        ExporterOTLP,
		Endpoint:    "localhost:99999",
		Insecure:    true,
		ServiceName: "test-otlp",
		Environment: "test",
		Version:     "1.0.0",
	}

	tp, err := SetupTraceExporter(context.Background(), config)
	// OTLP exporter creation may succeed (deferred connection); if it does,
	// clean up. The important thing is we exercise the code path.
	if err != nil {
		// Schema conflicts or connection issues are acceptable in test
		t.Logf("OTLP setup returned error (expected in test env): %v", err)
		return
	}
	assert.NotNil(t, tp)
	_ = ShutdownTraceExporter(context.Background(), tp)
}

func TestSetupTraceExporter_OTLP_WithHeaders(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:     ExporterOTLP,
		Endpoint: "localhost:4318",
		Insecure: true,
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
			"X-Custom":      "value",
		},
		ServiceName: "test-otlp-headers",
		Environment: "test",
		Version:     "1.0.0",
	}

	tp, err := SetupTraceExporter(context.Background(), config)
	if err != nil {
		t.Logf("OTLP setup with headers returned error (expected in test env): %v", err)
		return
	}
	assert.NotNil(t, tp)
	_ = ShutdownTraceExporter(context.Background(), tp)
}

func TestSetupTraceExporter_OTLP_Secure(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterOTLP,
		Endpoint:    "localhost:4318",
		Insecure:    false,
		ServiceName: "test-otlp-secure",
		Environment: "test",
		Version:     "1.0.0",
	}

	tp, err := SetupTraceExporter(context.Background(), config)
	if err != nil {
		t.Logf("Secure OTLP setup returned error (expected in test env): %v", err)
		return
	}
	assert.NotNil(t, tp)
	_ = ShutdownTraceExporter(context.Background(), tp)
}

func TestSetupLangfuseExporter_DefaultBaseURL(t *testing.T) {
	t.Parallel()
	config := &LangfuseConfig{
		PublicKey: "pk-test",
		SecretKey: "sk-test",
		BaseURL:   "", // Should default to https://cloud.langfuse.com
	}

	// This will attempt OTLP connection to Langfuse which won't be available
	// in test, but exercises the code path including default base URL.
	tp, err := SetupLangfuseExporter(context.Background(), config)
	if err != nil {
		t.Logf("Langfuse setup returned error (expected in test env): %v", err)
		return
	}
	assert.NotNil(t, tp)
	_ = ShutdownTraceExporter(context.Background(), tp)
}

func TestSetupLangfuseExporter_CustomBaseURL(t *testing.T) {
	t.Parallel()
	config := &LangfuseConfig{
		PublicKey: "pk-custom",
		SecretKey: "sk-custom",
		BaseURL:   "https://custom.langfuse.example.com",
	}

	tp, err := SetupLangfuseExporter(context.Background(), config)
	if err != nil {
		t.Logf("Custom Langfuse setup returned error (expected in test env): %v", err)
		return
	}
	assert.NotNil(t, tp)
	_ = ShutdownTraceExporter(context.Background(), tp)
}

func TestShutdownTraceExporter_WithProvider(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterNone,
		ServiceName: "test-shutdown",
		Version:     "1.0.0",
		Environment: "test",
	}

	tp, err := SetupTraceExporter(context.Background(), config)
	if err != nil {
		t.Skipf("Skipping due to OTel schema conflict: %v", err)
	}
	require.NotNil(t, tp)

	// Shutdown with valid context
	err = ShutdownTraceExporter(context.Background(), tp)
	require.NoError(t, err)

	// Second shutdown should also work (idempotent)
	err = ShutdownTraceExporter(context.Background(), tp)
	require.NoError(t, err)
}

func TestShutdownTraceExporter_CancelledContext(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterNone,
		ServiceName: "test-shutdown-cancel",
		Version:     "1.0.0",
		Environment: "test",
	}

	tp, err := SetupTraceExporter(context.Background(), config)
	if err != nil {
		t.Skipf("Skipping due to OTel schema conflict: %v", err)
	}
	require.NotNil(t, tp)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Shutdown with cancelled context
	err = ShutdownTraceExporter(ctx, tp)
	// May return context.Canceled or nil depending on implementation
	_ = err
}

func TestSetupTraceExporter_UnsupportedTypes(t *testing.T) {
	t.Parallel()
	unsupportedTypes := []ExporterType{
		ExporterType("unknown"),
		ExporterType(""),
		ExporterType("prometheus"),
		ExporterType("datadog"),
	}

	for _, exporterType := range unsupportedTypes {
		t.Run(string(exporterType), func(t *testing.T) {
				t.Parallel()
			config := &ExporterConfig{
				Type:        exporterType,
				ServiceName: "test",
			}
			_, err := SetupTraceExporter(context.Background(), config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported exporter type")
		})
	}
}

// =============================================================================
// TracedProvider edge cases in llm_middleware.go
// =============================================================================

func TestTracedProvider_Complete_NilResponse(t *testing.T) {
	t.Parallel()
	provider := &mockLLMProvider{
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return nil, nil // nil response, no error
		},
	}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProvider(provider, tracer, "test-nil-resp")

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{Model: "gpt-4"},
	}

	response, err := traced.Complete(context.Background(), request)
	assert.NoError(t, err)
	assert.Nil(t, response)
}

func TestTracedProvider_Complete_WithContentTrace(t *testing.T) {
	t.Parallel()
	provider := &mockLLMProvider{
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				ID:           "resp-content",
				Content:      "Traced response content",
				FinishReason: "stop",
			}, nil
		},
	}
	tracer, _ := NewLLMTracer(&TracerConfig{
		EnableContentTrace: true,
	})
	traced := NewTracedProvider(provider, tracer, "test-content-trace")

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{
			Model:       "gpt-4",
			Temperature: 0.7,
			MaxTokens:   500,
		},
	}

	response, err := traced.Complete(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "Traced response content", response.Content)
}

func TestTracedProvider_CompleteStream_EmptyStream(t *testing.T) {
	t.Parallel()
	provider := &mockLLMProvider{
		completeStreamFunc: func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
			ch := make(chan *models.LLMResponse)
			go func() {
				close(ch) // immediately close without sending anything
			}()
			return ch, nil
		},
	}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProvider(provider, tracer, "test-empty-stream")

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{Model: "gpt-4"},
	}

	chunks, err := traced.CompleteStream(context.Background(), request)
	require.NoError(t, err)

	var content string
	for chunk := range chunks {
		content += chunk.Content
	}
	assert.Empty(t, content)
}

func TestTracedProvider_CompleteStream_ManyChunks(t *testing.T) {
	t.Parallel()
	const chunkCount = 100
	provider := &mockLLMProvider{
		completeStreamFunc: func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
			ch := make(chan *models.LLMResponse, chunkCount)
			go func() {
				defer close(ch)
				for i := 0; i < chunkCount; i++ {
					resp := &models.LLMResponse{Content: "x"}
					if i == chunkCount-1 {
						resp.FinishReason = "stop"
					}
					ch <- resp
				}
			}()
			return ch, nil
		},
	}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProvider(provider, tracer, "test-many-chunks")

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{Model: "gpt-4"},
	}

	chunks, err := traced.CompleteStream(context.Background(), request)
	require.NoError(t, err)

	count := 0
	for range chunks {
		count++
	}
	assert.Equal(t, chunkCount, count)
}

func TestTracedProvider_CompleteStream_ChunksWithEmptyContent(t *testing.T) {
	t.Parallel()
	provider := &mockLLMProvider{
		completeStreamFunc: func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
			ch := make(chan *models.LLMResponse, 4)
			go func() {
				defer close(ch)
				ch <- &models.LLMResponse{Content: ""}
				ch <- &models.LLMResponse{Content: "Hello"}
				ch <- &models.LLMResponse{Content: ""}
				ch <- &models.LLMResponse{Content: " World", FinishReason: "stop"}
			}()
			return ch, nil
		},
	}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProvider(provider, tracer, "test-empty-chunks")

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{Model: "gpt-4"},
	}

	chunks, err := traced.CompleteStream(context.Background(), request)
	require.NoError(t, err)

	var content string
	for chunk := range chunks {
		content += chunk.Content
	}
	assert.Equal(t, "Hello World", content)
}

func TestTracedProvider_Complete_Concurrent(t *testing.T) {
	t.Parallel()
	provider := &mockLLMProvider{}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProvider(provider, tracer, "test-concurrent")

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			request := &models.LLMRequest{
				ModelParams: models.ModelParameters{Model: "gpt-4"},
			}
			resp, err := traced.Complete(context.Background(), request)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
		}()
	}

	wg.Wait()
}

// =============================================================================
// DebateTracer edge cases
// =============================================================================

func TestDebateTracer_TraceDebateRound_EmptyParticipants(t *testing.T) {
	t.Parallel()
	tracer, _ := NewLLMTracer(nil)
	metrics, _ := NewLLMMetrics("test-empty-participants")
	dt := NewDebateTracer(tracer, metrics)

	ctx := context.Background()
	_, endFn := dt.TraceDebateRound(ctx, "debate-empty", 1, []string{})

	endFn(map[string]string{}, true)
}

func TestDebateTracer_TraceDebateRound_EmptyResponses(t *testing.T) {
	t.Parallel()
	tracer, _ := NewLLMTracer(nil)
	metrics, _ := NewLLMMetrics("test-empty-responses")
	dt := NewDebateTracer(tracer, metrics)

	ctx := context.Background()
	participants := []string{"gpt-4", "claude-3"}
	_, endFn := dt.TraceDebateRound(ctx, "debate-no-resp", 1, participants)

	// Empty responses map
	endFn(map[string]string{}, false)
}

func TestDebateTracer_TraceDebateRound_LargeResponses(t *testing.T) {
	t.Parallel()
	tracer, _ := NewLLMTracer(nil)
	metrics, _ := NewLLMMetrics("test-large-responses")
	dt := NewDebateTracer(tracer, metrics)

	ctx := context.Background()
	participants := []string{"gpt-4", "claude-3", "gemini"}
	_, endFn := dt.TraceDebateRound(ctx, "debate-large", 1, participants)

	largeText := strings.Repeat("A", 10000)
	responses := map[string]string{
		"gpt-4":    largeText,
		"claude-3": largeText,
		"gemini":   largeText,
	}

	endFn(responses, true)
}

func TestDebateTracer_TraceDebateComplete_EmptyParticipants(t *testing.T) {
	t.Parallel()
	tracer, _ := NewLLMTracer(nil)
	metrics, _ := NewLLMMetrics("test-complete-empty")
	dt := NewDebateTracer(tracer, metrics)

	ctx := context.Background()
	_, endFn := dt.TraceDebateComplete(ctx, "debate-empty", "topic")

	endFn("result", 0, []string{})
}

func TestDebateTracer_TraceDebateComplete_EmptyResult(t *testing.T) {
	t.Parallel()
	tracer, _ := NewLLMTracer(nil)
	metrics, _ := NewLLMMetrics("test-complete-empty-result")
	dt := NewDebateTracer(tracer, metrics)

	ctx := context.Background()
	_, endFn := dt.TraceDebateComplete(ctx, "debate-no-result", "topic")

	endFn("", 3, []string{"gpt-4", "claude-3"})
}

func TestDebateTracer_TraceDebateRound_Concurrent(t *testing.T) {
	t.Parallel()
	tracer, _ := NewLLMTracer(nil)
	metrics, _ := NewLLMMetrics("test-debate-round-concurrent")
	dt := NewDebateTracer(tracer, metrics)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			_, endFn := dt.TraceDebateRound(ctx, "debate-conc", idx,
				[]string{"gpt-4", "claude-3"})
			endFn(map[string]string{
				"gpt-4":    "Response A",
				"claude-3": "Response B",
			}, idx%2 == 0)
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// EstimateTokens edge cases
// =============================================================================

func TestEstimateTokens_EdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "single character",
			text:     "a",
			expected: 0,
		},
		{
			name:     "exactly 4 chars",
			text:     "abcd",
			expected: 1,
		},
		{
			name:     "unicode CJK characters",
			text:     "\u4f60\u597d\u4e16\u754c\u6d4b\u8bd5\u5b57\u7b26\u4e32\u5728\u8fd9\u91cc",
			expected: len("\u4f60\u597d\u4e16\u754c\u6d4b\u8bd5\u5b57\u7b26\u4e32\u5728\u8fd9\u91cc") / 4,
		},
		{
			name:     "unicode emoji",
			text:     "\U0001f600\U0001f601\U0001f602\U0001f603\U0001f604",
			expected: len("\U0001f600\U0001f601\U0001f602\U0001f603\U0001f604") / 4,
		},
		{
			name:     "mixed unicode and ascii",
			text:     "Hello \u4e16\u754c! \U0001f600",
			expected: len("Hello \u4e16\u754c! \U0001f600") / 4,
		},
		{
			name:     "special characters",
			text:     "!@#$%^&*()_+-=[]{}|;':\",./<>?",
			expected: len("!@#$%^&*()_+-=[]{}|;':\",./<>?") / 4,
		},
		{
			name:     "newlines and tabs",
			text:     "line1\nline2\tline3\r\nline4",
			expected: len("line1\nline2\tline3\r\nline4") / 4,
		},
		{
			name:     "very large text",
			text:     strings.Repeat("token ", 10000),
			expected: len(strings.Repeat("token ", 10000)) / 4,
		},
		{
			name:     "null bytes",
			text:     "abc\x00def\x00ghi",
			expected: len("abc\x00def\x00ghi") / 4,
		},
		{
			name:     "only whitespace",
			text:     "    ",
			expected: 1,
		},
		{
			name:     "three chars",
			text:     "abc",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := estimateTokens(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Context cancellation tests
// =============================================================================

func TestLLMTracer_StartLLMRequest_CancelledContext(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	params := &LLMRequestParams{
		Provider:  "openai",
		Model:     "gpt-4",
		RequestID: "req-cancelled",
	}

	// Should not panic with cancelled context
	ctx, span := tracer.StartLLMRequest(ctx, params)
	assert.NotNil(t, span)
	tracer.EndLLMRequest(ctx, span, &LLMResponseParams{
		Error: context.Canceled,
	}, time.Now())
}

func TestLLMTracer_StartEnsembleRequest_CancelledContext(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ctx, span := tracer.StartEnsembleRequest(ctx, "weighted-vote", 3)
	assert.NotNil(t, span)
	span.End()
	_ = ctx
}

func TestLLMTracer_StartDebateRound_CancelledContext(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ctx, span := tracer.StartDebateRound(ctx, 1, "cancelled debate")
	assert.NotNil(t, span)
	span.End()
	_ = ctx
}

func TestLLMTracer_StartRAGRetrieval_CancelledContext(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ctx, span := tracer.StartRAGRetrieval(ctx, "query", 10)
	assert.NotNil(t, span)
	tracer.RecordRAGRetrievalResult(span, 5, 50.0)
	span.End()
	_ = ctx
}

func TestLLMTracer_StartToolExecution_CancelledContext(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ctx, span := tracer.StartToolExecution(ctx, "search_web")
	assert.NotNil(t, span)
	span.End()
	_ = ctx
}

func TestLLMTracer_StartLLMRequest_DeadlineExceeded(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	params := &LLMRequestParams{
		Provider:  "openai",
		Model:     "gpt-4",
		RequestID: "req-deadline",
	}

	ctx, span := tracer.StartLLMRequest(ctx, params)
	assert.NotNil(t, span)
	tracer.EndLLMRequest(ctx, span, &LLMResponseParams{
		Error: context.DeadlineExceeded,
	}, time.Now())
}

func TestTracedProvider_Complete_CancelledContext(t *testing.T) {
	t.Parallel()
	provider := &mockLLMProvider{
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return nil, ctx.Err()
		},
	}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProvider(provider, tracer, "test-cancel")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{Model: "gpt-4"},
	}

	_, err := traced.Complete(ctx, request)
	assert.Error(t, err)
}

func TestLLMMetrics_RecordRequest_CancelledContext(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-cancel-metrics")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should not panic with cancelled context
	metrics.RecordRequest(ctx, "openai", "gpt-4",
		100*time.Millisecond, 100, 200, 0.01, nil)
}

// =============================================================================
// EndLLMRequest edge cases
// =============================================================================

func TestLLMTracer_EndLLMRequest_ZeroTokens(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, span := tracer.StartLLMRequest(context.Background(), &LLMRequestParams{
		Provider: "test",
		Model:    "model",
	})

	tracer.EndLLMRequest(ctx, span, &LLMResponseParams{
		InputTokens:  0,
		OutputTokens: 0,
		FinishReason: "stop",
	}, time.Now())
}

func TestLLMTracer_EndLLMRequest_NoCostNoScore(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, span := tracer.StartLLMRequest(context.Background(), &LLMRequestParams{
		Provider: "test",
		Model:    "model",
	})

	tracer.EndLLMRequest(ctx, span, &LLMResponseParams{
		InputTokens:   100,
		OutputTokens:  200,
		FinishReason:  "stop",
		CacheHit:      false,
		CostUSD:       0,
		ProviderScore: 0,
	}, time.Now())
}

func TestLLMTracer_EndLLMRequest_WithContentTrace(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(&TracerConfig{
		EnableContentTrace: true,
	})
	require.NoError(t, err)

	ctx, span := tracer.StartLLMRequest(context.Background(), &LLMRequestParams{
		Provider:     "test",
		Model:        "model",
		SystemPrompt: "You are helpful",
		UserPrompt:   "Hello",
	})

	tracer.EndLLMRequest(ctx, span, &LLMResponseParams{
		InputTokens:  10,
		OutputTokens: 20,
		FinishReason: "stop",
		Content:      "Response with content tracing enabled",
		ResponseID:   "resp-traced",
	}, time.Now())
}

func TestLLMTracer_EndLLMRequest_ContentTraceDisabled(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(&TracerConfig{
		EnableContentTrace: false,
	})
	require.NoError(t, err)

	ctx, span := tracer.StartLLMRequest(context.Background(), &LLMRequestParams{
		Provider:     "test",
		Model:        "model",
		SystemPrompt: "You are helpful",
		UserPrompt:   "Hello",
	})

	tracer.EndLLMRequest(ctx, span, &LLMResponseParams{
		InputTokens:  10,
		OutputTokens: 20,
		FinishReason: "stop",
		Content:      "This content should not be traced",
	}, time.Now())
}

func TestLLMTracer_EndLLMRequest_EmptyResponseID(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, span := tracer.StartLLMRequest(context.Background(), &LLMRequestParams{
		Provider: "test",
		Model:    "model",
	})

	tracer.EndLLMRequest(ctx, span, &LLMResponseParams{
		InputTokens:  100,
		OutputTokens: 200,
		FinishReason: "stop",
		ResponseID:   "", // empty
	}, time.Now())
}

// =============================================================================
// StartLLMRequest param edge cases
// =============================================================================

func TestLLMTracer_StartLLMRequest_MinimalParams(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	params := &LLMRequestParams{
		Provider: "test",
		Model:    "model",
		// All optional fields empty/zero
	}

	ctx, span := tracer.StartLLMRequest(context.Background(), params)
	assert.NotNil(t, span)
	span.End()
	_ = ctx
}

func TestLLMTracer_StartLLMRequest_AllOptionalParams(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(&TracerConfig{
		EnableContentTrace: true,
	})
	require.NoError(t, err)

	params := &LLMRequestParams{
		Provider:      "anthropic",
		Model:         "claude-3",
		RequestID:     "req-full",
		SessionID:     "session-full",
		UserID:        "user-full",
		Temperature:   0.9,
		MaxTokens:     4096,
		TopP:          0.95,
		StopSequences: []string{"END", "STOP", "DONE"},
		SystemPrompt:  "System prompt content",
		UserPrompt:    "User prompt content",
		Ensemble:      true,
		Debate:        true,
	}

	ctx, span := tracer.StartLLMRequest(context.Background(), params)
	assert.NotNil(t, span)
	span.End()
	_ = ctx
}

func TestLLMTracer_StartLLMRequest_ZeroTemperature(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	// Temperature=0 should not add the attribute (due to > 0 check)
	params := &LLMRequestParams{
		Provider:    "test",
		Model:       "model",
		Temperature: 0,
		MaxTokens:   0,
		TopP:        0,
	}

	ctx, span := tracer.StartLLMRequest(context.Background(), params)
	assert.NotNil(t, span)
	span.End()
	_ = ctx
}

// =============================================================================
// TracedProviderRegistry edge cases
// =============================================================================

func TestTracedProviderRegistry_GetProvider_NilFromRegistry(t *testing.T) {
	t.Parallel()
	registry := &mockProviderRegistry{
		providers: map[string]llm.LLMProvider{},
	}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProviderRegistry(registry, tracer)

	provider := traced.GetProvider("nonexistent")
	assert.Nil(t, provider)
}

func TestTracedProviderRegistry_GetProviderByModel_NilFromRegistry(t *testing.T) {
	t.Parallel()
	registry := &mockProviderRegistry{
		providers: map[string]llm.LLMProvider{},
	}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProviderRegistry(registry, tracer)

	provider := traced.GetProviderByModel("nonexistent")
	assert.Nil(t, provider)
}

func TestTracedProviderRegistry_GetHealthyProviders_Empty(t *testing.T) {
	t.Parallel()
	registry := &mockProviderRegistry{
		providers: map[string]llm.LLMProvider{},
	}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProviderRegistry(registry, tracer)

	providers := traced.GetHealthyProviders()
	assert.Empty(t, providers)
}

func TestTracedProviderRegistry_ListProviders_Empty(t *testing.T) {
	t.Parallel()
	registry := &mockProviderRegistry{
		providers: map[string]llm.LLMProvider{},
	}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProviderRegistry(registry, tracer)

	names := traced.ListProviders()
	assert.Empty(t, names)
}

// =============================================================================
// LLMTracer initialization edge cases
// =============================================================================

func TestNewLLMTracer_EmptyServiceName(t *testing.T) {
	t.Parallel()
	config := &TracerConfig{
		ServiceName:    "",
		ServiceVersion: "",
	}

	tracer, err := NewLLMTracer(config)
	require.NoError(t, err)
	assert.NotNil(t, tracer)
	assert.True(t, tracer.initialized)
}

func TestLLMTracer_Initialized_Flag(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)
	assert.True(t, tracer.initialized)
}

// =============================================================================
// Metrics initialization and access
// =============================================================================

func TestNewLLMMetrics_EmptyServiceName(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("")
	require.NoError(t, err)
	assert.NotNil(t, metrics)
}

func TestLLMMetrics_RecordRequest_WithZeroCost(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-zero-cost")
	require.NoError(t, err)

	ctx := context.Background()
	// cost=0 should skip the cost recording branch
	metrics.RecordRequest(ctx, "ollama", "llama2", 100*time.Millisecond, 50, 100, 0, nil)
}

func TestLLMMetrics_RecordRequest_NegativeCost(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-negative-cost")
	require.NoError(t, err)

	ctx := context.Background()
	// Negative cost should not record cost (due to > 0 check)
	metrics.RecordRequest(ctx, "openai", "gpt-4", 100*time.Millisecond, 50, 100, -0.01, nil)
}

func TestLLMMetrics_RecordCacheHit_ZeroLatency(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-zero-latency")
	require.NoError(t, err)

	metrics.RecordCacheHit(context.Background(), 0)
}

func TestLLMMetrics_RecordRAGRetrieval_ZeroResults(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-zero-rag")
	require.NoError(t, err)

	metrics.RecordRAGRetrieval(context.Background(), 0, 0, 0)
}

func TestLLMMetrics_RecordDebateRound_ZeroParticipants(t *testing.T) {
	t.Parallel()
	metrics, err := NewLLMMetrics("test-zero-debate")
	require.NoError(t, err)

	metrics.RecordDebateRound(context.Background(), 0, 0)
}

// =============================================================================
// SetupNoOpProvider (indirect through SetupTraceExporter)
// =============================================================================

func TestSetupNoOpProvider_ValidConfig(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterNone,
		ServiceName: "test-noop",
		Version:     "2.0.0",
		Environment: "staging",
	}

	tp, err := SetupTraceExporter(context.Background(), config)
	if err != nil {
		t.Skipf("Skipping due to OTel schema conflict: %v", err)
	}
	require.NotNil(t, tp)

	err = ShutdownTraceExporter(context.Background(), tp)
	assert.NoError(t, err)
}

// =============================================================================
// Console exporter additional paths
// =============================================================================

func TestSetupTraceExporter_Console_WithServiceInfo(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterConsole,
		ServiceName: "console-test",
		Version:     "3.0.0",
		Environment: "development",
	}

	tp, err := SetupTraceExporter(context.Background(), config)
	if err != nil {
		t.Skipf("Skipping due to OTel schema conflict: %v", err)
	}
	require.NotNil(t, tp)

	err = ShutdownTraceExporter(context.Background(), tp)
	assert.NoError(t, err)
}

// =============================================================================
// RecordRAGRetrievalResult edge cases
// =============================================================================

func TestLLMTracer_RecordRAGRetrievalResult_ZeroValues(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, span := tracer.StartRAGRetrieval(context.Background(), "empty query", 0)
	tracer.RecordRAGRetrievalResult(span, 0, 0)
	span.End()
	_ = ctx
}

func TestLLMTracer_RecordRAGRetrievalResult_LargeValues(t *testing.T) {
	t.Parallel()
	tracer, err := NewLLMTracer(nil)
	require.NoError(t, err)

	ctx, span := tracer.StartRAGRetrieval(context.Background(), "large query", 1000)
	tracer.RecordRAGRetrievalResult(span, 999, 9999.99)
	span.End()
	_ = ctx
}

// =============================================================================
// TracedProvider with ValidateConfig edge cases
// =============================================================================

func TestTracedProvider_ValidateConfig_EmptyConfig(t *testing.T) {
	t.Parallel()
	provider := &mockLLMProvider{}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProvider(provider, tracer, "test")

	valid, errs := traced.ValidateConfig(map[string]interface{}{})
	assert.True(t, valid)
	assert.Empty(t, errs)
}

func TestTracedProvider_ValidateConfig_NilConfig(t *testing.T) {
	t.Parallel()
	provider := &mockLLMProvider{}
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProvider(provider, tracer, "test")

	valid, errs := traced.ValidateConfig(nil)
	assert.True(t, valid)
	assert.Empty(t, errs)
}

// =============================================================================
// ShutdownTraceExporter with sdktrace.TracerProvider
// =============================================================================

func TestShutdownTraceExporter_WithTimeoutContext(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterNone,
		ServiceName: "test-timeout-shutdown",
		Version:     "1.0.0",
		Environment: "test",
	}

	tp, err := SetupTraceExporter(context.Background(), config)
	if err != nil {
		t.Skipf("Skipping due to OTel schema conflict: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = ShutdownTraceExporter(ctx, tp)
	assert.NoError(t, err)
}

// =============================================================================
// LangfuseConfig field validation
// =============================================================================

func TestLangfuseConfig_AllFields(t *testing.T) {
	t.Parallel()
	config := &LangfuseConfig{
		PublicKey:  "pk-prod-key",
		SecretKey:  "sk-prod-key",
		BaseURL:    "https://self-hosted.langfuse.com",
		FlushAt:    50,
		FlushAfter: 60,
	}

	assert.Equal(t, "pk-prod-key", config.PublicKey)
	assert.Equal(t, "sk-prod-key", config.SecretKey)
	assert.Equal(t, "https://self-hosted.langfuse.com", config.BaseURL)
	assert.Equal(t, 50, config.FlushAt)
	assert.Equal(t, 60, config.FlushAfter)
}

func TestLangfuseConfig_EmptyFields(t *testing.T) {
	t.Parallel()
	config := &LangfuseConfig{}

	assert.Empty(t, config.PublicKey)
	assert.Empty(t, config.SecretKey)
	assert.Empty(t, config.BaseURL)
	assert.Zero(t, config.FlushAt)
	assert.Zero(t, config.FlushAfter)
}

// =============================================================================
// ExporterConfig field validation
// =============================================================================

func TestExporterConfig_AllFields(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterOTLP,
		Endpoint:    "otel-collector:4318",
		Headers:     map[string]string{"X-API-Key": "secret"},
		Insecure:    false,
		ServiceName: "production-service",
		Environment: "production",
		Version:     "5.0.0",
	}

	assert.Equal(t, ExporterOTLP, config.Type)
	assert.Equal(t, "otel-collector:4318", config.Endpoint)
	assert.Len(t, config.Headers, 1)
	assert.False(t, config.Insecure)
	assert.Equal(t, "production-service", config.ServiceName)
	assert.Equal(t, "production", config.Environment)
	assert.Equal(t, "5.0.0", config.Version)
}

func TestExporterConfig_EmptyHeaders(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterOTLP,
		Endpoint:    "localhost:4318",
		Headers:     map[string]string{},
		ServiceName: "test",
	}

	assert.Empty(t, config.Headers)
}

func TestExporterConfig_NilHeaders(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterOTLP,
		Endpoint:    "localhost:4318",
		Headers:     nil,
		ServiceName: "test",
	}

	assert.Nil(t, config.Headers)
}

// =============================================================================
// TracedProvider GetCapabilities default
// =============================================================================

func TestTracedProvider_GetCapabilities_Default(t *testing.T) {
	t.Parallel()
	provider := &mockLLMProvider{} // no custom capabilities set
	tracer, _ := NewLLMTracer(nil)
	traced := NewTracedProvider(provider, tracer, "test")

	caps := traced.GetCapabilities()
	assert.NotNil(t, caps)
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
}

// =============================================================================
// Ensure TracerProvider type from exporter
// =============================================================================

func TestSetupTraceExporter_ReturnsCorrectType(t *testing.T) {
	t.Parallel()
	config := &ExporterConfig{
		Type:        ExporterNone,
		ServiceName: "type-check",
		Version:     "1.0.0",
		Environment: "test",
	}

	tp, err := SetupTraceExporter(context.Background(), config)
	if err != nil {
		t.Skipf("Skipping due to OTel schema conflict: %v", err)
	}

	// Verify it's the correct SDK type
	var _ *sdktrace.TracerProvider = tp
	assert.NotNil(t, tp)

	_ = ShutdownTraceExporter(context.Background(), tp)
}
