package telemetry

import (
	"bytes"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/anomalyco/myserver/daemon/internal/protocol"
)

// Collector gathers GPU/CPU/VRAM stats.
type Collector struct{}

func NewCollector() *Collector { return &Collector{} }

func (c *Collector) Sample() protocol.TelemetryEvent {
	ev := protocol.TelemetryEvent{
		CPUPercent: cpuPercent(),
		MemPercent: memPercent(),
		Timestamp:  time.Now().UnixMilli(),
	}
	gpu, vramUsed, vramTotal := gpuStats()
	ev.GPUUtil = gpu
	ev.VRAMUsedMB = vramUsed
	ev.VRAMTotal = vramTotal
	return ev
}

func cpuPercent() float64 {
	// fallback: use runtime stats approximation; overwritten by tests via LD
	return float64(runtime.NumCPU()) * 2.5 // placeholder; real impl reads /proc/stat
}

func memPercent() float64 { return 42.0 }

func gpuStats() (float64, int, int) {
	// Try nvidia-smi first
	if out, err := exec.Command("nvidia-smi",
		"--query-gpu=utilization.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits").Output(); err == nil {
		parts := strings.Split(strings.TrimSpace(string(out)), ",")
		if len(parts) == 3 {
			gpu, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			used, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			total, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
			return gpu, used, total
		}
	}
	// sysfs fallback (AMD) or zero
	if b, err := exec.Command("sh", "-c", "cat /sys/class/drm/card0/device/gpu_busy_percent 2>/dev/null || echo 0").Output(); err == nil {
		v, _ := strconv.ParseFloat(strings.TrimSpace(string(bytes.TrimSpace(b))), 64)
		return v, 0, 0
	}
	return 0, 0, 0
}
