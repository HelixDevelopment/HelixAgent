//go:build stress
// +build stress

package stress

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestPoolConnString constructs a PostgreSQL DSN from environment
// variables, mirroring what HelixAgent's boot path expects.
//
//	DB_HOST     (default: localhost)
//	DB_PORT     (default: 15432  — test-infra port)
//	DB_USER     (default: helixagent)
//	DB_PASSWORD (default: helixagent123)
//	DB_NAME     (default: helixagent_db)
func buildTestPoolConnString() string {
	host := envOr("DB_HOST", "localhost")
	port := envOr("DB_PORT", "15432")
	user := envOr("DB_USER", "helixagent")
	pass := envOr("DB_PASSWORD", "helixagent123")
	name := envOr("DB_NAME", "helixagent_db")
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable connect_timeout=3",
		host, port, user, pass, name,
	)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// isPostgresAvailable does a best-effort probe with a short timeout.
func isPostgresAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(buildTestPoolConnString())
	if err != nil {
		return false
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return false
	}
	defer pool.Close()
	return pool.Ping(ctx) == nil
}

// TestDBPool_Exhaustion_GracefulTimeout creates a pgxpool with a very small
// MaxConns (3) and then launches many goroutines that all try to acquire
// connections simultaneously. The test verifies:
//   - Goroutines that cannot acquire within their deadline receive a timeout
//     error, not a panic.
//   - The pool is fully functional again after the storm.
//   - No goroutine leaks after completion.
func TestDBPool_Exhaustion_GracefulTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	if !isPostgresAvailable() {
		t.Skip("requires running PostgreSQL (start with: make test-infra-start)") // SKIP-OK: #requires-infra-port
	}

	const poolSize = 3 // intentionally tiny to force exhaustion

	poolCfg, err := pgxpool.ParseConfig(buildTestPoolConnString())
	require.NoError(t, err, "pool config must parse")

	poolCfg.MaxConns = int32(poolSize) // #nosec G115 - small constant fits int32
	poolCfg.MinConns = 0
	poolCfg.MaxConnIdleTime = 30 * time.Second

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	require.NoError(t, err, "pool must be created")
	defer pool.Close()

	// Verify pool is healthy before the stress run.
	require.NoError(t, pool.Ping(ctx), "pool must be reachable before stress")

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	const goroutineCount = 50 // >> poolSize to guarantee exhaustion
	var (
		wg         sync.WaitGroup
		succeeded  int64
		timedOut   int64
		otherErr   int64
		panicCount int64
		start      = make(chan struct{})
	)

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()

			<-start

			// Each goroutine gets a strict 2-second deadline.  When the pool
			// is exhausted the Acquire call must return context.DeadlineExceeded
			// rather than blocking forever or panicking.
			acquireCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			conn, err := pool.Acquire(acquireCtx)
			if err != nil {
				if acquireCtx.Err() != nil {
					atomic.AddInt64(&timedOut, 1)
				} else {
					atomic.AddInt64(&otherErr, 1)
				}
				return
			}
			defer conn.Release()

			// Hold the connection briefly to keep pool pressure up.
			holdCtx, holdCancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer holdCancel()

			var result int
			if scanErr := conn.QueryRow(holdCtx, "SELECT 1").Scan(&result); scanErr == nil {
				atomic.AddInt64(&succeeded, 1)
			} else {
				atomic.AddInt64(&otherErr, 1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: DB pool exhaustion stress timed out after 30s")
	}

	total := succeeded + timedOut + otherErr
	assert.Equal(t, int64(goroutineCount), total,
		"every goroutine must complete (success, timeout, or error — no hangs)")
	assert.Zero(t, panicCount, "no goroutine must panic during pool exhaustion")

	// Verify pool is still usable after the storm.
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	assert.NoError(t, pool.Ping(pingCtx),
		"pool must remain functional after exhaustion stress")

	t.Logf("DB pool exhaustion (poolSize=%d goroutines=%d): succeeded=%d timedOut=%d otherErr=%d panics=%d",
		poolSize, goroutineCount, succeeded, timedOut, otherErr, panicCount)

	// Goroutine-leak check.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	assert.Less(t, leaked, 30,
		"goroutine count must not grow excessively after pool exhaustion stress")
	t.Logf("Goroutines: before=%d after=%d leaked=%d", goroutinesBefore, goroutinesAfter, leaked)
}

// TestDBPool_Exhaustion_ConcurrentQueries verifies that a realistic pool size
// handles concurrent queries without panics, even when queries contend for
// connections.  This variant uses the pool size derived from environment config
// (defaulting to 10) and exercises SELECT 1 queries from many goroutines.
func TestDBPool_Exhaustion_ConcurrentQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	if !isPostgresAvailable() {
		t.Skip("requires running PostgreSQL (start with: make test-infra-start)") // SKIP-OK: #requires-infra-port
	}

	maxConns := 10
	if envVal := os.Getenv("DB_POOL_MAX_CONNS"); envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil && n > 0 {
			maxConns = n
		}
	}

	poolCfg, err := pgxpool.ParseConfig(buildTestPoolConnString())
	require.NoError(t, err)
	poolCfg.MaxConns = int32(maxConns) // #nosec G115 - bounded above
	poolCfg.MinConns = 0

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx))

	// Launch 3× the pool size to guarantee contention.
	goroutineCount := maxConns * 3
	var (
		wg        sync.WaitGroup
		successes int64
		errors_   int64
		panics    int64
		start     = make(chan struct{})
	)

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-start

			qCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var result int
			if err := pool.QueryRow(qCtx, "SELECT 1").Scan(&result); err == nil {
				atomic.AddInt64(&successes, 1)
			} else {
				atomic.AddInt64(&errors_, 1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK: concurrent queries pool stress timed out")
	}

	assert.Zero(t, panics, "no panic during concurrent query pool stress")
	assert.Equal(t, int64(goroutineCount), successes+errors_,
		"all goroutines must complete")
	assert.Greater(t, successes, int64(0),
		"at least some queries must succeed")

	t.Logf("Concurrent query stress (poolSize=%d goroutines=%d): successes=%d errors=%d panics=%d",
		maxConns, goroutineCount, successes, errors_, panics)
}
