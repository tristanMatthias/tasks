/**
 * Which subtrees are collapsed, persisted across refreshes. The tree defaults to
 * expanded, so we store only the collapsed ids (a compact delta, not a flag per
 * node). Membership is read through a reactive SvelteSet for O(1) row lookups;
 * every mutation is flushed to the persisted array.
 */
import { PersistedState } from "runed";
import { SvelteSet } from "svelte/reactivity";
import { StorageKey } from "$shared/platform/storage.js";

export class CollapseState {
  readonly #persisted = new PersistedState<string[]>(StorageKey.Collapsed, []);
  readonly #ids = new SvelteSet<string>(this.#persisted.current);

  /** The collapsed ids, for reactive membership checks. */
  get ids(): ReadonlySet<string> {
    return this.#ids;
  }

  toggle(id: string): void {
    if (this.#ids.has(id)) this.#ids.delete(id);
    else this.#ids.add(id);
    this.#flush();
  }

  collapseAll(ids: Iterable<string>): void {
    this.#ids.clear();
    for (const id of ids) this.#ids.add(id);
    this.#flush();
  }

  expandAll(): void {
    this.#ids.clear();
    this.#flush();
  }

  #flush(): void {
    this.#persisted.current = [...this.#ids];
  }
}
