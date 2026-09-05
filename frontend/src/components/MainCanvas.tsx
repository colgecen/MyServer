import { useEffect, useState } from "react";
import type { DaemonMessage } from "../lib/ws";

type ExecEntry = {
  id: string;
  cmd: string;
  output: string;
  exitCode: number;
  done: boolean;
  level: number;
};

type PendingConfirm = {
  request_id: string;
  command: string;
  level: number;
  classification: string;
};

type Props = {
  messages: DaemonMessage[];
  execHistory: {id: string; cmd: string; output: string; level: number}[];
  onSend: (msg: unknown) => void;
};

export default function MainCanvas({ messages, execHistory, onSend }: Props) {
  const [entries, setEntries] = useState<ExecEntry[]>([]);
  const [pending, setPending] = useState<PendingConfirm | null>(null);

  useEffect(() => {
    const last = messages[messages.length - 1];
    if (!last || typeof last === "string") return;

    const env = last as Record<string, unknown>;

    // Confirmation event from guardrail
    if (env.type === "event" && (env.action as string) === "exec_command") {
      const p = env.payload as Record<string, unknown>;
      if (p && p.request_id && p.command) {
        setPending({
          request_id: p.request_id as string,
          command: p.command as string,
          level: p.level as number,
          classification: p.classification as string,
        });
      }
      return;
    }

    // Stream events
    if (env.type === "stream") {
      const p = env.payload as Record<string, unknown>;
      if (!p) return;
      const reqId = p.request_id as string;
      const stream = p.stream as string;
      const data = p.data as string;
      const done = p.done as boolean;
      const exitCode = p.exit_code as number;

      setEntries(prev => {
        const existing = prev.find(e => e.id === reqId);
        if (existing) {
          if (done) {
            return prev.map(e => e.id === reqId ? {...e, done: true, exitCode: exitCode ?? 0} : e);
          }
          return prev.map(e => e.id === reqId ? {...e, output: e.output + (data || "")} : e);
        }
        if (!done) {
          return [...prev, {id: reqId, cmd: "", output: data || "", exitCode: 0, done: false, level: 0}];
        }
        return prev;
      });
      return;
    }

    // Response for auto-execute (L0) - start entry
    if (env.type === "response") {
      const p = env.payload as Record<string, unknown>;
      if (p && p.status === "ok" && env.id && !entries.find(e => e.id === env.id)) {
        // Will be populated by stream
      }
    }
  }, [messages]);

  const handleApprove = () => {
    if (!pending) return;
    onSend({
      id: `approve-${Date.now()}`,
      type: "request",
      action: "exec_approve",
      payload: { request_id: pending.request_id, approve: true },
    });
    setPending(null);
  };

  const handleDeny = () => {
    if (!pending) return;
    onSend({
      id: `deny-${Date.now()}`,
      type: "request",
      action: "exec_approve",
      payload: { request_id: pending.request_id, approve: false },
    });
    setPending(null);
  };

  return (
    <main style={{
      flex: 1,
      display: "flex",
      flexDirection: "column",
      padding: 20,
      overflowY: "auto",
      gap: 12,
    }} data-testid="main-canvas">
      {/* Confirmation dialog */}
      {pending && (
        <div style={{
          background: "rgba(0,0,0,0.4)",
          border: "1px solid #f59e0b",
          borderRadius: 8,
          padding: 16,
        }}>
          <p style={{ fontSize: 11, color: "#f59e0b", textTransform: "uppercase", letterSpacing: 1, marginBottom: 8 }}>
            Confirmation Required — {pending.classification}
          </p>
          <pre style={{
            fontFamily: "monospace",
            fontSize: 13,
            color: "#fff",
            background: "rgba(0,0,0,0.3)",
            padding: 10,
            borderRadius: 4,
            overflowX: "auto",
            whiteSpace: "pre-wrap",
            marginBottom: 12,
          }}>
            {pending.command}
          </pre>
          <div style={{ display: "flex", gap: 8 }}>
            <button
              onClick={handleApprove}
              style={{
                padding: "8px 20px",
                borderRadius: 6,
                background: "#22c55e",
                color: "#fff",
                border: "none",
                cursor: "pointer",
                fontSize: 13,
                fontWeight: 600,
              }}
            >
              Confirm
            </button>
            <button
              onClick={handleDeny}
              style={{
                padding: "8px 20px",
                borderRadius: 6,
                background: "transparent",
                color: "#888",
                border: "1px solid rgba(255,255,255,0.1)",
                cursor: "pointer",
                fontSize: 13,
              }}
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Command output history */}
      {entries.map(entry => (
        <div key={entry.id} style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <span style={{ fontSize: 11, color: entry.exitCode === 0 ? "#22c55e" : "#ef4444" }}>
              {entry.exitCode === 0 ? "✓" : "✗"} exit {entry.exitCode}
            </span>
          </div>
          {entry.output && (
            <pre style={{
              fontFamily: "monospace",
              fontSize: 13,
              color: "#d4d4d4",
              background: "rgba(0,0,0,0.3)",
              padding: 12,
              borderRadius: 6,
              overflowX: "auto",
              whiteSpace: "pre-wrap",
              border: "1px solid rgba(255,255,255,0.06)",
            }}>
              {entry.output}
            </pre>
          )}
        </div>
      ))}

      {/* Empty state */}
      {entries.length === 0 && !pending && (
        <div style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          color: "#444",
          gap: 8,
        }}>
          <span style={{ fontSize: 40 }}>▶</span>
          <p style={{ fontSize: 14 }}>Type a command below to get started</p>
        </div>
      )}
    </main>
  );
}
