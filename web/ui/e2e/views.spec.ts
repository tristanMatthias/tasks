import { test } from "./harness";

test.describe("views", () => {
  test("switches between tree and graph via the view switcher", async ({ board, server }) => {
    const epic = await server.api.create({ title: "Viewable epic", issue_type: "epic" });
    await board.open();
    await board.expectRowVisible(epic.id); // tree

    await board.view("graph");
    await board.expectGraphNodeVisible(epic.id); // graph, rooted on the epic

    await board.view("tree");
    await board.expectRowVisible(epic.id);
  });
});
