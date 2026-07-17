import { test, expect } from "./harness";

// The ⌘K command palette: fuzzy-search tasks and jump to them, and switch views.
test.describe("command palette", () => {
  test("opens with ⌘K, searches tasks, and navigates to one", async ({ board, server }) => {
    const alpha = await server.api.create({ title: "Alpha lexer rewrite" });
    await server.api.create({ title: "Beta parser cleanup" });
    await board.open();

    await board.openPalette();
    await board.paletteSearch("Alpha");

    // The matching task shows; the non-match is filtered out.
    await expect(board.paletteItem("Alpha lexer rewrite")).toBeVisible();
    await expect(board.paletteItem("Beta parser cleanup")).toHaveCount(0);

    await board.paletteItem("Alpha lexer rewrite").click();
    await board.expectViewingTask(alpha.id);
  });

  test("caps rendered rows on a large board (keeps typing fast)", async ({ board, server }) => {
    // Seed more tasks than the render cap so we can prove it doesn't render all.
    for (let i = 0; i < 40; i++) await server.api.create({ title: `bulk task ${i}` });
    await board.open();
    await board.openPalette();
    // Tasks are capped at 20; plus a handful of command actions → well under 40.
    const rows = await board.page.locator('[data-slot="command-item"]').count();
    expect(rows).toBeLessThanOrEqual(25);
  });

  test("switches the board view from the palette", async ({ board, server }) => {
    const t = await server.api.create({ title: "a widget" });
    await board.open(); // harness defaults to the tree view

    await board.openPalette();
    await board.paletteItem("Graph").click();

    // The graph view is now active — the task shows as a node.
    await board.expectGraphNodeVisible(t.id);
  });
});
