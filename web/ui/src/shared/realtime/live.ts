/**
 * A resilient WebSocket to /api/ws. The server pushes a small "changed" signal
 * after ANY mutation (HTTP/MCP/CLI, from this tab or elsewhere); the app reacts
 * by re-fetching through its normal endpoints, so nothing about the store shape
 * changes. Same-origin, so the session cookie authenticates the socket. Auto-
 * reconnects with capped exponential backoff.
 */

/** The wire message, mirroring httpapi.wsMessage (Go). */
export interface LiveMessage {
  type: "changed";
  /** Ids of the affected task(s); empty/absent means "scope unknown, refetch". */
  ids?: string[];
  /** Server freshness stamp (seconds), mirrors /api/meta mtime. */
  mtime: number;
}

const MAX_BACKOFF_MS = 15_000;

export class LiveConnection {
  #ws: WebSocket | null = null;
  readonly #onChange: (msg: LiveMessage) => void;
  #stopped = false;
  #retry = 0;
  #timer: ReturnType<typeof setTimeout> | null = null;

  constructor(onChange: (msg: LiveMessage) => void) {
    this.#onChange = onChange;
  }

  start(): void {
    if (typeof window === "undefined" || this.#stopped || this.#ws) return;
    this.#open();
  }

  stop(): void {
    this.#stopped = true;
    if (this.#timer) clearTimeout(this.#timer);
    this.#timer = null;
    // Detach handlers before closing so the close doesn't schedule a reconnect.
    if (this.#ws) {
      this.#ws.onclose = null;
      this.#ws.onerror = null;
      this.#ws.close();
      this.#ws = null;
    }
  }

  #url(): string {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    return `${proto}//${location.host}/api/ws`;
  }

  #open(): void {
    let ws: WebSocket;
    try {
      ws = new WebSocket(this.#url());
    } catch {
      this.#scheduleReconnect();
      return;
    }
    this.#ws = ws;
    ws.onopen = () => {
      this.#retry = 0; // healthy connection resets backoff
    };
    ws.onmessage = (e: MessageEvent) => {
      try {
        const msg = JSON.parse(String(e.data)) as LiveMessage;
        if (msg && msg.type === "changed") this.#onChange(msg);
      } catch {
        /* ignore malformed frames */
      }
    };
    ws.onclose = () => {
      this.#ws = null;
      this.#scheduleReconnect();
    };
    ws.onerror = () => {
      ws.close(); // let onclose drive the reconnect
    };
  }

  #scheduleReconnect(): void {
    if (this.#stopped || this.#timer) return;
    const backoff = Math.min(1000 * 2 ** this.#retry, MAX_BACKOFF_MS);
    const jitter = Math.random() * 500;
    this.#retry++;
    this.#timer = setTimeout(() => {
      this.#timer = null;
      if (!this.#stopped) this.#open();
    }, backoff + jitter);
  }
}
