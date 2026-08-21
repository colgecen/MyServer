export type WSState = "connecting" | "open" | "closed";
export class DaemonWS {
  private ws: WebSocket | null = null;
  private url: string;
  private retries = 0;
  private maxRetries = 10;
  constructor(url = "ws://127.0.0.1:4096/ws") { this.url = url; }
  connect(onMessage:(d:any)=>void, onState:(s:WSState)=>void) {
    onState("connecting");
    const ws = new WebSocket(this.url);
    this.ws = ws;
    ws.onopen = () => { this.retries=0; onState("open"); };
    ws.onmessage = e => { try{ onMessage(JSON.parse(e.data)); }catch{ onMessage(e.data);} };
    ws.onclose = () => {
      onState("closed");
      if (this.retries < this.maxRetries) {
        this.retries++;
        const delay = Math.min(1000*Math.pow(1.5, this.retries), 10000);
        setTimeout(()=> this.connect(onMessage,onState), delay);
      }
    };
  }
  send(data: unknown){ this.ws?.send(JSON.stringify(data)); }
  close(){ this.ws?.close(); }
}
