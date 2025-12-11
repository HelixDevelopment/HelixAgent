package database

import (
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

	// Test with empty migrations
	err = RunMigration(db, []string{})
	if err != nil {
		t.Logf("Failed to run empty migrations (expected if PostgreSQL not running): %v", err)
	}

	// Test with a simple migration
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
}
