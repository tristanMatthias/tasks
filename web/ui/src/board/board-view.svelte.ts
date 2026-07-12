/**
 * Which board view is active (tree / graph / dashboard), persisted across
 * refreshes (and synced across tabs) via runed's PersistedState — the same
 * pattern as the board filter. Read and set through `.current`.
 */
import { PersistedState } from "runed";
import { StorageKey } from "$shared/platform/storage.js";

/** The three ways to look at the board. */
export const BoardView = {
  Tree: "tree",
  Graph: "graph",
  Dashboard: "dashboard",
} as const;
export type BoardView = (typeof BoardView)[keyof typeof BoardView];

export function createPersistedView(): PersistedState<BoardView> {
  return new PersistedState<BoardView>(StorageKey.View, BoardView.Tree);
}
