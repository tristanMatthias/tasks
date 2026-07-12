/**
 * Pure roll-up of each epic's progress from its descendants. No DOM, no
 * framework — the dashboard view renders whatever this returns.
 */
import { buildHierarchy, naturalCompare } from "$tasks/model/hierarchy.js";
import { IssueType, Status, type Task } from "$tasks/model/issue.js";

/** Per-epic progress: how its whole subtree breaks down by status. */
export interface EpicSummary {
  epic: Task;
  /** Descendant count (the epic's whole subtree, excluding itself). */
  total: number;
  /** Descendants grouped by status. */
  counts: Record<Status, number>;
  /** Closed descendants. */
  done: number;
  /** Completion 0–100 (0 when the epic has no children). */
  percent: number;
}

const ZERO_COUNTS = (): Record<Status, number> => ({
  [Status.Open]: 0,
  [Status.InProgress]: 0,
  [Status.Deferred]: 0,
  [Status.Closed]: 0,
});

/**
 * Summarize every epic in the task list, ordered so the work that needs
 * attention floats up: active/incomplete epics first (least complete first),
 * then finished epics, then empty ones — ties broken by natural id.
 */
export function epicSummaries(tasks: readonly Task[]): EpicSummary[] {
  const hierarchy = buildHierarchy(tasks);

  const descendantsOf = (id: string): string[] => {
    const out: string[] = [];
    const stack = [...(hierarchy.children.get(id) ?? [])];
    while (stack.length) {
      const cur = stack.pop()!;
      out.push(cur);
      const kids = hierarchy.children.get(cur);
      if (kids) stack.push(...kids);
    }
    return out;
  };

  const summaries = tasks
    .filter((task) => task.issue_type === IssueType.Epic)
    .map((epic): EpicSummary => {
      const counts = ZERO_COUNTS();
      const ids = descendantsOf(epic.id);
      for (const id of ids) {
        const task = hierarchy.byId.get(id)!;
        counts[task.status] += 1;
      }
      const total = ids.length;
      const done = counts[Status.Closed];
      const percent = total === 0 ? 0 : Math.round((done / total) * 100);
      return { epic, total, counts, done, percent };
    });

  // 0 = active (has remaining work), 1 = complete, 2 = empty.
  const rank = (s: EpicSummary) => (s.total === 0 ? 2 : s.percent >= 100 ? 1 : 0);
  summaries.sort((a, b) => {
    const byRank = rank(a) - rank(b);
    if (byRank !== 0) return byRank;
    if (rank(a) === 0 && a.percent !== b.percent) return a.percent - b.percent;
    return naturalCompare(a.epic.id, b.epic.id);
  });
  return summaries;
}
