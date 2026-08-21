import { ReasoningPanel } from "./ReasoningPanel";
import { CodeBlock } from "./CodeBlock";
export default function MainCanvas() {
  return (
    <main style={{flex:1,display:"flex",flexDirection:"column",gap:12}} data-testid="main-canvas">
      <ReasoningPanel thinking={"The user wants a file walker with ignore rules. I will check .gitignore and..."} />
      <CodeBlock code={"go test ./..."} language="bash" />
      <button data-testid="run-command" style={{alignSelf:"flex-start",padding:"10px 18px",borderRadius:999,background:"linear-gradient(90deg,var(--neon-cyan),var(--neon-magenta))",color:"#fff",border:"none",cursor:"pointer",fontWeight:600}}>RUN COMMAND</button>
    </main>
  );
}
