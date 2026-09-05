import { useEffect, useRef, useState, useCallback } from "react";
import { DaemonWS, WSState, DaemonMessage } from "../lib/ws";

export type MessageHandler = (msg: DaemonMessage) => void;

export function useWebSocket(url?: string) {
  const ref = useRef<DaemonWS | null>(null);
  const [state, setState] = useState<WSState>("closed");
  const [lastMessage, setLastMessage] = useState<DaemonMessage | null>(null);
  const handlersRef = useRef<MessageHandler[]>([]);

  useEffect(() => {
    const ws = new DaemonWS(url);
    ref.current = ws;
    ws.connect(
      m => {
        setLastMessage(m);
        handlersRef.current.forEach(h => h(m));
      },
      setState,
    );
    return () => ws.close();
  }, [url]);

  const send = useCallback((envelope: unknown) => {
    ref.current?.send(envelope);
  }, []);

  const onMessage = useCallback((h: MessageHandler) => {
    handlersRef.current.push(h);
  }, []);

  return { state, send, lastMessage, onMessage };
}
