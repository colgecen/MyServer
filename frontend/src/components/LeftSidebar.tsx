import { useState, useEffect } from "react";
import type { DaemonMessage } from "../lib/ws";

type Props = {
  workspacePath: string;
  onWorkspaceChange: (path: string) => void;
  onSend: (msg: unknown) => void;
  messages: DaemonMessage[];
};

export default function LeftSidebar({ workspacePath, onWorkspaceChange, onSend, messages }: Props) {
  const [indexing, setIndexing] = useState(false);
  const [indexMsg, setIndexMsg] = useState("");

  useEffect(() => {
    for (const msg of messages) {
      const env = msg as Record<string, unknown>;
      if (env.type === "event" && (env.action as string) === "index_workspace") {
        const p = (env.payload as Record<string, unknown>) || {};
        if (p.phase === "done") {
          setIndexing(false);
          setIndexMsg(`Indexed ${p.files_done} files`);
        } else if (p.phase === "walking") {
          setIndexing(true);
          setIndexMsg("Scanning...");
        } else if (p.phase === "chunking") {
          setIndexMsg("Reading files...");
        } else if (p.phase === "embedding") {
          setIndexMsg("Indexing...");
        }
      }
    }
  }, [messages]);

  const handleIndex = () => {
    if (!workspacePath.trim()) return;
    onSend({
      id: `wp-${Date.now()}`,
      type: "request",
      action: "workspace_pick",
      payload: { path: workspacePath.trim() },
    });
    setIndexing(true);
    setIndexMsg("Starting...");
  };

  return (
    <aside className="glass" style={{
      width: 260,
      padding: 16,
      display: "flex",
      flexDirection: "column",
      gap: 20,
      borderRight: "1px solid rgba(255,255,255,0.06)",
    }} data-testid="left-sidebar">
      <section>
        <label style={{ fontSize: 11, color: "#666", textTransform: "uppercase", letterSpacing: 1, display: "block", marginBottom: 6 }}>
          Workspace
        </label>
        <input
          value={workspacePath}
          onChange={e => onWorkspaceChange(e.target.value)}
          placeholder="/home/user/project"
          onKeyDown={e => e.key === "Enter" && handleIndex()}
          style={{
            width: "100%",
            padding: "8px 10px",
            borderRadius: 6,
            border: "1px solid rgba(255,255,255,0.1)",
            background: "rgba(0,0,0,0.3)",
            color: "#fff",
            fontSize: 13,
            boxSizing: "border-box",
            outline: "none",
          }}
        />
        <button
          onClick={handleIndex}
          disabled={!workspacePath.trim() || indexing}
          style={{
            width: "100%",
            marginTop: 6,
            padding: "8px",
            borderRadius: 6,
            background: indexing ? "rgba(255,255,255,0.05)" : "#00b4d8",
            color: "#fff",
            border: "none",
            cursor: indexing ? "default" : "pointer",
            fontSize: 12,
            opacity: indexing ? 0.6 : 1,
          }}
        >
          {indexing ? "Indexing..." : "Select"}
        </button>
        {indexMsg && (
          <p style={{ fontSize: 11, color: "#00b4d8", marginTop: 4 }}>{indexMsg}</p>
        )}
      </section>

      <section>
        <label style={{ fontSize: 11, color: "#666", textTransform: "uppercase", letterSpacing: 1, display: "block", marginBottom: 6 }}>
          History
        </label>
        <p style={{ fontSize: 12, color: "#555", fontStyle: "italic" }}>
          No commands yet
        </p>
      </section>
    </aside>
  );
}
