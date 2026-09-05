import { useState, useCallback } from "react";
import HeaderBar from "./components/HeaderBar";
import LeftSidebar from "./components/LeftSidebar";
import MainCanvas from "./components/MainCanvas";
import InputHUD from "./components/InputHUD";
import { useWebSocket } from "./hooks/useWebSocket";

export default function App() {
  const { state, send } = useWebSocket();
  const [wsState, setWsState] = useState<string>(state);

  // Re-render on WS state change
  if (wsState !== state) {
    setWsState(state);
  }

  const handleSend = useCallback((envelope: unknown) => {
    send(envelope);
  }, [send]);

  return (
    <div className="app-shell" data-testid="app-shell">
      <HeaderBar wsState={state} />
      <div className="app-body">
        <LeftSidebar onSend={handleSend} />
        <MainCanvas onSend={handleSend} />
      </div>
      <InputHUD onSend={handleSend} disabled={state !== "open"} />
    </div>
  );
}
