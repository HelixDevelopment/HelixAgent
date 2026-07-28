-- =============================================================================
-- HelixAgent SQL Schema: Ensemble Synchronization
-- =============================================================================
-- Domain: Cluster-wide distributed locks and persisted CRDT state.
-- Source migrations: 015_ensemble_synchronization.sql
--
-- Consumed by:
--   internal/ensemble/synchronization/manager.go  (SyncManager: AcquireLock,
--     ReleaseLock, GetLockInfo, ListLocks, renewLock, cleanupLoop, GetCRDT,
--     UpdateCRDT, MergeCRDTs)
--   internal/clis/instance_manager.go             (healthCheckLoop expired-lock sweep)
--
-- This file is the per-domain mirror of internal/database/migrations/015_ensemble_synchronization.sql.
-- It lives here because sql/schema/ is the directory mounted into the Postgres
-- container's /docker-entrypoint-initdb.d (docker-compose.ci.yml:46) and is the
-- path named by tests/integration/ensemble_integration_test.go:67 — it is the
-- only automated apply path in this repository. Keep both files identical in
-- DDL; the numbered migration is the authoritative source.
--
-- Every column, type, and constraint is derived from a real query in the Go
-- source (cited inline) — nothing is speculative.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- distributed_locks — cluster-wide advisory locks with TTL + owner fencing
-- -----------------------------------------------------------------------------
-- Queries this table must satisfy:
--   manager.go:104  INSERT (name, owner, node_id, expires_at, acquired_at)
--                   ... ON CONFLICT (name) DO UPDATE ... RETURNING true
--   manager.go:168  DELETE WHERE name = $1 AND owner = $2
--   manager.go:197  SELECT name, owner, node_id, acquired_at, expires_at WHERE name = $1
--   manager.go:224  SELECT ... WHERE expires_at > NOW() ORDER BY acquired_at DESC
--   manager.go:392  UPDATE SET expires_at = $1 WHERE name = $2 AND owner = $3
--   manager.go:421  DELETE WHERE expires_at < NOW()
--   instance_manager.go:899  DELETE WHERE expires_at < NOW()
--
-- `name` is the PRIMARY KEY (not a surrogate UUID) because manager.go:106 uses
-- ON CONFLICT (name), which requires a unique index on `name` alone.
-- All columns are NOT NULL because manager.go:199 scans them into plain
-- string / time.Time fields (LockInfo, manager.go:212-218).
-- `node_id` is VARCHAR and not UUID because NewSyncManager takes a free-form
-- node id (manager.go:65-70) and the integration suite passes "test-node"
-- (tests/integration/ensemble_integration_test.go:83).
CREATE TABLE IF NOT EXISTS distributed_locks (
    name        VARCHAR(255) PRIMARY KEY,
    owner       VARCHAR(255) NOT NULL,
    node_id     VARCHAR(255) NOT NULL,
    acquired_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_distributed_locks_expires_at
    ON distributed_locks (expires_at);

CREATE INDEX IF NOT EXISTS idx_distributed_locks_acquired_at
    ON distributed_locks (acquired_at DESC);

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

-- -----------------------------------------------------------------------------
-- crdt_state — persisted CRDT payloads with vector clocks
-- -----------------------------------------------------------------------------
-- Queries this table must satisfy:
--   manager.go:281  SELECT state, vector_clock WHERE crdt_type = $1 AND crdt_key = $2
--   manager.go:327  INSERT (crdt_type, crdt_key, state, vector_clock, instance_id, updated_at)
--                   ... ON CONFLICT (crdt_type, crdt_key) DO UPDATE ...
--
-- (crdt_type, crdt_key) is the composite PRIMARY KEY because manager.go:329
-- uses ON CONFLICT on that exact column pair.
-- `state` / `vector_clock` are JSONB: `state` from CRDT.ToJSON (manager.go:315),
-- `vector_clock` from json.Marshal (manager.go:324); both read back into []byte
-- (manager.go:279).
-- `instance_id` is VARCHAR and not a foreign key: manager.go:334 passes
-- sm.nodeID, the same free-form node identifier described above.
CREATE TABLE IF NOT EXISTS crdt_state (
    crdt_type    VARCHAR(50)  NOT NULL,
    crdt_key     VARCHAR(255) NOT NULL,
    state        JSONB        NOT NULL,
    vector_clock JSONB        NOT NULL,
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
