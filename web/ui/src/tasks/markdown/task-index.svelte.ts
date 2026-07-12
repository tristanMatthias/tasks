/**
 * A tiny id index for linkifying task references in text. Built once from the
 * task list (O(n)) and queried O(1) per candidate, so markdown rendering never
 * scans the task list. Rebuilt only when the list changes.
 *
 * `version` is a reactive tick: read `indexVersion()` inside a $derived so a
 * render re-runs when the index fills (the list may arrive after first paint).
 */
import { shortId, type Task } from "$tasks/model/issue.js";

let fullIds = new Set<string>();
let shortToFull = new Map<string, string>();
let version = $state(0);

/** Rebuild the index from the current task list. */
export function indexTasks(tasks: readonly Task[]): void {
  const full = new Set<string>();
  const short = new Map<string, string>();
  for (const task of tasks) {
    full.add(task.id);
    // Short (prefix-free) id → full id. Selectors are unique within a board.
    short.set(shortId(task.id), task.id);
  }
  fullIds = full;
  shortToFull = short;
  version++;
}

/**
 * Resolve a text token to a real task's full id, or null if it isn't one. A
 * candidate may be a full id ("proj-ps3t.2") or a short selector ("ps3t.2").
 * The membership check is the safety net: only real ids ever become links.
 */
export function resolveTaskRef(candidate: string): string | null {
  if (fullIds.has(candidate)) return candidate;
  return shortToFull.get(candidate) ?? null;
}

/** Reactive tick — read in a $derived so renders update when the index changes. */
export function indexVersion(): number {
  return version;
}
