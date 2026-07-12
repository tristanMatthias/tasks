<script lang="ts">
  import { buildHierarchy, filterHierarchy } from "$tasks/model/hierarchy.js";
  import type { Task } from "$tasks/model/issue.js";
  import { isFilterActive, matchesFilter, type TaskFilter } from "$tasks/model/filter.js";
  import { CollapseState } from "../collapse.svelte.js";
  import { createVirtualList } from "$shared/platform/virtual-list.svelte.js";
  import { flattenVisible } from "./flatten.js";
  import { TreeLayout } from "./tree-layout.js";
  import TreeRow from "./TreeRow.svelte";

  interface Props {
    tasks: readonly Task[];
    filter: TaskFilter;
    selectedId?: string | null;
    onSelect?: (id: string) => void;
    onPatch: (id: string, patch: Partial<Task>) => void;
    /** Show only the subtree under this task (its children) instead of the roots. */
    rootId?: string | null;
    /** Grow to fit all rows (no internal scroll/virtualization) for embedding. */
    autoHeight?: boolean;
  }
  let {
    tasks,
    filter,
    selectedId = null,
    onSelect,
    onPatch,
    rootId = null,
    autoHeight = false,
  }: Props = $props();

  const hierarchy = $derived(buildHierarchy(tasks));
  // Scope to a task's children first (for the detail subtree), so filtering only
  // ever projects within that subtree...
  const scoped = $derived(
    rootId ? { ...hierarchy, roots: hierarchy.children.get(rootId) ?? [] } : hierarchy,
  );
  // ...then, when filtering, project down to matching nodes (hoisting matches out
  // of filtered-out parents); otherwise show the scope as-is.
  const view = $derived(
    isFilterActive(filter) ? filterHierarchy(scoped, (task) => matchesFilter(task, filter)) : scoped,
  );
  // A text search reveals every match (auto-expands); status/type facets are
  // subtractive and honor the user's collapse state.
  const revealMatches = $derived(filter.query.trim() !== "");

  // Collapsed subtrees, persisted across refreshes.
  const collapse = new CollapseState();

  // The tree, flattened to the ordered rows currently on screen. Cheap to
  // recompute; the DOM cost is bounded by the virtualizer, not by row count.
  const rows = $derived(flattenVisible(view, collapse.ids, revealMatches));

  let scrollEl = $state<HTMLDivElement | null>(null);

  const list = createVirtualList({
    count: () => rows.length,
    getScrollElement: () => scrollEl,
    estimateSize: () => TreeLayout.RowHeightPx,
    overscan: TreeLayout.Overscan,
  });

  function toggle(id: string): void {
    collapse.toggle(id);
  }

  export function expandAll(): void {
    collapse.expandAll();
  }
  export function collapseAll(): void {
    collapse.collapseAll(hierarchy.children.keys());
  }
</script>

{#snippet treeRow(row: (typeof rows)[number])}
  <TreeRow
    task={hierarchy.byId.get(row.id)!}
    depth={row.depth}
    selected={row.id === selectedId}
    hasChildren={row.hasChildren}
    childCount={row.childCount}
    open={row.open}
    onToggle={() => toggle(row.id)}
    onSelect={() => onSelect?.(row.id)}
    {onPatch}
  />
{/snippet}

{#if autoHeight}
  <!-- Embedded (e.g. detail subtree): grow to fit, let the outer view scroll. -->
  <div class="px-1.5 py-1.5">
    {#each rows as row (row.id)}
      {@render treeRow(row)}
    {/each}
  </div>
{:else}
  <div bind:this={scrollEl} class="h-full overflow-y-auto px-1.5 py-1.5">
    <div style="height: {list.totalSize}px; position: relative;">
      {#each list.items as item (item.index)}
        {@const row = rows[item.index]}
        {#if row}
          <div
            style="position: absolute; top: 0; left: 0; right: 0; height: {item.size}px; transform: translateY({item.start}px);"
          >
            {@render treeRow(row)}
          </div>
        {/if}
      {/each}
    </div>
  </div>
{/if}
