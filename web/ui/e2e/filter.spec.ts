import { test } from "./harness";

test.describe("filters", () => {
  test("unchecking a status hides those rows", async ({ board, server }) => {
    const open = await server.api.create({ title: "Still open", issue_type: "task" });
    const done = await server.api.create({ title: "All done", issue_type: "task" });
    await server.api.close(done.id);
    await board.open();
    await board.expectRowVisible(open.id);
    await board.expectRowVisible(done.id);

    await board.toggleStatusFilter("closed");
    await board.expectRowVisible(open.id);
    await board.expectRowHidden(done.id);
  });

  test("unchecking a type hides those rows", async ({ board, server }) => {
    const epic = await server.api.create({ title: "An epic", issue_type: "epic" });
    const bug = await server.api.create({ title: "A bug", issue_type: "bug" });
    await board.open();
    await board.expectRowVisible(epic.id);
    await board.expectRowVisible(bug.id);

    await board.toggleTypeFilter("bug");
    await board.expectRowVisible(epic.id);
    await board.expectRowHidden(bug.id);
  });
});
