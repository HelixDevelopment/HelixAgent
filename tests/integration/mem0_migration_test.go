package integration

import (
	"os"
	"strings"
	"testing"

	"dev.helix.agent/internal/config"
)

// TestMem0MigrationValidation ensures the Cognee → Mem0 migration is complete
// and cannot regress. This is a critical test that must always pass.
func TestMem0MigrationValidation(t *testing.T) {
	t.Run("Cognee disabled by default in config", func(t *testing.T) {
		cfg := config.Load()
		if cfg.Cognee.Enabled {
			t.Error("Cognee is enabled but should be disabled by default")
		}
	})

	t.Run("AutoCognify disabled by default", func(t *testing.T) {
		cfg := config.Load()
		if cfg.Cognee.AutoCognify {
			t.Error("AutoCognify is enabled but should be disabled by default")
		}
	})

	t.Run("Cognee service not required in defaults", func(t *testing.T) {
		serviceCfg := config.DefaultServicesConfig()
		if serviceCfg.Cognee.Required {
			t.Error("Cognee service is marked as required but should not be")
		}
		if serviceCfg.Cognee.Enabled {
			t.Error("Cognee service is enabled in defaults but should be disabled")
		}
	})

	t.Run("Memory field exists in Config struct", func(t *testing.T) {
		cfg := config.Load()
		// If this compiles, the Memory field exists
		_ = cfg.Memory
	})

	t.Run("Config file has Cognee disabled", func(t *testing.T) {
		data, err := os.ReadFile("configs/development.yaml")
		if err != nil {
			t.Skipf("Skipping - development.yaml not found: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			return
		}

		content := string(data)

		// Check main cognee section
		if !strings.Contains(content, "cognee:") {
			t.Error("cognee section not found in development.yaml")
		}

		// Check for enabled: false
		if !strings.Contains(content, "enabled: false") {
			t.Error("Cognee should be disabled in development.yaml")
		}
	})

	t.Run("Config file has Mem0 configuration", func(t *testing.T) {
		data, err := os.ReadFile("configs/development.yaml")
		if err != nil {
			t.Skipf("Skipping - development.yaml not found: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			return
		}

		content := string(data)

		if !strings.Contains(content, "memory:") {
			t.Error("Mem0 memory configuration not found in development.yaml")
		}

		if !strings.Contains(content, "storage_type") {
			t.Error("Mem0 storage_type not configured")
		}

		if !strings.Contains(content, "postgres") {
			t.Error("Mem0 should use PostgreSQL backend")
		}
	})

	t.Run("No hardcoded Cognee enabling in code", func(t *testing.T) {
		data, err := os.ReadFile("internal/config/config.go")
		if err != nil {
			t.Skipf("Skipping - config.go not found: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			return
		}

		content := string(data)
		lines := strings.Split(content, "\n")

		// Track if we're in a Cognee-related section
		inCogneeSection := false

		for i, line := range lines {
			// Skip comments
			if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}

			// Detect Cognee section start
			if strings.Contains(line, "Cognee:") && strings.Contains(line, "ServiceEndpoint{") {
				inCogneeSection = true
			}

			// Detect Cognee CogneeConfig section
			if strings.Contains(line, "Cognee:") && strings.Contains(line, "CogneeConfig{") {
				inCogneeSection = true
			}

			// Detect section end
			if inCogneeSection && strings.Contains(line, "},") {
				inCogneeSection = false
			}

			// Only check for hardcoded values in Cognee-related sections
			if inCogneeSection {
				if strings.Contains(line, "Enabled:") && strings.Contains(line, "true") && !strings.Contains(line, "getBoolEnv") {
					t.Errorf("Line %d: Found hardcoded Cognee Enabled: true (should be false): %s", i+1, strings.TrimSpace(line))
				}

				if strings.Contains(line, "Required:") && strings.Contains(line, "true") {
					t.Errorf("Line %d: Found hardcoded Cognee Required: true (should be false): %s", i+1, strings.TrimSpace(line))
				}

				if strings.Contains(line, "AutoCognify:") && strings.Contains(line, "true") && !strings.Contains(line, "getBoolEnv") {
					t.Errorf("Line %d: Found hardcoded AutoCognify: true (should be false): %s", i+1, strings.TrimSpace(line))
				}
			}
		}
	})

	t.Run("Memory service checks Cognee enabled flag", func(t *testing.T) {
		data, err := os.ReadFile("internal/services/memory_service.go")
		if err != nil {
			t.Skipf("Skipping - memory_service.go not found: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			return
		}

		content := string(data)

		if !strings.Contains(content, "cfg.Cognee.Enabled") {
			t.Error("Memory service should check cfg.Cognee.Enabled before initialization")
		}
	})

	t.Run("CogneeService checks enabled flag before operations", func(t *testing.T) {
		data, err := os.ReadFile("internal/services/cognee_service.go")
		if err != nil {
			t.Skipf("Skipping - cognee_service.go not found: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			return
		}

		content := string(data)

		// SearchMemory should check enabled flag
		if !strings.Contains(content, "!s.config.Enabled") {
			t.Error("CogneeService should check config.Enabled before operations")
		}
	})
}

// TestCogneeDisabledBehavior validates that when Cognee is disabled,
// the system behaves correctly without errors.
func TestCogneeDisabledBehavior(t *testing.T) {
	// Set environment to ensure Cognee is disabled
	os.Setenv("COGNEE_ENABLED", "false")
	os.Setenv("COGNEE_AUTO_COGNIFY", "false")
	defer os.Unsetenv("COGNEE_ENABLED")
	defer os.Unsetenv("COGNEE_AUTO_COGNIFY")

	t.Run("Config loads with Cognee disabled", func(t *testing.T) {
		cfg := config.Load()

		if cfg.Cognee.Enabled {
			t.Error("Cognee should be disabled when COGNEE_ENABLED=false")
		}

		if cfg.Cognee.AutoCognify {
			t.Error("AutoCognify should be disabled when COGNEE_AUTO_COGNIFY=false")
		}
	})

	t.Run("Services config has Cognee disabled", func(t *testing.T) {
		serviceCfg := config.DefaultServicesConfig()

		if serviceCfg.Cognee.Enabled {
			t.Error("Cognee service should be disabled by default")
		}

		if serviceCfg.Cognee.Required {
			t.Error("Cognee service should not be required")
		}
	})
}

// TestMem0ConfigurationPresent ensures Mem0 is properly configured
func TestMem0ConfigurationPresent(t *testing.T) {
	t.Run("Config struct has Memory field", func(t *testing.T) {
		cfg := config.Load()

		// The Memory field should exist (if this compiles, it does)
		_ = cfg.Memory
	})

	t.Run("Development YAML has Mem0 section", func(t *testing.T) {
		data, err := os.ReadFile("configs/development.yaml")
		if err != nil {
			t.Skipf("Skipping - development.yaml not found: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			return
		}

		content := string(data)

		requiredFields := []string{
			"memory:",
			"storage_type:",
			"vectordb_endpoint:",
			"embedding_model:",
			"enable_graph:",
			"max_memories_per_user:",
		}

		for _, field := range requiredFields {
			if !strings.Contains(content, field) {
				t.Errorf("Mem0 configuration missing required field: %s", field)
			}
		}
	})
}
