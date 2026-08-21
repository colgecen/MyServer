import { useState } from "react";
export default function InputHUD() {
  const [prompt, setPrompt] = useState("");
  return (
    <footer className="glass" style={{margin:16,padding:12,borderRadius:24,display:"flex",gap:8,alignItems:"center"}} data-testid="input-hud">
      <input aria-label="prompt" value={prompt} onChange={e=> setPrompt(e.target.value)} placeholder="Ask or type a command..." style={{flex:1,padding:"12px 16px",borderRadius:999,border:"1px solid var(--border)",background:"var(--glass)",color:"var(--text)"}} />
      <label style={{padding:"8px 12px",borderRadius:999,border:"1px solid var(--border)",cursor:"pointer"}}>Attach
        <input type="file" hidden data-testid="file-attach" />
      </label>
      <button aria-label="execute in terminal" style={{padding:"10px 16px",borderRadius:999,background:"var(--neon-violet)",color:"#fff",border:"none",cursor:"pointer"}}>↵</button>
    </footer>
  );
}
