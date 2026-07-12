/**
 * A tiny, typed client router over the History API. Path-based and
 * deep-linkable: refreshing any URL re-derives state from `router.path`.
 */
class Router {
  /** Current pathname (reactive). */
  path = $state(typeof location !== "undefined" ? location.pathname : "/");

  constructor() {
    if (typeof window !== "undefined") {
      window.addEventListener("popstate", () => {
        this.path = location.pathname;
      });
    }
  }

  /** Navigate to a path, pushing history (or replacing the current entry). */
  navigate(to: string, options: { replace?: boolean } = {}): void {
    if (to === this.path) return;
    if (options.replace) history.replaceState(null, "", to);
    else history.pushState(null, "", to);
    this.path = to;
  }
}

/** The app-wide router singleton. */
export const router = new Router();
