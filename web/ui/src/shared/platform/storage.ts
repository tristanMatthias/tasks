/** localStorage keys, namespaced in one place. */
export const StorageKey = {
  /** Persisted sizes of the main resizable panes. */
  PaneLayout: "tasks:pane-layout",
  /** Persisted search + facet filter. */
  Filter: "tasks:filter",
  /** Persisted board view (tree / graph / dashboard). */
  View: "tasks:view",
  /** Persisted sort order (field + direction). */
  Sort: "tasks:sort",
  /** Persisted set of collapsed subtree ids (only the collapsed ones). */
  Collapsed: "tasks:collapsed",
  /** Cached slim task list (for instant load). */
  TaskListCache: "tasks:tree-list",
} as const;
