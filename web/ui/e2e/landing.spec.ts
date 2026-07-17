import { test, expect } from "./harness";

// The public landing (shown to signed-out visitors) carries a GitHub link in the
// header, to the right of the Log in button.
test.describe("landing header", () => {
  test("shows a GitHub link next to Log in", async ({ customBoard, customServer }) => {
    // Load without a session → custom mode renders the public landing.
    await customBoard.page.goto(`${customServer.baseURL}/`, { waitUntil: "networkidle" });

    const github = customBoard.page.getByRole("link", { name: "GitHub repository" });
    await expect(github).toBeVisible();
    await expect(github).toHaveAttribute("href", /github\.com/);
    await expect(github).toHaveAttribute("target", "_blank");
  });
});
