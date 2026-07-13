/**
 * Build the app under test ONCE before the suite: the UI bundle (so the freshly
 * added test hooks are embedded) and the `tasksd` engine binary that serves it.
 * Everything runs locally against a throwaway server — production is never
 * involved. Skip the rebuild with E2E_SKIP_BUILD=1 for a faster inner loop.
 */
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { TASKSD_BIN } from "./server";

const here = dirname(fileURLToPath(import.meta.url));
const uiDir = join(here, "..", ".."); // web/ui
const repoRoot = join(uiDir, "..", ".."); // repo root (embeds web/static via go:embed)

export default function globalSetup(): void {
  if (process.env.E2E_SKIP_BUILD === "1") return;
  execFileSync("npm", ["run", "build"], { cwd: uiDir, stdio: "inherit" });
  execFileSync("go", ["build", "-o", TASKSD_BIN, "./cmd/tasksd"], { cwd: repoRoot, stdio: "inherit" });
}
