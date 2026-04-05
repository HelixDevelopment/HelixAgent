// Package clis provides CLI agent integration for HelixAgent.
package clis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// InstancePool manages a pool of reusable agent instances.
type InstancePool struct {
	agentType AgentType

	// Pool configuration
	minIdle   int
	maxIdle   int
	maxActive int
	maxLifetime time.Duration

	// Idle instances available for use
	idle []*AgentInstance
	idleCh chan *AgentInstance

	// Active instances currently in use
	active map[string]*AgentInstance

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
}

// DefaultPoolConfig returns default pool configuration.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MinIdle:     2,
		MaxIdle:     10,
		MaxActive:   50,
		MaxLifetime: 1 * time.Hour,
	}
}

// NewInstancePool creates a new instance pool.
func NewInstancePool(
	agentType AgentType,
	config PoolConfig,
	factory func() (*AgentInstance, error),
) *InstancePool {
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &InstancePool{
		agentType:   agentType,
		minIdle:     config.MinIdle,
		maxIdle:     config.MaxIdle,
		maxActive:   config.MaxActive,
		maxLifetime: config.MaxLifetime,
		idle:        make([]*AgentInstance, 0, config.MaxIdle),
		idleCh:      make(chan *AgentInstance, config.MaxIdle),
		active:      make(map[string]*AgentInstance),
		factory:     factory,
		ctx:         ctx,
		cancel:      cancel,
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
		for i, idleInst := range p.idle {
			if idleInst.ID == inst.ID {
				p.idle = append(p.idle[:i], p.idle[i+1:]...)
				break
			}
		}
		p.active[inst.ID] = inst
		p.mu.Unlock()
		return inst, nil
	default:
		// No idle instance available on channel
	}

	// Single lock for check-and-modify on idle slice AND active count check.
	// This eliminates the RLock-to-Lock gap that allowed races.
	p.mu.Lock()

	// Try idle slice under the same lock
	if len(p.idle) > 0 {
		inst := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]
		p.active[inst.ID] = inst
		p.mu.Unlock()
		atomic.AddUint64(&p.hits, 1)
		return inst, nil
	}

	// Check if we can create a new instance. If yes, reserve the slot
	// by using a placeholder so other goroutines see the correct active count.
	if len(p.active) >= p.maxActive {
		p.mu.Unlock()
		// Pool exhausted, wait for one to become available
		select {
		case inst := <-p.idleCh:
			atomic.AddUint64(&p.hits, 1)
			p.mu.Lock()
			for i, idleInst := range p.idle {
				if idleInst.ID == inst.ID {
					p.idle = append(p.idle[:i], p.idle[i+1:]...)
					break
				}
			}
			p.active[inst.ID] = inst
			p.mu.Unlock()
			return inst, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Second):
			return nil, fmt.Errorf("pool exhausted, timeout waiting for instance")
		}
	}

	// Reserve a slot with a placeholder key so concurrent goroutines see
	// the updated active count. We use a unique placeholder that cannot
	// collide with real instance IDs.
	seq := atomic.AddUint64(&p.placeholderSeq, 1)
	placeholderID := fmt.Sprintf("__placeholder_%d__", seq)
	p.active[placeholderID] = nil
	p.mu.Unlock()

	// Create instance OUTSIDE the lock to avoid holding it during I/O
	atomic.AddUint64(&p.misses, 1)
	inst, err := p.factory()
	if err != nil {
		// Remove the placeholder on failure
		p.mu.Lock()
		delete(p.active, placeholderID)
		p.mu.Unlock()
		return nil, fmt.Errorf("factory error: %w", err)
	}

	// Swap placeholder for real instance
	p.mu.Lock()
	delete(p.active, placeholderID)
	p.active[inst.ID] = inst
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
	delete(p.active, inst.ID)
	
	// Check if pool is full
	if len(p.idle) >= p.maxIdle {
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
	p.idle = append(p.idle, inst)
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
	delete(p.active, inst.ID)
	
	// Remove from idle if present
	for i, idleInst := range p.idle {
		if idleInst.ID == inst.ID {
			p.idle = append(p.idle[:i], p.idle[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
	
	return p.terminateInstance(inst)
}

// Stats returns pool statistics.
func (p *InstancePool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	totalHits := atomic.LoadUint64(&p.hits)
	totalMisses := atomic.LoadUint64(&p.misses)
	totalRequests := totalHits + totalMisses
	
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(totalHits) / float64(totalRequests)
	}
	
	return map[string]interface{}{
		"agent_type":     p.agentType,
		"idle_count":     len(p.idle),
		"active_count":   len(p.active),
		"hits":           totalHits,
		"misses":         totalMisses,
		"hit_rate":       hitRate,
		"evicts":         atomic.LoadUint64(&p.evicts),
		"max_idle":       p.maxIdle,
		"max_active":     p.maxActive,
	}
}

// Close shuts down the pool.
func (p *InstancePool) Close() error {
	p.cancel()
	
	// Wait for goroutines
	p.wg.Wait()
	
	// Terminate all instances
	p.mu.Lock()
	instances := make([]*AgentInstance, 0, len(p.idle)+len(p.active))
	instances = append(instances, p.idle...)
	for _, inst := range p.active {
		instances = append(instances, inst)
	}
	p.idle = p.idle[:0]
	p.active = make(map[string]*AgentInstance)
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
	p.mu.Lock()
	defer p.mu.Unlock()
	
	now := time.Now()
	var kept []*AgentInstance
	
	for _, inst := range p.idle {
		if now.Sub(inst.UpdatedAt) > p.maxLifetime {
			// Instance expired, terminate asynchronously
			go p.terminateInstance(inst)
			atomic.AddUint64(&p.evicts, 1)
		} else {
			kept = append(kept, inst)
		}
	}
	
	p.idle = kept
}

// ensureMinIdle ensures minimum number of idle instances.
func (p *InstancePool) ensureMinIdle() {
	p.mu.Lock()
	currentIdle := len(p.idle)
	p.mu.Unlock()
	
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
		
		inst, err := p.factory()
		if err != nil {
			continue
		}
		
		p.mu.Lock()
		if len(p.idle) < p.maxIdle {
			p.idle = append(p.idle, inst)
			select {
			case p.idleCh <- inst:
			default:
			}
		} else {
			// Pool full, terminate new instance
			go p.terminateInstance(inst)
		}
		p.mu.Unlock()
	}
}

// prewarm pre-warms the pool to minIdle.
func (p *InstancePool) prewarm() {
	defer p.wg.Done()
	
	for {
		p.mu.Lock()
		currentIdle := len(p.idle)
		p.mu.Unlock()
		
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
		
		p.mu.Lock()
		if len(p.idle) < p.maxIdle {
			p.idle = append(p.idle, inst)
			select {
			case p.idleCh <- inst:
			default:
			}
		} else {
			go p.terminateInstance(inst)
		}
		p.mu.Unlock()
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
