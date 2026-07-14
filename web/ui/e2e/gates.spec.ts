import { test, expect } from "./harness";

// The task detail pane shows a read-only Acceptance gates section reflecting
// each gate's verification state. Gates are defined/verified via the CLI/API;
// the UI only displays them.
test.describe("acceptance gates", () => {
  test("renders each gate with its verified / pending state and command", async ({ board, server }) => {
    const t = await server.api.create({ title: "gated work" });
    await server.api.addGate(t.id, "go test ./...", "unit tests pass");
    const g2 = await server.api.addGate(t.id, "true", "smoke works");
    await server.api.verifyGate(t.id, g2); // verify the second gate only

    await board.open();
    await board.openTask(t.id);

    // Both gates render; one verified, one still pending.
    expect(await board.gateCount()).toBe(2);
    await expect(board.gate("verified")).toHaveCount(1);
    await expect(board.gate("pending")).toHaveCount(1);

    // The verified one shows its description + a Verified badge; the pending one
    // shows its command.
    await expect(board.gate("verified")).toContainText("smoke works");
    await expect(board.gate("verified")).toContainText(/verified/i);
    await expect(board.gate("pending")).toContainText("unit tests pass");
  });

  test("a task with no gates shows no gates section", async ({ board, server }) => {
    const t = await server.api.create({ title: "ungated" });
    await board.open();
    await board.openTask(t.id);
    expect(await board.gateCount()).toBe(0);
  });
});
