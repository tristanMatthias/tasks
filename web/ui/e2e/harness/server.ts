/**
 * A composable local test server: spins up the REAL `tasksd` engine on the
 * loopback interface with a throwaway temp SQLite DB and a random token. Nothing
 * here ever touches production — no agenttasks.sh, no Clerk, no shared state.
 * Each call gets its own process + DB, so tests are fully isolated.
 */
import { spawn, type ChildProcess } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import net from "node:net";

/** The engine binary built by global-setup (embeds the freshly built UI). */
export const TASKSD_BIN = join(tmpdir(), "tasksd-e2e-bin");

/** Minimal task shape used by the seeding client (mirrors model.Task). */
export interface SeedTask {
  title: string;
  issue_type?: string;
  priority?: number | null;
  parent?: string;
  deps?: string[];
  description?: string;
}
export interface Task {
  id: string;
  title: string;
  status: string;
  issue_type: string;
  priority: number | null;
  [k: string]: unknown;
}

/** An HTTP client for seeding + asserting server state, independent of the UI. */
export interface Api {
  create(t: SeedTask): Promise<Task>;
  get(id: string): Promise<Task>;
  patch(id: string, patch: Record<string, unknown>): Promise<Task>;
  close(id: string, reason?: string): Promise<void>;
  dep(blocked: string, blocker: string, type?: string): Promise<void>;
  list(): Promise<Task[]>;
}

export interface TestServer {
  baseURL: string;
  token: string;
  api: Api;
  stop(): Promise<void>;
}

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.once("error", reject);
    srv.listen(0, "127.0.0.1", () => {
      const port = (srv.address() as net.AddressInfo).port;
      srv.close(() => resolve(port));
    });
  });
}

async function waitForHealth(baseURL: string, timeoutMs = 15000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const r = await fetch(baseURL + "/healthz");
      if (r.ok) return;
    } catch {
      /* not up yet */
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error("tasksd did not become healthy at " + baseURL);
}

function makeApi(baseURL: string, token: string): Api {
  const hdr = { Authorization: "Bearer " + token, "Content-Type": "application/json" };
  const req = async (method: string, path: string, body?: unknown): Promise<Response> => {
    const r = await fetch(baseURL + path, {
      method,
      headers: hdr,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!r.ok) throw new Error(`${method} ${path} -> ${r.status} ${await r.text()}`);
    return r;
  };
  return {
    async create(t) {
      return (await req("POST", "/api/v1/tasks", t)).json() as Promise<Task>;
    },
    async get(id) {
      return (await req("GET", `/api/v1/tasks/${encodeURIComponent(id)}`)).json() as Promise<Task>;
    },
    async patch(id, patch) {
      return (await req("PATCH", `/api/v1/tasks/${encodeURIComponent(id)}`, patch)).json() as Promise<Task>;
    },
    async close(id, reason) {
      await req("POST", `/api/v1/tasks/${encodeURIComponent(id)}/close`, { reason: reason ?? "" });
    },
    async dep(blocked, blocker, type) {
      // Omit type unless explicitly given — it defaults to "blocks" server-side.
      const body: Record<string, unknown> = { blocked, blocker };
      if (type) body.type = type;
      await req("POST", "/api/v1/deps", body);
    },
    async list() {
      const d = (await req("GET", "/api/issues?view=tree")).json() as Promise<{ issues: Task[] }>;
      return (await d).issues ?? [];
    },
  };
}

export async function startServer(): Promise<TestServer> {
  const dir = mkdtempSync(join(tmpdir(), "tasksd-e2e-"));
  const db = join(dir, "test.db");
  const token = "e2e-" + Math.random().toString(36).slice(2);
  const port = await freePort();
  const baseURL = `http://127.0.0.1:${port}`;

  const proc: ChildProcess = spawn(
    TASKSD_BIN,
    ["-addr", `127.0.0.1:${port}`, "--db", db, "--token", token, "--prefix", "e2e"],
    { stdio: "pipe" },
  );
  let stderr = "";
  proc.stderr?.on("data", (b) => (stderr += String(b)));
  proc.on("exit", (code) => {
    if (code && code !== 0) console.error(`tasksd exited ${code}: ${stderr.slice(-500)}`);
  });

  try {
    await waitForHealth(baseURL);
  } catch (e) {
    proc.kill("SIGKILL");
    throw new Error(`${e}\n--- tasksd stderr ---\n${stderr.slice(-1000)}`);
  }

  return {
    baseURL,
    token,
    api: makeApi(baseURL, token),
    async stop() {
      proc.kill("SIGKILL");
      rmSync(dir, { recursive: true, force: true });
    },
  };
}
