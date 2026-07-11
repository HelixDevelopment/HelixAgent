//go:build cache_e2e

// Package cache — Phase-2 live Redis persistence proof.
//
// Guarded by the `cache_e2e` build tag so it is EXCLUDED from the normal unit
// suite (which uses miniredis) and only runs against a LIVE Redis:
//
//	REDIS_ADDR=localhost:8110 \
//	  go test -tags=cache_e2e -run TestE2E_RedisClient_LivePersist -v ./internal/cache/
//
// It drives HelixAgent's own RedisClient.Set / RedisClient.Get (no miniredis,
// no mock) against a real Redis booted via the containers submodule
// orchestrator (§11.4.50 / §11.4.5 / §11.4.69).
package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestE2E_RedisClient_LivePersist(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		// SKIP-OK: live Redis not provided — honest §11.4.3 skip, never a
		// faked PASS. Set REDIS_ADDR to a live Redis (e.g. localhost:8110).
		t.Skip("SKIP-OK: REDIS_ADDR not set — set it to the live Redis (e.g. localhost:8110) to run the live cache E2E proof")
	}

	rc := &RedisClient{client: redis.NewClient(&redis.Options{Addr: addr})}
	t.Cleanup(func() { _ = rc.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, rc.Ping(ctx), "live Redis must be reachable at %s", addr)

	// The key is emitted so the harness can sink-verify it with redis-cli.
	key := "phase2:e2e:cache:proof"
	want := "helixagent-phase2-cache-value"

	require.NoError(t, rc.Set(ctx, key, want, 5*time.Minute),
		"HelixAgent RedisClient.Set must persist to live Redis")

	var got string
	require.NoError(t, rc.Get(ctx, key, &got),
		"HelixAgent RedisClient.Get must read back the persisted value")
	require.Equal(t, want, got, "round-tripped value must match what was written")

	t.Logf("LIVE CACHE PROOF: addr=%s key=%q value=%q round-trip OK", addr, key, got)
}
