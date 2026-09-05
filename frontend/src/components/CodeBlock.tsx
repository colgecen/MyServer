export function CodeBlock({ code, language = "bash" }: { code: string; language?: string }) {
  return (
    <pre style={{
      background: "rgba(0,0,0,0.3)",
      border: "1px solid rgba(255,255,255,0.06)",
      borderRadius: 6,
      padding: 12,
      overflowX: "auto",
    }}>
      <code style={{ color: "#d4d4d4", fontSize: 13, fontFamily: "monospace" }}>{code}</code>
      <span style={{ float: "right", fontSize: 11, color: "#555" }}>{language}</span>
    </pre>
  );
}
