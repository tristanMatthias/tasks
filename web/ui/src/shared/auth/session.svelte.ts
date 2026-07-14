/**
 * The app-wide auth session: loads /api/authinfo once and exposes it reactively,
 * so the gate can present the right login flow and the header can offer logout.
 *
 * Auth is SERVER-DRIVEN: whether the caller is signed in is whatever
 * /api/authinfo says, based on the HttpOnly session cookie. There is no
 * client-side trust signal — a first-party session cookie is long-lived and
 * always fresh, so we never second-guess the server (a client fallback is
 * exactly what let a stale cookie keep a "logged out" user looking signed in).
 */
import { fetchAuthInfo, logout as apiLogout, type AuthInfo, type AuthMode } from "./auth.js";

class Session {
  #info = $state<AuthInfo | null>(null);
  #loading = $state(true);

  async load(): Promise<void> {
    this.#loading = true;
    this.#info = await fetchAuthInfo();
    this.#loading = false;
  }

  async logout(): Promise<void> {
    await apiLogout(); // clears the HttpOnly session cookie server-side
    await this.load();
  }

  get loading(): boolean {
    return this.#loading;
  }
  get mode(): AuthMode {
    return this.#info?.mode ?? "none";
  }
  get authenticated(): boolean {
    return !!this.#info?.authenticated;
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
