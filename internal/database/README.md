# Database Package

The database package provides PostgreSQL database connectivity and connection management for SuperAgent.

## Overview

This package handles:
- PostgreSQL connection pooling with pgx/v5
- Database migrations
- Connection health monitoring
- Transaction management

## Components

### Database Connection

Create a database connection:

```go
db, err := database.NewConnection(cfg)
if err != nil {
    log.Fatal("Failed to connect to database:", err)
}
defer db.Close()
```

### Connection Configuration

Configure via environment or config file:

```yaml
database:
  host: "${DB_HOST:localhost}"
  port: ${DB_PORT:5432}
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"
  name: "${DB_NAME:superagent}"
  ssl_mode: "prefer"
  max_connections: 25
  min_connections: 5
  max_conn_lifetime: "1h"
  max_conn_idle_time: "30m"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | Database hostname | localhost |
| `DB_PORT` | Database port | 5432 |
| `DB_USER` | Database user | - |
| `DB_PASSWORD` | Database password | - |
| `DB_NAME` | Database name | superagent |
| `DB_SSL_MODE` | SSL mode | prefer |

## Connection Pool

The package uses pgx's connection pool for efficient database access:

```go
// Get a connection from the pool
conn, err := db.Acquire(ctx)
if err != nil {
    return err
}
defer conn.Release()

// Execute a query
rows, err := conn.Query(ctx, "SELECT * FROM users WHERE id = $1", userID)
```

## Health Checks

Perform health checks on the database:

```go
if err := db.Ping(ctx); err != nil {
    log.Error("Database health check failed:", err)
}
```

## Migrations

Database migrations are managed separately. See `migrations/` directory for schema files.

## Usage with Repository Pattern

This package is typically used with the repository pattern:

```go
type UserRepository struct {
    db *database.Connection
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    var user User
    err := r.db.QueryRow(ctx, "SELECT * FROM users WHERE id = $1", id).Scan(&user)
    return &user, err
}
```
