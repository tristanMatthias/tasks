import { test } from "./harness";

test.describe("tree", () => {
  test("search filters to matching rows and clearing restores them", async ({ board, server }) => {
    const a = await server.api.create({ title: "Apple pie" });
    const b = await server.api.create({ title: "Banana bread" });
    await board.open();
    await board.expectRowVisible(a.id);
    await board.expectRowVisible(b.id);

    await board.search("Apple");
    await board.expectRowVisible(a.id);
    await board.expectRowHidden(b.id);

    await board.clearSearch();
    await board.expectRowVisible(b.id);
  });

  test("a parent can be collapsed while a search is active", async ({ board, server }) => {
    // Both match the query, so the parent is shown WITH its child (not hoisted).
    const parent = await server.api.create({ title: "Zeta epic", issue_type: "epic" });
    const child = await server.api.create({ title: "Zeta child", parent: parent.id });
    await board.open();

    await board.search("Zeta");
    await board.expectRowVisible(parent.id);
    await board.expectRowVisible(child.id);

    await board.collapse(parent.id); // collapse must win, even during search
    await board.expectRowHidden(child.id);
  });
});
