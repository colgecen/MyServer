import { useEffect, useState } from "react";
export function StreamingResponse({ stream }: { stream: AsyncIterable<string> }) {
  const [text, setText] = useState("");
  useEffect(() => {
    let cancelled = false;
    (async () => {
      for await (const chunk of stream) {
        if (cancelled) break;
        setText(t => t + chunk);
      }
    })();
    return () => { cancelled = true; };
  }, [stream]);
  return <div className="glass" style={{padding:12,borderRadius:12,whiteSpace:"pre-wrap"}} data-testid="streaming">{text}</div>;
}
