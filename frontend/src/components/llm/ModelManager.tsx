import { useEffect, useState } from "react";
type Model = { name: string };
export default function ModelManager({ apiBase="http://127.0.0.1:11434" }: { apiBase?: string }) {
  const [models,setModels]=useState<Model[]>([]);
  const [pull,setPull]=useState("");
  useEffect(()=>{ fetch(`${apiBase}/api/tags`).then(r=>r.json()).then(d=> setModels(d.models||[])).catch(()=>{}); },[apiBase]);
  return (
    <div className="glass" style={{padding:12,borderRadius:12}} data-testid="model-manager">
      <h3>Models</h3>
      <ul>{models.map(m=> <li key={m.name}>{m.name} <button onClick={()=> fetch(`${apiBase}/api/delete`,{method:"DELETE",body:JSON.stringify({name:m.name})})}>Delete</button></li>)}</ul>
      <div style={{display:"flex",gap:8,marginTop:8}}>
        <input aria-label="model name" value={pull} onChange={e=> setPull(e.target.value)} placeholder="llama3.1:8b" style={{flex:1,padding:8,borderRadius:8,border:"1px solid var(--border)"}} />
        <button onClick={()=> fetch(`${apiBase}/api/pull`,{method:"POST",body:JSON.stringify({name:pull})})}>Pull</button>
        <select aria-label="quantize" defaultValue="Q4_K_M"><option>Q4_K_M</option><option>Q8_0</option></select>
      </div>
    </div>
  );
}
