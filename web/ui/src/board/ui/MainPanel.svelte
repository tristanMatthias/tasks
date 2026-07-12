<!--
  The board's main (left) pane: one persistent toolbar over a body that swaps
  between the tree, dashboard, and graph views. The toolbar — and therefore the
  single FilterMenu that hosts the view switcher — stays mounted across view
  changes, so switching never re-creates or orphans the popover. The detail pane
  next to this one is untouched, so selecting anything just updates it.
-->
<script lang="ts">
  import type { Task } from "$tasks/model/issue.js";
  import type { TaskFilter } from "$tasks/model/filter.js";
  import type { TaskSort } from "$tasks/model/sort.js";
  import type { PersistedState } from "runed";
  import { BoardView } from "$board/board-view.svelte.js";
  import TreeToolbar from "./TreeToolbar.svelte";
  import Tree from "./Tree.svelte";
  import DashboardView from "./DashboardView.svelte";
  import NetworkIcon from "@lucide/svelte/icons/network";
  import { Copy } from "$shared/copy.js";

  interface Props {
    tasks: readonly Task[];
    filter: TaskFilter;
    view: PersistedState<BoardView>;
    sort: PersistedState<TaskSort>;
    selectedId?: string | null;
    onSelect: (id: string) => void;
    onPatch: (id: string, patch: Partial<Task>) => void;
  }
  let { tasks, filter, view, sort, selectedId = null, onSelect, onPatch }: Props = $props();

  let tree = $state<{ expandAll(): void; collapseAll(): void }>();
  const isTree = $derived(view.current === BoardView.Tree);
</script>

<div class="flex h-full min-h-0 flex-col">
  <TreeToolbar
    {filter}
    viewState={view}
    {sort}
    treeControls={isTree}
    onExpandAll={() => tree?.expandAll()}
    onCollapseAll={() => tree?.collapseAll()}
  />
  <div class="min-h-0 flex-1">
    {#if isTree}
      <Tree bind:this={tree} {tasks} {filter} sort={sort.current} {selectedId} {onSelect} {onPatch} />
    {:else if view.current === BoardView.Dashboard}
      <DashboardView {tasks} query={filter.query} sort={sort.current} {onSelect} />
    {:else}
      <div class="flex h-full flex-col items-center justify-center gap-2 p-8 text-muted-foreground">
        <NetworkIcon class="size-8" />
        <p class="text-sm">{Copy.GraphComingSoon}</p>
      </div>
    {/if}
  </div>
</div>
