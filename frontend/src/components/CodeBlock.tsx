export function CodeBlock({ code, language="ts" }: { code:string; language?:string }) {
  return <pre data-testid="code-block" style={{background:"#0f1220",border:"1px solid var(--border)",borderRadius:12,padding:12,overflowX:"auto"}}><code>{code}</code><span style={{float:"right",fontSize:11,color:"var(--muted)"}}>{language}</span></pre>;
}
