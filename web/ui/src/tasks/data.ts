/**
 * Task data access. The list is a slim projection (`?view=tree`) so the initial
 * load is tiny; the full record (description, etc.) is fetched per task on demand
 * when one is opened.
 */
import type { Task } from "./model/issue.js";
import { StorageKey } from "$shared/platform/storage.js";

interface IssuesPayload {
  issues?: Task[];
}

declare global {
  interface Window {
    // The slim-list fetch kicked off in index.html, in parallel with the JS bundle.
    __bootTasks?: Promise<IssuesPayload | null>;
    // Its resolved payload, if the fetch finished before the app mounted.
    __bootData?: IssuesPayload | null;
  }
}

const TREE_LIST_URL = "/api/issues?view=tree";
const CACHE_KEY = StorageKey.TaskListCache;

function readCache(): Task[] {
  try {
    const raw = localStorage.getItem(CACHE_KEY);
    return raw ? ((JSON.parse(raw) as IssuesPayload).issues ?? []) : [];
  } catch {
    return [];
  }
}

function writeCache(tasks: Task[]): void {
  try {
    localStorage.setItem(CACHE_KEY, JSON.stringify({ issues: tasks }));
  } catch {
    /* quota / private mode — ignore */
  }
}

/** The task list available SYNCHRONOUSLY at mount, so the tree paints with data
 *  on the first frame (no empty-then-populated flicker): the just-arrived preload
 *  if ready, else the last-known list cached in localStorage. loadTaskList()
 *  refreshes it in the background. */
export function initialTasks(): Task[] {
  const boot = typeof window !== "undefined" ? window.__bootData : undefined;
  if (boot?.issues?.length) return boot.issues;
  return readCache();
}

/** Fetch the fresh slim list (via the index.html preload when present) and cache it. */
export function loadTaskList(): Promise<Task[]> {
  const preloaded = typeof window !== "undefined" ? window.__bootTasks : undefined;
  const payload = preloaded ?? fetch(TREE_LIST_URL).then((r) => (r.ok ? r.json() : null));
  return payload.then((data) => {
    const list = data?.issues ?? [];
    if (list.length) writeCache(list);
    return list;
  });
}

/** The full record for one task (description, dependencies, …). */
export function fetchTaskDetail(id: string): Promise<Task | null> {
  return fetch(`/api/v1/tasks/${encodeURIComponent(id)}`)
    .then((r) => (r.ok ? r.json() : null))
    .catch(() => null);
}

/** Patch a task and resolve to the fresh full record (null on failure). */
export function updateTask(id: string, patch: Partial<Task>): Promise<Task | null> {
  return fetch(`/api/v1/tasks/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(patch),
  })
    .then((r) => (r.ok ? r.json() : null))
    .catch(() => null);
}
