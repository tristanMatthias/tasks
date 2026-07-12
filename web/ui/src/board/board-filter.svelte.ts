/**
 * The board's search + facet filter, persisted across refreshes (and synced
 * across tabs) via runed's PersistedState — the standard Svelte 5 rune for
 * persisted reactive state. Read and mutate through `.current`.
 */
import { PersistedState } from "runed";
import { DEFAULT_FILTER, type TaskFilter } from "$tasks/model/filter.js";
import { StorageKey } from "$shared/platform/storage.js";

export function createPersistedFilter(): PersistedState<TaskFilter> {
  return new PersistedState<TaskFilter>(StorageKey.Filter, DEFAULT_FILTER);
}
