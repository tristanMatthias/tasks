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
  /** Authenticate (establishes the session cookie for this server's auth mode)
   *  and load `path`. Works for both token and custom-mode servers. */
  async open(path = "/"): Promise<this> {
    await this.page.goto(`${this.server.baseURL}${this.server.authPath}`);
    // Deterministic defaults so tests don't depend on prior persisted UI state.
    await this.page.evaluate(() => {
      localStorage.setItem("tasks:view", JSON.stringify("tree"));
      localStorage.setItem("mode-watcher-mode", "dark");
    });
    await this.page.goto(`${this.server.baseURL}${path}`, { waitUntil: "networkidle" });
    return this;
  }

  /** Set a cookie in the page (e.g. to plant a stale client-trust signal). */
  async setCookie(name: string, value: string): Promise<this> {
    await this.page.evaluate(([n, v]) => (document.cookie = `${n}=${v}; path=/`), [name, value]);
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

  /** Assert the URL reflects viewing task `id` (its detail is open). */
  async expectViewingTask(id: string): Promise<this> {
    await this.page.waitForURL((u) => u.href.includes(`/tasks/${id}`));
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

  async setType(type: string): Promise<this> {
    await this.page.getByTestId("type-trigger").first().click();
    await this.page.getByTestId(`type-option-${type}`).click();
    return this;
  }

  async setPriority(priority: number): Promise<this> {
    await this.page.getByTestId("priority-trigger").first().click();
    await this.page.getByTestId(`priority-option-${priority}`).click();
    return this;
  }

  async expectDetailType(label: string): Promise<this> {
    await expect(this.page.getByTestId("type-trigger").first()).toContainText(new RegExp(label, "i"));
    return this;
  }

  /** From the detail pane, jump into the graph rooted on this task. */
  async viewInGraph(): Promise<this> {
    await this.page.getByTestId("view-in-graph").click();
    return this;
  }

  /** Delete the open task via the detail action + confirm dialog. */
  async deleteTask(): Promise<this> {
    await this.page.getByTestId("delete-task").click();
    await this.page.getByTestId("delete-confirm").click();
    return this;
  }

  /** True when the detail pane offers a delete action (humans only). */
  async hasDeleteAction(): Promise<boolean> {
    return (await this.page.getByTestId("delete-task").count()) > 0;
  }

  /** Follow an in-detail task link (parent / blocking / child) by its title.
   *  Uses the real <button> element (detail links), not the tree rows, which are
   *  role="button" divs and would otherwise clash on the same title. */
  async openDetailLink(title: string): Promise<this> {
    await this.page.locator("button", { hasText: title }).first().click();
    return this;
  }

  // --------------------------------------------------------------- gates
  async gateCount(): Promise<number> {
    return this.page.getByTestId("gate-item").count();
  }

  /** Gate rows filtered by verification state ("pending" | "verified"). */
  gate(status: "pending" | "verified"): Locator {
    return this.page.locator(`[data-testid="gate-item"][data-gate-status="${status}"]`);
  }

  /** Assert a read-only detail section (Description, Notes, …) shows `text`. */
  async expectSection(text: string): Promise<this> {
    await expect(this.page.getByText(text, { exact: false }).first()).toBeVisible();
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

  // ---------------------------------------------------------- views + filters
  private async openFilterMenu(): Promise<void> {
    // The detail pane can embed a child tree with its own filter menu, so target
    // the main board's toolbar (first in the DOM).
    await this.page.getByTestId("filter-menu-trigger").first().click();
  }
  private async closeMenu(): Promise<void> {
    await this.page.keyboard.press("Escape");
  }

  /** Switch the board view via the real filter-menu view switcher. */
  async view(name: "tree" | "graph" | "dashboard"): Promise<this> {
    await this.openFilterMenu();
    await this.page.getByTestId(`view-${name}`).click();
    await this.closeMenu();
    return this;
  }

  /** Toggle a status facet on/off (hides/shows those rows). */
  async toggleStatusFilter(status: string): Promise<this> {
    await this.openFilterMenu();
    await this.page.getByTestId(`filter-status-${status}`).click();
    await this.closeMenu();
    await this.page.waitForTimeout(150);
    return this;
  }

  /** Toggle a type facet on/off. */
  async toggleTypeFilter(type: string): Promise<this> {
    await this.openFilterMenu();
    await this.page.getByTestId(`filter-type-${type}`).click();
    await this.closeMenu();
    await this.page.waitForTimeout(150);
    return this;
  }

  // --------------------------------------------------------------- tree bulk
  async expandAll(): Promise<this> {
    await this.page.getByTitle("Expand all").click();
    return this;
  }
  async collapseAll(): Promise<this> {
    await this.page.getByTitle("Collapse all").click();
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

  async graphKind(key: "stack" | "blocking" | "subtree"): Promise<this> {
    await this.page.getByTestId(`graph-kind-${key}`).click();
    return this;
  }

  /** Tap a graph node → selects it (updates the detail pane). */
  async clickNode(id: string): Promise<this> {
    await this.graphNode(id).click();
    return this;
  }

  /** Double-tap a graph node → re-roots the graph on it. */
  async recenterOnNode(id: string): Promise<this> {
    await this.graphNode(id).dblclick();
    return this;
  }

  async centerHere(): Promise<this> {
    await this.page.getByTestId("graph-center").click();
    return this;
  }

  async openFullscreen(): Promise<this> {
    await this.page.getByTestId("graph-fullscreen").click();
    return this;
  }

  async closeFullscreen(): Promise<this> {
    await this.page.getByLabel("Close full page").click();
    return this;
  }

  /** In full page the graph header adds its own search (the board toolbar is
   *  covered), so the last search box on the page is the full-page one. */
  async expectFullscreen(): Promise<this> {
    await expect(this.page.getByLabel("Close full page")).toBeVisible();
    await expect(this.page.getByTestId("search").last()).toBeVisible();
    return this;
  }

  // ---------------------------------------------------------------- dashboard
  dashboardCard(epicId: string): Locator {
    return this.page.locator(`[data-testid="dashboard-card"][data-epic-id="${epicId}"]`);
  }
  async dashboardCardCount(): Promise<number> {
    return this.page.getByTestId("dashboard-card").count();
  }
  async clickDashboardCard(epicId: string): Promise<this> {
    await this.dashboardCard(epicId).click();
    return this;
  }

  // ----------------------------------------------------------------- settings
  /** Open Settings via the account menu (real navigation). */
  async openSettings(): Promise<this> {
    await this.page.getByTestId("account-menu-trigger").click();
    await this.page.getByRole("menuitem", { name: "Settings" }).click();
    await this.page.waitForURL(/\/settings/);
    return this;
  }

  async gotoSettings(section: "account" | "keys" | "connect"): Promise<this> {
    await this.page.goto(`${this.server.baseURL}/settings/${section}`, { waitUntil: "networkidle" });
    return this;
  }

  async toggleTheme(): Promise<this> {
    await this.page.getByTestId("account-menu-trigger").click();
    await this.page.getByRole("menuitem", { name: /theme/i }).click();
    return this;
  }

  async currentTheme(): Promise<string | null> {
    return this.page.evaluate(() => document.documentElement.classList.contains("dark") ? "dark" : "light");
  }

  async createApiKey(label: string): Promise<this> {
    await this.page.locator("#key-label").fill(label);
    await this.page.getByRole("button", { name: "Create", exact: true }).click();
    return this;
  }

  async revokeFirstApiKey(): Promise<this> {
    await this.page.getByTitle("Revoke").first().click();
    return this;
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
