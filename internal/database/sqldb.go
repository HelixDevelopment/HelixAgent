package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	// pgx stdlib registers itself as the "pgx" database/sql driver
	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenSQLDB opens a database/sql connection using the standard
// HelixAgent DB_* environment variables. Returns (nil, err) when the
// connection fails — callers should treat this as "DB not available"
// and fall back to in-memory mode rather than aborting boot.
//
// CONST-035 §c: closes #ensemble-db-wiring by giving InstanceManager
// and Coordinator a real *sql.DB so ensemble sessions persist across
// restarts when Postgres is reachable. The nil-guards added in
// rounds 22-25 keep the in-memory fallback intact when it isn't.
func OpenSQLDB(ctx context.Context) (*sql.DB, error) {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "helixagent"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "helixagent_db"
	}
	dbSSL := os.Getenv("DB_SSL_MODE")
	if dbSSL == "" {
		dbSSL = "disable"
	}

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&connect_timeout=5",
		dbUser, dbPassword, dbHost, dbPort, dbName, dbSSL,
	)

	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	// Sensible pool defaults — overridable via DB_POOL_* env vars in a
	// follow-up if needed.
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(15 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Verify connectivity within the caller's context budget.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close() //nolint:errcheck
		return nil, fmt.Errorf("PingContext: %w", err)
	}

	return db, nil
}
