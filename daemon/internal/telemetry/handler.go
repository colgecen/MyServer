package telemetry

import (
	"context"

	"github.com/anomalyco/myserver/daemon/internal/protocol"
)

// Handler exposes telemetry over WS: on ActionTelemetry request, reply with latest sample.
type Handler struct {
	collector *Collector
}

func NewHandler(c *Collector) *Handler { return &Handler{collector: c} }

func (h *Handler) Handle() (*protocol.Envelope, error) {
	ev := h.collector.Sample()
	return protocol.NewEvent(protocol.ActionTelemetry, ev)
}

// Ensure context usage for future streaming
var _ = context.Background
