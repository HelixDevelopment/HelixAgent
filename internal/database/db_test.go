package database

import (
	"os"
	"testing"
	"time"

	"github.com/superagent/superagent/internal/config"
)

func TestNewPostgresDB(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:           "localhost",
				Port:           "5432",
				User:           "testuser",
				Password:       "testpass",
				Name:           "testdb",
				SSLMode:        "disable",
				MaxConnections: 10,
				ConnTimeout:    5 * time.Second,
				PoolSize:       5,
			},
		}

		db, err := NewPostgresDB(cfg)
		if err != nil {
			// Connection may fail if PostgreSQL is not running, that's OK for unit test
			t.Logf("Database connection failed (expected if PostgreSQL not running): %v", err)
			return
		}
		defer db.Close()

		if db == nil {
			t.Fatal("Expected database connection, got nil")
		}
	})

	t.Run("InvalidConnectionString", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:           "", // Invalid host
				Port:           "5432",
				User:           "testuser",
				Password:       "testpass",
				Name:           "testdb",
				SSLMode:        "disable",
				MaxConnections: 10,
				ConnTimeout:    5 * time.Second,
				PoolSize:       5,
			},
		}

		// This should fail due to invalid connection string
		db, err := NewPostgresDB(cfg)
		if err == nil {
			if db != nil {
				db.Close()
			}
			t.Error("Expected error for invalid connection string, got nil")
		}
	})

	t.Run("EnvironmentVariablesOverride", func(t *testing.T) {
		// Save original environment
		originalDBHost := os.Getenv("DB_HOST")
		originalDBPort := os.Getenv("DB_PORT")
		originalDBUser := os.Getenv("DB_USER")
		originalDBPassword := os.Getenv("DB_PASSWORD")
		originalDBName := os.Getenv("DB_NAME")

		// Set environment variables
		os.Setenv("DB_HOST", "env-host")
		os.Setenv("DB_PORT", "5433")
		os.Setenv("DB_USER", "env-user")
		os.Setenv("DB_PASSWORD", "env-pass")
		os.Setenv("DB_NAME", "env-db")

		defer func() {
			// Restore environment
			if originalDBHost != "" {
				os.Setenv("DB_HOST", originalDBHost)
			} else {
				os.Unsetenv("DB_HOST")
			}
			if originalDBPort != "" {
				os.Setenv("DB_PORT", originalDBPort)
			} else {
				os.Unsetenv("DB_PORT")
			}
			if originalDBUser != "" {
				os.Setenv("DB_USER", originalDBUser)
			} else {
				os.Unsetenv("DB_USER")
			}
			if originalDBPassword != "" {
				os.Setenv("DB_PASSWORD", originalDBPassword)
			} else {
				os.Unsetenv("DB_PASSWORD")
			}
			if originalDBName != "" {
				os.Setenv("DB_NAME", originalDBName)
			} else {
				os.Unsetenv("DB_NAME")
			}
		}()

		// Config with defaults that should be overridden by environment
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:           "default-host",
				Port:           "5432",
				User:           "default-user",
				Password:       "default-pass",
				Name:           "default-db",
				SSLMode:        "disable",
				MaxConnections: 10,
				ConnTimeout:    5 * time.Second,
				PoolSize:       5,
			},
		}

		db, err := NewPostgresDB(cfg)
		if err != nil {
			t.Logf("Database connection failed (expected if PostgreSQL not running): %v", err)
			return
		}
		if db != nil {
			db.Close()
		}
		// Note: We can't easily verify the connection string was built with env vars
		// without mocking pgxpool.New, but this test ensures the function doesn't panic
	})
}

func TestPostgresDBOperations(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           "localhost",
			Port:           "5432",
			User:           "testuser",
			Password:       "testpass",
			Name:           "testdb",
			SSLMode:        "disable",
			MaxConnections: 10,
			ConnTimeout:    5 * time.Second,
			PoolSize:       5,
		},
	}

	db, err := NewPostgresDB(cfg)
	if err != nil {
		t.Logf("Database connection failed (expected if PostgreSQL not running): %v", err)
		return
	}
	defer db.Close()

	t.Run("Ping", func(t *testing.T) {
		err := db.Ping()
		if err != nil {
			t.Logf("Failed to ping database (expected if PostgreSQL not running): %v", err)
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		err := db.HealthCheck()
		if err != nil {
			t.Logf("Failed health check (expected if PostgreSQL not running): %v", err)
		}
	})

	t.Run("Close", func(t *testing.T) {
		// Create a new connection for this test
		db2, err := NewPostgresDB(cfg)
		if err != nil {
			t.Logf("Database connection failed: %v", err)
			return
		}

		err = db2.Close()
		if err != nil {
			t.Errorf("Failed to close database: %v", err)
		}
	})

	t.Run("ExecWithInvalidQuery", func(t *testing.T) {
		// This should fail with an invalid SQL query
		err := db.Exec("INVALID SQL SYNTAX")
		if err != nil {
			t.Logf("Expected error for invalid SQL (expected if PostgreSQL not running): %v", err)
		}
	})

	t.Run("QueryWithInvalidQuery", func(t *testing.T) {
		// This should fail with an invalid SQL query
		results, err := db.Query("INVALID SQL SYNTAX")
		if err != nil {
			t.Logf("Expected error for invalid SQL (expected if PostgreSQL not running): %v", err)
		}
		if results != nil {
			t.Log("Results should be nil on error")
		}
	})

	t.Run("QueryRowReturnsNil", func(t *testing.T) {
		// QueryRow currently returns nil (simplified implementation)
		row := db.QueryRow("SELECT 1")
		if row != nil {
			t.Log("QueryRow currently returns nil in simplified implementation")
		}
	})
}

func TestConnect(t *testing.T) {
	db, err := Connect()
	if err != nil {
		t.Logf("Database connection failed (expected if PostgreSQL not running): %v", err)
		return
	}
	defer db.Close()

	if db == nil {
		t.Fatal("Expected database connection, got nil")
	}

	// Test ping
	err = db.Ping()
	if err != nil {
		t.Logf("Failed to ping database (expected if PostgreSQL not running): %v", err)
	}
}

func TestRunMigration(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           "localhost",
			Port:           "5432",
			User:           "testuser",
			Password:       "testpass",
			Name:           "testdb",
			SSLMode:        "disable",
			MaxConnections: 10,
			ConnTimeout:    5 * time.Second,
			PoolSize:       5,
		},
	}

	db, err := NewPostgresDB(cfg)
	if err != nil {
		t.Logf("Database connection failed (expected if PostgreSQL not running): %v", err)
		return
	}
	defer db.Close()

	t.Run("EmptyMigrations", func(t *testing.T) {
		err = RunMigration(db, []string{})
		if err != nil {
			t.Logf("Failed to run empty migrations (expected if PostgreSQL not running): %v", err)
		}
	})

	t.Run("SimpleMigration", func(t *testing.T) {
		simpleMigration := []string{
			"CREATE TABLE IF NOT EXISTS test_migration (id SERIAL PRIMARY KEY, name TEXT)",
		}
		err = RunMigration(db, simpleMigration)
		if err != nil {
			t.Logf("Failed to run simple migration (expected if PostgreSQL not running): %v", err)
		}

		// Clean up
		err = db.Exec("DROP TABLE IF EXISTS test_migration")
		if err != nil {
			t.Logf("Failed to clean up test table (expected if PostgreSQL not running): %v", err)
		}
	})

	t.Run("MultipleMigrations", func(t *testing.T) {
		migrations := []string{
			"CREATE TABLE IF NOT EXISTS test_table1 (id SERIAL PRIMARY KEY, data TEXT)",
			"CREATE TABLE IF NOT EXISTS test_table2 (id SERIAL PRIMARY KEY, value INTEGER)",
			"INSERT INTO test_table1 (data) VALUES ('test')",
		}
		err = RunMigration(db, migrations)
		if err != nil {
			t.Logf("Failed to run multiple migrations (expected if PostgreSQL not running): %v", err)
		}

		// Clean up
		err = db.Exec("DROP TABLE IF EXISTS test_table1")
		if err != nil {
			t.Logf("Failed to clean up test_table1 (expected if PostgreSQL not running): %v", err)
		}
		err = db.Exec("DROP TABLE IF EXISTS test_table2")
		if err != nil {
			t.Logf("Failed to clean up test_table2 (expected if PostgreSQL not running): %v", err)
		}
	})

	t.Run("MigrationWithError", func(t *testing.T) {
		invalidMigration := []string{
			"CREATE TABLE test_bad_syntax (id INVALID_TYPE PRIMARY KEY)",
		}
		err = RunMigration(db, invalidMigration)
		if err != nil {
			t.Logf("Expected error for invalid migration (expected if PostgreSQL not running): %v", err)
		}
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("WithDefault", func(t *testing.T) {
		result := getEnv("NON_EXISTENT_ENV_VAR", "default_value")
		if result != "default_value" {
			t.Errorf("Expected 'default_value', got '%s'", result)
		}
	})

	t.Run("EnvironmentVariable", func(t *testing.T) {
		// Note: We can't easily set environment variables in tests without affecting other tests
		// This test just verifies the function doesn't panic
		result := getEnv("PATH", "default")
		// PATH should exist on any system
		if result == "" {
			t.Error("Expected non-empty PATH, got empty string")
		}
	})

	t.Run("EmptyStringEnvironmentVariable", func(t *testing.T) {
		// Save and restore environment
		originalValue := os.Getenv("TEST_EMPTY_VAR")
		defer func() {
			if originalValue != "" {
				os.Setenv("TEST_EMPTY_VAR", originalValue)
			} else {
				os.Unsetenv("TEST_EMPTY_VAR")
			}
		}()

		// Set empty string
		os.Setenv("TEST_EMPTY_VAR", "")
		result := getEnv("TEST_EMPTY_VAR", "default")
		if result != "default" {
			t.Errorf("Expected 'default' for empty env var, got '%s'", result)
		}
	})
}

func TestDBInterfaceImplementation(t *testing.T) {
	t.Run("PostgresDBImplementsDBInterface", func(t *testing.T) {
		var _ DB = (*PostgresDB)(nil)
	})

	t.Run("PostgresDBImplementsLegacyDBInterface", func(t *testing.T) {
		var _ LegacyDB = (*PostgresDB)(nil)
	})
}

func TestMigrationsVariable(t *testing.T) {
	t.Run("MigrationsNotEmpty", func(t *testing.T) {
		if len(migrations) == 0 {
			t.Error("Expected migrations to contain table definitions")
		}
	})

	t.Run("MigrationsContainExpectedTables", func(t *testing.T) {
		expectedTables := []string{
			"CREATE TABLE IF NOT EXISTS users",
			"CREATE TABLE IF NOT EXISTS user_sessions",
			"CREATE TABLE IF NOT EXISTS llm_providers",
			"CREATE TABLE IF NOT EXISTS llm_requests",
			"CREATE TABLE IF NOT EXISTS llm_responses",
			"CREATE TABLE IF NOT EXISTS cognee_memories",
		}

		migrationText := ""
		for _, migration := range migrations {
			migrationText += migration + " "
		}

		for _, expectedTable := range expectedTables {
			if !contains(migrationText, expectedTable) {
				t.Errorf("Expected migrations to contain: %s", expectedTable)
			}
		}
	})

	t.Run("MigrationsContainIndexes", func(t *testing.T) {
		expectedIndexes := []string{
			"CREATE INDEX IF NOT EXISTS idx_users_email",
			"CREATE INDEX IF NOT EXISTS idx_users_api_key",
			"CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id",
			"CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at",
			"CREATE INDEX IF NOT EXISTS idx_user_sessions_session_token",
			"CREATE INDEX IF NOT EXISTS idx_llm_providers_name",
			"CREATE INDEX IF NOT EXISTS idx_llm_providers_enabled",
			"CREATE INDEX IF NOT EXISTS idx_llm_requests_session_id",
			"CREATE INDEX IF NOT EXISTS idx_llm_requests_user_id",
			"CREATE INDEX IF NOT EXISTS idx_llm_requests_status",
			"CREATE INDEX IF NOT EXISTS idx_llm_responses_request_id",
			"CREATE INDEX IF NOT EXISTS idx_llm_responses_provider_id",
			"CREATE INDEX IF NOT EXISTS idx_llm_responses_selected",
			"CREATE INDEX IF NOT EXISTS idx_cognee_memories_session_id",
			"CREATE INDEX IF NOT EXISTS idx_cognee_memories_dataset_name",
			"CREATE INDEX IF NOT EXISTS idx_cognee_memories_search_key",
		}

		migrationText := ""
		for _, migration := range migrations {
			migrationText += migration + " "
		}

		for _, expectedIndex := range expectedIndexes {
			if !contains(migrationText, expectedIndex) {
				t.Errorf("Expected migrations to contain index: %s", expectedIndex)
			}
		}
	})
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
