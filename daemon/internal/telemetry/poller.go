package telemetry

import (
	"context"
	"time"

	"github.com/anomalyco/myserver/daemon/internal/protocol"
)

// Broadcaster abstracts daemon's WS broadcast.
type Broadcaster interface {
	Broadcast(*protocol.Envelope) error
}

// Poller samples every interval and broadcasts delta if changed.
type Poller struct {
	collector *Collector
	interval  time.Duration
	bcast     Broadcaster
	last      protocol.TelemetryEvent
}

func NewPoller(c *Collector, b Broadcaster) *Poller {
	return &Poller{collector: c, interval: time.Second, bcast: b}
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ev := p.collector.Sample()
			if ev == p.last {
				continue
			}
			p.last = ev
			if p.bcast != nil {
				if env, err := protocol.NewEvent(protocol.ActionTelemetry, ev); err == nil {
					_ = p.bcast.Broadcast(env)
				}
			}
		}
	}
}
