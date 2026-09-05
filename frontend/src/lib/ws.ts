export type WSState = "connecting" | "open" | "closed";

export interface DaemonEnvelope {
  id?: string;
  type?: string;
  action?: string;
  protocol_version?: string;
  payload?: unknown;
  error?: { code: string; message: string };
}

export type DaemonMessage = DaemonEnvelope | string;

export class DaemonWS {
  private ws: WebSocket | null = null;
  private url: string;
  private retries = 0;
  private maxRetries = 10;
  private onOpenHandlers: Array<() => void> = [];

  constructor(url = "ws://127.0.0.1:4096/ws") {
    this.url = url;
  }

  connect(
    onMessage: (d: DaemonMessage) => void,
    onState: (s: WSState) => void,
  ) {
    onState("connecting");
    const ws = new WebSocket(this.url);
    this.ws = ws;

    ws.onopen = () => {
      this.retries = 0;
      onState("open");
      this.onOpenHandlers.forEach(h => h());
    };

    ws.onmessage = e => {
      try {
        const parsed = JSON.parse(e.data);
        onMessage(parsed as DaemonMessage);
      } catch {
        onMessage(e.data as string);
      }
    };

    ws.onclose = () => {
      onState("closed");
      if (this.retries < this.maxRetries) {
        this.retries++;
        const delay = Math.min(1000 * Math.pow(1.5, this.retries), 10000);
        setTimeout(() => this.connect(onMessage, onState), delay);
      }
    };

    ws.onerror = () => {
      onState("closed");
    };
  }

  onOpen(handler: () => void) {
    this.onOpenHandlers.push(handler);
  }

  send(data: unknown) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  close() {
    this.ws?.close();
  }
}
