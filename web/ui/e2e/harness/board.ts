/**
 * Board: a page-object of composable "bricks" over the real UI. Every action is
 * a small method that drives one real interaction and returns `this`, so tests
 * read as chained flows:
 *
 *   await board.open();
 *   await board.search("api").expectRowVisible(id);
 *   await board.openTask(id).setStatus("closed").expectDetailStatus("closed");
 *   await board.reload().expectDetailStatus("closed");   // persisted
 *   await board.logout().expectSignedOut();
 *
 * Queries return values; everything else returns the Board for chaining. Use
 * `board.api` (from the server) to seed/assert server state independently.
 */
import { expect, type Page, type Locator } from "@playwright/test";
import type { TestServer, Api } from "./server";

export class Board {
  constructor(
    readonly page: Page,
    readonly server: TestServer,
  ) {}

  get api(): Api {
    return this.server.api;
  }

  // ---------------------------------------------------------------- lifecycle
  /** Authenticate (token login sets the session cookie) and load `path`. */
  async open(path = "/"): Promise<this> {
    await this.page.goto(`${this.server.baseURL}/auth?token=${this.server.token}`);
    // Deterministic defaults so tests don't depend on prior persisted UI state.
    await this.page.evaluate(() => {
      localStorage.setItem("tasks:view", JSON.stringify("tree"));
      localStorage.setItem("mode-watcher-mode", "dark");
    });
    await this.page.goto(`${this.server.baseURL}${path}`, { waitUntil: "networkidle" });
    return this;
  }

  async reload(): Promise<this> {
    await this.page.reload({ waitUntil: "networkidle" });
    return this;
  }

  // ------------------------------------------------------------------ auth UI
  async expectSignedIn(): Promise<this> {
    await expect(this.page.getByTestId("account-menu-trigger")).toBeVisible();
    return this;
  }

  /** True landing/logged-out state: the account menu is gone. */
  async expectSignedOut(): Promise<this> {
    await expect(this.page.getByTestId("account-menu-trigger")).toHaveCount(0);
    return this;
  }

  async logout(): Promise<this> {
    await this.page.getByTestId("account-menu-trigger").click();
    await this.page.getByTestId("logout").click();
    return this;
  }

  // --------------------------------------------------------------------- tree
  row(id: string): Locator {
    return this.page.locator(`[data-testid="tree-row"][data-task-id="${id}"]`);
  }

  async rowCount(): Promise<number> {
    return this.page.getByTestId("tree-row").count();
  }

  async expectRowVisible(id: string): Promise<this> {
    await expect(this.row(id)).toBeVisible();
    return this;
  }

  async expectRowHidden(id: string): Promise<this> {
    await expect(this.row(id)).toHaveCount(0);
    return this;
  }

  async search(q: string): Promise<this> {
    await this.page.getByTestId("search").fill(q);
    await this.page.waitForTimeout(150); // debounce/filter settle
    return this;
  }

  async clearSearch(): Promise<this> {
    await this.page.getByLabel("Clear search").click();
    return this;
  }

  /** Collapse a row's subtree via its twisty (works even while searching). */
  async collapse(id: string): Promise<this> {
    await this.row(id).getByLabel("Collapse").click();
    return this;
  }

  async expand(id: string): Promise<this> {
    await this.row(id).getByLabel("Expand").click();
    return this;
  }

  // ----------------------------------------------------------- open + detail
  async openTask(id: string): Promise<this> {
    await this.row(id).click();
    await this.page.waitForURL(new RegExp(`/tasks/${escapeRe(id)}`));
    return this;
  }

  async back(): Promise<this> {
    await this.page.goBack({ waitUntil: "networkidle" });
    return this;
  }

  /** The detail pane's status trigger (first on the page = the meta bar). */
  private statusTrigger(): Locator {
    return this.page.getByTestId("status-trigger").first();
  }

  async setStatus(status: string): Promise<this> {
    await this.statusTrigger().click();
    await this.page.getByTestId(`status-option-${status}`).click();
    return this;
  }

  async expectDetailTitle(title: string): Promise<this> {
    await expect(this.page.getByRole("heading", { name: title })).toBeVisible();
    return this;
  }

  /** Assert the detail's status badge reflects `status` (aria-label carries it). */
  async expectDetailStatus(label: string): Promise<this> {
    await expect(this.statusTrigger().getByLabel(label, { exact: false }).first()).toBeVisible();
    return this;
  }

  // -------------------------------------------------------------------- views
  /** Switch the board view via the real filter-menu view switcher. */
  async view(name: "tree" | "graph" | "dashboard"): Promise<this> {
    await this.page.getByTestId("filter-menu-trigger").click();
    await this.page.getByTestId(`view-${name}`).click();
    await this.page.keyboard.press("Escape"); // close the menu
    return this;
  }

  // -------------------------------------------------------------------- graph
  graphNode(id: string): Locator {
    return this.page.locator(`[data-node="${id}"]`);
  }

  async graphNodeCount(): Promise<number> {
    return this.page.locator("[data-node]").count();
  }

  async expectGraphNodeVisible(id: string): Promise<this> {
    await expect(this.graphNode(id)).toBeVisible();
    return this;
  }

  async expectGraphNodeAbsent(id: string): Promise<this> {
    await expect(this.graphNode(id)).toHaveCount(0);
    return this;
  }

  /** The graph header breadcrumb — the focused task's short id. */
  async graphFocusShortId(): Promise<string | null> {
    const el = this.page.locator(".font-mono").first();
    return (await el.count()) ? (await el.textContent())?.trim() ?? null : null;
  }

  // --------------------------------------------------------------------- misc
  /** Wait for a live (WebSocket) refresh to reflect a predicate, no reload. */
  async waitForLive(predicate: () => Promise<boolean> | boolean, timeoutMs = 5000): Promise<this> {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      if (await predicate()) return this;
      await this.page.waitForTimeout(150);
    }
    throw new Error("live update did not arrive within timeout");
  }
}

function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
