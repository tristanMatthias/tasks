import { test, expect } from "./harness";

// A task id mentioned in another task's markdown renders as a status pill: a
// status-colored dot + the referenced task's title, as a link to it.
test.describe("task references", () => {
  test("a referenced ticket renders as a status-dot pill with its title", async ({ board, server }) => {
    const target = await server.api.create({ title: "Lexer rewrite" });
    const holder = await server.api.create({
      title: "Holder",
      description: `Blocked on ${target.id} until it lands.`,
    });

    await board.open();
    await board.openTask(holder.id);

    const shortId = target.id.slice(target.id.lastIndexOf("-") + 1);
    const pill = board.page.locator(`a.taskref-pill[data-taskref="${target.id}"]`);
    await expect(pill).toBeVisible();
    await expect(pill.locator(".taskref-dot")).toHaveCount(1); // status indicator
    await expect(pill.locator(".taskref-id")).toHaveText(shortId); // the number
    await expect(pill).toContainText("Lexer rewrite"); // then the title
    await expect(pill).toHaveAttribute("data-s", "open");

    // Clicking it navigates to the referenced task (SPA intercept).
    await pill.click();
    await board.expectViewingTask(target.id);
  });
});
