package container

import (
	"context"
	"sync"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// Pool maintains pre-warmed execution capacity for low-latency dispatch.
type Pool struct {
	primaryRuntime  Runtime
	fallbackRuntime Runtime
	capacity        int
	mu              sync.RWMutex
	activeCount     int
	idleCount       int
}

// NewPool initializes a pre-warmed runner pool.
func NewPool(primary Runtime, fallback Runtime, capacity int) *Pool {
	if capacity <= 0 {
		capacity = 2
	}
	p := &Pool{
		primaryRuntime:  primary,
		fallbackRuntime: fallback,
		capacity:        capacity,
		idleCount:       capacity,
	}
	return p
}

// ActiveRuntime selects the available isolation engine.
func (p *Pool) ActiveRuntime(ctx context.Context) Runtime {
	if p.primaryRuntime != nil && p.primaryRuntime.IsAvailable(ctx) {
		return p.primaryRuntime
	}
	return p.fallbackRuntime
}

// Execute leases execution capacity, runs the command, and updates metrics.
func (p *Pool) Execute(ctx context.Context, cmdStr string, opts ExecOptions) (*types.ExecutionResult, error) {
	p.mu.Lock()
	if p.idleCount > 0 {
		p.idleCount--
	}
	p.activeCount++
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.activeCount--
		if p.idleCount < p.capacity {
			p.idleCount++
		}
		p.mu.Unlock()
	}()

	rt := p.ActiveRuntime(ctx)
	return rt.Execute(ctx, cmdStr, opts)
}

// Stats returns the active and idle worker counts.
func (p *Pool) Stats() (active int, idle int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.activeCount, p.idleCount
}

// Close releases resources across all engines.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var err error
	if p.primaryRuntime != nil {
		if e := p.primaryRuntime.Close(); e != nil {
			err = e
		}
	}
	if p.fallbackRuntime != nil {
		if e := p.fallbackRuntime.Close(); e != nil {
			err = e
		}
	}
	return err
}
