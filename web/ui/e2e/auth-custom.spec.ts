import { test } from "./harness";

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

  test("a signed-out visitor sees the landing with a login affordance", async ({ customBoard, customServer }) => {
    // Load WITHOUT establishing a session → custom mode shows the public landing.
    await customBoard.page.goto(`${customServer.baseURL}/`, { waitUntil: "networkidle" });
    await customBoard.expectSignedOut();
  });
});
