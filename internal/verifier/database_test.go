package verifier

import (
	"testing"
	"time"
)

func TestNewDatabaseBridgeWithPool_NilPool(t *testing.T) {
	bridge := NewDatabaseBridgeWithPool(nil)
	if bridge == nil {
		t.Fatal("NewDatabaseBridgeWithPool returned nil")
	}
	if bridge.pool != nil {
		t.Error("pool should be nil when no pool provided")
	}
}

func TestDatabaseBridge_Close_NilPool(t *testing.T) {
	bridge := NewDatabaseBridgeWithPool(nil)

	// Should not panic and return nil
	err := bridge.Close()
	if err != nil {
		t.Errorf("Close should return nil for nil pool, got: %v", err)
	}
}

func TestDatabaseBridge_GetPool_NilPool(t *testing.T) {
	bridge := NewDatabaseBridgeWithPool(nil)

	pool := bridge.GetPool()
	if pool != nil {
		t.Error("GetPool should return nil when pool is nil")
	}
}

func TestVerificationResult_Fields(t *testing.T) {
	now := time.Now()
	result := &VerificationResult{
		ID:                     123,
		ModelID:                "gpt-4",
		ProviderName:           "openai",
		VerificationType:       "full",
		Status:                 "verified",
		OverallScore:           95.5,
		CodeCapabilityScore:    90.0,
		ResponsivenessScore:    92.0,
		ReliabilityScore:       88.0,
		FeatureRichnessScore:   85.0,
		ValuePropositionScore:  90.0,
		SupportsCodeGeneration: true,
		SupportsCodeCompletion: true,
		SupportsCodeReview:     true,
		SupportsStreaming:      true,
		SupportsReasoning:      true,
		AvgLatencyMs:           150,
		P95LatencyMs:           300,
		ThroughputRPS:          50.0,
		VerifiedAt:             now,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if result.ID != 123 {
		t.Error("ID mismatch")
	}
	if result.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if result.ProviderName != "openai" {
		t.Error("ProviderName mismatch")
	}
	if result.Status != "verified" {
		t.Error("Status mismatch")
	}
	if result.OverallScore != 95.5 {
		t.Error("OverallScore mismatch")
	}
	if !result.SupportsCodeGeneration {
		t.Error("SupportsCodeGeneration mismatch")
	}
	if result.AvgLatencyMs != 150 {
		t.Error("AvgLatencyMs mismatch")
	}
}

func TestVerificationScore_Fields(t *testing.T) {
	now := time.Now()
	score := &VerificationScore{
		ID:              456,
		ModelID:         "claude-3-opus",
		OverallScore:    93.1,
		SpeedScore:      90.0,
		EfficiencyScore: 88.0,
		CostScore:       85.0,
		CapabilityScore: 95.0,
		RecencyScore:    92.0,
		ScoreSuffix:     "(SC:9.3)",
		DataSource:      "models.dev",
		CalculatedAt:    now,
		CreatedAt:       now,
	}

	if score.ID != 456 {
		t.Error("ID mismatch")
	}
	if score.ModelID != "claude-3-opus" {
		t.Error("ModelID mismatch")
	}
	if score.OverallScore != 93.1 {
		t.Error("OverallScore mismatch")
	}
	if score.SpeedScore != 90.0 {
		t.Error("SpeedScore mismatch")
	}
	if score.ScoreSuffix != "(SC:9.3)" {
		t.Error("ScoreSuffix mismatch")
	}
	if score.DataSource != "models.dev" {
		t.Error("DataSource mismatch")
	}
}

func TestVerificationResult_ZeroValue(t *testing.T) {
	var result VerificationResult

	if result.ID != 0 {
		t.Error("zero ID should be 0")
	}
	if result.ModelID != "" {
		t.Error("zero ModelID should be empty")
	}
	if result.OverallScore != 0 {
		t.Error("zero OverallScore should be 0")
	}
	if result.SupportsCodeGeneration {
		t.Error("zero SupportsCodeGeneration should be false")
	}
}

func TestVerificationScore_ZeroValue(t *testing.T) {
	var score VerificationScore

	if score.ID != 0 {
		t.Error("zero ID should be 0")
	}
	if score.ModelID != "" {
		t.Error("zero ModelID should be empty")
	}
	if score.OverallScore != 0 {
		t.Error("zero OverallScore should be 0")
	}
}

func TestPostgresConfig_Fields(t *testing.T) {
	cfg := &PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Database: "testdb",
		SSLMode:  "disable",
	}

	if cfg.Host != "localhost" {
		t.Error("Host mismatch")
	}
	if cfg.Port != 5432 {
		t.Error("Port mismatch")
	}
	if cfg.User != "postgres" {
		t.Error("User mismatch")
	}
	if cfg.Password != "secret" {
		t.Error("Password mismatch")
	}
	if cfg.Database != "testdb" {
		t.Error("Database mismatch")
	}
	if cfg.SSLMode != "disable" {
		t.Error("SSLMode mismatch")
	}
}

func TestDatabaseBridge_Methods_Exist(t *testing.T) {
	bridge := NewDatabaseBridgeWithPool(nil)

	// Verify all expected methods exist (compile-time check)
	_ = bridge.SaveVerificationResult
	_ = bridge.Close
	_ = bridge.GetPool
}

func TestVerificationResult_Capabilities(t *testing.T) {
	result := &VerificationResult{
		SupportsCodeGeneration: true,
		SupportsCodeCompletion: true,
		SupportsCodeReview:     false,
		SupportsStreaming:      true,
		SupportsReasoning:      true,
	}

	if !result.SupportsCodeGeneration {
		t.Error("SupportsCodeGeneration should be true")
	}
	if !result.SupportsCodeCompletion {
		t.Error("SupportsCodeCompletion should be true")
	}
	if result.SupportsCodeReview {
		t.Error("SupportsCodeReview should be false")
	}
	if !result.SupportsStreaming {
		t.Error("SupportsStreaming should be true")
	}
	if !result.SupportsReasoning {
		t.Error("SupportsReasoning should be true")
	}
}

func TestVerificationResult_Scores(t *testing.T) {
	result := &VerificationResult{
		OverallScore:          90.0,
		CodeCapabilityScore:   85.0,
		ResponsivenessScore:   88.0,
		ReliabilityScore:      92.0,
		FeatureRichnessScore:  80.0,
		ValuePropositionScore: 87.0,
	}

	total := result.CodeCapabilityScore + result.ResponsivenessScore +
		result.ReliabilityScore + result.FeatureRichnessScore +
		result.ValuePropositionScore

	if total <= 0 {
		t.Error("total scores should be positive")
	}
}

func TestVerificationResult_LatencyMetrics(t *testing.T) {
	result := &VerificationResult{
		AvgLatencyMs:  100,
		P95LatencyMs:  250,
		ThroughputRPS: 50.0,
	}

	if result.AvgLatencyMs >= result.P95LatencyMs {
		// This is expected - P95 should be higher than avg in most cases
	}

	if result.ThroughputRPS <= 0 {
		t.Error("ThroughputRPS should be positive")
	}
}

// Note: Integration tests with actual database would go in a separate file
// with build tags for integration testing, e.g.:
// //go:build integration
// package verifier_test
