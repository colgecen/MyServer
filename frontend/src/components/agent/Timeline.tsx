type Step = { tool:string; args:string; result:string; status:"pending"|"done"|"denied" };
export default function Timeline({ steps }:{ steps: Step[] }){
  return <div className="glass" style={{padding:12,borderRadius:12}} data-testid="agent-timeline">
    {steps.map((s,i)=><div key={i} style={{display:"flex",gap:8,padding:"8px 0",borderBottom:"1px solid var(--border)"}}>
      <span style={{color:s.status==="denied"?"#ff0066":s.status==="done"?"var(--neon-cyan)":"var(--muted)"}}>●</span>
      <span style={{fontFamily:"monospace",fontSize:13}}>{s.tool} {s.args}</span>
      <span style={{marginLeft:"auto",fontSize:12,color:"var(--muted)"}}>{s.result}</span>
    </div>)}
  </div>;
}
