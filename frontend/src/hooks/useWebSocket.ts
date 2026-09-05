import { useEffect, useRef, useState, useCallback } from "react";
import { DaemonWS, WSState, DaemonMessage } from "../lib/ws";

export function useWebSocket(url?: string) {
  const ref = useRef<DaemonWS | null>(null);
  const [state, setState] = useState<WSState>("closed");
  const [messages, setMessages] = useState<DaemonMessage[]>([]);
  const [lastMessage, setLastMessage] = useState<DaemonMessage | null>(null);

  useEffect(() => {
    const ws = new DaemonWS(url);
    ref.current = ws;
    ws.connect(
      m => {
        setLastMessage(m);
        setMessages(v => [...v, m]);
      },
      setState,
    );
    return () => ws.close();
  }, [url]);

  const send = useCallback((envelope: unknown) => {
    ref.current?.send(envelope);
  }, []);

  const clearMessages = useCallback(() => {
    setMessages([]);
    setLastMessage(null);
  }, []);

  return { state, messages, lastMessage, send, clearMessages, ref };
}
