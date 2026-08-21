import { useEffect, useState } from "react";
type Props = { version?: string; model?: string };
export default function HeaderBar({ version = "v0.1.0", model = "llama3.1:8b" }: Props) {
  const [vram, setVram] = useState<number[]>(Array.from({length:20},()=> 40+Math.random()*40));
  useEffect(()=>{ const id=setInterval(()=> setVram(v=>[ ...v.slice(1), 30+Math.random()*60]), 1200); return ()=> clearInterval(id);},[]);
  return (
    <header className="glass" style={{display:"flex",alignItems:"center",justifyContent:"space-between",padding:"12px 16px",position:"sticky",top:0,zIndex:10}} data-testid="header-bar">
      <div style={{display:"flex",gap:12,alignItems:"center"}}>
        <span style={{padding:"4px 10px",borderRadius:999,border:"1px solid var(--neon-cyan)",color:"var(--neon-cyan)",fontSize:12}}>{version}</span>
        <select aria-label="model selector" defaultValue={model} style={{background:"transparent",color:"var(--text)",border:"1px solid var(--border)",borderRadius:8,padding:"6px 10px"}}>
          <option>{model}</option><option>qwen2.5:7b</option><option>mistral:7b</option>
        </select>
      </div>
      <div style={{display:"flex",gap:4,alignItems:"flex-end",height:28}} aria-label="GPU VRAM telemetry">
        {vram.map((v,i)=><span key={i} style={{width:6,height:`${v}%`,background:"linear-gradient(180deg,var(--neon-cyan),var(--neon-violet))",borderRadius:4,display:"inline-block"}} />)}
      </div>
    </header>
  );
}
