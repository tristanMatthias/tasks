/**
 * The task domain model — the single source of truth for issue vocabulary.
 * Every status / type / dependency string used in the UI comes from here, so
 * there are no bare string literals scattered across components.
 */

/** Lifecycle state of a task. */
export const Status = {
  Open: "open",
  InProgress: "in_progress",
  Deferred: "deferred",
  Closed: "closed",
} as const;
export type Status = (typeof Status)[keyof typeof Status];

/** Kind of work item. */
export const IssueType = {
  Epic: "epic",
  Feature: "feature",
  Task: "task",
  Bug: "bug",
  Chore: "chore",
} as const;
export type IssueType = (typeof IssueType)[keyof typeof IssueType];

/** Relationship between two tasks. */
export const DependencyType = {
  ParentChild: "parent-child",
  Parent: "parent",
  Blocks: "blocks",
  Related: "related",
  RelatesTo: "relates-to",
  DiscoveredFrom: "discovered-from",
  Supersedes: "supersedes",
} as const;
export type DependencyType = (typeof DependencyType)[keyof typeof DependencyType];

/** The dependency types that express containment (parent → child). */
export const CONTAINMENT_DEPENDENCY_TYPES: readonly DependencyType[] = [
  DependencyType.ParentChild,
  DependencyType.Parent,
];

/** A directed dependency edge (matches the API's issue shape). */
export interface Dependency {
  issue_id: string;
  depends_on_id: string;
  type: DependencyType;
}

/** Verification state of an acceptance gate. */
export const GateStatus = {
  Pending: "pending",
  Verified: "verified",
} as const;
export type GateStatus = (typeof GateStatus)[keyof typeof GateStatus];

/** Kind of acceptance gate (only `command` is implemented server-side today). */
export const GateType = {
  Command: "command",
} as const;
export type GateType = (typeof GateType)[keyof typeof GateType];

/** An acceptance gate: a check that must pass before a task can be closed.
 *  Read-only in the UI — gates are defined + verified via the CLI/API. */
export interface Gate {
  id: string;
  type: GateType;
  status: GateStatus;
  description?: string;
  command?: string;
  verified_at?: string;
  verified_by?: string;
  exit_code?: number;
  evidence?: string;
}

/** A single task/issue (a subset of the API shape the UI reads). */
export interface Task {
  id: string;
  title: string;
  status: Status;
  issue_type: IssueType;
  priority: number | null;
  description?: string;
  acceptance_criteria?: string;
  design?: string;
  notes?: string;
  labels?: string[];
  assignee?: string;
  dependencies?: Dependency[];
  comments?: Comment[];
  gates?: Gate[];
}

/** A comment / activity entry on a task (e.g. a GitHub PR link). */
export interface Comment {
  id: string;
  author?: string;
  text: string;
  created_at?: string;
}

/** Ids this task is blocked by (its own `blocks` dependency edges). */
export function blockedByIds(task: Task): string[] {
  return (task.dependencies ?? [])
    .filter((dep) => dep.type === DependencyType.Blocks)
    .map((dep) => dep.depends_on_id);
}

/** Tasks that `id` is blocking: those whose `blocks` edge points back at it. */
export function blockingTasks(id: string, all: readonly Task[]): Task[] {
  return all.filter((task) =>
    (task.dependencies ?? []).some((dep) => dep.type === DependencyType.Blocks && dep.depends_on_id === id),
  );
}

/** Sibling sort weight by type (epics first, chores last). */
export const ISSUE_TYPE_SORT_WEIGHT: Readonly<Record<IssueType, number>> = {
  [IssueType.Epic]: 0,
  [IssueType.Feature]: 1,
  [IssueType.Task]: 2,
  [IssueType.Bug]: 3,
  [IssueType.Chore]: 4,
};

/** Selectable priorities, highest (P0) to lowest (P4). */
export const PRIORITIES = [0, 1, 2, 3, 4] as const;

/** Fallbacks used when a value is missing, so sorting stays total + stable. */
export const SORT_FALLBACK = {
  /** Weight for an unrecognized issue type (sorts after all known types). */
  TypeWeight: 9,
  /** Priority assumed for a task with no priority (sorts after all set ones). */
  Priority: 9,
} as const;

/** True when the task is done. */
export function isClosed(task: Task): boolean {
  return task.status === Status.Closed;
}

/** The short, project-prefix-free id (e.g. "ps3t.1" from "proj-ps3t.1"). */
export function shortId(id: string): string {
  const lastDash = id.lastIndexOf("-");
  return lastDash >= 0 ? id.slice(lastDash + 1) : id;
}
