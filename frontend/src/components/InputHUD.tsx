import { useState, useCallback } from "react";

type Props = {
  onSend: (msg: unknown) => void;
  disabled?: boolean;
  workspacePath?: string;
};

export default function InputHUD({ onSend, disabled, workspacePath }: Props) {
  const [cmd, setCmd] = useState("");

  const handleSend = useCallback(() => {
    if (!cmd.trim() || disabled) return;
    onSend({
      id: `cmd-${Date.now()}`,
      type: "request",
      action: "exec_command",
      payload: {
        command: cmd.trim(),
        shell: "bash",
        workdir: workspacePath || "",
      },
    });
    setCmd("");
  }, [cmd, disabled, onSend, workspacePath]);

  return (
    <footer style={{
      padding: "12px 20px",
      borderTop: "1px solid rgba(255,255,255,0.06)",
      display: "flex",
      gap: 8,
    }} data-testid="input-hud">
      <input
        value={cmd}
        onChange={e => setCmd(e.target.value)}
        onKeyDown={e => {
          if (e.key === "Enter") handleSend();
        }}
        placeholder={disabled ? "Daemon offline..." : "Enter command..."}
        disabled={disabled}
        style={{
          flex: 1,
          padding: "10px 14px",
          borderRadius: 8,
          border: "1px solid rgba(255,255,255,0.1)",
          background: "rgba(0,0,0,0.3)",
          color: "#fff",
          fontSize: 14,
          outline: "none",
        }}
      />
      <button
        onClick={handleSend}
        disabled={disabled || !cmd.trim()}
        style={{
          padding: "10px 20px",
          borderRadius: 8,
          background: disabled || !cmd.trim() ? "rgba(255,255,255,0.05)" : "#00b4d8",
          color: "#fff",
          border: "none",
          cursor: disabled || !cmd.trim() ? "default" : "pointer",
          fontSize: 13,
          fontWeight: 600,
          opacity: disabled || !cmd.trim() ? 0.5 : 1,
        }}
      >
        Run
      </button>
    </footer>
  );
}
