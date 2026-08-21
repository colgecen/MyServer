import { useEffect, useState } from "react";
import { useWebSocket } from "../hooks/useWebSocket";
type Sample = { gpu_util:number; vram_used_mb:number; cpu_percent:number; timestamp:number };
export default function TelemetryCharts(){
  const { messages } = useWebSocket();
  const [samples, setSamples] = useState<Sample[]>([]);
  useEffect(()=>{
    const last = messages[messages.length-1];
    if(last?.action==="telemetry" && last.payload){
      try{
        const p = typeof last.payload==="string"? JSON.parse(last.payload): last.payload;
        setSamples(s=> [...s.slice(-59), p]);
      }catch{}
    }
  },[messages]);
  return <div className="glass" style={{padding:12,borderRadius:12}} data-testid="telemetry-charts">
    <div style={{display:"flex",gap:2,height:40,alignItems:"flex-end"}}>
      {samples.map((s,i)=><span key={i} style={{flex:1,background:"linear-gradient(180deg,var(--neon-cyan),var(--neon-violet))",height:`${Math.min(100,s.gpu_util)}%`,borderRadius:2}}/>)}
    </div>
    <div style={{fontSize:11,color:"var(--muted)",marginTop:6}}>GPU {samples.at(-1)?.gpu_util?.toFixed(1)??0}% • VRAM {samples.at(-1)?.vram_used_mb??0} MB</div>
  </div>;
}
