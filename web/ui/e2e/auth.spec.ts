import { test } from "./harness";

test.describe("auth", () => {
  test("signs in and shows the board", async ({ board, server }) => {
    await server.api.create({ title: "Alpha task" });
    await board.open();
    await board.expectSignedIn();
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
