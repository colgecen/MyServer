package telemetry

import (
	"sync"

	"github.com/anomalyco/myserver/daemon/internal/protocol"
)

// Ring holds last N telemetry samples.
type Ring struct {
	mu   sync.Mutex
	buf  []protocol.TelemetryEvent
	cap  int
	head int
	size int
}

func NewRing(cap int) *Ring {
	if cap <= 0 {
		cap = 300
	}
	return &Ring{cap: cap, buf: make([]protocol.TelemetryEvent, cap)}
}

func (r *Ring) Push(ev protocol.TelemetryEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = ev
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
}

func (r *Ring) Recent(n int) []protocol.TelemetryEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n > r.size {
		n = r.size
	}
	out := make([]protocol.TelemetryEvent, n)
	for i := 0; i < n; i++ {
		idx := (r.head - n + i + r.cap) % r.cap
		out[i] = r.buf[idx]
	}
	return out
}

func (r *Ring) Len() int { r.mu.Lock(); defer r.mu.Unlock(); return r.size }
