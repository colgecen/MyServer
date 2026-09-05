export function ReasoningPanel({ thinking }: { thinking: string }) {
  return (
    <details className="glass" style={{ padding: 12, borderRadius: 8 }}>
      <summary style={{ cursor: "pointer", color: "#00b4d8", fontSize: 12 }}>
        Reasoning
      </summary>
      <pre style={{
        whiteSpace: "pre-wrap",
        fontFamily: "monospace",
        fontSize: 13,
        color: "#888",
        marginTop: 8,
      }}>
        {thinking}
      </pre>
    </details>
  );
}
