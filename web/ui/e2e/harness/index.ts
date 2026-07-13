/**
 * The test entrypoint: a @playwright/test `test` extended with two composable
 * fixtures — a fresh local `server` (its own tasksd + temp DB) and a `board`
 * (page-object) wired to it. Import { test, expect } from here in every spec.
 */
import { test as base, expect } from "@playwright/test";
import { startServer, type TestServer } from "./server";
import { Board } from "./board";

export { expect, Board };
export type { TestServer };

export const test = base.extend<{ server: TestServer; board: Board }>({
  // eslint-disable-next-line no-empty-pattern
  server: async ({}, use) => {
    const server = await startServer();
    await use(server);
    await server.stop();
  },
  board: async ({ page, server }, use) => {
    await use(new Board(page, server));
  },
});
