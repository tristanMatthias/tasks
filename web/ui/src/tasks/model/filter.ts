/**
 * The board's search + facet filter: a plain, serializable value plus the pure
 * predicates that apply it. No framework here — persistence and UI live elsewhere.
 */
import { IssueType, Status, type Task } from "./issue.js";

export const ALL_STATUSES: readonly Status[] = Object.values(Status);
export const ALL_TYPES: readonly IssueType[] = Object.values(IssueType);

/** Search text + the statuses/types to include. */
export interface TaskFilter {
  query: string;
  statuses: Status[];
  types: IssueType[];
}

export const DEFAULT_FILTER: TaskFilter = {
  query: "",
  statuses: [...ALL_STATUSES],
  types: [...ALL_TYPES],
};

/** A fresh, independently-mutable default filter (e.g. a local subtree filter). */
export function newFilter(): TaskFilter {
  return { query: "", statuses: [...ALL_STATUSES], types: [...ALL_TYPES] };
}

/** Whether the filter narrows anything (else the whole tree is shown as-is). */
export function isFilterActive(filter: TaskFilter): boolean {
  return (
    filter.query.trim() !== "" ||
    filter.statuses.length < ALL_STATUSES.length ||
    filter.types.length < ALL_TYPES.length
  );
}

/** Whether a single task passes the filter. */
export function matchesFilter(task: Task, filter: TaskFilter): boolean {
  if (!filter.statuses.includes(task.status)) return false;
  if (!filter.types.includes(task.issue_type)) return false;
  const query = filter.query.trim().toLowerCase();
  if (query) {
    const haystack = `${task.id} ${task.title ?? ""}`.toLowerCase();
    if (!haystack.includes(query)) return false;
  }
  return true;
}

/** Add/remove a value from a facet list (immutably, for reactive assignment). */
export function toggleValue<T>(values: readonly T[], value: T): T[] {
  return values.includes(value) ? values.filter((v) => v !== value) : [...values, value];
}
