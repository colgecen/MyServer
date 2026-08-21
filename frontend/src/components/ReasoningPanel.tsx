export function ReasoningPanel({ thinking }: { thinking: string }) {
  return <details className="glass" style={{padding:12,borderRadius:12}} data-testid="reasoning-panel"><summary style={{cursor:"pointer",color:"var(--neon-cyan)"}}>Reasoning</summary><pre style={{whiteSpace:"pre-wrap",fontFamily:"var(--font-mono, monospace)",fontSize:13,marginTop:8}}>{thinking}</pre></details>;
}
