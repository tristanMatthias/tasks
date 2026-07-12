/**
 * The app-wide auth session: loads /api/authinfo once and exposes it reactively,
 * so the gate can present the right login flow and the header can offer logout.
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
    return this.#info?.authenticated ?? false;
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
}

export const session = new Session();
