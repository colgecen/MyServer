import type { WSState } from "../lib/ws";

type Props = {
  version?: string;
  wsState?: WSState;
};

export default function HeaderBar({ version = "v1.14.33", wsState = "closed" }: Props) {
  const connected = wsState === "open";
  const color = connected ? "#22c55e" : "#ef4444";
  const label = connected ? "CONNECTED" : "OFFLINE";

  return (
    <header className="glass" style={{
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      padding: "12px 20px",
      borderBottom: "1px solid rgba(255,255,255,0.06)",
    }} data-testid="header-bar">
      <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
        <span style={{ fontSize: 16, fontWeight: 700, color: "#fff", letterSpacing: 1 }}>
          MyServer
        </span>
        <span style={{
          padding: "2px 8px",
          borderRadius: 4,
          background: "rgba(255,255,255,0.06)",
          color: "#888",
          fontSize: 11,
        }}>
          {version}
        </span>
      </div>
      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <span style={{
          width: 8,
          height: 8,
          borderRadius: "50%",
          background: color,
          boxShadow: `0 0 6px ${color}`,
        }} />
        <span style={{ fontSize: 11, color: "#888", letterSpacing: 1 }}>{label}</span>
      </div>
    </header>
  );
}
