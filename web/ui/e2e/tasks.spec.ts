import { test, expect } from "./harness";

test.describe("task editing", () => {
  test("changing status in the UI persists on the server and across reload", async ({ board, server }) => {
    const t = await server.api.create({ title: "Persist me", issue_type: "task" });
    await board.open();
    await board.openTask(t.id);
    await board.setStatus("closed");

    // Server-side truth: the write actually stuck.
    await expect.poll(async () => (await server.api.get(t.id)).status).toBe("closed");

    // And the UI still shows it after a full reload (no stale cache).
    await board.reload();
    await board.expectDetailStatus("closed");
  });

  test("a change from another client streams in live (no reload)", async ({ board, server }) => {
    const t = await server.api.create({ title: "Live original", issue_type: "task" });
    await board.open();
    await board.openTask(t.id);
    await board.expectDetailTitle("Live original");

    // Simulate another user/agent mutating via the API.
    await server.api.patch(t.id, { title: "Live updated" });

    // The open board reflects it without any refresh (WebSocket push).
    await board.expectDetailTitle("Live updated");
  });
});
