package messaging

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// MiddlewareChain Tests
// =============================================================================

func TestNewMiddlewareChain(t *testing.T) {
	chain := NewMiddlewareChain()

	require.NotNil(t, chain)
	assert.Zero(t, chain.middleware.Len())
}

func TestNewMiddlewareChain_WithMiddleware(t *testing.T) {
	mw1 := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error { return next(ctx, msg) }
	}
	mw2 := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error { return next(ctx, msg) }
	}

	chain := NewMiddlewareChain(mw1, mw2)

	require.NotNil(t, chain)
	assert.Equal(t, 2, chain.middleware.Len())
}

func TestMiddlewareChain_Add(t *testing.T) {
	chain := NewMiddlewareChain()

	mw := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error { return next(ctx, msg) }
	}

	chain.Add(mw)

	assert.Equal(t, 1, chain.middleware.Len())
}

func TestMiddlewareChain_Add_Multiple(t *testing.T) {
	chain := NewMiddlewareChain()

	mw1 := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error { return next(ctx, msg) }
	}
	mw2 := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error { return next(ctx, msg) }
	}
	mw3 := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error { return next(ctx, msg) }
	}

	chain.Add(mw1, mw2, mw3)

	assert.Equal(t, 3, chain.middleware.Len())
}

func TestMiddlewareChain_Prepend(t *testing.T) {
	order := make([]int, 0)

	mw1 := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			order = append(order, 1)
			return next(ctx, msg)
		}
	}
	mw2 := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			order = append(order, 2)
			return next(ctx, msg)
		}
	}

	chain := NewMiddlewareChain(mw1)
	chain.Prepend(mw2)

	handler := chain.Wrap(func(ctx context.Context, msg *Message) error {
		order = append(order, 0)
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	_ = handler(context.Background(), msg)

	// mw2 should execute first because it was prepended
	assert.Equal(t, []int{2, 1, 0}, order)
}

func TestMiddlewareChain_Clear(t *testing.T) {
	chain := NewMiddlewareChain()

	mw := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error { return next(ctx, msg) }
	}

	chain.Add(mw, mw, mw)
	assert.Equal(t, 3, chain.middleware.Len())

	chain.Clear()
	assert.Zero(t, chain.middleware.Len())
}

func TestMiddlewareChain_Wrap(t *testing.T) {
	order := make([]int, 0)

	mw1 := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			order = append(order, 1)
			err := next(ctx, msg)
			order = append(order, 4)
			return err
		}
	}
	mw2 := func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			order = append(order, 2)
			err := next(ctx, msg)
			order = append(order, 3)
			return err
		}
	}

	chain := NewMiddlewareChain(mw1, mw2)

	handler := chain.Wrap(func(ctx context.Context, msg *Message) error {
		order = append(order, 0)
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	_ = handler(context.Background(), msg)

	// Middleware should execute in order: mw1 -> mw2 -> handler -> mw2 -> mw1
	assert.Equal(t, []int{1, 2, 0, 3, 4}, order)
}

func TestMiddlewareChain_Wrap_Empty(t *testing.T) {
	chain := NewMiddlewareChain()

	called := false
	handler := chain.Wrap(func(ctx context.Context, msg *Message) error {
		called = true
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
	assert.True(t, called)
}

func TestMiddlewareChain_ConcurrentAccess(t *testing.T) {
	chain := NewMiddlewareChain()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mw := func(next MessageHandler) MessageHandler {
				return func(ctx context.Context, msg *Message) error { return next(ctx, msg) }
			}
			chain.Add(mw)
		}()
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler := chain.Wrap(func(ctx context.Context, msg *Message) error { return nil })
			msg := NewMessage("test", []byte("payload"))
			_ = handler(context.Background(), msg)
		}()
	}

	wg.Wait()
}

// =============================================================================
// DefaultLogger Tests
// =============================================================================

func TestDefaultLogger_Debug(t *testing.T) {
	logger := &DefaultLogger{}
	// Should not panic
	logger.Debug("test message", "key", "value")
}

func TestDefaultLogger_Info(t *testing.T) {
	logger := &DefaultLogger{}
	// Should not panic
	logger.Info("test message", "key", "value")
}

func TestDefaultLogger_Warn(t *testing.T) {
	logger := &DefaultLogger{}
	// Should not panic
	logger.Warn("test message", "key", "value")
}

func TestDefaultLogger_Error(t *testing.T) {
	logger := &DefaultLogger{}
	// Should not panic
	logger.Error("test message", "key", "value")
}

// =============================================================================
// LoggingMiddleware Tests
// =============================================================================

func TestLoggingMiddleware_NilLogger(t *testing.T) {
	mw := LoggingMiddleware(nil)
	require.NotNil(t, mw)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
}

func TestLoggingMiddleware_WithLogger(t *testing.T) {
	logger := &DefaultLogger{}
	mw := LoggingMiddleware(logger)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
}

func TestLoggingMiddleware_WithError(t *testing.T) {
	logger := &DefaultLogger{}
	mw := LoggingMiddleware(logger)
	expectedErr := errors.New("handler error")

	handler := mw(func(ctx context.Context, msg *Message) error {
		return expectedErr
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Equal(t, expectedErr, err)
}

// =============================================================================
// RetryConfig Tests
// =============================================================================

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
	assert.Contains(t, config.RetryableErrors, ErrCodeConnectionFailed)
	assert.Contains(t, config.RetryableErrors, ErrCodeConnectionTimeout)
	assert.Contains(t, config.RetryableErrors, ErrCodeBrokerUnavailable)
}

// =============================================================================
// RetryMiddleware Tests
// =============================================================================

func TestRetryMiddleware_NilConfig(t *testing.T) {
	mw := RetryMiddleware(nil)
	require.NotNil(t, mw)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
}

func TestRetryMiddleware_Success(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	mw := RetryMiddleware(config)

	callCount := 0
	handler := mw(func(ctx context.Context, msg *Message) error {
		callCount++
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount) // Should succeed on first try
}

func TestRetryMiddleware_RetryOnRetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      2,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: []ErrorCode{ErrCodeConnectionFailed},
	}
	mw := RetryMiddleware(config)

	callCount := 0
	handler := mw(func(ctx context.Context, msg *Message) error {
		callCount++
		if callCount < 3 {
			return NewBrokerError(ErrCodeConnectionFailed, "connection failed", nil)
		}
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount) // 1 initial + 2 retries
}

func TestRetryMiddleware_ExhaustsRetries(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      2,
		InitialDelay:    5 * time.Millisecond,
		MaxDelay:        50 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: []ErrorCode{ErrCodeConnectionFailed},
	}
	mw := RetryMiddleware(config)

	callCount := 0
	expectedErr := NewBrokerError(ErrCodeConnectionFailed, "connection failed", nil)
	handler := mw(func(ctx context.Context, msg *Message) error {
		callCount++
		return expectedErr
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Error(t, err)
	assert.Equal(t, 3, callCount) // 1 initial + 2 retries
}

func TestRetryMiddleware_NoRetryOnNonRetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      3,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: []ErrorCode{ErrCodeConnectionFailed},
	}
	mw := RetryMiddleware(config)

	callCount := 0
	nonRetryableErr := NewBrokerError(ErrCodeMessageInvalid, "invalid message", nil)
	handler := mw(func(ctx context.Context, msg *Message) error {
		callCount++
		return nonRetryableErr
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Error(t, err)
	assert.Equal(t, 1, callCount) // Should not retry
}

func TestRetryMiddleware_ContextCancellation(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      10,
		InitialDelay:    50 * time.Millisecond,
		MaxDelay:        1 * time.Second,
		Multiplier:      2.0,
		RetryableErrors: []ErrorCode{ErrCodeConnectionFailed},
	}
	mw := RetryMiddleware(config)

	callCount := atomic.Int32{}
	handler := mw(func(ctx context.Context, msg *Message) error {
		callCount.Add(1)
		return NewBrokerError(ErrCodeConnectionFailed, "connection failed", nil)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	msg := NewMessage("test", []byte("payload"))
	err := handler(ctx, msg)

	assert.Error(t, err)
	assert.True(t, callCount.Load() < 10) // Should be cancelled before exhausting retries
}

func TestRetryMiddleware_ExponentialBackoff(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      3,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        1 * time.Second,
		Multiplier:      2.0,
		RetryableErrors: []ErrorCode{ErrCodeConnectionFailed},
	}
	mw := RetryMiddleware(config)

	timestamps := make([]time.Time, 0)
	handler := mw(func(ctx context.Context, msg *Message) error {
		timestamps = append(timestamps, time.Now())
		if len(timestamps) < 4 {
			return NewBrokerError(ErrCodeConnectionFailed, "connection failed", nil)
		}
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	_ = handler(context.Background(), msg)

	// Verify delays are increasing (exponential backoff)
	if len(timestamps) >= 3 {
		delay1 := timestamps[1].Sub(timestamps[0])
		delay2 := timestamps[2].Sub(timestamps[1])
		// Second delay should be approximately double the first
		assert.True(t, delay2 >= delay1)
	}
}

func TestRetryMiddleware_MaxDelayRespected(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      5,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        150 * time.Millisecond, // Low max delay
		Multiplier:      10.0,                   // High multiplier
		RetryableErrors: []ErrorCode{ErrCodeConnectionFailed},
	}
	mw := RetryMiddleware(config)

	timestamps := make([]time.Time, 0)
	handler := mw(func(ctx context.Context, msg *Message) error {
		timestamps = append(timestamps, time.Now())
		return NewBrokerError(ErrCodeConnectionFailed, "connection failed", nil)
	})

	msg := NewMessage("test", []byte("payload"))
	_ = handler(context.Background(), msg)

	// Verify no delay exceeds maxDelay (with some tolerance for timing)
	for i := 1; i < len(timestamps); i++ {
		delay := timestamps[i].Sub(timestamps[i-1])
		assert.True(t, delay <= 200*time.Millisecond) // maxDelay + tolerance
	}
}

// =============================================================================
// TimeoutMiddleware Tests
// =============================================================================

func TestTimeoutMiddleware_Success(t *testing.T) {
	mw := TimeoutMiddleware(1 * time.Second)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
}

func TestTimeoutMiddleware_Timeout(t *testing.T) {
	mw := TimeoutMiddleware(50 * time.Millisecond)

	handler := mw(func(ctx context.Context, msg *Message) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestTimeoutMiddleware_HandlerError(t *testing.T) {
	mw := TimeoutMiddleware(1 * time.Second)
	expectedErr := errors.New("handler error")

	handler := mw(func(ctx context.Context, msg *Message) error {
		return expectedErr
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Equal(t, expectedErr, err)
}

// =============================================================================
// RecoveryMiddleware Tests
// =============================================================================

func TestRecoveryMiddleware_NilLogger(t *testing.T) {
	mw := RecoveryMiddleware(nil)
	require.NotNil(t, mw)

	handler := mw(func(ctx context.Context, msg *Message) error {
		panic("test panic")
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "panic recovered")
}

func TestRecoveryMiddleware_WithLogger(t *testing.T) {
	logger := &DefaultLogger{}
	mw := RecoveryMiddleware(logger)

	handler := mw(func(ctx context.Context, msg *Message) error {
		panic("test panic")
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Error(t, err)
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	logger := &DefaultLogger{}
	mw := RecoveryMiddleware(logger)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
}

func TestRecoveryMiddleware_HandlerError(t *testing.T) {
	logger := &DefaultLogger{}
	mw := RecoveryMiddleware(logger)
	expectedErr := errors.New("handler error")

	handler := mw(func(ctx context.Context, msg *Message) error {
		return expectedErr
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Equal(t, expectedErr, err)
}

// =============================================================================
// TracingMiddleware Tests
// =============================================================================

func TestTracingMiddleware_GeneratesTraceID(t *testing.T) {
	mw := TracingMiddleware()

	var capturedTraceID string
	handler := mw(func(ctx context.Context, msg *Message) error {
		capturedTraceID = GetTraceID(ctx)
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
	assert.NotEmpty(t, capturedTraceID)
	assert.NotEmpty(t, msg.TraceID)
}

func TestTracingMiddleware_UsesExistingTraceID(t *testing.T) {
	mw := TracingMiddleware()

	var capturedTraceID string
	handler := mw(func(ctx context.Context, msg *Message) error {
		capturedTraceID = GetTraceID(ctx)
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	msg.TraceID = "existing-trace-id"
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
	assert.Equal(t, "existing-trace-id", capturedTraceID)
	assert.Equal(t, "existing-trace-id", msg.TraceID)
}

func TestGetTraceID_NoTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := GetTraceID(ctx)

	assert.Empty(t, traceID)
}

func TestGetTraceID_WithTraceID(t *testing.T) {
	ctx := context.WithValue(context.Background(), traceIDKey, "test-trace-id")
	traceID := GetTraceID(ctx)

	assert.Equal(t, "test-trace-id", traceID)
}

// =============================================================================
// MetricsMiddleware Tests
// =============================================================================

func TestMetricsMiddleware_NilMetrics(t *testing.T) {
	mw := MetricsMiddleware(nil)
	require.NotNil(t, mw)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
}

func TestMetricsMiddleware_RecordsSuccess(t *testing.T) {
	metrics := NewBrokerMetrics()
	mw := MetricsMiddleware(metrics)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("test-payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(1), metrics.MessagesProcessed.Load())
	assert.Equal(t, int64(0), metrics.MessagesFailed.Load())
}

func TestMetricsMiddleware_RecordsFailure(t *testing.T) {
	metrics := NewBrokerMetrics()
	mw := MetricsMiddleware(metrics)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return errors.New("handler error")
	})

	msg := NewMessage("test", []byte("test-payload"))
	err := handler(context.Background(), msg)

	assert.Error(t, err)
	assert.Equal(t, int64(1), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(0), metrics.MessagesProcessed.Load())
	assert.Equal(t, int64(1), metrics.MessagesFailed.Load())
}

func TestMetricsMiddleware_RecordsBytesReceived(t *testing.T) {
	metrics := NewBrokerMetrics()
	mw := MetricsMiddleware(metrics)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	payload := []byte("test-payload-12345") // 18 bytes
	msg := NewMessage("test", payload)
	_ = handler(context.Background(), msg)

	assert.Equal(t, int64(18), metrics.BytesReceived.Load())
}

// =============================================================================
// ValidationMiddleware Tests
// =============================================================================

func TestValidationMiddleware_NoValidators(t *testing.T) {
	mw := ValidationMiddleware()

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
}

func TestValidationMiddleware_PassesValidation(t *testing.T) {
	validator := func(msg *Message) error {
		return nil
	}
	mw := ValidationMiddleware(validator)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
}

func TestValidationMiddleware_FailsValidation(t *testing.T) {
	validator := func(msg *Message) error {
		return errors.New("validation failed")
	}
	mw := ValidationMiddleware(validator)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidationMiddleware_MultipleValidators(t *testing.T) {
	callOrder := make([]int, 0)
	v1 := func(msg *Message) error {
		callOrder = append(callOrder, 1)
		return nil
	}
	v2 := func(msg *Message) error {
		callOrder = append(callOrder, 2)
		return nil
	}
	v3 := func(msg *Message) error {
		callOrder = append(callOrder, 3)
		return nil
	}

	mw := ValidationMiddleware(v1, v2, v3)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, callOrder)
}

func TestValidationMiddleware_StopsOnFirstFailure(t *testing.T) {
	callOrder := make([]int, 0)
	v1 := func(msg *Message) error {
		callOrder = append(callOrder, 1)
		return nil
	}
	v2 := func(msg *Message) error {
		callOrder = append(callOrder, 2)
		return errors.New("v2 failed")
	}
	v3 := func(msg *Message) error {
		callOrder = append(callOrder, 3)
		return nil
	}

	mw := ValidationMiddleware(v1, v2, v3)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.Error(t, err)
	assert.Equal(t, []int{1, 2}, callOrder) // v3 should not be called
}

// =============================================================================
// RequiredFieldsValidator Tests
// =============================================================================

func TestRequiredFieldsValidator_ValidMessage(t *testing.T) {
	msg := NewMessage("test", []byte("payload"))

	err := RequiredFieldsValidator(msg)

	assert.NoError(t, err)
}

func TestRequiredFieldsValidator_MissingID(t *testing.T) {
	msg := NewMessage("test", []byte("payload"))
	msg.ID = ""

	err := RequiredFieldsValidator(msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message ID is required")
}

func TestRequiredFieldsValidator_MissingType(t *testing.T) {
	msg := NewMessage("", []byte("payload"))

	err := RequiredFieldsValidator(msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message type is required")
}

func TestRequiredFieldsValidator_MissingPayload(t *testing.T) {
	msg := NewMessage("test", []byte{})

	err := RequiredFieldsValidator(msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message payload is required")
}

// =============================================================================
// MaxSizeValidator Tests
// =============================================================================

func TestMaxSizeValidator_UnderLimit(t *testing.T) {
	validator := MaxSizeValidator(100)
	msg := NewMessage("test", []byte("small payload"))

	err := validator(msg)

	assert.NoError(t, err)
}

func TestMaxSizeValidator_OverLimit(t *testing.T) {
	validator := MaxSizeValidator(10)
	msg := NewMessage("test", []byte("this is a payload that exceeds the limit"))

	err := validator(msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum size")
}

func TestMaxSizeValidator_ExactLimit(t *testing.T) {
	validator := MaxSizeValidator(10)
	msg := NewMessage("test", []byte("1234567890")) // Exactly 10 bytes

	err := validator(msg)

	assert.NoError(t, err)
}

// =============================================================================
// ExpirationValidator Tests
// =============================================================================

func TestExpirationValidator_NotExpired(t *testing.T) {
	msg := NewMessage("test", []byte("payload"))
	msg.Expiration = time.Now().Add(1 * time.Hour)

	err := ExpirationValidator(msg)

	assert.NoError(t, err)
}

func TestExpirationValidator_Expired(t *testing.T) {
	msg := NewMessage("test", []byte("payload"))
	msg.Expiration = time.Now().Add(-1 * time.Hour)

	err := ExpirationValidator(msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message has expired")
}

func TestExpirationValidator_NoExpiration(t *testing.T) {
	msg := NewMessage("test", []byte("payload"))
	// No expiration set

	err := ExpirationValidator(msg)

	assert.NoError(t, err)
}

// =============================================================================
// DeduplicationMiddleware Tests
// =============================================================================

func TestDeduplicationMiddleware_NewMessage(t *testing.T) {
	store := NewInMemoryDeduplicationStore(1 * time.Minute)
	defer store.Stop()
	mw := DeduplicationMiddleware(store)

	called := false
	handler := mw(func(ctx context.Context, msg *Message) error {
		called = true
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
	assert.True(t, called)
	assert.True(t, store.Exists(msg.ID))
}

func TestDeduplicationMiddleware_DuplicateMessage(t *testing.T) {
	store := NewInMemoryDeduplicationStore(1 * time.Minute)
	defer store.Stop()
	mw := DeduplicationMiddleware(store)

	callCount := 0
	handler := mw(func(ctx context.Context, msg *Message) error {
		callCount++
		return nil
	})

	msg := NewMessage("test", []byte("payload"))

	// First call
	_ = handler(context.Background(), msg)
	// Second call with same message
	_ = handler(context.Background(), msg)

	assert.Equal(t, 1, callCount) // Should only be called once
}

func TestDeduplicationMiddleware_ErrorDoesNotMarkAsProcessed(t *testing.T) {
	store := NewInMemoryDeduplicationStore(1 * time.Minute)
	defer store.Stop()
	mw := DeduplicationMiddleware(store)

	callCount := 0
	handler := mw(func(ctx context.Context, msg *Message) error {
		callCount++
		if callCount == 1 {
			return errors.New("first call fails")
		}
		return nil
	})

	msg := NewMessage("test", []byte("payload"))

	// First call - should fail
	err1 := handler(context.Background(), msg)
	assert.Error(t, err1)

	// Second call - should succeed because first failed
	err2 := handler(context.Background(), msg)
	assert.NoError(t, err2)

	assert.Equal(t, 2, callCount)
}

// =============================================================================
// InMemoryDeduplicationStore Tests
// =============================================================================

func TestInMemoryDeduplicationStore_Add(t *testing.T) {
	store := NewInMemoryDeduplicationStore(1 * time.Minute)
	defer store.Stop()

	store.Add("msg-1")

	assert.True(t, store.Exists("msg-1"))
}

func TestInMemoryDeduplicationStore_Exists(t *testing.T) {
	store := NewInMemoryDeduplicationStore(1 * time.Minute)
	defer store.Stop()

	assert.False(t, store.Exists("msg-1"))

	store.Add("msg-1")

	assert.True(t, store.Exists("msg-1"))
}

func TestInMemoryDeduplicationStore_Remove(t *testing.T) {
	store := NewInMemoryDeduplicationStore(1 * time.Minute)
	defer store.Stop()

	store.Add("msg-1")
	assert.True(t, store.Exists("msg-1"))

	store.Remove("msg-1")
	assert.False(t, store.Exists("msg-1"))
}

func TestInMemoryDeduplicationStore_Clear(t *testing.T) {
	store := NewInMemoryDeduplicationStore(1 * time.Minute)
	defer store.Stop()

	store.Add("msg-1")
	store.Add("msg-2")
	store.Add("msg-3")

	store.Clear()

	assert.False(t, store.Exists("msg-1"))
	assert.False(t, store.Exists("msg-2"))
	assert.False(t, store.Exists("msg-3"))
}

func TestInMemoryDeduplicationStore_Expiration(t *testing.T) {
	store := NewInMemoryDeduplicationStore(50 * time.Millisecond)
	defer store.Stop()

	store.Add("msg-1")
	assert.True(t, store.Exists("msg-1"))

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	assert.False(t, store.Exists("msg-1"))
}

func TestInMemoryDeduplicationStore_ConcurrentAccess(t *testing.T) {
	store := NewInMemoryDeduplicationStore(1 * time.Minute)
	defer store.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := "msg-" + string(rune('A'+idx%26))
			store.Add(id)
			_ = store.Exists(id)
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// RateLimitMiddleware Tests
// =============================================================================

func TestRateLimitMiddleware_Allowed(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 10)
	mw := RateLimitMiddleware(limiter)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
}

func TestRateLimitMiddleware_RateLimited(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 0.001) // Very low refill rate
	mw := RateLimitMiddleware(limiter)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))

	// First call should succeed
	err1 := handler(context.Background(), msg)
	assert.NoError(t, err1)

	// Second call should be rate limited
	err2 := handler(context.Background(), msg)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "rate limit exceeded")
}

// =============================================================================
// TokenBucketLimiter Tests
// =============================================================================

func TestNewTokenBucketLimiter(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)

	assert.NotNil(t, limiter)
	assert.Equal(t, 10.0, limiter.maxTokens)
	assert.Equal(t, 5.0, limiter.refillRate)
}

func TestTokenBucketLimiter_Allow(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 10)

	// Should allow up to max tokens
	for i := 0; i < 10; i++ {
		assert.True(t, limiter.Allow())
	}

	// Next one should be denied
	assert.False(t, limiter.Allow())
}

func TestTokenBucketLimiter_Refill(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 100) // High refill rate

	// Use the token
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow())

	// Wait for refill
	time.Sleep(50 * time.Millisecond)

	// Should be able to use again
	assert.True(t, limiter.Allow())
}

func TestTokenBucketLimiter_Wait(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 100) // High refill rate

	// Use the token
	assert.True(t, limiter.Allow())

	ctx := context.Background()
	err := limiter.Wait(ctx)

	assert.NoError(t, err)
}

func TestTokenBucketLimiter_Wait_ContextCancellation(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 0.001) // Very low refill rate

	// Use the token
	assert.True(t, limiter.Allow())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := limiter.Wait(ctx)

	assert.Error(t, err)
}

func TestTokenBucketLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewTokenBucketLimiter(100, 100)

	var wg sync.WaitGroup
	allowed := atomic.Int32{}

	for i := 0; i < 150; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow() {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	// Should have allowed approximately maxTokens requests
	assert.True(t, allowed.Load() <= 100)
}

// =============================================================================
// CircuitBreakerMiddleware Tests
// =============================================================================

func TestCircuitBreakerMiddleware_Closed(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)
	mw := CircuitBreakerMiddleware(cb)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return nil
	})

	msg := NewMessage("test", []byte("payload"))
	err := handler(context.Background(), msg)

	assert.NoError(t, err)
	assert.Equal(t, CircuitClosed, cb.State())
}

func TestCircuitBreakerMiddleware_OpensOnFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)
	mw := CircuitBreakerMiddleware(cb)

	handler := mw(func(ctx context.Context, msg *Message) error {
		return errors.New("handler error")
	})

	msg := NewMessage("test", []byte("payload"))

	// Trigger failures to open circuit
	for i := 0; i < 3; i++ {
		_ = handler(context.Background(), msg)
	}

	assert.Equal(t, CircuitOpen, cb.State())

	// Next request should be blocked
	err := handler(context.Background(), msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker open")
}

func TestCircuitBreakerMiddleware_HalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	mw := CircuitBreakerMiddleware(cb)

	failureCount := 0
	handler := mw(func(ctx context.Context, msg *Message) error {
		if failureCount < 3 {
			failureCount++
			return errors.New("handler error")
		}
		return nil
	})

	msg := NewMessage("test", []byte("payload"))

	// Trigger failures to open circuit
	for i := 0; i < 3; i++ {
		_ = handler(context.Background(), msg)
	}

	assert.Equal(t, CircuitOpen, cb.State())

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Next request should go through (half-open state)
	err := handler(context.Background(), msg)
	assert.NoError(t, err)
}

// =============================================================================
// CircuitBreaker Tests
// =============================================================================

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)

	assert.NotNil(t, cb)
	assert.Equal(t, CircuitClosed, cb.State())
	assert.Equal(t, 5, cb.threshold)
	assert.Equal(t, 30*time.Second, cb.resetTimeout)
}

func TestCircuitBreaker_Allow_Closed(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_Allow_Open(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Hour)

	// Force open state
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	assert.False(t, cb.Allow())
}

func TestCircuitBreaker_Allow_HalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)

	// Force open state
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	assert.False(t, cb.Allow())

	// Wait for reset
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open
	assert.True(t, cb.Allow())
	assert.Equal(t, CircuitHalfOpen, cb.State())
}

func TestCircuitBreaker_RecordSuccess_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	cb.RecordFailure() // One failure
	cb.RecordSuccess() // Should reset failures

	// Should still be closed
	assert.Equal(t, CircuitClosed, cb.State())
	assert.Equal(t, 0, cb.failures)
}

func TestCircuitBreaker_RecordSuccess_HalfOpenState(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)

	// Open circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // Transition to half-open

	// Record successes to close circuit
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()

	assert.Equal(t, CircuitClosed, cb.State())
}

func TestCircuitBreaker_RecordFailure_HalfOpenState(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)

	// Open circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // Transition to half-open

	// Record failure - should reopen circuit
	cb.RecordFailure()

	assert.Equal(t, CircuitOpen, cb.State())
}

func TestCircuitBreaker_HalfOpen_LimitedRequests(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)

	// Open circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Should allow limited requests in half-open state
	allowedCount := 0
	for i := 0; i < 10; i++ {
		if cb.Allow() {
			allowedCount++
		}
	}

	// Should only allow 3 requests in half-open (first triggers transition)
	assert.LessOrEqual(t, allowedCount, 4) // 1 to trigger + 3 allowed
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(100, 1*time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				cb.RecordSuccess()
			} else {
				cb.RecordFailure()
			}
			_ = cb.State()
			_ = cb.Allow()
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestGenerateTraceID(t *testing.T) {
	id1 := generateTraceID()
	time.Sleep(time.Millisecond)
	id2 := generateTraceID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestShouldRetry(t *testing.T) {
	retryableCodes := []ErrorCode{ErrCodeConnectionFailed, ErrCodeBrokerUnavailable}

	// Retryable broker error
	err1 := NewBrokerError(ErrCodeConnectionFailed, "connection failed", nil)
	assert.True(t, shouldRetry(err1, retryableCodes))

	// Non-retryable broker error
	err2 := NewBrokerError(ErrCodeMessageInvalid, "invalid message", nil)
	assert.False(t, shouldRetry(err2, retryableCodes))

	// Regular error
	err3 := errors.New("regular error")
	assert.False(t, shouldRetry(err3, retryableCodes))
}
