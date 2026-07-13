import { test, expect } from "./harness";

test.describe("settings", () => {
  test("opens settings from the account menu", async ({ board }) => {
    await board.open();
    await board.openSettings();
    await expect(board.page).toHaveURL(/\/settings/);
  });

  test("creates and revokes an API key", async ({ board }) => {
    await board.open();
    await board.gotoSettings("keys");
    await board.createApiKey("my bot key");
    await expect(board.page.getByText("my bot key")).toBeVisible();
    await board.revokeFirstApiKey();
    await expect(board.page.getByText("my bot key")).toHaveCount(0);
  });

  test("toggles the theme", async ({ board }) => {
    await board.open(); // harness opens in dark
    const before = await board.currentTheme();
    await board.toggleTheme();
    await expect.poll(() => board.currentTheme()).not.toBe(before);
  });

  test("connect page shows the board's MCP url", async ({ board }) => {
    await board.open();
    await board.gotoSettings("connect");
    await expect(board.page.getByText("/mcp").first()).toBeVisible();
  });
});
