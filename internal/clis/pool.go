// Package clis provides CLI agent integration for HelixAgent.
package clis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// InstancePool manages a pool of reusable agent instances.
//
// Concurrency (CONST-029 — Pattern Zeta):
//   - idle and active are stored in safe.Slice / safe.Store so that
//     single-key reads/writes and Snapshot() work without an external
//     lock.
//   - mu is RETAINED intentionally. It serialises the compound state
//     transition in Acquire() which spans three fields:
//     (a) remove from idle slice, (b) insert into active map,
//     (c) on the miss path, reserve a placeholder key in active,
//     run factory() outside the lock, then swap placeholder → real ID.
//     A concurrent Acquire caller must see a consistent active count
//     when deciding whether the pool is exhausted; splitting across
//     independent safe.Store ops would break that invariant.
//   - mu ALSO guards inst.Status mutations in terminateInstance so that
//     concurrent Release/terminate on the same AgentInstance pointer do
//     not race.
//   - safe.Store/safe.Slice operations INSIDE the mu region are fine —
//     they just add their own narrow lock under the outer RWMutex.
//   - See docs/development/concurrency-playbook.md §Pattern Zeta.
type InstancePool struct {
	agentType AgentType

	// Pool configuration
	minIdle        int
	maxIdle        int
	maxActive      int
	maxLifetime    time.Duration
	acquireTimeout time.Duration

	// Idle instances available for use
	idle   *safe.Slice[*AgentInstance]
	idleCh chan *AgentInstance

	// Active instances currently in use
	active *safe.Store[string, *AgentInstance]

	// Factory for creating new instances
	factory func() (*AgentInstance, error)

	// Metrics
	hits   uint64
	misses uint64
	evicts uint64

	// placeholderSeq is an atomic counter for generating unique placeholder IDs.
	placeholderSeq uint64

	// Control
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PoolConfig contains pool configuration.
type PoolConfig struct {
	MinIdle     int
	MaxIdle     int
	MaxActive   int
	MaxLifetime time.Duration
	// AcquireTimeout is the maximum time to wait when the pool is exhausted.
	// Defaults to 30 seconds if zero.
	AcquireTimeout time.Duration
}

// DefaultPoolConfig returns default pool configuration.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MinIdle:        2,
		MaxIdle:        10,
		MaxActive:      50,
		MaxLifetime:    1 * time.Hour,
		AcquireTimeout: 30 * time.Second,
	}
}

// NewInstancePool creates a new instance pool.
func NewInstancePool(
	agentType AgentType,
	config PoolConfig,
	factory func() (*AgentInstance, error),
) *InstancePool {
	ctx, cancel := context.WithCancel(context.Background())

	acquireTimeout := config.AcquireTimeout
	if acquireTimeout <= 0 {
		acquireTimeout = 30 * time.Second
	}

	pool := &InstancePool{
		agentType:      agentType,
		minIdle:        config.MinIdle,
		maxIdle:        config.MaxIdle,
		maxActive:      config.MaxActive,
		maxLifetime:    config.MaxLifetime,
		acquireTimeout: acquireTimeout,
		idle:           safe.NewSlice[*AgentInstance](),
		idleCh:         make(chan *AgentInstance, config.MaxIdle),
		active:         safe.NewStore[string, *AgentInstance](),
		factory:        factory,
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start maintenance goroutine
	pool.wg.Add(1)
	go pool.maintenanceLoop()

	// Pre-warm pool to minIdle
	pool.wg.Add(1)
	go pool.prewarm()

	return pool
}

// Acquire gets an instance from the pool.
func (p *InstancePool) Acquire(ctx context.Context) (*AgentInstance, error) {
	// Try to get from idle channel first (non-blocking, channel is thread-safe)
	select {
	case inst := <-p.idleCh:
		atomic.AddUint64(&p.hits, 1)
		p.mu.Lock()
		// Remove from idle slice if present (channel and slice can be out of sync)
		p.idle.Delete(func(idleInst *AgentInstance) bool { return idleInst.ID == inst.ID })
		p.active.Put(inst.ID, inst)
		p.mu.Unlock()
		return inst, nil
	default:
		// No idle instance available on channel
	}

	// Single lock for check-and-modify on idle slice AND active count check.
	// This eliminates the RLock-to-Lock gap that allowed races.
	p.mu.Lock()

	// Try idle slice under the same lock
	if n := p.idle.Len(); n > 0 {
		last, ok := p.idle.At(n - 1)
		if ok {
			// Remove the last element by id equality to keep invariants.
			p.idle.Delete(func(x *AgentInstance) bool { return x == last })
			p.active.Put(last.ID, last)
			p.mu.Unlock()
			atomic.AddUint64(&p.hits, 1)
			return last, nil
		}
	}

	// Check if we can create a new instance. If yes, reserve the slot
	// by using a placeholder so other goroutines see the correct active count.
	if p.active.Len() >= p.maxActive {
		p.mu.Unlock()
		// Pool exhausted, wait for one to become available
		select {
		case inst := <-p.idleCh:
			atomic.AddUint64(&p.hits, 1)
			p.mu.Lock()
			p.idle.Delete(func(idleInst *AgentInstance) bool { return idleInst.ID == inst.ID })
			p.active.Put(inst.ID, inst)
			p.mu.Unlock()
			return inst, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(p.acquireTimeout):
			return nil, fmt.Errorf("pool exhausted, timeout waiting for instance")
		}
	}

	// Reserve a slot with a placeholder key so concurrent goroutines see
	// the updated active count. We use a unique placeholder that cannot
	// collide with real instance IDs.
	seq := atomic.AddUint64(&p.placeholderSeq, 1)
	placeholderID := fmt.Sprintf("__placeholder_%d__", seq)
	p.active.Put(placeholderID, nil)
	p.mu.Unlock()

	// Create instance OUTSIDE the lock to avoid holding it during I/O
	atomic.AddUint64(&p.misses, 1)
	inst, err := p.factory()
	if err != nil {
		// Remove the placeholder on failure
		p.mu.Lock()
		p.active.Delete(placeholderID)
		p.mu.Unlock()
		return nil, fmt.Errorf("factory error: %w", err)
	}

	// Swap placeholder for real instance
	p.mu.Lock()
	p.active.Delete(placeholderID)
	p.active.Put(inst.ID, inst)
	p.mu.Unlock()

	return inst, nil
}

// Release returns an instance to the pool.
func (p *InstancePool) Release(inst *AgentInstance) error {
	if inst == nil {
		return nil
	}

	// Remove from active
	p.mu.Lock()
	p.active.Delete(inst.ID)

	// Check if pool is full
	if p.idle.Len() >= p.maxIdle {
		p.mu.Unlock()
		// Terminate instance instead of returning to pool
		return p.terminateInstance(inst)
	}

	// Reset instance state
	inst.SessionID = ""
	inst.TaskID = ""
	inst.Status = StatusIdle
	inst.UpdatedAt = time.Now()

	// Add to idle pool
	p.idle.Append(inst)
	p.mu.Unlock()

	// Try to add to channel (non-blocking)
	select {
	case p.idleCh <- inst:
	default:
		// Channel full, instance is in idle slice
	}

	return nil
}

// Invalidate removes an instance from the pool and terminates it.
func (p *InstancePool) Invalidate(inst *AgentInstance) error {
	if inst == nil {
		return nil
	}

	p.mu.Lock()
	p.active.Delete(inst.ID)
	// Remove from idle if present
	p.idle.Delete(func(idleInst *AgentInstance) bool { return idleInst.ID == inst.ID })
	p.mu.Unlock()

	return p.terminateInstance(inst)
}

// Stats returns pool statistics.
func (p *InstancePool) Stats() map[string]interface{} {
	// Read idle/active counts under the outer mu to preserve the compound
	// invariant: a caller that sees active_count == maxActive also sees
	// idle_count reflecting the same transition.
	p.mu.RLock()
	idleCount := p.idle.Len()
	activeCount := p.active.Len()
	p.mu.RUnlock()

	totalHits := atomic.LoadUint64(&p.hits)
	totalMisses := atomic.LoadUint64(&p.misses)
	totalRequests := totalHits + totalMisses

	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(totalHits) / float64(totalRequests)
	}

	return map[string]interface{}{
		"agent_type":   p.agentType,
		"idle_count":   idleCount,
		"active_count": activeCount,
		"hits":         totalHits,
		"misses":       totalMisses,
		"hit_rate":     hitRate,
		"evicts":       atomic.LoadUint64(&p.evicts),
		"max_idle":     p.maxIdle,
		"max_active":   p.maxActive,
	}
}

// Close shuts down the pool.
func (p *InstancePool) Close() error {
	p.cancel()

	// Wait for goroutines
	p.wg.Wait()

	// Terminate all instances
	p.mu.Lock()
	instances := make([]*AgentInstance, 0, p.idle.Len()+p.active.Len())
	instances = append(instances, p.idle.Snapshot()...)
	p.active.Range(func(_ string, inst *AgentInstance) bool {
		if inst != nil {
			instances = append(instances, inst)
		}
		return true
	})
	p.idle.Clear()
	p.active.Clear()
	p.mu.Unlock()

	for _, inst := range instances {
		p.terminateInstance(inst)
	}

	close(p.idleCh)

	return nil
}

// maintenanceLoop performs periodic maintenance.
func (p *InstancePool) maintenanceLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupExpired()
			p.ensureMinIdle()
		case <-p.ctx.Done():
			return
		}
	}
}

// cleanupExpired removes instances that have exceeded max lifetime.
func (p *InstancePool) cleanupExpired() {
	// Collect expired instances under lock, then terminate outside the lock
	// so that terminateInstance can safely acquire p.mu, and we can track
	// the goroutines with p.wg to prevent leaks on Close().
	p.mu.Lock()

	now := time.Now()
	var kept []*AgentInstance
	var expired []*AgentInstance

	for _, inst := range p.idle.Snapshot() {
		if now.Sub(inst.UpdatedAt) > p.maxLifetime {
			expired = append(expired, inst)
			atomic.AddUint64(&p.evicts, 1)
		} else {
			kept = append(kept, inst)
		}
	}

	p.idle.Replace(kept)
	p.mu.Unlock()

	// Terminate expired instances outside the lock with goroutine tracking.
	for _, inst := range expired {
		inst := inst // capture loop variable
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.terminateInstance(inst)
		}()
	}
}

// ensureMinIdle ensures minimum number of idle instances.
func (p *InstancePool) ensureMinIdle() {
	p.mu.RLock()
	currentIdle := p.idle.Len()
	p.mu.RUnlock()

	if currentIdle >= p.minIdle {
		return
	}

	needed := p.minIdle - currentIdle
	for i := 0; i < needed; i++ {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		// factory() is called outside the lock to avoid blocking
		// other pool operations during instance creation (D1 prevention).
		inst, err := p.factory()
		if err != nil {
			continue
		}

		var overflow *AgentInstance
		p.mu.Lock()
		if p.idle.Len() < p.maxIdle {
			p.idle.Append(inst)
			select {
			case p.idleCh <- inst:
			default:
			}
		} else {
			// Pool full — record for termination outside the lock.
			overflow = inst
		}
		p.mu.Unlock()

		// Terminate overflow outside the lock with goroutine tracking.
		if overflow != nil {
			p.wg.Add(1)
			go func() {
				defer p.wg.Done()
				p.terminateInstance(overflow)
			}()
		}
	}
}

// prewarm pre-warms the pool to minIdle.
func (p *InstancePool) prewarm() {
	defer p.wg.Done()

	for {
		p.mu.RLock()
		currentIdle := p.idle.Len()
		p.mu.RUnlock()

		if currentIdle >= p.minIdle {
			return
		}

		select {
		case <-p.ctx.Done():
			return
		default:
		}

		inst, err := p.factory()
		if err != nil {
			// Retry after delay
			time.Sleep(1 * time.Second)
			continue
		}

		var overflow *AgentInstance
		p.mu.Lock()
		if p.idle.Len() < p.maxIdle {
			p.idle.Append(inst)
			select {
			case p.idleCh <- inst:
			default:
			}
		} else {
			overflow = inst
		}
		p.mu.Unlock()

		// Terminate overflow outside the lock with goroutine tracking.
		if overflow != nil {
			p.wg.Add(1)
			go func() {
				defer p.wg.Done()
				p.terminateInstance(overflow)
			}()
		}
	}
}

// terminateInstance terminates an instance.
// Must NOT be called while holding p.mu.
func (p *InstancePool) terminateInstance(inst *AgentInstance) error {
	if inst == nil {
		return nil
	}
	// This would call the instance manager to terminate.
	// Hold the lock while mutating inst.Status so concurrent Release/terminate
	// calls on the same instance pointer do not race.
	p.mu.Lock()
	inst.Status = StatusTerminated
	p.mu.Unlock()
	return nil
}
