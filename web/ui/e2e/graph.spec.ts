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
});
