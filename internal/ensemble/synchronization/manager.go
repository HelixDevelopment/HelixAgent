// Package synchronization provides distributed state synchronization for HelixAgent.
package synchronization

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SyncManager manages distributed state synchronization.
type SyncManager struct {
	db     *sql.DB
	logger *log.Logger
	nodeID string

	// Distributed locks
	locks map[string]*DistributedLock
	mu    sync.RWMutex

	// CRDTs
	crdts map[string]CRDT

	// Control
	ctx    context.Context
	cancel context.CancelFunc
}

// DistributedLock represents a distributed lock.
type DistributedLock struct {
	Name     string
	Owner    string
	NodeID   string
	Acquired time.Time
	Expires  time.Time
	TTL      time.Duration

	// Local state
	held      bool
	renewStop chan struct{}
	mu        sync.Mutex
}

// CRDT is the interface for conflict-free replicated data types.
type CRDT interface {
	Type() string
	Key() string
	Merge(other CRDT) CRDT
	ToJSON() ([]byte, error)
	FromJSON(data []byte) error
}

// NewSyncManager creates a new synchronization manager.
func NewSyncManager(db *sql.DB, logger *log.Logger, nodeID string) *SyncManager {
	ctx, cancel := context.WithCancel(context.Background())

	if nodeID == "" {
		nodeID = uuid.New().String()
	}

	sm := &SyncManager{
		db:     db,
		logger: logger,
		nodeID: nodeID,
		locks:  make(map[string]*DistributedLock),
		crdts:  make(map[string]CRDT),
		ctx:    ctx,
		cancel: cancel,
	}

	// Start cleanup goroutine
	go sm.cleanupLoop()

	return sm
}

// AcquireLock acquires a distributed lock.
func (sm *SyncManager) AcquireLock(
	ctx context.Context,
	name string,
	ttl time.Duration,
) (*DistributedLock, error) {
	if ttl == 0 {
		ttl = 30 * time.Second
	}

	owner := fmt.Sprintf("%s-%d", sm.nodeID, time.Now().UnixNano())
	expires := time.Now().Add(ttl)

	// Try to acquire in database
	var acquired bool
	err := sm.db.QueryRowContext(ctx,
		`INSERT INTO distributed_locks (name, owner, node_id, expires_at, acquired_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (name) DO UPDATE
		 SET owner = EXCLUDED.owner,
		     node_id = EXCLUDED.node_id,
		     expires_at = EXCLUDED.expires_at,
		     acquired_at = NOW()
		 WHERE distributed_locks.expires_at < NOW()
		 RETURNING true`,
		name, owner, sm.nodeID, expires,
	).Scan(&acquired)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("lock %s is held by another node", name)
	}
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	if !acquired {
		return nil, fmt.Errorf("lock %s is held by another node", name)
	}

	// Create local lock object
	lock := &DistributedLock{
		Name:      name,
		Owner:     owner,
		NodeID:    sm.nodeID,
		Acquired:  time.Now(),
		Expires:   expires,
		TTL:       ttl,
		held:      true,
		renewStop: make(chan struct{}),
	}

	// Store in local registry
	sm.mu.Lock()
	sm.locks[name] = lock
	sm.mu.Unlock()

	// Start renewal goroutine
	go sm.renewLock(lock)

	sm.logger.Printf("Acquired lock %s", name)

	return lock, nil
}

// ReleaseLock releases a distributed lock.
func (sm *SyncManager) ReleaseLock(lock *DistributedLock) error {
	if lock == nil {
		return nil
	}

	lock.mu.Lock()
	if !lock.held {
		lock.mu.Unlock()
		return nil
	}
	lock.held = false
	lock.mu.Unlock()

	// Stop renewal
	close(lock.renewStop)

	// Remove from database
	_, err := sm.db.ExecContext(sm.ctx,
		"DELETE FROM distributed_locks WHERE name = $1 AND owner = $2",
		lock.Name, lock.Owner,
	)

	// Remove from local registry
	sm.mu.Lock()
	delete(sm.locks, lock.Name)
	sm.mu.Unlock()

	sm.logger.Printf("Released lock %s", lock.Name)

	return err
}

// IsLocked checks if a lock is held.
func (sm *SyncManager) IsLocked(name string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	lock, ok := sm.locks[name]
	if !ok {
		return false
	}

	lock.mu.Lock()
	defer lock.mu.Unlock()
	return lock.held
}

// GetLockInfo returns information about a lock.
func (sm *SyncManager) GetLockInfo(ctx context.Context, name string) (*LockInfo, error) {
	var info LockInfo
	err := sm.db.QueryRowContext(ctx,
		`SELECT name, owner, node_id, acquired_at, expires_at
		 FROM distributed_locks WHERE name = $1`,
		name,
	).Scan(&info.Name, &info.Owner, &info.NodeID, &info.AcquiredAt, &info.ExpiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &info, nil
}

// LockInfo represents lock metadata.
type LockInfo struct {
	Name       string
	Owner      string
	NodeID     string
	AcquiredAt time.Time
	ExpiresAt  time.Time
}

// ListLocks returns all active locks.
func (sm *SyncManager) ListLocks(ctx context.Context) ([]*LockInfo, error) {
	rows, err := sm.db.QueryContext(ctx,
		`SELECT name, owner, node_id, acquired_at, expires_at
		 FROM distributed_locks WHERE expires_at > NOW()
		 ORDER BY acquired_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locks []*LockInfo
	for rows.Next() {
		var info LockInfo
		if err := rows.Scan(&info.Name, &info.Owner, &info.NodeID, &info.AcquiredAt, &info.ExpiresAt); err != nil {
			continue
		}
		locks = append(locks, &info)
	}

	return locks, rows.Err()
}

// CRDT operations

// GetCRDT retrieves a CRDT by key.
func (sm *SyncManager) GetCRDT(ctx context.Context, crdtType, key string) (CRDT, error) {
	// Check local cache
	cacheKey := fmt.Sprintf("%s:%s", crdtType, key)
	sm.mu.RLock()
	crdt, ok := sm.crdts[cacheKey]
	sm.mu.RUnlock()

	if ok {
		return crdt, nil
	}

	// Load from database
	var stateJSON, vectorClockJSON []byte
	err := sm.db.QueryRowContext(ctx,
		`SELECT state, vector_clock FROM crdt_state
		 WHERE crdt_type = $1 AND crdt_key = $2`,
		crdtType, key,
	).Scan(&stateJSON, &vectorClockJSON)

	if err == sql.ErrNoRows {
		// Create new CRDT
		switch crdtType {
		case "g_counter":
			crdt = NewGCounter(key)
		case "pn_counter":
			crdt = NewPNCounter(key)
		case "g_set":
			crdt = NewGSet(key)
		case "lww_register":
			crdt = NewLWWRegister(key)
		default:
			return nil, fmt.Errorf("unknown CRDT type: %s", crdtType)
		}
	} else if err != nil {
		return nil, err
	} else {
		// Parse existing CRDT
		if err := crdt.FromJSON(stateJSON); err != nil {
			return nil, err
		}
	}

	// Cache locally
	sm.mu.Lock()
	sm.crdts[cacheKey] = crdt
	sm.mu.Unlock()

	return crdt, nil
}

// UpdateCRDT updates a CRDT.
func (sm *SyncManager) UpdateCRDT(ctx context.Context, crdt CRDT) error {
	stateJSON, err := crdt.ToJSON()
	if err != nil {
		return err
	}

	// Create vector clock
	vectorClock := map[string]int64{
		sm.nodeID: time.Now().UnixNano(),
	}
	vectorClockJSON, _ := json.Marshal(vectorClock)

	_, err = sm.db.ExecContext(ctx,
		`INSERT INTO crdt_state (crdt_type, crdt_key, state, vector_clock, instance_id, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (crdt_type, crdt_key) DO UPDATE
		 SET state = EXCLUDED.state,
		     vector_clock = EXCLUDED.vector_clock,
		     instance_id = EXCLUDED.instance_id,
		     updated_at = NOW()`,
		crdt.Type(), crdt.Key(), stateJSON, vectorClockJSON, sm.nodeID,
	)

	return err
}

// MergeCRDTs merges a remote CRDT with the local one.
func (sm *SyncManager) MergeCRDTs(ctx context.Context, local, remote CRDT) (CRDT, error) {
	merged := local.Merge(remote)

	if err := sm.UpdateCRDT(ctx, merged); err != nil {
		return nil, err
	}

	// Update cache
	cacheKey := fmt.Sprintf("%s:%s", merged.Type(), merged.Key())
	sm.mu.Lock()
	sm.crdts[cacheKey] = merged
	sm.mu.Unlock()

	return merged, nil
}

// Close shuts down the sync manager.
func (sm *SyncManager) Close() error {
	sm.cancel()

	// Release all held locks
	sm.mu.RLock()
	locks := make([]*DistributedLock, 0, len(sm.locks))
	for _, lock := range sm.locks {
		locks = append(locks, lock)
	}
	sm.mu.RUnlock()

	for _, lock := range locks {
		sm.ReleaseLock(lock)
	}

	return nil
}

// Internal methods

func (sm *SyncManager) renewLock(lock *DistributedLock) {
	ticker := time.NewTicker(lock.TTL / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lock.mu.Lock()
			if !lock.held {
				lock.mu.Unlock()
				return
			}
			lock.mu.Unlock()

			// Renew in database
			newExpires := time.Now().Add(lock.TTL)
			_, err := sm.db.ExecContext(sm.ctx,
				"UPDATE distributed_locks SET expires_at = $1 WHERE name = $2 AND owner = $3",
				newExpires, lock.Name, lock.Owner,
			)
			if err != nil {
				sm.logger.Printf("Error renewing lock %s: %v", lock.Name, err)
				// Lock may be lost
				lock.mu.Lock()
				lock.held = false
				lock.mu.Unlock()
				return
			}

		case <-lock.renewStop:
			return
		case <-sm.ctx.Done():
			return
		}
	}
}

func (sm *SyncManager) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Clean expired locks
			_, err := sm.db.ExecContext(sm.ctx,
				"DELETE FROM distributed_locks WHERE expires_at < NOW()",
			)
			if err != nil && sm.logger != nil {
				sm.logger.Printf("Error cleaning locks: %v", err)
			}

		case <-sm.ctx.Done():
			return
		}
	}
}

// CRDT implementations

// GCounter is a grow-only counter CRDT.
type GCounter struct {
	Key_   string           `json:"key"`
	Values map[string]int64 `json:"values"`
}

// NewGCounter creates a new G-Counter.
func NewGCounter(key string) *GCounter {
	return &GCounter{
		Key_:   key,
		Values: make(map[string]int64),
	}
}

func (c *GCounter) Type() string { return "g_counter" }
func (c *GCounter) Key() string  { return c.Key_ }

func (c *GCounter) Increment(nodeID string, delta int64) {
	c.Values[nodeID] += delta
}

func (c *GCounter) Value() int64 {
	var sum int64
	for _, v := range c.Values {
		sum += v
	}
	return sum
}

func (c *GCounter) Merge(other CRDT) CRDT {
	otherC, ok := other.(*GCounter)
	if !ok {
		return c
	}

	merged := NewGCounter(c.Key_)
	for node, value := range c.Values {
		merged.Values[node] = value
	}
	for node, value := range otherC.Values {
		if value > merged.Values[node] {
			merged.Values[node] = value
		}
	}
	return merged
}

func (c *GCounter) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

func (c *GCounter) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

// PNCounter is a positive-negative counter CRDT.
type PNCounter struct {
	Key_ string
	P    *GCounter // Increments
	N    *GCounter // Decrements
}

// NewPNCounter creates a new PN-Counter.
func NewPNCounter(key string) *PNCounter {
	return &PNCounter{
		Key_: key,
		P:    NewGCounter(key + ":P"),
		N:    NewGCounter(key + ":N"),
	}
}

func (c *PNCounter) Type() string { return "pn_counter" }
func (c *PNCounter) Key() string  { return c.Key_ }

func (c *PNCounter) Increment(nodeID string, delta int64) {
	if delta >= 0 {
		c.P.Increment(nodeID, delta)
	} else {
		c.N.Increment(nodeID, -delta)
	}
}

func (c *PNCounter) Value() int64 {
	return c.P.Value() - c.N.Value()
}

func (c *PNCounter) Merge(other CRDT) CRDT {
	otherC, ok := other.(*PNCounter)
	if !ok {
		return c
	}

	merged := NewPNCounter(c.Key_)
	merged.P = c.P.Merge(otherC.P).(*GCounter)
	merged.N = c.N.Merge(otherC.N).(*GCounter)
	return merged
}

func (c *PNCounter) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"key": c.Key_,
		"p":   c.P,
		"n":   c.N,
	})
}

func (c *PNCounter) FromJSON(data []byte) error {
	var aux struct {
		Key string `json:"key"`
		P   *GCounter
		N   *GCounter
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.Key_ = aux.Key
	c.P = aux.P
	c.N = aux.N
	return nil
}

// GSet is a grow-only set CRDT.
type GSet struct {
	Key_ string
	Data map[string]struct{}
}

// NewGSet creates a new G-Set.
func NewGSet(key string) *GSet {
	return &GSet{
		Key_: key,
		Data: make(map[string]struct{}),
	}
}

func (s *GSet) Type() string { return "g_set" }
func (s *GSet) Key() string  { return s.Key_ }

func (s *GSet) Add(value string) {
	s.Data[value] = struct{}{}
}

func (s *GSet) Contains(value string) bool {
	_, ok := s.Data[value]
	return ok
}

func (s *GSet) Merge(other CRDT) CRDT {
	otherS, ok := other.(*GSet)
	if !ok {
		return s
	}

	merged := NewGSet(s.Key_)
	for v := range s.Data {
		merged.Data[v] = struct{}{}
	}
	for v := range otherS.Data {
		merged.Data[v] = struct{}{}
	}
	return merged
}

func (s *GSet) ToJSON() ([]byte, error) {
	values := make([]string, 0, len(s.Data))
	for v := range s.Data {
		values = append(values, v)
	}
	return json.Marshal(map[string]interface{}{
		"key":    s.Key_,
		"values": values,
	})
}

func (s *GSet) FromJSON(data []byte) error {
	var aux struct {
		Key    string   `json:"key"`
		Values []string `json:"values"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	s.Key_ = aux.Key
	s.Data = make(map[string]struct{})
	for _, v := range aux.Values {
		s.Data[v] = struct{}{}
	}
	return nil
}

// LWWRegister is a last-write-wins register CRDT.
type LWWRegister struct {
	Key_      string
	Value     interface{}
	Timestamp int64
	NodeID    string
}

// NewLWWRegister creates a new LWW-Register.
func NewLWWRegister(key string) *LWWRegister {
	return &LWWRegister{
		Key_: key,
	}
}

func (r *LWWRegister) Type() string { return "lww_register" }
func (r *LWWRegister) Key() string  { return r.Key_ }

func (r *LWWRegister) Set(value interface{}, timestamp int64, nodeID string) {
	if timestamp > r.Timestamp {
		r.Value = value
		r.Timestamp = timestamp
		r.NodeID = nodeID
	}
}

func (r *LWWRegister) Get() interface{} {
	return r.Value
}

func (r *LWWRegister) Merge(other CRDT) CRDT {
	otherR, ok := other.(*LWWRegister)
	if !ok {
		return r
	}

	if otherR.Timestamp > r.Timestamp {
		return otherR
	}
	return r
}

func (r *LWWRegister) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

func (r *LWWRegister) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

// Helper for JSON marshaling
var _ = json.Marshal // Ensure json package is used
