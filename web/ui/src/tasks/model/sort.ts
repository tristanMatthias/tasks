/**
 * How the board orders tasks. A serializable {field, dir} value plus the pure,
 * total, stable comparator it maps to — used for the tree's sibling order and
 * the dashboard's epic order alike. "Default" is the natural hierarchy order
 * (type → priority → id), so an untouched board looks exactly as before.
 */
import { ISSUE_TYPE_SORT_WEIGHT, SORT_FALLBACK, Status, type Task } from "./issue.js";
import { defaultTaskCompare, naturalCompare } from "./hierarchy.js";

export const SortField = {
  Default: "default",
  Priority: "priority",
  Status: "status",
  Type: "type",
  Title: "title",
  Id: "id",
} as const;
export type SortField = (typeof SortField)[keyof typeof SortField];

export type SortDir = "asc" | "desc";

export interface TaskSort {
  field: SortField;
  dir: SortDir;
}

export const DEFAULT_SORT: TaskSort = { field: SortField.Default, dir: "asc" };

/** Fields in menu order. Labels live in the UI (Copy), not the model. */
export const SORT_FIELDS: readonly SortField[] = [
  SortField.Default,
  SortField.Priority,
  SortField.Status,
  SortField.Type,
  SortField.Title,
  SortField.Id,
];

/** Sort weight by status: active work first, done last. */
const STATUS_WEIGHT: Readonly<Record<Status, number>> = {
  [Status.InProgress]: 0,
  [Status.Open]: 1,
  [Status.Deferred]: 2,
  [Status.Closed]: 3,
};

const priority = (t: Task) => t.priority ?? SORT_FALLBACK.Priority;
const typeWeight = (t: Task) => ISSUE_TYPE_SORT_WEIGHT[t.issue_type] ?? SORT_FALLBACK.TypeWeight;

/** Compare by the chosen field, breaking ties by natural id for stability. */
function compareByField(field: SortField, a: Task, b: Task): number {
  switch (field) {
    case SortField.Priority:
      return priority(a) - priority(b) || naturalCompare(a.id, b.id);
    case SortField.Status:
      return STATUS_WEIGHT[a.status] - STATUS_WEIGHT[b.status] || naturalCompare(a.id, b.id);
    case SortField.Type:
      return typeWeight(a) - typeWeight(b) || naturalCompare(a.id, b.id);
    case SortField.Title:
      return (a.title ?? "").localeCompare(b.title ?? "") || naturalCompare(a.id, b.id);
    case SortField.Id:
      return naturalCompare(a.id, b.id);
    default:
      return defaultTaskCompare(a, b);
  }
}

/** A total, stable comparator for the given sort (direction applied last). */
export function makeTaskComparator(sort: TaskSort): (a: Task, b: Task) => number {
  const sign = sort.dir === "desc" ? -1 : 1;
  return (a, b) => sign * compareByField(sort.field, a, b);
}
