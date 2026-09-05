import { useEffect, useState } from "react";
import { useWebSocket } from "../hooks/useWebSocket";
import type { DaemonEnvelope } from "../lib/ws";

type Sample = {
  gpu_util: number;
  vram_used_mb: number;
  cpu_percent: number;
  timestamp: number;
};

export default function TelemetryCharts() {
  const { onMessage } = useWebSocket();
  const [samples, setSamples] = useState<Sample[]>([]);

  useEffect(() => {
    const handler = (msg: unknown) => {
      const env = msg as DaemonEnvelope;
      if (env.action === "telemetry" && env.payload) {
        try {
          const p = env.payload as Sample;
          if (typeof p.gpu_util === "number") {
            setSamples(s => [...s.slice(-59), p]);
          }
        } catch {
          // ignore parse errors
        }
      }
    };
    onMessage(handler);
  }, [onMessage]);

  const last = samples.length > 0 ? samples[samples.length - 1] : null;

  return (
    <div
      className="glass"
      style={{ padding: 12, borderRadius: 12 }}
      data-testid="telemetry-charts"
    >
      <div
        style={{
          display: "flex",
          gap: 2,
          height: 40,
          alignItems: "flex-end",
        }}
      >
        {samples.map((s, i) => (
          <span
            key={i}
            style={{
              flex: 1,
              height: `${Math.min(100, s.gpu_util)}%`,
              background:
                "linear-gradient(180deg,var(--neon-cyan),var(--neon-violet))",
              borderRadius: 2,
            }}
          />
        ))}
      </div>
      <div style={{ fontSize: 11, color: "var(--muted)", marginTop: 6 }}>
        GPU{" "}
        {last?.gpu_util?.toFixed(1) ?? 0}% • VRAM{" "}
        {last?.vram_used_mb ?? 0} MB
      </div>
    </div>
  );
}
