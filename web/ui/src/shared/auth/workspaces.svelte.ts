/**
 * Workspaces façade over the control-plane REST API (/api/workspaces*). Clerk (or
 * whatever provider) still authenticates the user; membership, roles, and invites
 * are owned by our own backend, so this talks to it — not to any identity SDK.
 *
 * Feature-detects via the API itself: if GET /api/workspaces isn't authorized or
 * isn't there, `available` stays false and every workspace surface hides. This
 * works in local token/none dev mode too (the whole system is self-hosted).
 */
import { toast } from "svelte-sonner";
import { clearTaskListCache } from "$tasks/data.js";

/** A workspace the user can switch into. */
export interface Workspace {
  id: string;
  name: string;
  prefix?: string;
  role: string;
  isPersonal: boolean;
}

/** A member of the active workspace. */
export interface Member {
  id: string;
  userId: string;
  email: string;
  name: string;
  role: string;
  imageUrl?: string;
}

/** A pending invite; `url` is the shareable link, `revoke` cancels it. */
export interface Invite {
  id: string;
  email: string;
  role: string;
  url: string;
  revoke: () => Promise<void>;
}

/** A role that can be assigned. */
export interface Role {
  key: string;
  label: string;
}

/** Classify admin vs member (tolerates a Clerk-style "org:admin" too). */
export function roleIsAdmin(role?: string): boolean {
  return !!role && role.toLowerCase().replace(/^org:/, "").includes("admin");
}

/** Human label for a role key ("member" → "Member"). */
export function roleLabel(role: string): string {
  const bare = role.replace(/^org:/, "");
  return bare.charAt(0).toUpperCase() + bare.slice(1);
}

/** Stable object for the "no workspace / still loading" case, so repeated reads
 * of `active` don't churn identity or momentarily report admin. */
const PERSONAL_FALLBACK: Workspace = { id: "", name: "Personal", role: "member", isPersonal: true };

const JSON_HEADERS = { "Content-Type": "application/json" };

/** Reload onto the board after a workspace change, dropping the cached task list
 * so the new board never flashes the previous workspace's tasks. */
function reloadToBoard(): void {
  clearTaskListCache();
  location.assign("/");
}

async function getJSON<T>(url: string): Promise<T | null> {
  try {
    const r = await fetch(url);
    return r.ok ? ((await r.json()) as T) : null;
  } catch {
    return null;
  }
}

/** A mutating request that surfaces failures to the user (toast) instead of
 * silently no-op'ing, and returns the parsed body on success. */
async function send(
  url: string,
  method: string,
  body?: unknown,
): Promise<{ ok: boolean; data: unknown }> {
  try {
    const r = await fetch(url, {
      method,
      headers: body === undefined ? undefined : JSON_HEADERS,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (r.ok) return { ok: true, data: await r.json().catch(() => null) };
    const err = (await r.json().catch(() => null)) as { error?: string } | null;
    toast.error(err?.error || `Request failed (${r.status})`);
    return { ok: false, data: null };
  } catch {
    toast.error("Network error — please try again.");
    return { ok: false, data: null };
  }
}

class Workspaces {
  #available = $state(false);
  #loaded = $state(false);
  #list = $state<Workspace[]>([]);
  #activeId = $state<string | null>(null);
  #userId = $state<string | null>(null);
  #loading: Promise<void> | null = null;

  get available(): boolean {
    return this.#available;
  }
  get ready(): boolean {
    return this.#loaded;
  }
  get userId(): string | null {
    return this.#userId;
  }
  get workspaces(): Workspace[] {
    return this.#list;
  }
  get activeId(): string | null {
    return this.#activeId;
  }
  get active(): Workspace {
    return (
      this.#list.find((w) => w.id === this.#activeId) ??
      this.#list.find((w) => w.isPersonal) ??
      PERSONAL_FALLBACK
    );
  }
  get isAdmin(): boolean {
    return roleIsAdmin(this.active.role);
  }

  /** Load the workspace list once; safe to call often. Concurrent callers share
   * the same in-flight request rather than each firing their own. */
  async ensureLoaded(): Promise<void> {
    if (this.#loaded) return;
    if (!this.#loading) this.#loading = this.reload().finally(() => (this.#loading = null));
    await this.#loading;
  }

  async reload(): Promise<void> {
    // Wait for the session cookie to be fresh (Clerk mode) before the first call.
    const ready = (window as { __authReady?: Promise<unknown> }).__authReady;
    if (ready) {
      try {
        await ready;
      } catch {
        /* proceed */
      }
    }
    const data = await getJSON<{
      workspaces: { id: string; name: string; prefix?: string; role: string; personal?: boolean }[];
      active: string;
      me: string;
    }>("/api/workspaces");
    if (!data) {
      this.#available = false;
      this.#loaded = true;
      return;
    }
    this.#list = (data.workspaces ?? []).map((w) => ({
      id: w.id,
      name: w.name,
      prefix: w.prefix,
      role: w.role,
      isPersonal: !!w.personal,
    }));
    this.#activeId = data.active ?? null;
    this.#userId = data.me ?? null;
    this.#available = true;
    this.#loaded = true;
  }

  // ---- mutations (each hard-reloads when the active board changes) ----

  async switchTo(id: string): Promise<void> {
    if (id === this.#activeId) return;
    const { ok } = await send("/api/workspaces/switch", "POST", { id });
    if (ok) reloadToBoard(); // only reload once the cookie actually changed
  }

  async create(name: string): Promise<void> {
    const { ok } = await send("/api/workspaces", "POST", { name });
    if (ok) reloadToBoard(); // server set the active cookie to the new workspace
  }

  #ws(): string {
    return this.#activeId ?? "";
  }

  async getMembers(): Promise<Member[]> {
    const data = await getJSON<{ user_id: string; email: string; name: string; role: string }[]>(
      `/api/workspaces/${encodeURIComponent(this.#ws())}/members`,
    );
    return (data ?? []).map((m) => ({
      id: m.user_id,
      userId: m.user_id,
      email: m.email,
      name: m.name,
      role: m.role,
    }));
  }

  // Roles are fixed in the self-hosted model; the server validates.
  async getRoles(): Promise<Role[]> {
    return [
      { key: "member", label: "Member" },
      { key: "admin", label: "Admin" },
    ];
  }

  async inviteMember(email: string, role: string): Promise<Invite | null> {
    if (!this.#ws()) return null;
    const { ok, data } = await send(`/api/workspaces/${encodeURIComponent(this.#ws())}/invites`, "POST", {
      email,
      role,
    });
    return ok && data ? this.#toInvite(data as InviteJSON) : null;
  }

  async getInvitations(): Promise<Invite[]> {
    if (!this.#ws()) return [];
    const data = await getJSON<InviteJSON[]>(`/api/workspaces/${encodeURIComponent(this.#ws())}/invites`);
    return (data ?? []).map((i) => this.#toInvite(i));
  }

  #toInvite(i: InviteJSON): Invite {
    const ws = this.#ws();
    return {
      id: i.token,
      email: i.email,
      role: i.role,
      url: i.url,
      revoke: async () => {
        await send(`/api/workspaces/${encodeURIComponent(ws)}/invites/${encodeURIComponent(i.token)}`, "DELETE");
      },
    };
  }

  async updateMemberRole(userId: string, role: string): Promise<boolean> {
    if (!this.#ws()) return false;
    const { ok } = await send(
      `/api/workspaces/${encodeURIComponent(this.#ws())}/members/${encodeURIComponent(userId)}`,
      "PATCH",
      { role },
    );
    return ok;
  }

  async removeMember(userId: string): Promise<boolean> {
    if (!this.#ws()) return false;
    const { ok } = await send(
      `/api/workspaces/${encodeURIComponent(this.#ws())}/members/${encodeURIComponent(userId)}`,
      "DELETE",
    );
    return ok;
  }

  async updateName(name: string): Promise<boolean> {
    if (!this.#ws()) return false;
    const { ok } = await send(`/api/workspaces/${encodeURIComponent(this.#ws())}`, "PATCH", { name });
    if (ok) await this.reload();
    return ok;
  }

  /** Leave the active workspace, then drop to personal. No-op (with a toast) if
   * we don't yet know who the user is, so we never falsely claim to have left. */
  async leave(): Promise<void> {
    if (!this.#ws() || !this.#userId) return;
    const { ok } = await send(
      `/api/workspaces/${encodeURIComponent(this.#ws())}/members/${encodeURIComponent(this.#userId)}`,
      "DELETE",
    );
    if (ok) reloadToBoard();
  }

  /** Delete the active workspace (admin), then drop to personal. */
  async destroy(): Promise<void> {
    if (!this.#ws()) return;
    const { ok } = await send(`/api/workspaces/${encodeURIComponent(this.#ws())}`, "DELETE");
    if (ok) reloadToBoard();
  }
}

/** The invites endpoint's JSON shape. */
interface InviteJSON {
  token: string;
  email: string;
  role: string;
  url: string;
}

export const workspaces = new Workspaces();
