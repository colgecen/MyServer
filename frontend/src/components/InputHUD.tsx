import { useState, useCallback, KeyboardEvent } from "react";
import type { DaemonEnvelope } from "../lib/ws";

type Props = {
  onSend?: (envelope: unknown) => void;
  disabled?: boolean;
};

export default function InputHUD({ onSend, disabled = false }: Props) {
  const [prompt, setPrompt] = useState("");

  const handleSend = useCallback(() => {
    if (!prompt.trim() || !onSend) return;
    const text = prompt.trim();
    setPrompt("");
    onSend({
      id: `chat-${Date.now()}`,
      type: "request",
      action: "exec_command",
      payload: {
        command: text,
        shell: "bash",
        workdir: "",
      },
    });
  }, [prompt, onSend]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend],
  );

  return (
    <footer
      className="glass"
      style={{
        margin: 16,
        padding: 12,
        borderRadius: 24,
        display: "flex",
        gap: 8,
        alignItems: "center",
      }}
      data-testid="input-hud"
    >
      <input
        aria-label="prompt"
        value={prompt}
        onChange={e => setPrompt(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={disabled ? "Daemon disconnected..." : "Ask or type a command..."}
        disabled={disabled}
        style={{
          flex: 1,
          padding: "12px 16px",
          borderRadius: 999,
          border: "1px solid var(--border)",
          background: "var(--glass)",
          color: "var(--text)",
          fontSize: 14,
          outline: "none",
        }}
      />
      <label
        style={{
          padding: "8px 12px",
          borderRadius: 999,
          border: "1px solid var(--border)",
          cursor: disabled ? "not-allowed" : "pointer",
          opacity: disabled ? 0.5 : 1,
        }}
      >
        Attach
        <input
          type="file"
          hidden
          disabled={disabled}
          data-testid="file-attach"
        />
      </label>
      <button
        onClick={handleSend}
        disabled={disabled || !prompt.trim()}
        aria-label="execute command"
        style={{
          padding: "10px 16px",
          borderRadius: 999,
          background: "var(--neon-violet)",
          color: "#fff",
          border: "none",
          cursor: disabled || !prompt.trim() ? "wait" : "pointer",
          opacity: disabled || !prompt.trim() ? 0.5 : 1,
        }}
      >
        ↵
      </button>
    </footer>
  );
}
