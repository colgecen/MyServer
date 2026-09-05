import { useState } from "react";
import HeaderBar from "./components/HeaderBar";
import LeftSidebar from "./components/LeftSidebar";
import MainCanvas from "./components/MainCanvas";
import InputHUD from "./components/InputHUD";
import { useWebSocket } from "./hooks/useWebSocket";
import type { WSState, DaemonMessage } from "./lib/ws";

export default function App() {
  const { state, messages, lastMessage, send, clearMessages } = useWebSocket();
  const [workspacePath, setWorkspacePath] = useState("");
  const [execHistory, setExecHistory] = useState<{id: string; cmd: string; output: string; level: number}[]>([]);

  return (
    <div className="app-shell" data-testid="app-shell">
      <HeaderBar
        version="v1.14.33"
        wsState={state}
      />
      <div className="app-body">
        <LeftSidebar
          workspacePath={workspacePath}
          onWorkspaceChange={setWorkspacePath}
          onSend={send}
          messages={messages}
        />
        <MainCanvas
          messages={messages}
          execHistory={execHistory}
          onSend={send}
        />
      </div>
      <InputHUD
        onSend={send}
        disabled={state !== "open"}
        workspacePath={workspacePath}
      />
    </div>
  );
}
