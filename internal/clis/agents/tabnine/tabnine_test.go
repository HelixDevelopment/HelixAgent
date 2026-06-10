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
			// Reconciled (§11.4.120): complete now returns an honest error —
			// Tabnine is an IDE plugin with no headless CLI, so it cannot produce
			// a completion here and refuses to fabricate one (BLUFF-001).
			name:    "complete command returns honest error (no headless CLI)",
			command: "complete",
			params: map[string]interface{}{
				"prefix":   "func main()",
				"suffix":   "}",
				"language": "go",
			},
			wantErr: true,
		},
		{
			// Still errors: prefix present but capability unavailable (honest).
			name:    "complete returns honest error regardless of language",
			command: "complete",
			params: map[string]interface{}{
				"prefix": "func main()",
			},
			wantErr: true,
		},
		{
			// Reconciled (§11.4.120): chat now returns an honest error.
			name:    "chat command returns honest error (no headless CLI)",
			command: "chat",
			params: map[string]interface{}{
				"message": "Hello Tabnine",
			},
			wantErr: true,
		},
		{
			name:    "chat without message fails",
			command: "chat",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			// Reconciled (§11.4.120): review now returns an honest error.
			name:    "review command returns honest error (no headless CLI)",
			command: "review",
			params: map[string]interface{}{
				"code": "func main() {}",
			},
			wantErr: true,
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

// TestTabnineCompleteNoFabrication reconciles the former TestTabnineComplete /
// TestTabnineCompleteLanguages (§11.4.120): those tests asserted a templated
// completion ("// Tabnine completion" etc.) was returned WITHOUT any real
// engine call — codifying BLUFF-001. complete now returns an honest error
// (ErrNoHeadlessCLI). Standing GREEN regression guard (§11.4.135): reverting to
// a fabricated completion (err == nil) makes this FAIL.
func TestTabnineCompleteNoFabrication(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	if err := tn.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	for _, lang := range []string{"go", "python", "javascript", "typescript", "unknown"} {
		result, err := tn.Execute(ctx, "complete", map[string]interface{}{
			"prefix":   "func main(",
			"language": lang,
		})
		if err == nil {
			t.Errorf("complete(lang=%s) returned nil error — must return an honest error, never a fabricated completion (BLUFF-001); got result %v", lang, result)
		}
		if result != nil {
			t.Errorf("complete(lang=%s) returned a result payload %v — must be nil when no real completion ran", lang, result)
		}
	}
}

// TestTabnineChatNoFabrication reconciles the former TestTabnineChat
// (§11.4.120): it asserted a "Tabnine: <msg>" echo was returned. chat now
// returns an honest error; this guard pins that no echo is fabricated.
func TestTabnineChatNoFabrication(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	if err := tn.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := tn.Execute(ctx, "chat", map[string]interface{}{
		"message": "Explain Go concurrency",
	})
	if err == nil {
		t.Errorf("chat returned nil error — must return an honest error, never a fabricated reply (BLUFF-003); got %v", result)
	}
	if result != nil {
		t.Errorf("chat returned a result payload %v — must be nil when no real reply ran", result)
	}
}

// TestTabnineReviewNoFabrication reconciles the former TestTabnineReview
// (§11.4.120): it asserted a fabricated "Code review by Tabnine" + a fixed
// issue. review now returns an honest error; this guard pins that.
func TestTabnineReviewNoFabrication(t *testing.T) {
	t.Parallel()
	tn := New()
	ctx := context.Background()

	if err := tn.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := tn.Execute(ctx, "review", map[string]interface{}{
		"code": "func main() { fmt.Println('Hello') }",
	})
	if err == nil {
		t.Errorf("review returned nil error — must return an honest error, never fabricated findings (BLUFF-001); got %v", result)
	}
	if result != nil {
		t.Errorf("review returned a result payload %v — must be nil when no real review ran", result)
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
