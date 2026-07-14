import { test, expect } from "./harness";

test.describe("task detail", () => {
  test("edits type and priority; both persist server-side and across reload", async ({ board, server }) => {
    const t = await server.api.create({ title: "Editable", issue_type: "task", priority: 2 });
    await board.open();
    await board.openTask(t.id);

    await board.setType("bug");
    await expect.poll(async () => (await server.api.get(t.id)).issue_type).toBe("bug");

    await board.setPriority(0);
    await expect.poll(async () => (await server.api.get(t.id)).priority).toBe(0);

    await board.reload();
    await board.expectDetailType("bug");
  });

  test("renders read-only sections (description)", async ({ board, server }) => {
    const t = await server.api.create({ title: "Documented", description: "The full description text." });
    await board.open();
    await board.openTask(t.id);
    await board.expectSection("The full description text.");
  });

  test("'view in graph' opens the graph rooted on the task", async ({ board, server }) => {
    const t = await server.api.create({ title: "Graph me", issue_type: "epic" });
    await board.open();
    await board.openTask(t.id);
    await board.viewInGraph();
    await board.expectGraphNodeVisible(t.id);
  });

  test("shows GitHub link activity with the PR name as a link", async ({ board, server }) => {
    const t = await server.api.create({ title: "Linked task" });
    // A ghlink-style comment (what the webhook records): the PR name is the link.
    await server.api.comment(t.id, "Closed by [Fix the parser](https://github.com/x/y/pull/42)");
    await board.open();
    await board.openTask(t.id);
    const activity = board.page.getByTestId("activity");
    await expect(activity).toBeVisible();
    // The link copy is the PR name, pointing at the PR.
    const link = activity.getByRole("link", { name: "Fix the parser" });
    await expect(link).toHaveAttribute("href", "https://github.com/x/y/pull/42");
  });

  test("the parent link navigates to the parent task", async ({ board, server }) => {
    const parent = await server.api.create({ title: "The parent task", issue_type: "epic" });
    const child = await server.api.create({ title: "The child task", parent: parent.id });
    await board.open();
    await board.openTask(child.id);
    await board.openDetailLink("The parent task");
    await board.expectViewingTask(parent.id);
  });
});
