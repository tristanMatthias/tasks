/**
 * The app-wide auth session: loads /api/authinfo once and exposes it reactively,
 * so the gate can present the right login flow and the header can offer logout.
 */
import { fetchAuthInfo, logout as apiLogout, type AuthInfo, type AuthMode } from "./auth.js";

interface ClerkLike {
  user?: unknown;
  session?: { getToken?: (opts?: { skipCache?: boolean }) => Promise<unknown> } | null;
  addListener?: (cb: (e: { user?: unknown }) => void) => void;
  signOut?: (opts?: unknown) => Promise<unknown>;
}
const clerk = (): ClerkLike | undefined => (window as { Clerk?: ClerkLike }).Clerk;

/**
 * Clerk drops a non-HttpOnly `__client_uat` cookie on the app domain — the unix
 * seconds of the last sign-in, `0` when signed out. It's Clerk's own signal for
 * "this browser has an active session", readable before (and independently of)
 * ClerkJS finishing its load. We trust it to decide app-vs-landing so a hard
 * refresh never strands a signed-in user on the public page while ClerkJS is
 * still rehydrating `Clerk.user`. (Newer Clerk may suffix the name, e.g.
 * `__client_uat_<hash>`, so match either.)
 */
function clerkSessionCookie(): boolean {
  try {
    const m = document.cookie.match(/(?:^|;\s*)__client_uat(?:_[^=]*)?=(\d+)/);
    return !!m && Number(m[1]) > 0;
  } catch {
    return false;
  }
}

class Session {
  #info = $state<AuthInfo | null>(null);
  #loading = $state(true);
  #clerkUser = $state(false);
  #watching = false;

  async load(): Promise<void> {
    this.#loading = true;
    // Render FAST. We decide app-vs-landing on two cheap signals only — the
    // server's /api/authinfo (a local call, no Clerk dependency) and Clerk's
    // synchronous __client_uat cookie — and never block the first paint on
    // ClerkJS finishing its (slow) session hydration. That was the refresh
    // spinner. Any read made with a momentarily-stale cookie self-heals via the
    // boot script's 401 retry, and #watchClerk reconciles once ClerkJS settles.
    this.#clerkUser = !!clerk()?.user;
    this.#info = await fetchAuthInfo();
    this.#loading = false;
    this.#watchClerk();
    // Best-effort cookie refresh in the BACKGROUND (does not gate rendering).
    void this.#syncClerk();
  }

  /**
   * Reconcile with ClerkJS: record whether a signed-in user is present, and —
   * because the server's short-lived __session cookie can be momentarily stale
   * on a hard refresh — force-refresh it when Clerk has a live session so the
   * next /api/authinfo agrees.
   */
  async #syncClerk(): Promise<void> {
    const c = clerk();
    this.#clerkUser = !!c?.user;
    if (c?.session?.getToken) {
      try {
        await c.session.getToken({ skipCache: true });
      } catch {
        /* keep whatever cookie exists */
      }
    }
  }

  #watchClerk(): void {
    if (this.#watching) return;
    const c = clerk();
    if (!c?.addListener) return;
    this.#watching = true;
    c.addListener((e) => {
      const has = !!e?.user;
      if (has === this.#clerkUser) return; // nothing that affects our gate changed
      // Flip the client-trusted view immediately so the gate swaps landing↔app
      // without a round trip, then re-verify (and refresh the cookie) so API
      // calls made after this point carry a valid session.
      this.#clerkUser = has;
      void this.#syncClerk().then(async () => {
        this.#info = await fetchAuthInfo();
      });
    });
  }

  async logout(): Promise<void> {
    // In custom (Clerk) mode, clearing only the server session cookie isn't
    // enough: ClerkJS still holds the client session, so __client_uat / Clerk.user
    // keep `authenticated` true and the user appears logged in. Sign out of Clerk
    // first — that clears the client session, __session and __client_uat.
    const c = clerk();
    if (this.mode === "custom" && c?.signOut) {
      try {
        await c.signOut();
      } catch {
        /* fall through to server logout + reload */
      }
      this.#clerkUser = false;
    }
    await apiLogout();
    await this.load();
  }

  get loading(): boolean {
    return this.#loading;
  }
  get mode(): AuthMode {
    return this.#info?.mode ?? "none";
  }
  get authenticated(): boolean {
    if (this.#info?.authenticated) return true;
    // Client-trusted fallback for the stale-cookie-on-refresh case (custom mode):
    // ClerkJS reports a user, OR Clerk's __client_uat cookie says this browser
    // has a live session (which survives even before Clerk.user rehydrates). If
    // the session is actually dead, API calls 401 and ClerkJS clears the cookie.
    return this.mode === "custom" && (this.#clerkUser || clerkSessionCookie());
  }
  get loginUrl(): string | undefined {
    return this.#info?.login_url;
  }
  /** True once loaded and the user must sign in (open mode never needs it). */
  get needsLogin(): boolean {
    return !this.#loading && this.#info !== null && !this.authenticated && this.mode !== "none";
  }
  /** Whether a logout action makes sense (a real session exists). */
  get canLogout(): boolean {
    return this.authenticated && this.mode !== "none";
  }
  /** Server-verified active workspace (org id), if the embedder supplies it. */
  get org(): string | undefined {
    return this.#info?.org;
  }
  /** Server-verified active workspace slug (the task-id prefix source). */
  get orgSlug(): string | undefined {
    return this.#info?.org_slug;
  }
  /** Server-verified role in the active workspace (e.g. "org:admin"). */
  get orgRole(): string | undefined {
    return this.#info?.org_role;
  }
}

export const session = new Session();
