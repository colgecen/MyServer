package telemetry_test

import (
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/telemetry"
)

func BenchmarkCollector_Sample(b *testing.B) {
	c := telemetry.NewCollector()
	for i := 0; i < b.N; i++ {
		_ = c.Sample()
	}
}

func BenchmarkRing_Push(b *testing.B) {
	r := telemetry.NewRing(300)
	ev := telemetry.NewCollector().Sample()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Push(ev)
	}
}
