import { test, expect } from "./harness";

// Deleting a task is a HUMAN-only affordance: the server gates the DELETE route
// on a real session (never API keys / MCP / CLI / bots), and the UI only renders
// the action when `onDelete` is wired. These two specs pin both halves.
test.describe("delete", () => {
  test("a human deletes a task from the detail pane and it's gone", async ({ customBoard, customServer }) => {
    const keep = await customServer.api.create({ title: "Keep me" });
    const doomed = await customServer.api.create({ title: "Delete me" });

    await customBoard.open();
    await customBoard.expectRowVisible(doomed.id);

    await customBoard.openTask(doomed.id);
    await customBoard.deleteTask();

    // Back on the list, its row is gone and we're no longer viewing it.
    await customBoard.expectRowHidden(doomed.id);
    await customBoard.expectRowVisible(keep.id);

    // And it's really gone server-side (survives a reload).
    await customBoard.reload();
    await customBoard.expectRowHidden(doomed.id);
    const ids = (await customServer.api.list()).map((t) => t.id);
    expect(ids).not.toContain(doomed.id);
    expect(ids).toContain(keep.id);
  });

  test("token/agent mode never exposes a delete action", async ({ board, server }) => {
    const t = await server.api.create({ title: "Agent-owned" });
    await board.open();
    await board.openTask(t.id);
    expect(await board.hasDeleteAction()).toBe(false);
  });
});
