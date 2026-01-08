package verifier

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true by default")
	}
	if cfg.Verification.VerificationTimeout <= 0 {
		t.Error("VerificationTimeout should be positive")
	}
	if cfg.Verification.RetryCount <= 0 {
		t.Error("RetryCount should be positive")
	}
	if cfg.Health.CheckInterval <= 0 {
		t.Error("Health.CheckInterval should be positive")
	}
}

func TestConfig_Fields(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		Database: DatabaseConfig{
			Path:              "/data/test.db",
			EncryptionEnabled: true,
			EncryptionKey:     "secret-key",
		},
		Verification: VerificationConfig{
			MandatoryCodeCheck:   true,
			CodeVisibilityPrompt: "Do you see my code?",
			VerificationTimeout:  30 * time.Second,
			RetryCount:           3,
			RetryDelay:           5 * time.Second,
			Tests:                []string{"existence", "latency"},
		},
		Scoring: ScoringConfig{
			Weights: ScoringWeightsConfig{
				ResponseSpeed:     0.25,
				ModelEfficiency:   0.20,
				CostEffectiveness: 0.25,
				Capability:        0.20,
				Recency:           0.10,
			},
			ModelsDevEnabled:  true,
			ModelsDevEndpoint: "https://api.models.dev",
			CacheTTL:          24 * time.Hour,
		},
		Health: HealthConfig{
			CheckInterval:     30 * time.Second,
			Timeout:           10 * time.Second,
			FailureThreshold:  5,
			RecoveryThreshold: 3,
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:         true,
				HalfOpenTimeout: time.Minute,
			},
		},
		API: APIConfig{
			Enabled:   true,
			Port:      "8081",
			BasePath:  "/api/v1/verifier",
			JWTSecret: "jwt-secret",
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 100,
			},
		},
	}

	if !cfg.Enabled {
		t.Error("Enabled mismatch")
	}
	if cfg.Database.Path != "/data/test.db" {
		t.Error("Database.Path mismatch")
	}
	if !cfg.Verification.MandatoryCodeCheck {
		t.Error("Verification.MandatoryCodeCheck mismatch")
	}
	if cfg.Scoring.Weights.ResponseSpeed != 0.25 {
		t.Error("Scoring.Weights.ResponseSpeed mismatch")
	}
	if cfg.Health.CheckInterval != 30*time.Second {
		t.Error("Health.CheckInterval mismatch")
	}
	if cfg.API.Port != "8081" {
		t.Error("API.Port mismatch")
	}
}

func TestConfig_ZeroValue(t *testing.T) {
	var cfg Config

	if cfg.Enabled {
		t.Error("zero Enabled should be false")
	}
	if cfg.Database.Path != "" {
		t.Error("zero Database.Path should be empty")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "default config is valid",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid weights sum",
			cfg: &Config{
				Scoring: ScoringConfig{
					Weights: ScoringWeightsConfig{
						ResponseSpeed:     0.5,
						ModelEfficiency:   0.5, // Sum > 1.0
						CostEffectiveness: 0.5,
						Capability:        0.5,
						Recency:           0.5,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidationError(t *testing.T) {
	err := &ConfigValidationError{
		Field:   "test_field",
		Message: "test error",
	}

	expected := "test_field: test error"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
verifier:
  enabled: true
  database:
    path: "/tmp/test.db"
  verification:
    mandatory_code_check: true
    verification_timeout: 30s
  scoring:
    weights:
      response_speed: 0.25
      model_efficiency: 0.20
      cost_effectiveness: 0.25
      capability: 0.20
      recency: 0.10
  health:
    check_interval: 30s
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("Database.Path = %s, want /tmp/test.db", cfg.Database.Path)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/non/existent/path.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadConfigFromBytes(t *testing.T) {
	configContent := []byte(`
verifier:
  enabled: true
  database:
    path: "/tmp/test.db"
  scoring:
    weights:
      response_speed: 0.25
      model_efficiency: 0.20
      cost_effectiveness: 0.25
      capability: 0.20
      recency: 0.10
`)

	cfg, err := LoadConfigFromBytes(configContent)
	if err != nil {
		t.Fatalf("LoadConfigFromBytes failed: %v", err)
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestLoadConfigFromBytes_InvalidYAML(t *testing.T) {
	invalidContent := []byte(`
verifier:
  enabled: true
  invalid yaml content here!!!
  - not a valid structure
`)

	_, err := LoadConfigFromBytes(invalidContent)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestExpandEnvVars(t *testing.T) {
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	result := ExpandEnvVars("$TEST_VAR")
	if result != "test_value" {
		t.Errorf("ExpandEnvVars() = %s, want test_value", result)
	}
}

func TestConfig_MaskSensitiveData(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			EncryptionKey: "secret-key",
		},
		API: APIConfig{
			JWTSecret: "jwt-secret",
		},
		Events: EventsConfig{
			Slack: SlackConfig{
				WebhookURL: "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXX",
			},
			Telegram: TelegramConfig{
				BotToken: "bot-token",
			},
		},
	}

	masked := cfg.MaskSensitiveData()

	if masked.Database.EncryptionKey != "***" {
		t.Error("EncryptionKey should be masked")
	}
	if masked.API.JWTSecret != "***" {
		t.Error("JWTSecret should be masked")
	}
	if masked.Events.Telegram.BotToken != "***" {
		t.Error("BotToken should be masked")
	}
	if masked.Events.Slack.WebhookURL == cfg.Events.Slack.WebhookURL {
		t.Error("WebhookURL should be masked")
	}

	// Original should be unchanged
	if cfg.Database.EncryptionKey == "***" {
		t.Error("original should not be modified")
	}
}

func TestDatabaseConfig_Fields(t *testing.T) {
	cfg := DatabaseConfig{
		Path:              "/data/db.sqlite",
		EncryptionEnabled: true,
		EncryptionKey:     "key",
	}

	if cfg.Path != "/data/db.sqlite" {
		t.Error("Path mismatch")
	}
	if !cfg.EncryptionEnabled {
		t.Error("EncryptionEnabled mismatch")
	}
}

func TestVerificationConfig_Fields(t *testing.T) {
	cfg := VerificationConfig{
		MandatoryCodeCheck:   true,
		CodeVisibilityPrompt: "prompt",
		VerificationTimeout:  time.Minute,
		RetryCount:           5,
		RetryDelay:           10 * time.Second,
		Tests:                []string{"test1", "test2"},
	}

	if !cfg.MandatoryCodeCheck {
		t.Error("MandatoryCodeCheck mismatch")
	}
	if len(cfg.Tests) != 2 {
		t.Error("Tests length mismatch")
	}
}

func TestScoringConfig_Fields(t *testing.T) {
	cfg := ScoringConfig{
		ModelsDevEnabled:  true,
		ModelsDevEndpoint: "https://api.example.com",
		CacheTTL:          time.Hour,
	}

	if !cfg.ModelsDevEnabled {
		t.Error("ModelsDevEnabled mismatch")
	}
	if cfg.CacheTTL != time.Hour {
		t.Error("CacheTTL mismatch")
	}
}

func TestHealthConfig_Fields(t *testing.T) {
	cfg := HealthConfig{
		CheckInterval:     30 * time.Second,
		Timeout:           10 * time.Second,
		FailureThreshold:  5,
		RecoveryThreshold: 3,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:         true,
			HalfOpenTimeout: time.Minute,
		},
	}

	if cfg.CheckInterval != 30*time.Second {
		t.Error("CheckInterval mismatch")
	}
	if !cfg.CircuitBreaker.Enabled {
		t.Error("CircuitBreaker.Enabled mismatch")
	}
}

func TestAPIConfig_Fields(t *testing.T) {
	cfg := APIConfig{
		Enabled:   true,
		Port:      "7061",
		BasePath:  "/api",
		JWTSecret: "secret",
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 60,
		},
	}

	if !cfg.Enabled {
		t.Error("Enabled mismatch")
	}
	if cfg.Port != "7061" {
		t.Error("Port mismatch")
	}
	if cfg.RateLimit.RequestsPerMinute != 60 {
		t.Error("RateLimit.RequestsPerMinute mismatch")
	}
}

func TestEventsConfig_Fields(t *testing.T) {
	cfg := EventsConfig{
		Slack: SlackConfig{
			Enabled:    true,
			WebhookURL: "https://hooks.slack.com/...",
		},
		Email: EmailConfig{
			Enabled:  true,
			SMTPHost: "smtp.example.com",
			SMTPPort: 587,
		},
		Telegram: TelegramConfig{
			Enabled:  true,
			BotToken: "token",
			ChatID:   "123456",
		},
		WebSocket: WebSocketConfig{
			Enabled: true,
			Path:    "/ws",
		},
	}

	if !cfg.Slack.Enabled {
		t.Error("Slack.Enabled mismatch")
	}
	if cfg.Email.SMTPPort != 587 {
		t.Error("Email.SMTPPort mismatch")
	}
	if !cfg.Telegram.Enabled {
		t.Error("Telegram.Enabled mismatch")
	}
	if !cfg.WebSocket.Enabled {
		t.Error("WebSocket.Enabled mismatch")
	}
}

func TestMonitoringConfig_Fields(t *testing.T) {
	cfg := MonitoringConfig{
		Prometheus: PrometheusConfig{
			Enabled: true,
			Path:    "/metrics",
		},
		Grafana: GrafanaConfig{
			Enabled:       true,
			DashboardPath: "/dashboard",
		},
	}

	if !cfg.Prometheus.Enabled {
		t.Error("Prometheus.Enabled mismatch")
	}
	if !cfg.Grafana.Enabled {
		t.Error("Grafana.Enabled mismatch")
	}
}

func TestBrotliConfig_Fields(t *testing.T) {
	cfg := BrotliConfig{
		Enabled:          true,
		HTTP3Support:     true,
		CompressionLevel: 6,
	}

	if !cfg.Enabled {
		t.Error("Enabled mismatch")
	}
	if !cfg.HTTP3Support {
		t.Error("HTTP3Support mismatch")
	}
	if cfg.CompressionLevel != 6 {
		t.Error("CompressionLevel mismatch")
	}
}

func TestChallengesConfig_Fields(t *testing.T) {
	cfg := ChallengesConfig{
		Enabled:           true,
		ProviderDiscovery: true,
		ModelVerification: true,
		ConfigGeneration:  true,
	}

	if !cfg.Enabled {
		t.Error("Enabled mismatch")
	}
	if !cfg.ProviderDiscovery {
		t.Error("ProviderDiscovery mismatch")
	}
}

func TestSchedulingConfig_Fields(t *testing.T) {
	cfg := SchedulingConfig{
		ReVerification: ScheduleConfig{
			Enabled:  true,
			Interval: 24 * time.Hour,
		},
		ScoreRecalculation: ScheduleConfig{
			Enabled:  true,
			Interval: 12 * time.Hour,
		},
	}

	if !cfg.ReVerification.Enabled {
		t.Error("ReVerification.Enabled mismatch")
	}
	if cfg.ScoreRecalculation.Interval != 12*time.Hour {
		t.Error("ScoreRecalculation.Interval mismatch")
	}
}

func TestMaskURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXX",
			expected: "https://hooks.slack.com/***/***/***/***",
		},
		{
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		result := maskURL(tt.input)
		if result != tt.expected {
			t.Errorf("maskURL(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}
