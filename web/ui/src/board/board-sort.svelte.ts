/**
 * The board's sort order (field + direction), persisted across refreshes (and
 * synced across tabs) via runed's PersistedState — same pattern as the filter
 * and view. Read and set through `.current`.
 */
import { PersistedState } from "runed";
import { DEFAULT_SORT, type TaskSort } from "$tasks/model/sort.js";
import { StorageKey } from "$shared/platform/storage.js";

export function createPersistedSort(): PersistedState<TaskSort> {
  return new PersistedState<TaskSort>(StorageKey.Sort, DEFAULT_SORT);
}
