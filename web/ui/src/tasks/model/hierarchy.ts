/**
 * Pure functions that turn the flat task list into a sorted hierarchy and
 * compute search visibility. No DOM, no framework — trivially testable.
 */
import {
  CONTAINMENT_DEPENDENCY_TYPES,
  ISSUE_TYPE_SORT_WEIGHT,
  SORT_FALLBACK,
  type Task,
} from "./issue.js";

/** An indexed, sorted view of the task forest. */
export interface Hierarchy {
  /** id → task */
  readonly byId: ReadonlyMap<string, Task>;
  /** parent id → ordered child ids */
  readonly children: ReadonlyMap<string, readonly string[]>;
  /** ordered ids of tasks with no parent */
  readonly roots: readonly string[];
}

/** Natural ordering: compare digit runs numerically so `ps3t.2` < `ps3t.11`. */
export function naturalCompare(a: string, b: string): number {
  const chunk = /\d+|\D+/g;
  const as = a.match(chunk) ?? [];
  const bs = b.match(chunk) ?? [];
  const count = Math.min(as.length, bs.length);
  for (let i = 0; i < count; i++) {
    const x = as[i];
    const y = bs[i];
    const bothNumeric = isDigit(x) && isDigit(y);
    if (bothNumeric) {
      const diff = parseInt(x, 10) - parseInt(y, 10);
      if (diff !== 0) return diff;
    } else if (x !== y) {
      return x < y ? -1 : 1;
    }
  }
  return as.length - bs.length;
}

function isDigit(chunk: string): boolean {
  const code = chunk.charCodeAt(0);
  return code >= 48 && code <= 57; // '0'..'9'
}

function parentIdOf(task: Task): string | null {
  for (const dep of task.dependencies ?? []) {
    const isContainment = CONTAINMENT_DEPENDENCY_TYPES.includes(dep.type);
    if (isContainment && dep.depends_on_id && dep.depends_on_id !== task.id) {
      return dep.depends_on_id;
    }
  }
  return null;
}

/** The parent id of a task within a built hierarchy, or null if it's a root. */
export function parentId(hierarchy: Hierarchy, id: string): string | null {
  for (const [parent, kids] of hierarchy.children) {
    if (kids.includes(id)) return parent;
  }
  return null;
}

/** A total order over tasks (used to order siblings and roots). */
export type TaskComparator = (a: Task, b: Task) => number;

/** The default order: type, then priority, then natural id — like beads. */
export const defaultTaskCompare: TaskComparator = (a, b) => {
  const weight = (task: Task) => ISSUE_TYPE_SORT_WEIGHT[task.issue_type] ?? SORT_FALLBACK.TypeWeight;
  const priority = (task: Task) => task.priority ?? SORT_FALLBACK.Priority;
  const byType = weight(a) - weight(b);
  if (byType !== 0) return byType;
  const byPriority = priority(a) - priority(b);
  if (byPriority !== 0) return byPriority;
  return naturalCompare(a.id, b.id);
};

/**
 * Build the sorted hierarchy from a flat list of tasks. Siblings (and roots) are
 * ordered by `compare`, defaulting to the natural hierarchy order.
 */
export function buildHierarchy(
  tasks: readonly Task[],
  compare: TaskComparator = defaultTaskCompare,
): Hierarchy {
  const byId = new Map<string, Task>();
  for (const task of tasks) byId.set(task.id, task);

  const children = new Map<string, string[]>();
  const hasParent = new Set<string>();
  for (const task of tasks) {
    const parentId = parentIdOf(task);
    if (parentId !== null && byId.has(parentId)) {
      const siblings = children.get(parentId) ?? [];
      siblings.push(task.id);
      children.set(parentId, siblings);
      hasParent.add(task.id);
    }
  }

  const cmp = (aId: string, bId: string) => compare(byId.get(aId)!, byId.get(bId)!);
  for (const siblings of children.values()) siblings.sort(cmp);
  const roots = tasks
    .map((task) => task.id)
    .filter((id) => !hasParent.has(id))
    .sort(cmp);

  return { byId, children, roots };
}

/**
 * Project the hierarchy down to the nodes that match, hoisting each match up to
 * its nearest matching ancestor (or to a root). A filtered-out parent is dropped,
 * not kept as an empty shell — so hiding epics lifts their tasks up a level
 * rather than leaving orphaned tasks nested under a hidden epic. Sibling order is
 * inherited from the source, so the result feeds straight into the flattener.
 */
export function filterHierarchy(source: Hierarchy, matches: (task: Task) => boolean): Hierarchy {
  const byId = new Map<string, Task>();
  const children = new Map<string, string[]>();
  const roots: string[] = [];

  const visit = (id: string, matchedAncestor: string | null): void => {
    const task = source.byId.get(id)!;
    let nextAncestor = matchedAncestor;
    if (matches(task)) {
      byId.set(id, task);
      children.set(id, []);
      if (matchedAncestor === null) roots.push(id);
      else children.get(matchedAncestor)!.push(id);
      nextAncestor = id;
    }
    for (const childId of source.children.get(id) ?? []) visit(childId, nextAncestor);
  };
  for (const rootId of source.roots) visit(rootId, null);

  return { byId, children, roots };
}
