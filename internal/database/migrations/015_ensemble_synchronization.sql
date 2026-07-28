-- Migration: 015_ensemble_synchronization
-- Description: Distributed lock + CRDT state tables backing internal/ensemble/synchronization
-- Date: 2026-07-28
-- Author: HelixAgent Team
--
-- Root cause this migration closes: internal/ensemble/synchronization/manager.go
-- and internal/clis/instance_manager.go query `distributed_locks` and
-- `crdt_state` on every lock acquire/renew/release and on a 30 s (SyncManager)
-- and 60 s (InstanceManager) cleanup tick, but NO migration or schema file in
-- this repository ever created either table. A running helixagent therefore
-- logs `ERROR: relation "distributed_locks" does not exist (SQLSTATE 42P01)`
-- forever and the distributed-locking feature is non-functional.
--
-- Every column, type, and constraint below is derived from an actual query in
-- the source (file:line cited per column group) — nothing is speculative.

-- =============================================================================
-- distributed_locks
-- =============================================================================
-- Cluster-wide advisory locks with TTL + owner fencing.
--
-- Queries this table must satisfy:
--   internal/ensemble/synchronization/manager.go:104  INSERT (name, owner, node_id, expires_at, acquired_at)
--                                                     ... ON CONFLICT (name) DO UPDATE ... RETURNING true
--   internal/ensemble/synchronization/manager.go:168  DELETE WHERE name = $1 AND owner = $2
--   internal/ensemble/synchronization/manager.go:197  SELECT name, owner, node_id, acquired_at, expires_at WHERE name = $1
--   internal/ensemble/synchronization/manager.go:224  SELECT ... WHERE expires_at > NOW() ORDER BY acquired_at DESC
--   internal/ensemble/synchronization/manager.go:392  UPDATE SET expires_at = $1 WHERE name = $2 AND owner = $3
--   internal/ensemble/synchronization/manager.go:421  DELETE WHERE expires_at < NOW()
--   internal/clis/instance_manager.go:899             DELETE WHERE expires_at < NOW()
--
-- `name` is the PRIMARY KEY (not a surrogate UUID): manager.go:106 uses
-- `ON CONFLICT (name)`, which requires a unique index on `name` alone.
--
-- All five columns are NOT NULL: manager.go:199 scans them into plain
-- `string` / `time.Time` fields (LockInfo, manager.go:212-218), so a NULL in
-- any of them is a scan error at runtime.
--
-- `node_id` is VARCHAR, NOT UUID: NewSyncManager accepts an arbitrary caller
-- supplied node id (manager.go:65-70) and tests/integration/ensemble_integration_test.go:83
-- passes the literal "test-node", which is not a valid UUID.
CREATE TABLE IF NOT EXISTS distributed_locks (
    -- Lock name; caller-supplied identifier. PK because of ON CONFLICT (name).
    name        VARCHAR(255) PRIMARY KEY,

    -- Fencing token: "<node_id>-<UnixNano>" (manager.go:98). Release (:168) and
    -- renew (:371) both match on it so a node cannot release/renew a lock that
    -- was reclaimed by another node after expiry.
    owner       VARCHAR(255) NOT NULL,

    -- Owning node. Arbitrary string, see note above.
    node_id     VARCHAR(255) NOT NULL,

    -- Set to NOW() by both the INSERT and the ON CONFLICT DO UPDATE branch.
    acquired_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- TTL horizon; compared against NOW() by the acquire, list, and cleanup
    -- paths, and rewritten by the renewal goroutine (manager.go:375-410).
    expires_at  TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Hot path: the 30 s / 60 s cleanup sweeps and ListLocks both filter on
-- expires_at (manager.go:224, manager.go:421, instance_manager.go:899).
CREATE INDEX IF NOT EXISTS idx_distributed_locks_expires_at
    ON distributed_locks (expires_at);

-- ListLocks orders by acquired_at DESC over the live set (manager.go:224-225).
CREATE INDEX IF NOT EXISTS idx_distributed_locks_acquired_at
    ON distributed_locks (acquired_at DESC);

-- Release/renew match on (name, owner); name alone is already the PK, so this
-- covers the owner predicate without a heap fetch.
CREATE INDEX IF NOT EXISTS idx_distributed_locks_owner
    ON distributed_locks (owner);

COMMENT ON TABLE distributed_locks IS
    'Cluster-wide advisory locks with TTL and owner fencing. Backs internal/ensemble/synchronization.SyncManager.';
COMMENT ON COLUMN distributed_locks.name IS
    'Caller-supplied lock name. PRIMARY KEY — required by the ON CONFLICT (name) upsert in AcquireLock.';
COMMENT ON COLUMN distributed_locks.owner IS
    'Fencing token "<node_id>-<UnixNano>". Release and renew match on it to prevent cross-node interference.';
COMMENT ON COLUMN distributed_locks.node_id IS
    'Owning node identifier. Free-form string (may be a UUID, may not) — see NewSyncManager.';
COMMENT ON COLUMN distributed_locks.expires_at IS
    'TTL horizon. Rows with expires_at < NOW() are reclaimable by any node and are swept by the cleanup loops.';

-- =============================================================================
-- crdt_state
-- =============================================================================
-- Persisted conflict-free replicated data types (G-Counter, PN-Counter, G-Set,
-- LWW-Register) with vector clocks.
--
-- Queries this table must satisfy:
--   internal/ensemble/synchronization/manager.go:281  SELECT state, vector_clock WHERE crdt_type = $1 AND crdt_key = $2
--   internal/ensemble/synchronization/manager.go:327  INSERT (crdt_type, crdt_key, state, vector_clock, instance_id, updated_at)
--                                                     ... ON CONFLICT (crdt_type, crdt_key) DO UPDATE ...
--
-- (crdt_type, crdt_key) is the composite PRIMARY KEY: manager.go:329 uses
-- `ON CONFLICT (crdt_type, crdt_key)`, which requires a unique index on that
-- exact column pair.
--
-- `state` and `vector_clock` are JSONB: `state` is produced by CRDT.ToJSON
-- (manager.go:315) and `vector_clock` by json.Marshal (manager.go:324); both are
-- read back into []byte (manager.go:279).
--
-- `instance_id` is VARCHAR, NOT UUID and NOT a foreign key: manager.go:334
-- passes sm.nodeID, the same free-form node identifier described above.
CREATE TABLE IF NOT EXISTS crdt_state (
    -- One of: g_counter, pn_counter, g_set, lww_register (manager.go:256-267).
    crdt_type    VARCHAR(50)  NOT NULL,

    -- Application-level CRDT key.
    crdt_key     VARCHAR(255) NOT NULL,

    -- Serialized CRDT payload (CRDT.ToJSON).
    state        JSONB        NOT NULL,

    -- map[nodeID]UnixNano vector clock (manager.go:321-324).
    vector_clock JSONB        NOT NULL,

    -- Last writer's node id. Always supplied by UpdateCRDT.
    instance_id  VARCHAR(255) NOT NULL,

    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    PRIMARY KEY (crdt_type, crdt_key)
);

CREATE INDEX IF NOT EXISTS idx_crdt_state_updated_at
    ON crdt_state (updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_crdt_state_instance_id
    ON crdt_state (instance_id);

COMMENT ON TABLE crdt_state IS
    'Persisted CRDT payloads with vector clocks. Backs SyncManager.GetCRDT / UpdateCRDT / MergeCRDTs.';
COMMENT ON COLUMN crdt_state.state IS
    'CRDT payload as produced by CRDT.ToJSON and consumed by CRDT.FromJSON.';
COMMENT ON COLUMN crdt_state.vector_clock IS
    'JSON object mapping node id to a UnixNano logical timestamp.';
COMMENT ON COLUMN crdt_state.instance_id IS
    'Node id of the last writer. Free-form string, not a foreign key.';
