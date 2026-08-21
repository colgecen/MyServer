// Package health provides a self-monitoring heartbeat plus supervisor-style
// auto-restart of registered long-running subsystems (exec engine, workspace
// indexer, telemetry poller). The goal is to keep the daemon alive across
// transient subsystem panics without restarting the whole process.
package health

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Status describes the runtime health of a subsystem.
type Status string

const (
	StatusStarting Status = "starting"
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusStopped  Status = "stopped"
)

// Subsystem is anything that can be started, health-checked and restarted.
type Subsystem interface {
	Name() string
	Run(ctx context.Context) error
}

// Registry supervises a set of subsystems.
type Registry struct {
	mu        sync.RWMutex
	subs      []Subsystem
	status    map[string]Status
	restarts  map[string]int
	maxRetry  int
	backoff   time.Duration
	startedAt time.Time
	stopped   atomic.Bool
}

// New constructs a registry. maxRetry is the maximum consecutive restart
// attempts before a subsystem is marked Stopped.
func New(maxRetry int, backoff time.Duration) *Registry {
	if maxRetry <= 0 {
		maxRetry = 5
	}
	if backoff <= 0 {
		backoff = 2 * time.Second
	}
	return &Registry{
		status:    make(map[string]Status),
		restarts:  make(map[string]int),
		maxRetry:  maxRetry,
		backoff:   backoff,
		startedAt: time.Now(),
	}
}

// Register adds a subsystem to be supervised.
func (r *Registry) Register(s Subsystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs = append(r.subs, s)
	r.status[s.Name()] = StatusStarting
}

// Start launches all registered subsystems in their own goroutines. Cancel
// ctx to stop them. Restart is automatic (up to maxRetry) on non-nil Run
// errors.
func (r *Registry) Start(ctx context.Context) {
	r.mu.RLock()
	subs := append([]Subsystem(nil), r.subs...)
	r.mu.RUnlock()

	for _, s := range subs {
		go r.supervise(ctx, s)
	}
}

// supervise wraps a single subsystem with auto-restart logic.
func (r *Registry) supervise(ctx context.Context, s Subsystem) {
	for {
		if r.stopped.Load() || ctx.Err() != nil {
			r.setStatus(s.Name(), StatusStopped)
			return
		}
		log.Printf("health: starting subsystem %q", s.Name())
		err := s.Run(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			r.setStatus(s.Name(), StatusStopped)
			return
		}
		count := r.incRestart(s.Name())
		if count > r.maxRetry {
			log.Printf("health: subsystem %q exceeded max retries (%d): %v", s.Name(), r.maxRetry, err)
			r.setStatus(s.Name(), StatusDegraded)
			return
		}
		log.Printf("health: subsystem %q exited (%v); restarting (%d/%d) after %s",
			s.Name(), err, count, r.maxRetry, r.backoff)
		r.setStatus(s.Name(), StatusDegraded)
		select {
		case <-ctx.Done():
			return
		case <-time.After(r.backoff):
		}
	}
}

func (r *Registry) setStatus(name string, st Status) {
	r.mu.Lock()
	r.status[name] = st
	r.mu.Unlock()
}

func (r *Registry) incRestart(name string) int {
	r.mu.Lock()
	r.restarts[name]++
	n := r.restarts[name]
	r.mu.Unlock()
	return n
}

// Snapshot returns a copy of the current registry status, suitable for the
// /api/health endpoint.
func (r *Registry) Snapshot() map[string]Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]Status, len(r.status))
	for k, v := range r.status {
		out[k] = v
	}
	return out
}

// Stop marks the registry as stopped; supervise loops exit on their next
// iteration.
func (r *Registry) Stop() {
	r.stopped.Store(true)
}

// Summary is a one-line health summary for logs/metrics.
func (r *Registry) Summary() string {
	snap := r.Snapshot()
	if len(snap) == 0 {
		return "no subsystems registered"
	}
	return fmt.Sprintf("uptime=%s subsystems=%v", time.Since(r.startedAt).Truncate(time.Second), snap)
}
