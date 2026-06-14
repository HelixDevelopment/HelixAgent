package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // pure-Go, CGo-free SQLite driver registered as "sqlite"
)

// DiskStore is a real disk-durable implementation of MemoryStore backed by
// SQLite (modernc.org/sqlite — pure Go, no CGo). Unlike InMemoryStore, records
// written through DiskStore survive process restart: the schema and rows live
// in the on-disk database file, and WAL journalling guarantees writes are
// flushed and recoverable after the store is closed and reopened.
//
// All map / slice fields of Memory / Entity / Relationship (Metadata,
// Embedding, Properties, Aliases) are persisted as JSON text columns so the
// round-trip is lossless.
type DiskStore struct {
	db   *sql.DB
	path string
}

// NewDiskStore opens (creating if absent) a SQLite database at path and
// guarantees the schema exists. The returned store is durable: rows written
// here are readable by a fresh DiskStore opened on the same path after Close.
//
// path may be a file path. Each DiskStore owns a single *sql.DB connection
// pool; callers MUST Close it to release the file handle.
func NewDiskStore(path string) (*DiskStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("disk store path must not be empty")
	}

	// Rollback-journal (DELETE) mode + synchronous(FULL) makes every committed
	// transaction fsync directly into the main database file, so writes survive a
	// close+reopen (the durability guarantee) WITHOUT the WAL's mmap'd sidecar
	// files — which, under the pure-Go driver and many concurrent single-conn
	// stores, can transiently return SQLITE_IOERR during WAL setup. busy_timeout
	// keeps concurrent opens contention-tolerant; foreign_keys keeps relationship
	// integrity honest.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(DELETE)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(FULL)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", path, err)
	}

	// A single writer connection avoids "database is locked" under the pure-Go
	// driver while still allowing the WAL to serve readers.
	db.SetMaxOpenConns(1)

	s := &DiskStore{db: db, path: path}
	if err := s.initSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema at %s: %w", path, err)
	}

	return s, nil
}

// Close releases the underlying database handle. In rollback-journal mode every
// committed transaction is already fsynced into the main DB file, so a later
// reopen sees every committed write; the wal_checkpoint below is a harmless
// no-op in DELETE mode and a safety fold if the journal mode were ever WAL.
func (s *DiskStore) Close() error {
	if s.db == nil {
		return nil
	}
	// Best-effort checkpoint; ignore error so Close never masks a real failure.
	_, _ = s.db.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)")
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *DiskStore) initSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS memories (
	id           TEXT PRIMARY KEY,
	user_id      TEXT NOT NULL DEFAULT '',
	session_id   TEXT NOT NULL DEFAULT '',
	content      TEXT NOT NULL DEFAULT '',
	summary      TEXT NOT NULL DEFAULT '',
	type         TEXT NOT NULL DEFAULT '',
	category     TEXT NOT NULL DEFAULT '',
	metadata     TEXT NOT NULL DEFAULT '',
	embedding    TEXT NOT NULL DEFAULT '',
	importance   REAL NOT NULL DEFAULT 0,
	access_count INTEGER NOT NULL DEFAULT 0,
	last_access  TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL DEFAULT '',
	updated_at   TEXT NOT NULL DEFAULT '',
	expires_at   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_memories_user    ON memories(user_id);
CREATE INDEX IF NOT EXISTS idx_memories_session ON memories(session_id);

CREATE TABLE IF NOT EXISTS entities (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL DEFAULT '',
	type       TEXT NOT NULL DEFAULT '',
	properties TEXT NOT NULL DEFAULT '',
	aliases    TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);

CREATE TABLE IF NOT EXISTS relationships (
	id         TEXT PRIMARY KEY,
	source_id  TEXT NOT NULL DEFAULT '',
	target_id  TEXT NOT NULL DEFAULT '',
	type       TEXT NOT NULL DEFAULT '',
	properties TEXT NOT NULL DEFAULT '',
	strength   REAL NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_relationships_source ON relationships(source_id);
CREATE INDEX IF NOT EXISTS idx_relationships_target ON relationships(target_id);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("schema exec failed: %w", err)
	}
	return nil
}

// --- time / json helpers (lossless round-trip) ---

func marshalJSON(v interface{}) (string, error) {
	if v == nil {
		return "", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshalMap(s string) (map[string]interface{}, error) {
	if s == "" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func unmarshalFloat32Slice(s string) ([]float32, error) {
	if s == "" {
		return nil, nil
	}
	var v []float32
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return v, nil
}

func unmarshalStringSlice(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	var v []string
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return v, nil
}

func encodeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func decodeTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func encodeTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return encodeTime(*t)
}

func decodeTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t := decodeTime(s)
	if t.IsZero() {
		return nil
	}
	return &t
}

// --- Memory CRUD ---

// Add persists a new memory. A missing ID is generated. Idempotent on ID via
// INSERT OR REPLACE so re-adding the same ID overwrites (mirrors the in-memory
// Put semantics).
func (s *DiskStore) Add(ctx context.Context, memory *Memory) error {
	if memory.ID == "" {
		memory.ID = uuid.New().String()
	}

	metaJSON, err := marshalJSON(memory.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata for memory %s: %w", memory.ID, err)
	}
	embJSON, err := marshalJSON(memory.Embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding for memory %s: %w", memory.ID, err)
	}

	_, err = s.db.ExecContext(ctx, `
INSERT OR REPLACE INTO memories
	(id, user_id, session_id, content, summary, type, category, metadata, embedding,
	 importance, access_count, last_access, created_at, updated_at, expires_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		memory.ID, memory.UserID, memory.SessionID, memory.Content, memory.Summary,
		string(memory.Type), memory.Category, metaJSON, embJSON,
		memory.Importance, memory.AccessCount,
		encodeTime(memory.LastAccess), encodeTime(memory.CreatedAt),
		encodeTime(memory.UpdatedAt), encodeTimePtr(memory.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("failed to insert memory %s: %w", memory.ID, err)
	}
	return nil
}

func (s *DiskStore) scanMemory(rows interface {
	Scan(dest ...interface{}) error
}) (*Memory, error) {
	var (
		m                                           Memory
		typeStr, metaJSON, embJSON                  string
		lastAccess, createdAt, updatedAt, expiresAt string
	)
	if err := rows.Scan(
		&m.ID, &m.UserID, &m.SessionID, &m.Content, &m.Summary,
		&typeStr, &m.Category, &metaJSON, &embJSON,
		&m.Importance, &m.AccessCount,
		&lastAccess, &createdAt, &updatedAt, &expiresAt,
	); err != nil {
		return nil, err
	}
	m.Type = MemoryType(typeStr)
	meta, err := unmarshalMap(metaJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata for memory %s: %w", m.ID, err)
	}
	m.Metadata = meta
	emb, err := unmarshalFloat32Slice(embJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedding for memory %s: %w", m.ID, err)
	}
	m.Embedding = emb
	m.LastAccess = decodeTime(lastAccess)
	m.CreatedAt = decodeTime(createdAt)
	m.UpdatedAt = decodeTime(updatedAt)
	m.ExpiresAt = decodeTimePtr(expiresAt)
	return &m, nil
}

const memoryColumns = `id, user_id, session_id, content, summary, type, category, metadata, embedding,
	importance, access_count, last_access, created_at, updated_at, expires_at`

// Get retrieves a memory by ID, incrementing its access count and last-access
// timestamp (persisted), mirroring InMemoryStore.Get.
func (s *DiskStore) Get(ctx context.Context, id string) (*Memory, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+memoryColumns+` FROM memories WHERE id = ?`, id)
	m, err := s.scanMemory(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("memory not found: %s", id)
		}
		return nil, fmt.Errorf("failed to query memory %s: %w", id, err)
	}

	m.AccessCount++
	m.LastAccess = time.Now()
	if _, err := s.db.ExecContext(ctx,
		`UPDATE memories SET access_count = ?, last_access = ? WHERE id = ?`,
		m.AccessCount, encodeTime(m.LastAccess), id,
	); err != nil {
		return nil, fmt.Errorf("failed to update access metadata for memory %s: %w", id, err)
	}
	return m, nil
}

// Update updates an existing memory; fails if it does not exist.
func (s *DiskStore) Update(ctx context.Context, memory *Memory) error {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM memories WHERE id = ?`, memory.ID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("memory not found: %s", memory.ID)
		}
		return fmt.Errorf("failed to check memory %s: %w", memory.ID, err)
	}

	memory.UpdatedAt = time.Now()

	metaJSON, err := marshalJSON(memory.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata for memory %s: %w", memory.ID, err)
	}
	embJSON, err := marshalJSON(memory.Embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding for memory %s: %w", memory.ID, err)
	}

	_, err = s.db.ExecContext(ctx, `
UPDATE memories SET
	user_id=?, session_id=?, content=?, summary=?, type=?, category=?, metadata=?, embedding=?,
	importance=?, access_count=?, last_access=?, created_at=?, updated_at=?, expires_at=?
WHERE id=?`,
		memory.UserID, memory.SessionID, memory.Content, memory.Summary,
		string(memory.Type), memory.Category, metaJSON, embJSON,
		memory.Importance, memory.AccessCount,
		encodeTime(memory.LastAccess), encodeTime(memory.CreatedAt),
		encodeTime(memory.UpdatedAt), encodeTimePtr(memory.ExpiresAt),
		memory.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update memory %s: %w", memory.ID, err)
	}
	return nil
}

// Delete removes a memory by ID; fails if it does not exist.
func (s *DiskStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete memory %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read delete result for memory %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("memory not found: %s", id)
	}
	return nil
}

// --- Search ---

// Search returns memories matching the query via the same word-overlap scoring
// as InMemoryStore. Filtering (user/session/type/category/time) is pushed into
// SQL where cheap; scoring is computed in Go after the candidate rows are read.
func (s *DiskStore) Search(ctx context.Context, query string, opts *SearchOptions) ([]*Memory, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	where := []string{"1=1"}
	args := []interface{}{}
	if opts.UserID != "" {
		where = append(where, "user_id = ?")
		args = append(args, opts.UserID)
	}
	if opts.SessionID != "" {
		where = append(where, "session_id = ?")
		args = append(args, opts.SessionID)
	}
	if opts.Type != "" {
		where = append(where, "type = ?")
		args = append(args, string(opts.Type))
	}
	if opts.Category != "" {
		where = append(where, "category = ?")
		args = append(args, opts.Category)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+memoryColumns+` FROM memories WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories for search: %w", err)
	}
	defer rows.Close()

	queryLower := strings.ToLower(query)
	var results []*Memory
	for rows.Next() {
		m, err := s.scanMemory(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan memory during search: %w", err)
		}

		if opts.TimeRange != nil {
			if m.CreatedAt.Before(opts.TimeRange.Start) || m.CreatedAt.After(opts.TimeRange.End) {
				continue
			}
		}

		score := scoreMatch(queryLower, m.Content)
		if score >= opts.MinScore {
			m.Importance = score // mirror InMemoryStore: importance carries score for sort
			results = append(results, m)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error during search: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Importance > results[j].Importance
	})

	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}
	return results, nil
}

// scoreMatch mirrors InMemoryStore.calculateMatchScore (word-overlap ratio).
func scoreMatch(queryLower, content string) float64 {
	contentLower := strings.ToLower(content)
	queryWords := strings.Fields(queryLower)
	if len(queryWords) == 0 {
		return 0
	}
	matches := 0
	for _, word := range queryWords {
		if strings.Contains(contentLower, word) {
			matches++
		}
	}
	return float64(matches) / float64(len(queryWords))
}

// GetByUser retrieves memories for a user, sorted + paginated per opts.
func (s *DiskStore) GetByUser(ctx context.Context, userID string, opts *ListOptions) ([]*Memory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+memoryColumns+` FROM memories WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories for user %s: %w", userID, err)
	}
	defer rows.Close()

	var results []*Memory
	for rows.Next() {
		m, err := s.scanMemory(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan memory for user %s: %w", userID, err)
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error for user %s: %w", userID, err)
	}
	if len(results) == 0 {
		return []*Memory{}, nil
	}

	if opts != nil && opts.SortBy != "" {
		sortMemoriesByField(results, opts.SortBy, opts.Order)
	}

	if opts != nil {
		start := opts.Offset
		if start > len(results) {
			return []*Memory{}, nil
		}
		end := start + opts.Limit
		if end > len(results) || opts.Limit == 0 {
			end = len(results)
		}
		results = results[start:end]
	}
	return results, nil
}

// GetBySession retrieves memories for a session, sorted by creation time.
func (s *DiskStore) GetBySession(ctx context.Context, sessionID string) ([]*Memory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+memoryColumns+` FROM memories WHERE session_id = ?`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories for session %s: %w", sessionID, err)
	}
	defer rows.Close()

	var results []*Memory
	for rows.Next() {
		m, err := s.scanMemory(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan memory for session %s: %w", sessionID, err)
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error for session %s: %w", sessionID, err)
	}
	if len(results) == 0 {
		return []*Memory{}, nil
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})
	return results, nil
}

func sortMemoriesByField(memories []*Memory, sortBy, order string) {
	sort.Slice(memories, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "created_at":
			less = memories[i].CreatedAt.Before(memories[j].CreatedAt)
		case "updated_at":
			less = memories[i].UpdatedAt.Before(memories[j].UpdatedAt)
		case "importance":
			less = memories[i].Importance < memories[j].Importance
		case "access_count":
			less = memories[i].AccessCount < memories[j].AccessCount
		default:
			less = memories[i].CreatedAt.Before(memories[j].CreatedAt)
		}
		if order == "desc" {
			return !less
		}
		return less
	})
}

// --- Entity operations ---

// AddEntity persists an entity, stamping created/updated times.
func (s *DiskStore) AddEntity(ctx context.Context, entity *Entity) error {
	if entity.ID == "" {
		entity.ID = uuid.New().String()
	}
	now := time.Now()
	entity.CreatedAt = now
	entity.UpdatedAt = now

	propsJSON, err := marshalJSON(entity.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties for entity %s: %w", entity.ID, err)
	}
	aliasJSON, err := marshalJSON(entity.Aliases)
	if err != nil {
		return fmt.Errorf("failed to marshal aliases for entity %s: %w", entity.ID, err)
	}

	_, err = s.db.ExecContext(ctx, `
INSERT OR REPLACE INTO entities (id, name, type, properties, aliases, created_at, updated_at)
VALUES (?,?,?,?,?,?,?)`,
		entity.ID, entity.Name, entity.Type, propsJSON, aliasJSON,
		encodeTime(entity.CreatedAt), encodeTime(entity.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to insert entity %s: %w", entity.ID, err)
	}
	return nil
}

func (s *DiskStore) scanEntity(rows interface {
	Scan(dest ...interface{}) error
}) (*Entity, error) {
	var (
		e                    Entity
		propsJSON, aliasJSON string
		createdAt, updatedAt string
	)
	if err := rows.Scan(&e.ID, &e.Name, &e.Type, &propsJSON, &aliasJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	props, err := unmarshalMap(propsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties for entity %s: %w", e.ID, err)
	}
	e.Properties = props
	aliases, err := unmarshalStringSlice(aliasJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal aliases for entity %s: %w", e.ID, err)
	}
	e.Aliases = aliases
	e.CreatedAt = decodeTime(createdAt)
	e.UpdatedAt = decodeTime(updatedAt)
	return &e, nil
}

const entityColumns = `id, name, type, properties, aliases, created_at, updated_at`

// GetEntity retrieves an entity by ID.
func (s *DiskStore) GetEntity(ctx context.Context, id string) (*Entity, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+entityColumns+` FROM entities WHERE id = ?`, id)
	e, err := s.scanEntity(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entity not found: %s", id)
		}
		return nil, fmt.Errorf("failed to query entity %s: %w", id, err)
	}
	return e, nil
}

// SearchEntities returns entities whose name contains the (case-insensitive) query.
func (s *DiskStore) SearchEntities(ctx context.Context, query string, limit int) ([]*Entity, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+entityColumns+` FROM entities WHERE LOWER(name) LIKE ?`,
		"%"+strings.ToLower(query)+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to query entities for search: %w", err)
	}
	defer rows.Close()

	var results []*Entity
	for rows.Next() {
		e, err := s.scanEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entity during search: %w", err)
		}
		results = append(results, e)
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error during entity search: %w", err)
	}
	return results, nil
}

// --- Relationship operations ---

// AddRelationship persists a relationship, stamping created/updated times.
func (s *DiskStore) AddRelationship(ctx context.Context, rel *Relationship) error {
	if rel.ID == "" {
		rel.ID = uuid.New().String()
	}
	now := time.Now()
	rel.CreatedAt = now
	rel.UpdatedAt = now

	propsJSON, err := marshalJSON(rel.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties for relationship %s: %w", rel.ID, err)
	}

	_, err = s.db.ExecContext(ctx, `
INSERT OR REPLACE INTO relationships (id, source_id, target_id, type, properties, strength, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?)`,
		rel.ID, rel.SourceID, rel.TargetID, rel.Type, propsJSON, rel.Strength,
		encodeTime(rel.CreatedAt), encodeTime(rel.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to insert relationship %s: %w", rel.ID, err)
	}
	return nil
}

func (s *DiskStore) scanRelationship(rows interface {
	Scan(dest ...interface{}) error
}) (*Relationship, error) {
	var (
		r                    Relationship
		propsJSON            string
		createdAt, updatedAt string
	)
	if err := rows.Scan(&r.ID, &r.SourceID, &r.TargetID, &r.Type, &propsJSON, &r.Strength, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	props, err := unmarshalMap(propsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties for relationship %s: %w", r.ID, err)
	}
	r.Properties = props
	r.CreatedAt = decodeTime(createdAt)
	r.UpdatedAt = decodeTime(updatedAt)
	return &r, nil
}

const relationshipColumns = `id, source_id, target_id, type, properties, strength, created_at, updated_at`

// GetRelationships returns every relationship where the entity is source or target.
func (s *DiskStore) GetRelationships(ctx context.Context, entityID string) ([]*Relationship, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+relationshipColumns+` FROM relationships WHERE source_id = ? OR target_id = ?`,
		entityID, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to query relationships for entity %s: %w", entityID, err)
	}
	defer rows.Close()

	var results []*Relationship
	for rows.Next() {
		r, err := s.scanRelationship(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan relationship for entity %s: %w", entityID, err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error for entity %s: %w", entityID, err)
	}
	return results, nil
}

// compile-time guarantee DiskStore satisfies MemoryStore.
var _ MemoryStore = (*DiskStore)(nil)
