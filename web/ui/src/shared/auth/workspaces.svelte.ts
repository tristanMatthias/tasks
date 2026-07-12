/**
 * Workspaces façade over the control-plane REST API (/api/workspaces*). Clerk (or
 * whatever provider) still authenticates the user; membership, roles, and invites
 * are owned by our own backend, so this talks to it — not to any identity SDK.
 *
 * Feature-detects via the API itself: if GET /api/workspaces isn't authorized or
 * isn't there, `available` stays false and every workspace surface hides. This
 * works in local token/none dev mode too (the whole system is self-hosted).
 */

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

const JSON_HEADERS = { "Content-Type": "application/json" };

async function getJSON<T>(url: string): Promise<T | null> {
  try {
    const r = await fetch(url);
    return r.ok ? ((await r.json()) as T) : null;
  } catch {
    return null;
  }
}

class Workspaces {
  #available = $state(false);
  #loaded = $state(false);
  #list = $state<Workspace[]>([]);
  #activeId = $state<string | null>(null);
  #userId = $state<string | null>(null);

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
      this.#list.find((w) => w.isPersonal) ?? { id: "", name: "Personal", role: "admin", isPersonal: true }
    );
  }
  get isAdmin(): boolean {
    return roleIsAdmin(this.active.role);
  }

  /** Load the workspace list once; safe to call often. */
  async ensureLoaded(): Promise<void> {
    if (this.#loaded) return;
    await this.reload();
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
    await fetch("/api/workspaces/switch", { method: "POST", headers: JSON_HEADERS, body: JSON.stringify({ id }) });
    location.assign("/");
  }

  async create(name: string): Promise<void> {
    const r = await fetch("/api/workspaces", { method: "POST", headers: JSON_HEADERS, body: JSON.stringify({ name }) });
    if (r.ok) location.assign("/"); // server set the active cookie to the new workspace
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
    const r = await fetch(`/api/workspaces/${encodeURIComponent(this.#ws())}/invites`, {
      method: "POST",
      headers: JSON_HEADERS,
      body: JSON.stringify({ email, role }),
    });
    if (!r.ok) return null;
    return this.#toInvite(await r.json());
  }

  async getInvitations(): Promise<Invite[]> {
    const data = await getJSON<{ token: string; email: string; role: string; url: string }[]>(
      `/api/workspaces/${encodeURIComponent(this.#ws())}/invites`,
    );
    return (data ?? []).map((i) => this.#toInvite(i));
  }

  #toInvite(i: { token: string; email: string; role: string; url: string }): Invite {
    const ws = this.#ws();
    return {
      id: i.token,
      email: i.email,
      role: i.role,
      url: i.url,
      revoke: async () => {
        await fetch(`/api/workspaces/${encodeURIComponent(ws)}/invites/${encodeURIComponent(i.token)}`, {
          method: "DELETE",
        });
      },
    };
  }

  async updateMemberRole(userId: string, role: string): Promise<boolean> {
    const r = await fetch(`/api/workspaces/${encodeURIComponent(this.#ws())}/members/${encodeURIComponent(userId)}`, {
      method: "PATCH",
      headers: JSON_HEADERS,
      body: JSON.stringify({ role }),
    });
    return r.ok;
  }

  async removeMember(userId: string): Promise<boolean> {
    const r = await fetch(`/api/workspaces/${encodeURIComponent(this.#ws())}/members/${encodeURIComponent(userId)}`, {
      method: "DELETE",
    });
    return r.ok;
  }

  async updateName(name: string): Promise<boolean> {
    const r = await fetch(`/api/workspaces/${encodeURIComponent(this.#ws())}`, {
      method: "PATCH",
      headers: JSON_HEADERS,
      body: JSON.stringify({ name }),
    });
    if (r.ok) await this.reload();
    return r.ok;
  }

  /** Leave the active workspace, then drop to personal. */
  async leave(): Promise<void> {
    if (this.#userId) {
      await fetch(`/api/workspaces/${encodeURIComponent(this.#ws())}/members/${encodeURIComponent(this.#userId)}`, {
        method: "DELETE",
      });
    }
    location.assign("/");
  }

  /** Delete the active workspace (admin), then drop to personal. */
  async destroy(): Promise<void> {
    await fetch(`/api/workspaces/${encodeURIComponent(this.#ws())}`, { method: "DELETE" });
    location.assign("/");
  }
}

export const workspaces = new Workspaces();
