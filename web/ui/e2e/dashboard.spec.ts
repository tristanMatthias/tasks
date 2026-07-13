import { test, expect } from "./harness";

test.describe("dashboard", () => {
  test("shows an epic card; clicking it opens the epic", async ({ board, server }) => {
    const epic = await server.api.create({ title: "Dashboard epic", issue_type: "epic" });
    await server.api.create({ title: "Its child", parent: epic.id });
    await board.open();

    await board.view("dashboard");
    await expect(board.dashboardCard(epic.id)).toBeVisible();

    await board.clickDashboardCard(epic.id);
    await board.expectViewingTask(epic.id);
  });
});
