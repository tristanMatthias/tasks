import { test, expect } from "./harness";

test.describe("auth", () => {
  test("signs in and shows the board", async ({ board, server }) => {
    await server.api.create({ title: "Alpha task" });
    await board.open();
    await board.expectSignedIn();
  });

  test("renders the board promptly without hanging on the auth spinner", async ({ board, server }) => {
    await server.api.create({ title: "Fast render" });
    await board.open();
    // The auth gate must resolve to the app quickly — never block first paint on
    // a slow identity-provider handshake.
    await expect(board.page.getByTestId("account-menu-trigger")).toBeVisible({ timeout: 3000 });
  });

  test("logout signs the user out and it sticks across reload", async ({ board }) => {
    await board.open();
    await board.expectSignedIn();
    await board.logout();
    await board.expectSignedOut();
    await board.reload();
    await board.expectSignedOut();
  });
});
