import { defineConfig, devices } from "@playwright/test";

/**
 * E2E config. Tests run entirely against a LOCAL tasksd (loopback + temp DB)
 * spun up per test by the harness — nothing contacts production. global-setup
 * builds the UI + engine once.
 */
export default defineConfig({
  testDir: "./e2e",
  globalSetup: "./e2e/harness/global-setup.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 2 : undefined,
  timeout: 30_000,
  expect: { timeout: 7_000 },
  reporter: [["list"]],
  use: {
    headless: true,
    trace: "retain-on-failure",
    ...devices["Desktop Chrome"],
  },
});
