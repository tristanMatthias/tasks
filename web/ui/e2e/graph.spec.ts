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
