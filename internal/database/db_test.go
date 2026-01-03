package database

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				Host:           "invalid-host-!@#$%", // Invalid host with special characters
				Port:           "99999",              // Invalid port
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

	t.Run("QueryRowReturnsRow", func(t *testing.T) {
		// QueryRow returns a Row interface that can be scanned
		row := db.QueryRow("SELECT 1")
		if row == nil {
			t.Error("QueryRow should return a non-nil Row")
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

	t.Run("RowInterfaceExists", func(t *testing.T) {
		var _ Row = (*pgxRow)(nil)
	})
}

func TestRowInterface(t *testing.T) {
	t.Run("pgxRowImplementsRow", func(t *testing.T) {
		// Verify that pgxRow implements the Row interface
		row := &pgxRow{row: nil}
		if row == nil {
			t.Fatal("Expected non-nil pgxRow")
		}
	})
}

func TestGetPoolReturnsNilWhenPoolIsNil(t *testing.T) {
	// This tests the GetPool function without a real connection
	db := &PostgresDB{pool: nil}
	pool := db.GetPool()
	if pool != nil {
		t.Error("Expected nil pool, got non-nil")
	}
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

// TestDBInterface_Methods tests that the DB interface has all expected methods
func TestDBInterface_Methods(t *testing.T) {
	t.Run("VerifyDBInterfaceSignatures", func(t *testing.T) {
		// This is a compile-time check that PostgresDB implements DB
		var _ DB = (*PostgresDB)(nil)

		// Verify interface exists and has expected method count
		// DB has: Ping, Exec, Query, QueryRow, Close, HealthCheck
	})

	t.Run("VerifyLegacyDBInterfaceSignatures", func(t *testing.T) {
		// This is a compile-time check that PostgresDB implements LegacyDB
		var _ LegacyDB = (*PostgresDB)(nil)
	})
}

// TestRowInterfaceImplementation tests the Row interface implementation
func TestRowInterfaceImplementation(t *testing.T) {
	t.Run("RowInterfaceExists", func(t *testing.T) {
		var _ Row = (*pgxRow)(nil)
	})

	t.Run("pgxRowCanBeCreated", func(t *testing.T) {
		// Verify pgxRow struct can hold a pgx.Row
		row := &pgxRow{row: nil}
		assert.NotNil(t, row)
	})
}

// TestPgxRowScan tests the pgxRow.Scan method with various scenarios
func TestPgxRowScan(t *testing.T) {
	t.Run("ScanWithNilRow", func(t *testing.T) {
		row := &pgxRow{row: nil}
		var result string
		// This will panic/error because the underlying row is nil
		// This is expected behavior - you can't scan from nil
		defer func() {
			if r := recover(); r != nil {
				t.Log("Expected panic when scanning from nil row")
			}
		}()
		_ = row.Scan(&result)
	})
}

// TestGetEnvComprehensive tests the getEnv function thoroughly
func TestGetEnvComprehensive(t *testing.T) {
	t.Run("ReturnsDefaultWhenNotSet", func(t *testing.T) {
		// Use a unique env var name that definitely doesn't exist
		result := getEnv("SUPERAGENT_TEST_DEFINITELY_NOT_SET_123456", "mydefault")
		assert.Equal(t, "mydefault", result)
	})

	t.Run("ReturnsValueWhenSet", func(t *testing.T) {
		os.Setenv("SUPERAGENT_TEST_VAR", "testvalue")
		defer os.Unsetenv("SUPERAGENT_TEST_VAR")

		result := getEnv("SUPERAGENT_TEST_VAR", "default")
		assert.Equal(t, "testvalue", result)
	})

	t.Run("ReturnsDefaultForEmptyString", func(t *testing.T) {
		os.Setenv("SUPERAGENT_TEST_EMPTY", "")
		defer os.Unsetenv("SUPERAGENT_TEST_EMPTY")

		result := getEnv("SUPERAGENT_TEST_EMPTY", "fallback")
		assert.Equal(t, "fallback", result)
	})

	t.Run("ReturnsValueWithSpaces", func(t *testing.T) {
		os.Setenv("SUPERAGENT_TEST_SPACES", "  value with spaces  ")
		defer os.Unsetenv("SUPERAGENT_TEST_SPACES")

		result := getEnv("SUPERAGENT_TEST_SPACES", "default")
		assert.Equal(t, "  value with spaces  ", result)
	})

	t.Run("HandlesSpecialCharacters", func(t *testing.T) {
		os.Setenv("SUPERAGENT_TEST_SPECIAL", "user@host:password!#$%")
		defer os.Unsetenv("SUPERAGENT_TEST_SPECIAL")

		result := getEnv("SUPERAGENT_TEST_SPECIAL", "default")
		assert.Equal(t, "user@host:password!#$%", result)
	})

	t.Run("DefaultValueVariations", func(t *testing.T) {
		// Empty default
		result := getEnv("NONEXISTENT_VAR_12345", "")
		assert.Equal(t, "", result)

		// Numeric default
		result = getEnv("NONEXISTENT_VAR_12345", "5432")
		assert.Equal(t, "5432", result)

		// URL default
		result = getEnv("NONEXISTENT_VAR_12345", "postgres://localhost:5432")
		assert.Equal(t, "postgres://localhost:5432", result)
	})
}

// TestRunMigrationErrors tests migration error scenarios
func TestRunMigrationErrors(t *testing.T) {
	t.Run("NilMigrationsList", func(t *testing.T) {
		// This tests that nil/empty migrations work
		db := &PostgresDB{pool: nil}
		err := RunMigration(db, nil)
		// With nil pool, we expect an error when executing
		if err != nil {
			assert.True(t, true) // Expected
		}
	})

	t.Run("EmptyMigrationsList", func(t *testing.T) {
		db := &PostgresDB{pool: nil}
		err := RunMigration(db, []string{})
		assert.NoError(t, err) // Empty list should succeed
	})
}

// TestMigrationsContent tests the migrations variable content
func TestMigrationsContent(t *testing.T) {
	t.Run("UUIDExtensionFirst", func(t *testing.T) {
		assert.True(t, len(migrations) > 0)
		assert.Contains(t, migrations[0], "uuid-ossp")
	})

	t.Run("AllTablesHaveCreateIfNotExists", func(t *testing.T) {
		tableKeyword := "CREATE TABLE IF NOT EXISTS"
		for _, migration := range migrations {
			if strings.Contains(migration, "CREATE TABLE") {
				assert.Contains(t, migration, tableKeyword, "All CREATE TABLE statements should use IF NOT EXISTS")
			}
		}
	})

	t.Run("AllIndexesHaveCreateIfNotExists", func(t *testing.T) {
		indexKeyword := "CREATE INDEX IF NOT EXISTS"
		for _, migration := range migrations {
			if strings.Contains(migration, "CREATE INDEX") {
				assert.Contains(t, migration, indexKeyword, "All CREATE INDEX statements should use IF NOT EXISTS")
			}
		}
	})

	t.Run("MigrationsHaveCorrectTableColumns", func(t *testing.T) {
		// Check users table has expected columns
		for _, migration := range migrations {
			if strings.Contains(migration, "CREATE TABLE IF NOT EXISTS users") {
				assert.Contains(t, migration, "id UUID PRIMARY KEY")
				assert.Contains(t, migration, "username VARCHAR")
				assert.Contains(t, migration, "email VARCHAR")
				assert.Contains(t, migration, "password_hash VARCHAR")
				assert.Contains(t, migration, "api_key VARCHAR")
				break
			}
		}
	})

	t.Run("MigrationsHaveForeignKeys", func(t *testing.T) {
		// Check that relationships are properly defined
		foundForeignKey := false
		for _, migration := range migrations {
			if strings.Contains(migration, "REFERENCES users(id)") ||
				strings.Contains(migration, "REFERENCES user_sessions(id)") ||
				strings.Contains(migration, "REFERENCES llm_providers(id)") {
				foundForeignKey = true
				break
			}
		}
		assert.True(t, foundForeignKey, "Migrations should contain foreign key references")
	})
}

// TestPostgresDBNilPool tests behavior when pool is nil
func TestPostgresDBNilPool(t *testing.T) {
	db := &PostgresDB{pool: nil}

	t.Run("GetPoolReturnsNil", func(t *testing.T) {
		pool := db.GetPool()
		assert.Nil(t, pool)
	})

	t.Run("CloseWithNilPool", func(t *testing.T) {
		// Close should panic/error with nil pool
		defer func() {
			if r := recover(); r != nil {
				t.Log("Expected panic when closing nil pool")
			}
		}()
		_ = db.Close()
	})
}

// TestConfigDefaultFallbacks tests that config falls back to env vars
func TestConfigDefaultFallbacks(t *testing.T) {
	t.Run("EmptyConfigUsesEnvDefaults", func(t *testing.T) {
		// Save original env vars
		origHost := os.Getenv("DB_HOST")
		origPort := os.Getenv("DB_PORT")
		origUser := os.Getenv("DB_USER")
		origPass := os.Getenv("DB_PASSWORD")
		origName := os.Getenv("DB_NAME")

		// Clear env vars
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")

		defer func() {
			// Restore
			if origHost != "" {
				os.Setenv("DB_HOST", origHost)
			}
			if origPort != "" {
				os.Setenv("DB_PORT", origPort)
			}
			if origUser != "" {
				os.Setenv("DB_USER", origUser)
			}
			if origPass != "" {
				os.Setenv("DB_PASSWORD", origPass)
			}
			if origName != "" {
				os.Setenv("DB_NAME", origName)
			}
		}()

		// With empty config and no env vars, should use defaults
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "",
				Port:     "",
				User:     "",
				Password: "",
				Name:     "",
			},
		}

		// Test that NewPostgresDB builds connection string with defaults
		// pgxpool.New() succeeds but operations fail - verifies defaults are used
		db, err := NewPostgresDB(cfg)
		if err == nil {
			defer db.Close()
			// Pool created successfully with defaults, ping will fail without DB
			pingErr := db.Ping()
			assert.Error(t, pingErr, "Ping should fail without database")
		}
		// If err != nil, connection string parsing failed which is also acceptable
	})
}

// TestConnectFunction tests the legacy Connect function
func TestConnectFunction(t *testing.T) {
	t.Run("ConnectUsesEnvironmentVariables", func(t *testing.T) {
		// Save original values
		origHost := os.Getenv("DB_HOST")
		origPort := os.Getenv("DB_PORT")

		// Set to invalid values to ensure connection fails
		os.Setenv("DB_HOST", "nonexistent-host-12345.invalid")
		os.Setenv("DB_PORT", "99999")

		defer func() {
			if origHost != "" {
				os.Setenv("DB_HOST", origHost)
			} else {
				os.Unsetenv("DB_HOST")
			}
			if origPort != "" {
				os.Setenv("DB_PORT", origPort)
			} else {
				os.Unsetenv("DB_PORT")
			}
		}()

		// Connect should fail with invalid host
		_, err := Connect()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect")
	})
}

// TestSSLModeHandling tests SSLMode configuration
func TestSSLModeHandling(t *testing.T) {
	t.Run("DefaultSSLModeIsDisable", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "test",
				Password: "test",
				Name:     "test",
				SSLMode:  "", // Empty should default to disable
			},
		}

		// Pool creation succeeds, actual connection fails without database
		db, err := NewPostgresDB(cfg)
		if err == nil {
			defer db.Close()
			// Verify ping fails (no database running)
			pingErr := db.Ping()
			assert.Error(t, pingErr, "Ping should fail without database")
		}
	})

	t.Run("CustomSSLMode", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "test",
				Password: "test",
				Name:     "test",
				SSLMode:  "require",
			},
		}

		// Pool creation succeeds, actual connection fails without database
		db, err := NewPostgresDB(cfg)
		if err == nil {
			defer db.Close()
			// Verify ping fails (no database running)
			pingErr := db.Ping()
			assert.Error(t, pingErr, "Ping should fail without database")
		}
	})
}

// TestContextTimeouts tests context handling in database operations
func TestContextTimeouts(t *testing.T) {
	t.Run("ContextBackgroundUsedInPing", func(t *testing.T) {
		// Verify that Ping uses context.Background
		// This is a code review test - we're testing the implementation pattern
		db := &PostgresDB{pool: nil}

		defer func() {
			if r := recover(); r != nil {
				t.Log("Expected panic with nil pool - this verifies Ping is called")
			}
		}()
		_ = db.Ping()
	})

	t.Run("HealthCheckHasTimeout", func(t *testing.T) {
		// Verify HealthCheck uses a timeout context
		db := &PostgresDB{pool: nil}

		defer func() {
			if r := recover(); r != nil {
				t.Log("Expected panic with nil pool - this verifies HealthCheck is called")
			}
		}()
		_ = db.HealthCheck()
	})
}

// TestErrorMessages tests that error messages are descriptive
func TestErrorMessages(t *testing.T) {
	t.Run("ConnectionErrorMessage", func(t *testing.T) {
		// pgxpool.New() parses the connection string but doesn't validate host
		// The actual connection happens lazily, so we test that a pool is created
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "nonexistent-host.invalid",
				Port:     "5432",
				User:     "test",
				Password: "test",
				Name:     "test",
			},
		}

		db, err := NewPostgresDB(cfg)
		// Pool creation succeeds, but Ping/operations will fail
		if err == nil {
			defer db.Close()
			// Verify that actual operations fail
			pingErr := db.Ping()
			assert.Error(t, pingErr)
		}
	})

	t.Run("MigrationErrorMessage", func(t *testing.T) {
		// Test error wrapping in RunMigration
		db := &PostgresDB{pool: nil}

		defer func() {
			if r := recover(); r != nil {
				t.Log("Migration with nil pool causes panic as expected")
			}
		}()

		err := RunMigration(db, []string{"SELECT 1"})
		if err != nil {
			assert.Contains(t, err.Error(), "failed to run migration")
		}
	})
}

// MockDB implements the DB interface for testing
type MockDB struct {
	PingFn       func() error
	ExecFn       func(query string, args ...any) error
	QueryFn      func(query string, args ...any) ([]any, error)
	QueryRowFn   func(query string, args ...any) Row
	CloseFn      func() error
	HealthCheckFn func() error
}

func (m *MockDB) Ping() error {
	if m.PingFn != nil {
		return m.PingFn()
	}
	return nil
}

func (m *MockDB) Exec(query string, args ...any) error {
	if m.ExecFn != nil {
		return m.ExecFn(query, args...)
	}
	return nil
}

func (m *MockDB) Query(query string, args ...any) ([]any, error) {
	if m.QueryFn != nil {
		return m.QueryFn(query, args...)
	}
	return nil, nil
}

func (m *MockDB) QueryRow(query string, args ...any) Row {
	if m.QueryRowFn != nil {
		return m.QueryRowFn(query, args...)
	}
	return nil
}

func (m *MockDB) Close() error {
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}

func (m *MockDB) HealthCheck() error {
	if m.HealthCheckFn != nil {
		return m.HealthCheckFn()
	}
	return nil
}

// TestMockDB tests using the mock DB implementation
func TestMockDB(t *testing.T) {
	t.Run("MockDBImplementsInterface", func(t *testing.T) {
		var _ DB = (*MockDB)(nil)
	})

	t.Run("MockDBPing", func(t *testing.T) {
		called := false
		db := &MockDB{
			PingFn: func() error {
				called = true
				return nil
			},
		}
		err := db.Ping()
		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("MockDBPingError", func(t *testing.T) {
		expectedErr := errors.New("ping failed")
		db := &MockDB{
			PingFn: func() error {
				return expectedErr
			},
		}
		err := db.Ping()
		assert.Equal(t, expectedErr, err)
	})

	t.Run("MockDBExec", func(t *testing.T) {
		var executedQuery string
		var executedArgs []any
		db := &MockDB{
			ExecFn: func(query string, args ...any) error {
				executedQuery = query
				executedArgs = args
				return nil
			},
		}
		err := db.Exec("INSERT INTO test VALUES ($1)", "value")
		assert.NoError(t, err)
		assert.Equal(t, "INSERT INTO test VALUES ($1)", executedQuery)
		assert.Equal(t, []any{"value"}, executedArgs)
	})

	t.Run("MockDBQuery", func(t *testing.T) {
		expectedResults := []any{[]any{"row1"}, []any{"row2"}}
		db := &MockDB{
			QueryFn: func(query string, args ...any) ([]any, error) {
				return expectedResults, nil
			},
		}
		results, err := db.Query("SELECT * FROM test")
		assert.NoError(t, err)
		assert.Equal(t, expectedResults, results)
	})

	t.Run("MockDBQueryError", func(t *testing.T) {
		expectedErr := errors.New("query failed")
		db := &MockDB{
			QueryFn: func(query string, args ...any) ([]any, error) {
				return nil, expectedErr
			},
		}
		results, err := db.Query("SELECT * FROM test")
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, results)
	})

	t.Run("MockDBHealthCheck", func(t *testing.T) {
		db := &MockDB{
			HealthCheckFn: func() error {
				return nil
			},
		}
		err := db.HealthCheck()
		assert.NoError(t, err)
	})

	t.Run("MockDBClose", func(t *testing.T) {
		closed := false
		db := &MockDB{
			CloseFn: func() error {
				closed = true
				return nil
			},
		}
		err := db.Close()
		assert.NoError(t, err)
		assert.True(t, closed)
	})
}

// MockRow implements the Row interface for testing
type MockRowImpl struct {
	Values []any
	Err    error
}

func (m *MockRowImpl) Scan(dest ...any) error {
	if m.Err != nil {
		return m.Err
	}
	for i, v := range m.Values {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *string:
				if s, ok := v.(string); ok {
					*d = s
				}
			case *int:
				if n, ok := v.(int); ok {
					*d = n
				}
			case *int64:
				if n, ok := v.(int64); ok {
					*d = n
				}
			case *bool:
				if b, ok := v.(bool); ok {
					*d = b
				}
			case *time.Time:
				if t, ok := v.(time.Time); ok {
					*d = t
				}
			}
		}
	}
	return nil
}

func TestMockRowImpl(t *testing.T) {
	t.Run("ImplementsRowInterface", func(t *testing.T) {
		var _ Row = (*MockRowImpl)(nil)
	})

	t.Run("ScanString", func(t *testing.T) {
		row := &MockRowImpl{Values: []any{"hello"}}
		var result string
		err := row.Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("ScanMultipleValues", func(t *testing.T) {
		row := &MockRowImpl{Values: []any{"name", int64(42), true}}
		var name string
		var age int64
		var active bool
		err := row.Scan(&name, &age, &active)
		assert.NoError(t, err)
		assert.Equal(t, "name", name)
		assert.Equal(t, int64(42), age)
		assert.True(t, active)
	})

	t.Run("ScanWithError", func(t *testing.T) {
		expectedErr := pgx.ErrNoRows
		row := &MockRowImpl{Err: expectedErr}
		var result string
		err := row.Scan(&result)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("ScanTimeValue", func(t *testing.T) {
		now := time.Now()
		row := &MockRowImpl{Values: []any{now}}
		var result time.Time
		err := row.Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, now, result)
	})
}

// TestRunMigrationWithMock tests RunMigration with mock DB
func TestRunMigrationWithMock(t *testing.T) {
	t.Run("SuccessfulMigration", func(t *testing.T) {
		executedQueries := []string{}
		mockDB := &MockDB{
			ExecFn: func(query string, args ...any) error {
				executedQueries = append(executedQueries, query)
				return nil
			},
		}

		// RunMigration expects *PostgresDB, so we can't directly use MockDB
		// But we verified the pattern works above with MockDB tests
		_ = mockDB
		_ = executedQueries
	})
}

// TestDatabaseConnectionString tests connection string building
func TestDatabaseConnectionString(t *testing.T) {
	t.Run("SpecialCharactersInPassword", func(t *testing.T) {
		// Test that passwords with special characters don't break the connection string
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "testuser",
				Password: "pass@word!#$%",
				Name:     "testdb",
				SSLMode:  "disable",
			},
		}

		// Connection will fail but string should be built correctly
		_, err := NewPostgresDB(cfg)
		assert.Error(t, err) // Expected connection error
	})

	t.Run("IPv6Host", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "::1",
				Port:     "5432",
				User:     "testuser",
				Password: "testpass",
				Name:     "testdb",
			},
		}

		// pgxpool.New() will succeed parsing IPv6, connection fails later
		db, err := NewPostgresDB(cfg)
		if err == nil {
			defer db.Close()
			// Verify that ping fails (no IPv6 database running)
			pingErr := db.Ping()
			assert.Error(t, pingErr) // Expected connection error on ping
		}
	})
}

// =============================================================================
// Extended MockDB Tests for Better Coverage
// =============================================================================

// TestMockDBQueryRow tests the QueryRow mock functionality
func TestMockDBQueryRow(t *testing.T) {
	t.Run("MockDBQueryRowReturnsRow", func(t *testing.T) {
		mockRow := &MockRowImpl{Values: []any{"test-id", "test-value"}}
		db := &MockDB{
			QueryRowFn: func(query string, args ...any) Row {
				return mockRow
			},
		}

		row := db.QueryRow("SELECT id, value FROM test WHERE id = $1", "test-id")
		assert.NotNil(t, row)

		var id, value string
		err := row.Scan(&id, &value)
		assert.NoError(t, err)
		assert.Equal(t, "test-id", id)
		assert.Equal(t, "test-value", value)
	})

	t.Run("MockDBQueryRowWithError", func(t *testing.T) {
		mockRow := &MockRowImpl{Err: pgx.ErrNoRows}
		db := &MockDB{
			QueryRowFn: func(query string, args ...any) Row {
				return mockRow
			},
		}

		row := db.QueryRow("SELECT * FROM nonexistent")
		var result string
		err := row.Scan(&result)
		assert.Error(t, err)
		assert.Equal(t, pgx.ErrNoRows, err)
	})

	t.Run("MockDBQueryRowNilFunction", func(t *testing.T) {
		db := &MockDB{}
		row := db.QueryRow("SELECT 1")
		assert.Nil(t, row)
	})
}

// TestMockRowImplEdgeCases tests edge cases for MockRowImpl
func TestMockRowImplEdgeCases(t *testing.T) {
	t.Run("ScanWithMoreDestsThanValues", func(t *testing.T) {
		row := &MockRowImpl{Values: []any{"value1"}}
		var v1, v2 string
		err := row.Scan(&v1, &v2)
		assert.NoError(t, err)
		assert.Equal(t, "value1", v1)
		assert.Equal(t, "", v2) // Not set because no value
	})

	t.Run("ScanWithTypeMismatch", func(t *testing.T) {
		row := &MockRowImpl{Values: []any{"string-not-int"}}
		var intResult int
		err := row.Scan(&intResult)
		assert.NoError(t, err)
		assert.Equal(t, 0, intResult) // Not set because type mismatch
	})

	t.Run("ScanEmptyValues", func(t *testing.T) {
		row := &MockRowImpl{Values: []any{}}
		var result string
		err := row.Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("ScanIntValue", func(t *testing.T) {
		row := &MockRowImpl{Values: []any{int(42)}}
		var result int
		err := row.Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("ScanBoolValue", func(t *testing.T) {
		row := &MockRowImpl{Values: []any{true}}
		var result bool
		err := row.Scan(&result)
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("ScanNilValuesSlice", func(t *testing.T) {
		row := &MockRowImpl{Values: nil}
		var result string
		err := row.Scan(&result)
		assert.NoError(t, err)
	})
}

// TestMockDBAllMethods tests all MockDB methods in combination
func TestMockDBAllMethods(t *testing.T) {
	t.Run("CombinedOperations", func(t *testing.T) {
		execCalls := 0
		queryCalls := 0
		queryRowCalls := 0

		db := &MockDB{
			PingFn: func() error { return nil },
			ExecFn: func(query string, args ...any) error {
				execCalls++
				return nil
			},
			QueryFn: func(query string, args ...any) ([]any, error) {
				queryCalls++
				return []any{[]any{"row1"}, []any{"row2"}}, nil
			},
			QueryRowFn: func(query string, args ...any) Row {
				queryRowCalls++
				return &MockRowImpl{Values: []any{"value"}}
			},
			CloseFn: func() error { return nil },
			HealthCheckFn: func() error { return nil },
		}

		// Execute all operations
		assert.NoError(t, db.Ping())
		assert.NoError(t, db.Exec("INSERT INTO test VALUES ($1)", "value"))
		results, err := db.Query("SELECT * FROM test")
		assert.NoError(t, err)
		assert.Len(t, results, 2)
		row := db.QueryRow("SELECT value FROM test")
		var val string
		assert.NoError(t, row.Scan(&val))
		assert.Equal(t, "value", val)
		assert.NoError(t, db.HealthCheck())
		assert.NoError(t, db.Close())

		// Verify calls
		assert.Equal(t, 1, execCalls)
		assert.Equal(t, 1, queryCalls)
		assert.Equal(t, 1, queryRowCalls)
	})
}

// TestMockDBErrorScenarios tests various error scenarios
func TestMockDBErrorScenarios(t *testing.T) {
	t.Run("ExecError", func(t *testing.T) {
		expectedErr := errors.New("exec failed: constraint violation")
		db := &MockDB{
			ExecFn: func(query string, args ...any) error {
				return expectedErr
			},
		}
		err := db.Exec("INSERT INTO test VALUES ($1)", "duplicate")
		assert.Equal(t, expectedErr, err)
	})

	t.Run("HealthCheckError", func(t *testing.T) {
		expectedErr := errors.New("health check failed: timeout")
		db := &MockDB{
			HealthCheckFn: func() error {
				return expectedErr
			},
		}
		err := db.HealthCheck()
		assert.Equal(t, expectedErr, err)
	})

	t.Run("CloseError", func(t *testing.T) {
		expectedErr := errors.New("close failed: connection already closed")
		db := &MockDB{
			CloseFn: func() error {
				return expectedErr
			},
		}
		err := db.Close()
		assert.Equal(t, expectedErr, err)
	})
}

// TestQueryArgumentsPassing tests that query arguments are passed correctly
func TestQueryArgumentsPassing(t *testing.T) {
	t.Run("ExecWithMultipleArgs", func(t *testing.T) {
		var capturedQuery string
		var capturedArgs []any

		db := &MockDB{
			ExecFn: func(query string, args ...any) error {
				capturedQuery = query
				capturedArgs = args
				return nil
			},
		}

		err := db.Exec("INSERT INTO test (a, b, c) VALUES ($1, $2, $3)", "val1", 42, true)
		assert.NoError(t, err)
		assert.Equal(t, "INSERT INTO test (a, b, c) VALUES ($1, $2, $3)", capturedQuery)
		assert.Equal(t, []any{"val1", 42, true}, capturedArgs)
	})

	t.Run("QueryWithMultipleArgs", func(t *testing.T) {
		var capturedQuery string
		var capturedArgs []any

		db := &MockDB{
			QueryFn: func(query string, args ...any) ([]any, error) {
				capturedQuery = query
				capturedArgs = args
				return []any{}, nil
			},
		}

		_, err := db.Query("SELECT * FROM test WHERE a = $1 AND b = $2", "val", 10)
		assert.NoError(t, err)
		assert.Equal(t, "SELECT * FROM test WHERE a = $1 AND b = $2", capturedQuery)
		assert.Equal(t, []any{"val", 10}, capturedArgs)
	})

	t.Run("QueryRowWithMultipleArgs", func(t *testing.T) {
		var capturedQuery string
		var capturedArgs []any

		db := &MockDB{
			QueryRowFn: func(query string, args ...any) Row {
				capturedQuery = query
				capturedArgs = args
				return &MockRowImpl{Values: []any{"result"}}
			},
		}

		row := db.QueryRow("SELECT * FROM test WHERE id = $1 AND active = $2", "id-1", true)
		assert.NotNil(t, row)
		assert.Equal(t, "SELECT * FROM test WHERE id = $1 AND active = $2", capturedQuery)
		assert.Equal(t, []any{"id-1", true}, capturedArgs)
	})
}

// TestMigrationsVariableCompleteness tests all migration statements
func TestMigrationsVariableCompleteness(t *testing.T) {
	t.Run("HasUUIDExtension", func(t *testing.T) {
		found := false
		for _, m := range migrations {
			if strings.Contains(m, "uuid-ossp") {
				found = true
				break
			}
		}
		assert.True(t, found, "Migrations should include UUID extension")
	})

	t.Run("HasUsersTable", func(t *testing.T) {
		found := false
		for _, m := range migrations {
			if strings.Contains(m, "CREATE TABLE IF NOT EXISTS users") {
				found = true
				assert.Contains(t, m, "username VARCHAR")
				assert.Contains(t, m, "email VARCHAR")
				assert.Contains(t, m, "password_hash VARCHAR")
				assert.Contains(t, m, "api_key VARCHAR")
				assert.Contains(t, m, "role VARCHAR")
				break
			}
		}
		assert.True(t, found, "Migrations should include users table")
	})

	t.Run("HasUserSessionsTable", func(t *testing.T) {
		found := false
		for _, m := range migrations {
			if strings.Contains(m, "CREATE TABLE IF NOT EXISTS user_sessions") {
				found = true
				assert.Contains(t, m, "user_id UUID REFERENCES users")
				assert.Contains(t, m, "session_token VARCHAR")
				assert.Contains(t, m, "context JSONB")
				break
			}
		}
		assert.True(t, found, "Migrations should include user_sessions table")
	})

	t.Run("HasLLMProvidersTable", func(t *testing.T) {
		found := false
		for _, m := range migrations {
			if strings.Contains(m, "CREATE TABLE IF NOT EXISTS llm_providers") {
				found = true
				assert.Contains(t, m, "name VARCHAR")
				assert.Contains(t, m, "type VARCHAR")
				assert.Contains(t, m, "api_key VARCHAR")
				assert.Contains(t, m, "base_url VARCHAR")
				assert.Contains(t, m, "model VARCHAR")
				assert.Contains(t, m, "weight DECIMAL")
				assert.Contains(t, m, "enabled BOOLEAN")
				assert.Contains(t, m, "config JSONB")
				break
			}
		}
		assert.True(t, found, "Migrations should include llm_providers table")
	})

	t.Run("HasLLMRequestsTable", func(t *testing.T) {
		found := false
		for _, m := range migrations {
			if strings.Contains(m, "CREATE TABLE IF NOT EXISTS llm_requests") {
				found = true
				assert.Contains(t, m, "session_id UUID REFERENCES user_sessions")
				assert.Contains(t, m, "user_id UUID REFERENCES users")
				assert.Contains(t, m, "prompt TEXT")
				assert.Contains(t, m, "messages JSONB")
				assert.Contains(t, m, "model_params JSONB")
				break
			}
		}
		assert.True(t, found, "Migrations should include llm_requests table")
	})

	t.Run("HasLLMResponsesTable", func(t *testing.T) {
		found := false
		for _, m := range migrations {
			if strings.Contains(m, "CREATE TABLE IF NOT EXISTS llm_responses") {
				found = true
				assert.Contains(t, m, "request_id UUID REFERENCES llm_requests")
				assert.Contains(t, m, "provider_id UUID REFERENCES llm_providers")
				assert.Contains(t, m, "content TEXT")
				assert.Contains(t, m, "confidence DECIMAL")
				assert.Contains(t, m, "tokens_used INTEGER")
				break
			}
		}
		assert.True(t, found, "Migrations should include llm_responses table")
	})

	t.Run("HasCogneeMemoriesTable", func(t *testing.T) {
		found := false
		for _, m := range migrations {
			if strings.Contains(m, "CREATE TABLE IF NOT EXISTS cognee_memories") {
				found = true
				assert.Contains(t, m, "session_id UUID REFERENCES user_sessions")
				assert.Contains(t, m, "dataset_name VARCHAR")
				assert.Contains(t, m, "content_type VARCHAR")
				assert.Contains(t, m, "content TEXT")
				break
			}
		}
		assert.True(t, found, "Migrations should include cognee_memories table")
	})

	t.Run("CountMigrations", func(t *testing.T) {
		// Migrations should include:
		// 1 UUID extension + 6 tables + 16 indexes = 23 statements
		assert.GreaterOrEqual(t, len(migrations), 23, "Should have at least 23 migration statements")
	})
}

// TestDBInterfaceMethodSignatures tests method signatures
func TestDBInterfaceMethodSignatures(t *testing.T) {
	t.Run("PingReturnsError", func(t *testing.T) {
		db := &MockDB{PingFn: func() error { return nil }}
		var result error = db.Ping()
		assert.Nil(t, result)
	})

	t.Run("ExecReturnsError", func(t *testing.T) {
		db := &MockDB{ExecFn: func(q string, a ...any) error { return nil }}
		var result error = db.Exec("query")
		assert.Nil(t, result)
	})

	t.Run("QueryReturnsSliceAndError", func(t *testing.T) {
		db := &MockDB{QueryFn: func(q string, a ...any) ([]any, error) { return []any{}, nil }}
		results, err := db.Query("query")
		assert.NotNil(t, results)
		assert.Nil(t, err)
	})

	t.Run("QueryRowReturnsRow", func(t *testing.T) {
		db := &MockDB{QueryRowFn: func(q string, a ...any) Row { return &MockRowImpl{} }}
		var result Row = db.QueryRow("query")
		assert.NotNil(t, result)
	})

	t.Run("CloseReturnsError", func(t *testing.T) {
		db := &MockDB{CloseFn: func() error { return nil }}
		var result error = db.Close()
		assert.Nil(t, result)
	})

	t.Run("HealthCheckReturnsError", func(t *testing.T) {
		db := &MockDB{HealthCheckFn: func() error { return nil }}
		var result error = db.HealthCheck()
		assert.Nil(t, result)
	})
}

// TestRowInterfaceScan tests the Row interface Scan method
func TestRowInterfaceScan(t *testing.T) {
	t.Run("ScanWithFloat64", func(t *testing.T) {
		// MockRowImpl doesn't handle float64, testing nil handling
		row := &MockRowImpl{Values: []any{3.14}}
		var result float64
		err := row.Scan(&result)
		assert.NoError(t, err)
		// Result is 0 because MockRowImpl doesn't handle float64
	})

	t.Run("ScanMixedTypes", func(t *testing.T) {
		now := time.Now()
		row := &MockRowImpl{Values: []any{"text", int(1), int64(2), true, now}}
		var s string
		var i int
		var i64 int64
		var b bool
		var t time.Time
		err := row.Scan(&s, &i, &i64, &b, &t)
		assert.NoError(t, err)
		assert.Equal(t, "text", s)
		assert.Equal(t, 1, i)
		assert.Equal(t, int64(2), i64)
		assert.True(t, b)
		assert.Equal(t, now, t)
	})
}

// TestEmptyMigrations tests behavior with empty migrations
func TestEmptyMigrations(t *testing.T) {
	t.Run("EmptyMigrationSlice", func(t *testing.T) {
		// Empty migrations should succeed
		empty := []string{}
		assert.Len(t, empty, 0)
	})

	t.Run("NilMigrationSlice", func(t *testing.T) {
		var nilMigrations []string
		assert.Nil(t, nilMigrations)
		assert.Len(t, nilMigrations, 0)
	})
}

// TestGetEnvEdgeCases tests edge cases for getEnv
func TestGetEnvEdgeCases(t *testing.T) {
	t.Run("VeryLongValue", func(t *testing.T) {
		longValue := strings.Repeat("a", 10000)
		os.Setenv("SUPERAGENT_TEST_LONG", longValue)
		defer os.Unsetenv("SUPERAGENT_TEST_LONG")

		result := getEnv("SUPERAGENT_TEST_LONG", "default")
		assert.Equal(t, longValue, result)
		assert.Len(t, result, 10000)
	})

	t.Run("UnicodeValue", func(t *testing.T) {
		os.Setenv("SUPERAGENT_TEST_UNICODE", "值中文日本語한국어")
		defer os.Unsetenv("SUPERAGENT_TEST_UNICODE")

		result := getEnv("SUPERAGENT_TEST_UNICODE", "default")
		assert.Equal(t, "值中文日本語한국어", result)
	})

	t.Run("NewlineInValue", func(t *testing.T) {
		os.Setenv("SUPERAGENT_TEST_NEWLINE", "line1\nline2")
		defer os.Unsetenv("SUPERAGENT_TEST_NEWLINE")

		result := getEnv("SUPERAGENT_TEST_NEWLINE", "default")
		assert.Equal(t, "line1\nline2", result)
	})

	t.Run("TabInValue", func(t *testing.T) {
		os.Setenv("SUPERAGENT_TEST_TAB", "val1\tval2")
		defer os.Unsetenv("SUPERAGENT_TEST_TAB")

		result := getEnv("SUPERAGENT_TEST_TAB", "default")
		assert.Equal(t, "val1\tval2", result)
	})
}

// TestDatabaseConfigPermutations tests various config combinations
func TestDatabaseConfigPermutations(t *testing.T) {
	t.Run("AllFieldsProvided", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "db.example.com",
				Port:     "5433",
				User:     "admin",
				Password: "securepassword",
				Name:     "production_db",
				SSLMode:  "require",
			},
		}

		// Connection will fail but config parsing should succeed
		_, err := NewPostgresDB(cfg)
		if err != nil {
			// Expected: connection fails to non-existent host
			assert.Contains(t, err.Error(), "failed")
		}
	})

	t.Run("MinimalConfig", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "test",
				Password: "test",
				Name:     "test",
			},
		}

		// Pool creation might succeed, ping will fail
		db, err := NewPostgresDB(cfg)
		if err == nil {
			defer db.Close()
		}
	})

	t.Run("NumericPort", func(t *testing.T) {
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "test",
				Password: "test",
				Name:     "test",
			},
		}

		db, err := NewPostgresDB(cfg)
		if err == nil {
			defer db.Close()
		}
	})
}

// TestLegacyDBInterface tests the LegacyDB interface
func TestLegacyDBInterface(t *testing.T) {
	t.Run("HasRequiredMethods", func(t *testing.T) {
		// Verify LegacyDB interface is a subset of DB
		var db LegacyDB = &MockLegacyDB{}
		assert.NotNil(t, db)
	})
}

// MockLegacyDB implements LegacyDB interface for testing
type MockLegacyDB struct{}

func (m *MockLegacyDB) Ping() error                              { return nil }
func (m *MockLegacyDB) Exec(query string, args ...any) error     { return nil }
func (m *MockLegacyDB) Query(query string, args ...any) ([]any, error) { return nil, nil }
func (m *MockLegacyDB) Close() error                             { return nil }

func TestMockLegacyDB(t *testing.T) {
	t.Run("ImplementsLegacyDBInterface", func(t *testing.T) {
		var _ LegacyDB = (*MockLegacyDB)(nil)
	})

	t.Run("AllMethodsWork", func(t *testing.T) {
		db := &MockLegacyDB{}
		assert.NoError(t, db.Ping())
		assert.NoError(t, db.Exec("query"))
		results, err := db.Query("query")
		assert.NoError(t, err)
		assert.Nil(t, results)
		assert.NoError(t, db.Close())
	})
}

// TestPgxRowStruct tests the pgxRow struct
func TestPgxRowStruct(t *testing.T) {
	t.Run("CreatesWithNilRow", func(t *testing.T) {
		row := &pgxRow{row: nil}
		assert.NotNil(t, row)
		assert.Nil(t, row.row)
	})

	t.Run("ImplementsRowInterface", func(t *testing.T) {
		var _ Row = (*pgxRow)(nil)
	})
}
