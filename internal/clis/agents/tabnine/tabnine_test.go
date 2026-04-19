// Package tabnine provides tests for Tabnine agent integration
package tabnine

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

func TestNewTabnine(t *testing.T) {
	t.Parallel()
	tn := New()

	if tn == nil {
		t.Fatal("New() = nil")
	}

	info := tn.Info()
	if info.Type != agents.TypeTabnine {
		t.Errorf("Info().Type = %q, want %q", info.Type, agents.TypeTabnine)
	}

	if info.Name != "Tabnine" {
		t.Errorf("Info().Name = %q, want %q", info.Name, "Tabnine")
	}

	if info.Vendor != "Tabnine" {
		t.Errorf("Info().Vendor = %q, want %q", info.Vendor, "Tabnine")
	}
}

func TestTabnineInitialize(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	tempDir := t.TempDir()
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
		APIKey:       "test-api-key",
		LocalMode:    false,
		ModelType:    "cloud",
		TeamMode:     true,
		PrivacyLevel: "team",
	}

	err := tn.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if tn.config.APIKey != "test-api-key" {
		t.Errorf("config.APIKey = %q, want %q", tn.config.APIKey, "test-api-key")
	}

	if tn.config.LocalMode != false {
		t.Errorf("config.LocalMode = %v, want %v", tn.config.LocalMode, false)
	}

	if tn.config.ModelType != "cloud" {
		t.Errorf("config.ModelType = %q, want %q", tn.config.ModelType, "cloud")
	}
}

func TestTabnineStartStop(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	err := tn.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	err = tn.Start(ctx)
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	if !tn.IsStarted() {
		t.Error("IsStarted() = false after Start()")
	}

	err = tn.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if tn.IsStarted() {
		t.Error("IsStarted() = true after Stop()")
	}
}

func TestTabnineExecute(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Each subtest below gets its own Tabnine — `configure` writes to
	// t.config fields WITHOUT synchronisation (race-debt BUGFIX #34);
	// the "configure command" subtest and "status command" subtest
	// would otherwise race on the same struct.
	newTabnine := func(t *testing.T) *Tabnine {
		t.Helper()
		tn := New()
		if err := tn.Initialize(ctx, nil); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		return tn
	}

	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "complete command",
			command: "complete",
			params: map[string]interface{}{
				"prefix":   "func main()",
				"suffix":   "}",
				"language": "go",
			},
			wantErr: false,
		},
		{
			name:    "complete with default language",
			command: "complete",
			params: map[string]interface{}{
				"prefix": "func main()",
			},
			wantErr: false,
		},
		{
			name:    "chat command",
			command: "chat",
			params: map[string]interface{}{
				"message": "Hello Tabnine",
			},
			wantErr: false,
		},
		{
			name:    "chat without message fails",
			command: "chat",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "review command",
			command: "review",
			params: map[string]interface{}{
				"code": "func main() {}",
			},
			wantErr: false,
		},
		{
			name:    "review without code fails",
			command: "review",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "status command",
			command: "status",
			params:  map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "configure command",
			command: "configure",
			params: map[string]interface{}{
				"local_mode": true,
				"model_type": "hybrid",
			},
			wantErr: false,
		},
		{
			name:    "unknown command",
			command: "unknown",
			params:  map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tn := newTabnine(t)
			result, err := tn.Execute(ctx, tt.command, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == nil {
				t.Error("Execute() result = nil, want non-nil")
			}
		})
	}
}

func TestTabnineComplete(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	err := tn.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := tn.Execute(ctx, "complete", map[string]interface{}{
		"prefix":   "func fibonacci(n int) int {",
		"suffix":   "}",
		"language": "go",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if resultMap["prefix"] != "func fibonacci(n int) int {" {
		t.Errorf("prefix = %v, want %v", resultMap["prefix"], "func fibonacci(n int) int {")
	}

	if resultMap["language"] != "go" {
		t.Errorf("language = %v, want %v", resultMap["language"], "go")
	}
}

func TestTabnineCompleteLanguages(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	err := tn.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	tests := []struct {
		language string
		prefix   string
	}{
		{"go", "func main("},
		{"python", "def main():"},
		{"javascript", "function main()"},
		{"typescript", "function main(): void"},
		{"unknown", "some code"},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			t.Parallel()
			result, err := tn.Execute(ctx, "complete", map[string]interface{}{
				"prefix":   tt.prefix,
				"language": tt.language,
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Fatal("result is not a map")
			}

			if resultMap["language"] != tt.language {
				t.Errorf("language = %v, want %v", resultMap["language"], tt.language)
			}
		})
	}
}

func TestTabnineChat(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	err := tn.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := tn.Execute(ctx, "chat", map[string]interface{}{
		"message": "Explain Go concurrency",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if resultMap["message"] != "Explain Go concurrency" {
		t.Errorf("message = %v, want %v", resultMap["message"], "Explain Go concurrency")
	}
}

func TestTabnineReview(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	err := tn.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := tn.Execute(ctx, "review", map[string]interface{}{
		"code": "func main() { fmt.Println('Hello') }",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if _, ok := resultMap["review"]; !ok {
		t.Error("review missing 'review' field")
	}

	if _, ok := resultMap["issues"]; !ok {
		t.Error("review missing 'issues' field")
	}
}

func TestTabnineConfigure(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	err := tn.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := tn.Execute(ctx, "configure", map[string]interface{}{
		"local_mode":    true,
		"model_type":    "local",
		"team_mode":     true,
		"privacy_level": "enterprise",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if resultMap["local_mode"] != true {
		t.Errorf("local_mode = %v, want %v", resultMap["local_mode"], true)
	}

	if resultMap["model_type"] != "local" {
		t.Errorf("model_type = %v, want %v", resultMap["model_type"], "local")
	}

	if resultMap["team_mode"] != true {
		t.Errorf("team_mode = %v, want %v", resultMap["team_mode"], true)
	}

	if resultMap["privacy_level"] != "enterprise" {
		t.Errorf("privacy_level = %v, want %v", resultMap["privacy_level"], "enterprise")
	}
}

func TestTabnineStatus(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	err := tn.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := tn.Execute(ctx, "status", nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if _, ok := resultMap["available"]; !ok {
		t.Error("status missing 'available' field")
	}

	if _, ok := resultMap["local_mode"]; !ok {
		t.Error("status missing 'local_mode' field")
	}

	if _, ok := resultMap["model_type"]; !ok {
		t.Error("status missing 'model_type' field")
	}
}

func TestTabnineCapabilities(t *testing.T) {
	t.Parallel()
	tn := New()
	info := tn.Info()

	expectedCapabilities := []string{
		"local_models",
		"privacy_focused",
		"team_learning",
		"code_completion",
		"chat",
		"code_review",
	}

	for _, cap := range expectedCapabilities {
		found := false
		for _, has := range info.Capabilities {
			if has == cap {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing capability: %s", cap)
		}
	}
}

func TestTabnineHealth(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	// Before start, health should fail
	if err := tn.Health(ctx); err == nil {
		t.Error("Health() before Start = nil, want error")
	}

	_ = tn.Initialize(ctx, nil)
	_ = tn.Start(ctx)

	// After start, health should pass
	if err := tn.Health(ctx); err != nil {
		t.Errorf("Health() after Start error = %v", err)
	}
}

func TestTabnineIsAvailable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		config    *Config
		available bool
	}{
		{
			name:      "with API key only",
			config:    &Config{APIKey: "test-key", LocalMode: false},
			available: true,
		},
		{
			name:      "with local mode only",
			config:    &Config{APIKey: "", LocalMode: true},
			available: true,
		},
		{
			name:      "with both API key and local mode",
			config:    &Config{APIKey: "test-key", LocalMode: true},
			available: true,
		},
		{
			name:      "with neither API key nor local mode",
			config:    &Config{APIKey: "", LocalMode: false},
			available: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tn := New()
			ctx := context.Background()
			_ = tn.Initialize(ctx, tt.config)

			if got := tn.IsAvailable(); got != tt.available {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.available)
			}
		})
	}
}

func TestTabnineConfigValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			config: &Config{
				APIKey:       "test-key",
				LocalMode:    true,
				ModelType:    "hybrid",
				TeamMode:     false,
				PrivacyLevel: "local",
			},
			wantErr: false,
		},
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
		{
			name: "empty config fields use defaults",
			config: &Config{
				APIKey:    "",
				ModelType: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tn := New()
			ctx := context.Background()
			err := tn.Initialize(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkTabnineExecute(b *testing.B) {
	tn := New()
	ctx := context.Background()
	_ = tn.Initialize(ctx, nil)
	_ = tn.Start(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tn.Execute(ctx, "complete", map[string]interface{}{
			"prefix":   "func main()",
			"language": "go",
		})
	}
}
