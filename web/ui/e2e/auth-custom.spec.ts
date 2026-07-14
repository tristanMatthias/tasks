import { test, expect } from "./harness";

// These run against the CUSTOM-auth engine (mode "custom", an embedder cookie
// session) — the exact shape agenttasks/GitHub uses. Token mode never exercises
// the `mode === "custom"` branches where both logout regressions hid.
test.describe("custom-mode auth", () => {
  test("logs in and shows the board", async ({ customBoard, customServer }) => {
    await customServer.api.create({ title: "Custom task" });
    await customBoard.open();
    await customBoard.expectSignedIn();
  });

  test("logout fully signs out — even with a stale client-trust cookie", async ({ customBoard }) => {
    await customBoard.open();
    await customBoard.expectSignedIn();

    // Plant exactly the kind of leftover client cookie that previously kept a
    // "logged out" user looking signed in (Clerk's __client_uat). Auth must be
    // server-driven, so this must NOT keep the session alive.
    await customBoard.setCookie("__client_uat", "9999999999");

    await customBoard.logout();
    await customBoard.expectSignedOut();

    // And it stays signed out across a reload (no client fallback resurrects it).
    await customBoard.reload();
    await customBoard.expectSignedOut();
  });

  test("github activity is one clickable row with the logo, PR name and #number", async ({ customBoard, customServer }) => {
    const t = await customServer.api.create({ title: "Has a PR" });
    await customServer.api.githubComment(t.id, "Closed by [Rework the lexer #12](https://github.com/x/y/pull/12)");
    await customBoard.open();
    await customBoard.openTask(t.id);
    const item = customBoard.page.getByTestId("activity-item");
    await expect(item).toBeVisible();
    // The WHOLE row is a single link to the PR, and the copy carries the #number.
    await expect(item).toHaveAttribute("href", "https://github.com/x/y/pull/12");
    await expect(item).toContainText("Rework the lexer #12");
  });

  test("settings offers the GitHub Connect action when the integration is configured", async ({ customBoard }) => {
    await customBoard.open();
    await customBoard.gotoSettings("connect");
    const connect = customBoard.page.getByTestId("github-connect");
    await expect(connect).toBeVisible();
    await expect(connect).toHaveAttribute("href", "/integrations/github/connect");
  });

  test("a signed-out visitor sees the landing with a login affordance", async ({ customBoard, customServer }) => {
    // Load WITHOUT establishing a session → custom mode shows the public landing.
    await customBoard.page.goto(`${customServer.baseURL}/`, { waitUntil: "networkidle" });
    await customBoard.expectSignedOut();
  });
});
