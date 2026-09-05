import { useState, useCallback, useRef, useEffect } from "react";
import { ReasoningPanel } from "./ReasoningPanel";
import { CodeBlock } from "./CodeBlock";
import { useWebSocket } from "../hooks/useWebSocket";
import type { DaemonEnvelope } from "../lib/ws";

type MessageEntry = {
  id: string;
  type: "user" | "assistant" | "system";
  reasoning?: string;
  code?: string;
  language?: string;
  stream?: string;
};

type PendingConfirm = {
  request_id: string;
  command: string;
  level: number;
  classification: string;
};

type Props = {
  onSend?: (envelope: unknown) => void;
};

export default function MainCanvas({ onSend }: Props) {
  const [messages, setMessages] = useState<MessageEntry[]>([]);
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm | null>(null);
  const [reasoning, setReasoning] = useState<string>("");
  const [streamingText, setStreamingText] = useState<string>("");
  const canvasRef = useRef<HTMLDivElement>(null);
  const { onMessage } = useWebSocket();

  // Auto-scroll to bottom
  useEffect(() => {
    if (canvasRef.current) {
      canvasRef.current.scrollTop = canvasRef.current.scrollHeight;
    }
  }, [messages, reasoning, streamingText]);

  // Listen for all WS messages
  onMessage((msg) => {
    const env = msg as DaemonEnvelope;

    // Confirmation request from guardrail
    if (env.action === "exec_command" && env.payload) {
      const p = env.payload as PendingConfirm;
      if (p.request_id && p.command) {
        setPendingConfirm(p);
        setMessages(prev => [
          ...prev,
          {
            id: `confirm-${p.request_id}`,
            type: "system",
            code: p.command,
          },
        ]);
      }
    }

    // Stream events (exec_command response)
    if (env.type === "stream" && env.payload) {
      const p = env.payload as {
        request_id: string;
        stream: string;
        data: string;
        exit_code: number;
        done: boolean;
        err: string;
      };
      if (p.stream === "stdout" && p.data) {
        setStreamingText(prev => prev + p.data);
      } else if (p.stream === "system" && p.done) {
        // Execution complete
        setMessages(prev => [
          ...prev,
          {
            id: `exec-${p.request_id}`,
            type: "system",
            code: streamingText || "(no output)",
            language: "bash",
          },
        ]);
        setStreamingText("");
      }
    }

    // Index workspace events
    if (env.action === "index_workspace" && env.payload) {
      const p = env.payload as { phase: string; message?: string };
      if (p.phase === "walking") {
        setReasoning("🔍 Scanning workspace...");
      } else if (p.phase === "chunking") {
        setReasoning("📄 Analyzing file contents...");
      } else if (p.phase === "embedding") {
        setReasoning("🧠 Generating embeddings...");
      } else if (p.phase === "done") {
        setReasoning("");
      }
    }
  });

  const handleApprove = useCallback(() => {
    if (!pendingConfirm || !onSend) return;
    onSend({
      id: `approve-${Date.now()}`,
      type: "request",
      action: "exec_approve",
      payload: { request_id: pendingConfirm.request_id, approve: true },
    });
    setPendingConfirm(null);
  }, [pendingConfirm, onSend]);

  const handleDeny = useCallback(() => {
    if (!pendingConfirm || !onSend) return;
    onSend({
      id: `deny-${Date.now()}`,
      type: "request",
      action: "exec_approve",
      payload: { request_id: pendingConfirm.request_id, approve: false },
    });
    setPendingConfirm(null);
  }, [pendingConfirm, onSend]);

  return (
    <main
      ref={canvasRef}
      style={{
        flex: 1,
        display: "flex",
        flexDirection: "column",
        gap: 12,
        padding: 16,
        overflowY: "auto",
      }}
      data-testid="main-canvas"
    >
      {/* Render message history */}
      {messages.map(m => {
        if (m.type === "system" && m.code) {
          return (
            <div key={m.id} style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <CodeBlock code={m.code} language={m.language || "bash"} />
            </div>
          );
        }
        if (m.type === "assistant" || m.type === "user") {
          return (
            <div key={m.id}>
              {m.reasoning && <ReasoningPanel thinking={m.reasoning} />}
              {m.code && <CodeBlock code={m.code} language={m.language || "bash"} />}
            </div>
          );
        }
        return null;
      })}

      {/* Live reasoning */}
      {reasoning && (
        <ReasoningPanel thinking={reasoning} />
      )}

      {/* Live streaming output */}
      {streamingText && (
        <CodeBlock code={streamingText} language="bash" />
      )}

      {/* Confirmation dialog */}
      {pendingConfirm && (
        <div
          className="glass"
          style={{
            padding: 16,
            borderRadius: 12,
            border: "1px solid var(--neon-magenta)",
            display: "flex",
            flexDirection: "column",
            gap: 12,
          }}
          data-testid="confirm-dialog"
        >
          <div>
            <span style={{ fontSize: 11, color: "var(--neon-magenta)", textTransform: "uppercase" }}>
              {pendingConfirm.classification}
            </span>
            <p style={{ fontFamily: "monospace", fontSize: 13, color: "var(--text)", marginTop: 4 }}>
              {pendingConfirm.command}
            </p>
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <button
              onClick={handleApprove}
              data-testid="confirm-run"
              style={{
                padding: "8px 18px",
                borderRadius: 8,
                background: "var(--neon-cyan)",
                color: "#000",
                border: "none",
                cursor: "pointer",
                fontWeight: 600,
              }}
            >
              CONFIRM
            </button>
            <button
              onClick={handleDeny}
              data-testid="cancel-run"
              style={{
                padding: "8px 18px",
                borderRadius: 8,
                background: "transparent",
                color: "var(--neon-magenta)",
                border: "1px solid var(--neon-magenta)",
                cursor: "pointer",
              }}
            >
              CANCEL
            </button>
          </div>
        </div>
      )}

      {/* Default placeholder when idle */}
      {messages.length === 0 && !reasoning && !streamingText && !pendingConfirm && (
        <div style={{ flex: 1, display: "flex", alignItems: "center", justifyContent: "center", flexDirection: "column", gap: 12, color: "var(--muted)" }}>
          <span style={{ fontSize: 48 }}>🤖</span>
          <p style={{ fontSize: 14 }}>Select a workspace and send a prompt to begin</p>
        </div>
      )}
    </main>
  );
}
