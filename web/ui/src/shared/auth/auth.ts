/**
 * Auth data access. The engine drives the login flow: /api/authinfo reports the
 * mode and whether the caller is authenticated; /api/login + /api/logout manage
 * the browser session; /api/v1/keys mints/lists/revokes API keys for bots/agents.
 */

/** How the server authenticates: open, a shared token, or an embedder's own. */
export type AuthMode = "none" | "token" | "custom";

export interface AuthInfo {
  mode: AuthMode;
  authenticated: boolean;
  /** For "custom" mode: where to send the user to sign in (e.g. Clerk). */
  login_url?: string;
  /** Server-verified active workspace (org id), when the embedder supplies it. */
  org?: string;
  /** Server-verified workspace slug (drives the task-id prefix). */
  org_slug?: string;
  /** Server-verified role in the active workspace (e.g. "org:admin"). */
  org_role?: string;
}

export interface ApiKey {
  id: string;
  label?: string;
  created_at?: string;
  last_used_at?: string;
  revoked_at?: string;
  /** The full token, present ONLY in the create response (shown once). */
  secret?: string;
}

/** Assume open access if the endpoint is missing/unreachable (e.g. old server). */
const OPEN: AuthInfo = { mode: "none", authenticated: true };

export function fetchAuthInfo(): Promise<AuthInfo> {
  return fetch("/api/authinfo")
    .then((r) => (r.ok ? (r.json() as Promise<AuthInfo>) : OPEN))
    .catch(() => OPEN);
}

export function login(token: string): Promise<boolean> {
  return fetch("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  })
    .then((r) => r.ok)
    .catch(() => false);
}

export function logout(): Promise<void> {
  return fetch("/api/logout", { method: "POST" })
    .then(() => undefined)
    .catch(() => undefined);
}

export function listKeys(): Promise<ApiKey[]> {
  return fetch("/api/v1/keys")
    .then((r) => (r.ok ? (r.json() as Promise<ApiKey[]>) : []))
    .then((keys) => keys ?? [])
    .catch(() => []);
}

export function createKey(label: string): Promise<ApiKey | null> {
  return fetch("/api/v1/keys", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ label }),
  })
    .then((r) => (r.ok ? (r.json() as Promise<ApiKey>) : null))
    .catch(() => null);
}

export function revokeKey(id: string): Promise<boolean> {
  return fetch(`/api/v1/keys/${encodeURIComponent(id)}/revoke`, { method: "POST" })
    .then((r) => r.ok)
    .catch(() => false);
}

/** The per-board MCP endpoint (same origin), for the connect helper. */
export function mcpUrl(): string {
  return `${location.origin}/mcp`;
}
