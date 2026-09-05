import { useState } from "react";
import { useWebSocket } from "../hooks/useWebSocket";
import type { DaemonEnvelope } from "../lib/ws";

type Props = {
  onSend?: (envelope: unknown) => void;
};

export default function LeftSidebar({ onSend }: Props) {
  const [workspace, setWorkspace] = useState("");
  const [indexing, setIndexing] = useState(false);
  const [indexStatus, setIndexStatus] = useState("");
  const { onMessage } = useWebSocket();

  // Listen for index workspace events
  onMessage((msg) => {
    const env = msg as DaemonEnvelope;
    if (env.action === "index_workspace" && env.payload) {
      const p = env.payload as { phase: string; files_done: number; files_total: number; chunks: number; message?: string };
      if (p.phase === "done") {
        setIndexing(false);
        setIndexStatus(`Indexed ${p.files_done} files, ${p.chunks} chunks`);
      } else if (p.phase === "walking") {
        setIndexing(true);
        setIndexStatus("Scanning files...");
      }
    }
  });

  const handleWorkspaceSelect = () => {
    if (!workspace.trim() || !onSend) return;
    onSend({
      id: `ws-pick-${Date.now()}`,
      type: "request",
      action: "workspace_pick",
      payload: { path: workspace.trim() },
    });
    setIndexing(true);
    setIndexStatus("Starting indexing...");
  };

  const handlePickFolder = async () => {
    // Use Tauri's native dialog or a simple prompt
    const path = prompt("Enter workspace path:", workspace);
    if (path) {
      setWorkspace(path);
      onSend?.({
        id: `ws-pick-${Date.now()}`,
        type: "request",
        action: "workspace_pick",
        payload: { path: path.trim() },
      });
      setIndexing(true);
      setIndexStatus("Starting indexing...");
    }
  };

  const chats = [
    { id: "1", title: "Explain walker" },
    { id: "2", title: "Guardrail bypass check" },
  ];

  return (
    <aside
      className="glass"
      style={{
        width: 280,
        padding: 12,
        borderRadius: 16,
        display: "flex",
        flexDirection: "column",
        gap: 12,
      }}
      data-testid="left-sidebar"
    >
      <section aria-label="workspace selector">
        <label style={{ fontSize: 12, color: "var(--muted)", display: "block", marginBottom: 6 }}>
          Workspace
        </label>
        <div style={{ display: "flex", gap: 6 }}>
          <input
            value={workspace}
            onChange={e => setWorkspace(e.target.value)}
            placeholder="/path/to/workspace"
            aria-label="workspace path"
            style={{
              flex: 1,
              padding: "8px 10px",
              borderRadius: 8,
              border: "1px solid var(--border)",
              background: "var(--glass)",
              color: "var(--text)",
              fontSize: 12,
            }}
            onKeyDown={e => {
              if (e.key === "Enter") handleWorkspaceSelect();
            }}
          />
          <button
            onClick={handlePickFolder}
            title="Browse folder"
            style={{
              padding: "8px 10px",
              borderRadius: 8,
              border: "1px solid var(--border)",
              background: "var(--glass)",
              color: "var(--neon-cyan)",
              cursor: "pointer",
            }}
          >
            📁
          </button>
        </div>
        <button
          onClick={handleWorkspaceSelect}
          disabled={!workspace.trim() || indexing}
          style={{
            width: "100%",
            marginTop: 6,
            padding: "8px 12px",
            borderRadius: 8,
            background: indexing
              ? "var(--glass)"
              : "linear-gradient(90deg,var(--neon-cyan),var(--neon-violet))",
            color: "#fff",
            border: "1px solid var(--neon-cyan)",
            cursor: indexing ? "wait" : "pointer",
            fontSize: 12,
            opacity: indexing ? 0.7 : 1,
          }}
        >
          {indexing ? "⏳ Indexing..." : "Select Workspace"}
        </button>
        {indexStatus && (
          <p style={{ fontSize: 11, color: "var(--neon-cyan)", marginTop: 4 }}>
            {indexStatus}
          </p>
        )}
      </section>

      <section aria-label="chat history">
        <h3 style={{ fontSize: 12, color: "var(--muted)", margin: "8px 0" }}>
          History
        </h3>
        {chats.map(c => (
          <div
            key={c.id}
            tabIndex={0}
            style={{
              padding: "8px 10px",
              borderRadius: 8,
              background: "var(--glass)",
              border: "1px solid var(--border)",
              marginBottom: 6,
              cursor: "pointer",
              color: "var(--text)",
              fontSize: 13,
            }}
          >
            {c.title}
          </div>
        ))}
      </section>

      <section aria-label="model downloader">
        <h3 style={{ fontSize: 12, color: "var(--muted)", margin: "8px 0" }}>
          Models
        </h3>
        <button
          style={{
            width: "100%",
            padding: "8px 12px",
            borderRadius: 8,
            background:
              "linear-gradient(90deg,var(--neon-cyan),var(--neon-violet))",
            color: "#fff",
            border: "none",
            cursor: "pointer",
            fontSize: 12,
          }}
        >
          Download Model
        </button>
      </section>
    </aside>
  );
}
