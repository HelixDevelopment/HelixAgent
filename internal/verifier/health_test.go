package verifier

import (
	"testing"
	"time"
)

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("CircuitState.String() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker("test-provider")
	if cb == nil {
		t.Fatal("NewCircuitBreaker returned nil")
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected initial state Closed, got %s", cb.State())
	}
	if !cb.IsAvailable() {
		t.Error("new circuit breaker should be available")
	}
}

func TestCircuitBreaker_RecordSuccess(t *testing.T) {
	cb := NewCircuitBreaker("test")

	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Error("state should remain closed after success")
	}
	if !cb.IsAvailable() {
		t.Error("should be available after success")
	}
}

func TestCircuitBreaker_RecordFailure_OpensCircuit(t *testing.T) {
	cb := NewCircuitBreaker("test")

	// Default threshold is 5
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Error("should still be closed after 4 failures")
	}

	cb.RecordFailure() // 5th failure should open
	if cb.State() != CircuitOpen {
		t.Errorf("expected Open after 5 failures, got %s", cb.State())
	}
	if cb.IsAvailable() {
		t.Error("should not be available when circuit is open")
	}
}

func TestCircuitBreaker_TransitionToHalfOpen(t *testing.T) {
	// Create a circuit breaker and manually set a short reset timeout for testing
	cb := NewCircuitBreaker("test")
	cb.resetTimeout = 100 * time.Millisecond

	// Open the circuit with 5 failures
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	if cb.State() != CircuitOpen {
		t.Fatal("circuit should be open")
	}

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)

	// Should transition to half-open when checked
	if !cb.IsAvailable() {
		t.Error("should be available (half-open) after reset timeout")
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected HalfOpen, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpen_SuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker("test")
	cb.resetTimeout = 100 * time.Millisecond

	// Open the circuit
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)
	cb.IsAvailable() // Trigger transition to half-open

	// Success should close the circuit
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Errorf("expected Closed after success in half-open, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpen_FailureOpens(t *testing.T) {
	cb := NewCircuitBreaker("test")
	cb.resetTimeout = 100 * time.Millisecond

	// Open the circuit
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)
	cb.IsAvailable() // Trigger transition to half-open

	// Failure should re-open the circuit
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("expected Open after failure in half-open, got %s", cb.State())
	}
}

func TestCircuitBreaker_Call_Success(t *testing.T) {
	cb := NewCircuitBreaker("test")

	err := cb.Call(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCircuitBreaker_Call_CircuitOpen(t *testing.T) {
	cb := NewCircuitBreaker("test")

	// Open the circuit
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	err := cb.Call(func() error {
		return nil
	})

	if err == nil {
		t.Error("expected error when circuit is open")
	}
}

func TestProviderHealth_Fields(t *testing.T) {
	now := time.Now()
	health := &ProviderHealth{
		ProviderID:    "test-provider",
		ProviderName:  "Test Provider",
		Healthy:       true,
		CircuitState:  "closed",
		FailureCount:  0,
		SuccessCount:  10,
		AvgResponseMs: 100,
		UptimePercent: 99.9,
		LastSuccessAt: now,
		LastFailureAt: time.Time{},
		LastCheckedAt: now,
	}

	if health.ProviderID != "test-provider" {
		t.Error("ProviderID mismatch")
	}
	if !health.Healthy {
		t.Error("Healthy should be true")
	}
	if health.SuccessCount != 10 {
		t.Error("SuccessCount mismatch")
	}
	if health.UptimePercent != 99.9 {
		t.Error("UptimePercent mismatch")
	}
}

func TestNewHealthService(t *testing.T) {
	svc := NewHealthService(nil)
	if svc == nil {
		t.Fatal("NewHealthService returned nil")
	}
	if svc.circuitBreakers == nil {
		t.Error("circuitBreakers map not initialized")
	}
	if svc.providerHealth == nil {
		t.Error("providerHealth map not initialized")
	}
}

func TestNewHealthService_WithConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Health.CheckInterval = time.Minute
	cfg.Health.Timeout = 15 * time.Second

	svc := NewHealthService(cfg)
	if svc == nil {
		t.Fatal("NewHealthService returned nil")
	}
	if svc.checkInterval != time.Minute {
		t.Errorf("expected checkInterval %v, got %v", time.Minute, svc.checkInterval)
	}
}

func TestHealthService_AddProvider(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test-provider", "Test Provider")

	health, err := svc.GetProviderHealth("test-provider")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health.ProviderID != "test-provider" {
		t.Error("ProviderID mismatch")
	}
	if health.ProviderName != "Test Provider" {
		t.Error("ProviderName mismatch")
	}
	if !health.Healthy {
		t.Error("new provider should be healthy")
	}
}

func TestHealthService_RemoveProvider(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test-provider", "Test")
	svc.RemoveProvider("test-provider")

	_, err := svc.GetProviderHealth("test-provider")
	if err == nil {
		t.Error("expected error for removed provider")
	}
}

func TestHealthService_GetProviderHealth_NotFound(t *testing.T) {
	svc := NewHealthService(nil)

	_, err := svc.GetProviderHealth("non-existent")
	if err == nil {
		t.Error("expected error for non-existent provider")
	}
}

func TestHealthService_RecordSuccess(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test", "Test")

	svc.RecordSuccess("test")

	health, _ := svc.GetProviderHealth("test")
	if health.SuccessCount != 1 {
		t.Errorf("expected SuccessCount 1, got %d", health.SuccessCount)
	}
	if !health.Healthy {
		t.Error("should be healthy after success")
	}
}

func TestHealthService_RecordFailure(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test", "Test")

	svc.RecordFailure("test")

	health, _ := svc.GetProviderHealth("test")
	if health.FailureCount != 1 {
		t.Errorf("expected FailureCount 1, got %d", health.FailureCount)
	}
}

func TestHealthService_RecordSuccess_NonExistent(t *testing.T) {
	svc := NewHealthService(nil)
	// Should not panic
	svc.RecordSuccess("non-existent")
}

func TestHealthService_RecordFailure_NonExistent(t *testing.T) {
	svc := NewHealthService(nil)
	// Should not panic
	svc.RecordFailure("non-existent")
}

func TestHealthService_IsProviderAvailable(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test", "Test")

	if !svc.IsProviderAvailable("test") {
		t.Error("newly added provider should be available")
	}

	if svc.IsProviderAvailable("non-existent") {
		t.Error("non-existent provider should not be available")
	}
}

func TestHealthService_GetHealthyProviders(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("healthy1", "Healthy 1")
	svc.AddProvider("healthy2", "Healthy 2")

	providers := svc.GetHealthyProviders()
	if len(providers) != 2 {
		t.Errorf("expected 2 healthy providers, got %d", len(providers))
	}
}

func TestHealthService_GetAllProviderHealth(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("p1", "Provider 1")
	svc.AddProvider("p2", "Provider 2")

	all := svc.GetAllProviderHealth()
	if len(all) != 2 {
		t.Errorf("expected 2 providers, got %d", len(all))
	}
}

func TestHealthService_GetFastestProvider(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("slow", "Slow")
	svc.AddProvider("fast", "Fast")

	// Manually set latency via providerHealth map
	svc.mu.Lock()
	svc.providerHealth["slow"].AvgResponseMs = 500
	svc.providerHealth["fast"].AvgResponseMs = 100
	svc.mu.Unlock()

	fastest, err := svc.GetFastestProvider([]string{"slow", "fast"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fastest != "fast" {
		t.Errorf("expected 'fast', got '%s'", fastest)
	}
}

func TestHealthService_GetFastestProvider_Empty(t *testing.T) {
	svc := NewHealthService(nil)

	_, err := svc.GetFastestProvider([]string{})
	if err == nil {
		t.Error("expected error for empty provider list")
	}
}

func TestHealthService_GetFastestProvider_NoHealthy(t *testing.T) {
	svc := NewHealthService(nil)

	_, err := svc.GetFastestProvider([]string{"non-existent"})
	if err == nil {
		t.Error("expected error when no healthy providers")
	}
}

func TestHealthService_GetProviderLatency(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test", "Test")

	// Set latency manually
	svc.mu.Lock()
	svc.providerHealth["test"].AvgResponseMs = 100
	svc.mu.Unlock()

	latency, err := svc.GetProviderLatency("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latency != 100 {
		t.Errorf("expected latency 100, got %d", latency)
	}
}

func TestHealthService_GetProviderLatency_NotFound(t *testing.T) {
	svc := NewHealthService(nil)

	_, err := svc.GetProviderLatency("non-existent")
	if err == nil {
		t.Error("expected error for non-existent provider")
	}
}

func TestHealthService_GetCircuitBreaker(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test", "Test")

	cb := svc.GetCircuitBreaker("test")
	if cb == nil {
		t.Error("expected circuit breaker for added provider")
	}

	cb2 := svc.GetCircuitBreaker("non-existent")
	if cb2 != nil {
		t.Error("expected nil for non-existent provider")
	}
}

func TestHealthService_ConcurrentAccess(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test", "Test")

	done := make(chan bool, 20)

	// Concurrent reads and writes
	for i := 0; i < 10; i++ {
		go func() {
			svc.RecordSuccess("test")
			done <- true
		}()
		go func() {
			svc.GetProviderHealth("test")
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestHealthService_UptimeCalculation(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test", "Test")

	// Record 8 successes and 2 failures
	for i := 0; i < 8; i++ {
		svc.RecordSuccess("test")
	}
	for i := 0; i < 2; i++ {
		svc.RecordFailure("test")
	}

	// Note: Uptime is calculated during health checks, not during RecordSuccess/RecordFailure
	// So we need to manually verify the counts
	health, _ := svc.GetProviderHealth("test")
	if health.SuccessCount != 8 {
		t.Errorf("expected 8 successes, got %d", health.SuccessCount)
	}
	if health.FailureCount != 2 {
		t.Errorf("expected 2 failures, got %d", health.FailureCount)
	}
}

func TestHealthService_CircuitBreakerIntegration(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("test", "Test")

	// Trip the circuit breaker (5 failures needed)
	for i := 0; i < 5; i++ {
		svc.RecordFailure("test")
	}

	cb := svc.GetCircuitBreaker("test")
	if cb == nil {
		t.Fatal("circuit breaker is nil")
	}

	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to be open after 5 failures, got %s", cb.State())
	}

	if svc.IsProviderAvailable("test") {
		t.Error("provider should not be available when circuit is open")
	}
}

func TestHealthService_StartStop(t *testing.T) {
	svc := NewHealthService(nil)

	// Start should succeed
	err := svc.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Starting again should fail
	err = svc.Start()
	if err == nil {
		t.Error("expected error when starting already running service")
	}

	// Stop should succeed
	svc.Stop()

	// Stop again should not panic
	svc.Stop()
}

func TestHealthService_ExecuteWithFailover(t *testing.T) {
	svc := NewHealthService(nil)
	svc.AddProvider("p1", "Provider 1")
	svc.AddProvider("p2", "Provider 2")

	called := ""
	err := svc.ExecuteWithFailover(nil, []string{"p1", "p2"}, func(providerID string) error {
		called = providerID
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called == "" {
		t.Error("operation should have been called")
	}
}

func TestHealthService_ExecuteWithFailover_AllFail(t *testing.T) {
	svc := NewHealthService(nil)

	err := svc.ExecuteWithFailover(nil, []string{"non-existent"}, func(providerID string) error {
		return nil
	})

	if err == nil {
		t.Error("expected error when all providers fail")
	}
}

func TestProviderHealth_ZeroValue(t *testing.T) {
	var health ProviderHealth

	if health.ProviderID != "" {
		t.Error("zero ProviderID should be empty")
	}
	if health.Healthy {
		t.Error("zero Healthy should be false")
	}
	if health.SuccessCount != 0 {
		t.Error("zero SuccessCount should be 0")
	}
}

func TestCircuitBreaker_Fields(t *testing.T) {
	cb := NewCircuitBreaker("test-cb")

	if cb.name != "test-cb" {
		t.Errorf("expected name 'test-cb', got '%s'", cb.name)
	}
	if cb.threshold != 5 {
		t.Errorf("expected threshold 5, got %d", cb.threshold)
	}
	if cb.resetTimeout != 30*time.Second {
		t.Errorf("expected resetTimeout 30s, got %v", cb.resetTimeout)
	}
}
