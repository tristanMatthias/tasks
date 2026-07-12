/**
 * Flattens a task hierarchy into the ordered list of rows currently on screen,
 * honoring collapsed state. This replaces recursive components with data, so the
 * tree can be virtualized (render only the visible window) and stays fast
 * regardless of task count. Filtering is applied upstream by projecting the
 * hierarchy (see filterHierarchy) — this only walks whatever it's handed.
 */
import type { Hierarchy } from "$tasks/model/hierarchy.js";

/** One row in the flattened, display-ordered tree. */
export interface FlatRow {
  id: string;
  depth: number;
  hasChildren: boolean;
  childCount: number;
  open: boolean;
}

export function flattenVisible(
  hierarchy: Hierarchy,
  collapsedIds: ReadonlySet<string>,
  revealMatches: boolean,
): FlatRow[] {
  const rows: FlatRow[] = [];

  const walk = (id: string, depth: number): void => {
    const children = hierarchy.children.get(id) ?? [];
    const hasChildren = children.length > 0;
    // A search reveals every match (force subtrees open); otherwise honor the
    // user's collapse state.
    const open = hasChildren && (revealMatches || !collapsedIds.has(id));

    rows.push({ id, depth, hasChildren, childCount: children.length, open });
    if (open) {
      for (const childId of children) walk(childId, depth + 1);
    }
  };

  for (const rootId of hierarchy.roots) walk(rootId, 0);
  return rows;
}
