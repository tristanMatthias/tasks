import { test } from "./harness";

test.describe("graph", () => {
  test("selecting a task in the tree re-roots the graph on it", async ({ board, server }) => {
    const e1 = await server.api.create({ title: "Epic one", issue_type: "epic" });
    const e2 = await server.api.create({ title: "Epic two", issue_type: "epic" });
    await board.open();

    // Pick e2 in the tree, then open the graph: it should be centered on e2.
    await board.openTask(e2.id);
    await board.view("graph");
    await board.expectGraphNodeVisible(e2.id);

    // Back to the tree, pick e1 — the graph re-roots on e1.
    await board.view("tree");
    await board.openTask(e1.id);
    await board.view("graph");
    await board.expectGraphNodeVisible(e1.id);
  });

  test("a blocker + blocked pair both appear in the graph", async ({ board, server }) => {
    const blocked = await server.api.create({ title: "Needs the blocker", issue_type: "task" });
    const blocker = await server.api.create({ title: "Must finish first", issue_type: "task" });
    await server.api.dep(blocked.id, blocker.id); // blocked depends on blocker
    await board.open();

    await board.openTask(blocked.id);
    await board.view("graph");
    await board.expectGraphNodeVisible(blocked.id);
    await board.expectGraphNodeVisible(blocker.id); // upstream blocker is in the stack
  });

  test("a bug child of a big epic still shows in the epic's graph", async ({ board, server }) => {
    const epic = await server.api.create({ title: "Big epic", issue_type: "epic" });
    // >12 task children: the old per-node fan-out cap (12) would fill up on these
    // (tasks sort before bugs), silently dropping the bug that sorts last.
    for (let i = 0; i < 13; i++) {
      await server.api.create({ title: `Child task ${i}`, issue_type: "task", parent: epic.id });
    }
    const bug = await server.api.create({ title: "The elusive bug", issue_type: "bug", parent: epic.id });

    await board.open();
    await board.openTask(epic.id);
    await board.view("graph");
    await board.expectGraphNodeVisible(epic.id);
    await board.expectGraphNodeVisible(bug.id); // was dropped by the fan-out cap
  });

  test("kind selector, node click, re-root and full page", async ({ board, server }) => {
    const epic = await server.api.create({ title: "Kind epic", issue_type: "epic" });
    const child = await server.api.create({ title: "Kind child", parent: epic.id });
    await board.open();
    await board.openTask(epic.id);
    await board.view("graph");
    await board.expectGraphNodeVisible(epic.id);
    await board.expectGraphNodeVisible(child.id);

    // Hierarchy kind still shows the breakdown.
    await board.graphKind("subtree");
    await board.expectGraphNodeVisible(child.id);
    await board.graphKind("stack");

    // Tapping the child node selects it (detail follows).
    await board.clickNode(child.id);
    await board.expectViewingTask(child.id);

    // Double-tap re-roots the graph on the child.
    await board.recenterOnNode(child.id);
    await board.expectGraphNodeVisible(child.id);

    // Full page surfaces search + close; closing returns to the panel.
    await board.openFullscreen();
    await board.expectFullscreen();
    await board.closeFullscreen();
  });
});
