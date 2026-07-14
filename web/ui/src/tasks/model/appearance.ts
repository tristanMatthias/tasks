/**
 * How domain values map to visual tokens. The concrete colors live as CSS
 * custom properties in app.css (so they are themeable); this module only names
 * which token each status/type uses. Nothing here hard-codes a color.
 */
import { GateStatus, IssueType, Status } from "./issue.js";

/** The CSS custom property that colors each issue type's (outlined) badge. */
export const TYPE_COLOR_VAR: Readonly<Record<IssueType, string>> = {
  [IssueType.Epic]: "var(--type-epic)",
  [IssueType.Feature]: "var(--type-feature)",
  [IssueType.Task]: "var(--type-task)",
  [IssueType.Bug]: "var(--type-bug)",
  [IssueType.Chore]: "var(--type-chore)",
};

/** The CSS custom property that colors each priority (P0 highest → P4 lowest). */
export const PRIORITY_COLOR_VAR: Readonly<Record<number, string>> = {
  0: "var(--priority-0)",
  1: "var(--priority-1)",
  2: "var(--priority-2)",
  3: "var(--priority-3)",
  4: "var(--priority-4)",
};

/** The CSS custom property that colors each status indicator. */
export const STATUS_COLOR_VAR: Readonly<Record<Status, string>> = {
  [Status.Open]: "var(--status-open)",
  [Status.InProgress]: "var(--status-in-progress)",
  [Status.Deferred]: "var(--status-deferred)",
  [Status.Closed]: "var(--status-closed)",
};

/** Human-readable label for a status (used in tooltips / a11y). */
export const STATUS_LABEL: Readonly<Record<Status, string>> = {
  [Status.Open]: "Open",
  [Status.InProgress]: "In progress",
  [Status.Deferred]: "Deferred",
  [Status.Closed]: "Closed",
};

/** Gate verification reuses the status palette: verified→open (green success),
 *  pending→deferred (amber "waiting"). No new color tokens needed. */
export const GATE_STATUS_COLOR_VAR: Readonly<Record<GateStatus, string>> = {
  [GateStatus.Verified]: "var(--status-open)",
  [GateStatus.Pending]: "var(--status-deferred)",
};

/** Human-readable label for a gate's verification state. */
export const GATE_STATUS_LABEL: Readonly<Record<GateStatus, string>> = {
  [GateStatus.Verified]: "Verified",
  [GateStatus.Pending]: "Pending",
};
