import { useEffect, useRef, useState } from "react";
import { DaemonWS, WSState } from "../lib/ws";
export function useWebSocket(url?: string) {
  const ref = useRef<DaemonWS | null>(null);
  const [state, setState] = useState<WSState>("closed");
  const [messages, setMessages] = useState<any[]>([]);
  useEffect(()=>{
    const ws = new DaemonWS(url);
    ref.current = ws;
    ws.connect(m=> setMessages(v=> [...v,m]), setState);
    return ()=> ws.close();
  },[url]);
  return { state, messages, send:(d:any)=> ref.current?.send(d) };
}
