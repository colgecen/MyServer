import { useState } from "react";
export default function LeftSidebar() {
  const [workspace, setWorkspace] = useState("/home/user/project");
  const chats = [{id:"1",title:"Explain walker"},{id:"2",title:"Guardrail bypass check"}];
  return (
    <aside className="glass" style={{width:280,padding:12,borderRadius:16,display:"flex",flexDirection:"column",gap:12}} data-testid="left-sidebar">
      <label style={{fontSize:12,color:"var(--muted)"}}>Workspace
        <input value={workspace} onChange={e=> setWorkspace(e.target.value)} placeholder="/path/to/workspace" aria-label="workspace selector" style={{width:"100%",marginTop:6,padding:"8px 10px",borderRadius:8,border:"1px solid var(--border)",background:"var(--glass)",color:"var(--text)"}} />
      </label>
      <section aria-label="chat history"><h3 style={{fontSize:12,color:"var(--muted)",margin:"8px 0"}}>History</h3>
        {chats.map(c=> <div key={c.id} className="focus-visible" tabIndex={0} style={{padding:"8px 10px",borderRadius:8,background:"var(--glass)",border:"1px solid var(--border)",marginBottom:6,cursor:"pointer"}}>{c.title}</div>)}
      </section>
      <section aria-label="model downloader"><h3 style={{fontSize:12,color:"var(--muted)"}}>Models</h3>
        <button style={{width:"100%",padding:"8px 12px",borderRadius:8,background:"linear-gradient(90deg,var(--neon-cyan),var(--neon-violet))",color:"#fff",border:"none",cursor:"pointer"}}>Download Model</button>
      </section>
    </aside>
  );
}
