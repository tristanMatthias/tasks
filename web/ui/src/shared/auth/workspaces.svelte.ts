/**
 * Workspaces façade over the ClerkJS frontend API (`window.Clerk`, loaded on the
 * board by the control plane). Clerk owns orgs, members, invites, and roles and
 * enforces permissions; this module is a thin, reactive, typed adapter the UI
 * talks to instead of touching `window.Clerk` directly.
 *
 * It feature-detects Clerk: in local token/none dev mode there is no Clerk, so
 * `available` is false and every workspace surface hides itself. A test/dev can
 * inject a fake via `__setClerk`.
 */

/** A workspace the user can switch into. `id === null` is the personal space. */
export interface Workspace {
  id: string | null;
  name: string;
  slug?: string;
  imageUrl?: string;
  role?: string;
  isPersonal: boolean;
}

/** A member of the active workspace. */
export interface Member {
  /** Membership id. */
  id: string;
  userId: string;
  name: string;
  email: string;
  imageUrl?: string;
  role: string;
}

/** A pending invitation, with its revoke action bound to the Clerk resource. */
export interface Invite {
  id: string;
  email: string;
  role: string;
  revoke: () => Promise<void>;
}

/** A role that can be assigned in a workspace. */
export interface Role {
  key: string;
  label: string;
}

// The Clerk global is loosely typed at this boundary; mappers below narrow it.
type Clerk = any; // eslint-disable-line @typescript-eslint/no-explicit-any
type ClerkResource = any; // eslint-disable-line @typescript-eslint/no-explicit-any

let clerkRef: () => Clerk | null =
  typeof window !== "undefined" ? () => (window as { Clerk?: Clerk }).Clerk ?? null : () => null;

/** Override the Clerk source (tests/dev mock). */
export function __setClerk(getter: () => Clerk | null): void {
  clerkRef = getter;
}

/** Strip Clerk's `org:` namespace and classify admin vs member. */
export function roleIsAdmin(role?: string): boolean {
  return !!role && role.toLowerCase().replace(/^org:/, "").includes("admin");
}

/** Human label for a role key ("org:admin" → "Admin"). */
export function roleLabel(role: string): string {
  const bare = role.replace(/^org:/, "");
  return bare.charAt(0).toUpperCase() + bare.slice(1);
}

const PERSONAL: Workspace = { id: null, name: "Personal", isPersonal: true };

function mapMember(m: ClerkResource): Member {
  const u = m.publicUserData ?? {};
  const name = [u.firstName, u.lastName].filter(Boolean).join(" ") || u.identifier || "";
  return {
    id: m.id,
    userId: u.userId ?? "",
    name,
    email: u.identifier ?? "",
    imageUrl: u.imageUrl,
    role: m.role,
  };
}

/** Clerk paginates some list endpoints ({data:[...]}) and returns bare arrays
 * for others; normalize both. */
function listData(res: ClerkResource): ClerkResource[] {
  if (Array.isArray(res)) return res;
  return res?.data ?? [];
}

class Workspaces {
  #ready = $state(false);
  #user = $state<ClerkResource | null>(null);
  #memberships = $state<ClerkResource[]>([]);
  #activeId = $state<string | null>(null);
  #unsub: (() => void) | null = null;

  /** Whether Clerk (and therefore workspaces) is available at all. */
  get available(): boolean {
    return clerkRef() !== null;
  }

  /** Load Clerk once and keep local state in sync with it. Safe to call often. */
  async ensureLoaded(): Promise<void> {
    const c = clerkRef();
    if (!c || this.#unsub) return;
    if (typeof c.load === "function" && !c.loaded) {
      try {
        await c.load();
      } catch {
        /* leave unavailable */
      }
    }
    this.#sync();
    this.#ready = true;
    if (typeof c.addListener === "function") {
      this.#unsub = c.addListener(() => this.#sync());
    }
  }

  #sync(): void {
    const c = clerkRef();
    if (!c) return;
    this.#user = c.user ?? null;
    this.#memberships = c.user?.organizationMemberships ?? [];
    this.#activeId = c.organization?.id ?? null;
  }

  get ready(): boolean {
    return this.#ready;
  }

  /** The signed-in user's Clerk id (to mark "you" and guard self-removal). */
  get userId(): string | undefined {
    return this.#user?.id;
  }

  get user(): { name: string; email: string; imageUrl?: string } | null {
    const u = this.#user;
    if (!u) return null;
    return {
      name: u.fullName ?? u.username ?? u.primaryEmailAddress?.emailAddress ?? "",
      email: u.primaryEmailAddress?.emailAddress ?? "",
      imageUrl: u.imageUrl,
    };
  }

  /** Personal first, then every org the user is a member of. */
  get workspaces(): Workspace[] {
    const orgs = this.#memberships.map((m) => ({
      id: m.organization.id as string,
      name: m.organization.name as string,
      slug: m.organization.slug as string | undefined,
      imageUrl: m.organization.imageUrl as string | undefined,
      role: m.role as string,
      isPersonal: false,
    }));
    return [PERSONAL, ...orgs];
  }

  get activeId(): string | null {
    return this.#activeId;
  }

  get active(): Workspace {
    return this.workspaces.find((w) => w.id === this.#activeId) ?? PERSONAL;
  }

  /** Whether the current user administers the active workspace. */
  get isAdmin(): boolean {
    return roleIsAdmin(this.active.role);
  }

  #activeOrg(): ClerkResource | null {
    return clerkRef()?.organization ?? null;
  }

  // ---- mutations ----

  /** Switch the active workspace, then hard-reload so the refreshed session
   * cookie (new org claim) drives every request and stale board state is gone. */
  async switchTo(orgId: string | null): Promise<void> {
    const c = clerkRef();
    if (!c) return;
    await c.setActive({ organization: orgId });
    try {
      await c.session?.getToken({ skipCache: true }); // force the __session cookie to refresh
    } catch {
      /* best effort */
    }
    location.assign("/");
  }

  /** Create a workspace and switch into it (so its first request carries the
   * slug that seeds the task-id prefix). */
  async create(name: string): Promise<void> {
    const c = clerkRef();
    if (!c) return;
    const org = await c.createOrganization({ name });
    await this.switchTo(org.id);
  }

  async getMembers(): Promise<Member[]> {
    const org = this.#activeOrg();
    if (!org) return [];
    const res = await org.getMemberships({ pageSize: 100 });
    return listData(res).map(mapMember);
  }

  async getRoles(): Promise<Role[]> {
    const org = this.#activeOrg();
    const fallback: Role[] = [
      { key: "org:member", label: "Member" },
      { key: "org:admin", label: "Admin" },
    ];
    if (!org || typeof org.getRoles !== "function") return fallback;
    try {
      const res = await org.getRoles({ pageSize: 20 });
      const roles = listData(res).map((r: ClerkResource) => ({
        key: r.key as string,
        label: (r.name as string) ?? roleLabel(r.key),
      }));
      return roles.length ? roles : fallback;
    } catch {
      return fallback;
    }
  }

  async inviteMember(email: string, role: string): Promise<boolean> {
    const org = this.#activeOrg();
    if (!org) return false;
    try {
      await org.inviteMember({ emailAddress: email, role });
      return true;
    } catch {
      return false;
    }
  }

  async getInvitations(): Promise<Invite[]> {
    const org = this.#activeOrg();
    if (!org) return [];
    const res = await org.getInvitations({ status: ["pending"] });
    return listData(res).map((i: ClerkResource) => ({
      id: i.id as string,
      email: i.emailAddress as string,
      role: i.role as string,
      revoke: () => i.revoke().then(() => undefined),
    }));
  }

  async updateMemberRole(userId: string, role: string): Promise<boolean> {
    const org = this.#activeOrg();
    if (!org) return false;
    try {
      await org.updateMember({ userId, role });
      return true;
    } catch {
      return false;
    }
  }

  async removeMember(userId: string): Promise<boolean> {
    const org = this.#activeOrg();
    if (!org) return false;
    try {
      await org.removeMember(userId);
      return true;
    } catch {
      return false;
    }
  }

  async updateName(name: string): Promise<boolean> {
    const org = this.#activeOrg();
    if (!org) return false;
    try {
      await org.update({ name });
      this.#sync();
      return true;
    } catch {
      return false;
    }
  }

  /** Leave the active workspace, then drop back to personal. */
  async leave(): Promise<void> {
    const c = clerkRef();
    const membership = this.#memberships.find((m) => m.organization.id === this.#activeId);
    if (membership) {
      try {
        await membership.destroy();
      } catch {
        /* fall through to reload */
      }
    }
    if (c) await this.switchTo(null);
  }

  /** Delete the active workspace (admin), then drop back to personal. */
  async destroy(): Promise<void> {
    const org = this.#activeOrg();
    if (org) {
      try {
        await org.destroy();
      } catch {
        /* fall through to reload */
      }
    }
    await this.switchTo(null);
  }
}

export const workspaces = new Workspaces();
