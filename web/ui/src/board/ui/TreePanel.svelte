<!--
  A tree with its action bar (search / filter / expand-collapse) above it. Used
  for both the main board and the detail page's children subtree — pass `rootId`
  + `autoHeight` for the embedded case.
-->
<script lang="ts">
  import type { Task } from "$tasks/model/issue.js";
  import type { TaskFilter } from "$tasks/model/filter.js";
  import TreeToolbar from "./TreeToolbar.svelte";
  import Tree from "./Tree.svelte";

  interface Props {
    tasks: readonly Task[];
    filter: TaskFilter;
    selectedId?: string | null;
    onSelect?: (id: string) => void;
    onPatch: (id: string, patch: Partial<Task>) => void;
    rootId?: string | null;
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

  let tree = $state<{ expandAll(): void; collapseAll(): void }>();
</script>

<div class={autoHeight ? "flex flex-col" : "flex h-full min-h-0 flex-col"}>
  <TreeToolbar
    {filter}
    onExpandAll={() => tree?.expandAll()}
    onCollapseAll={() => tree?.collapseAll()}
  />
  <div class={autoHeight ? "" : "min-h-0 flex-1"}>
    <Tree bind:this={tree} {tasks} {filter} {selectedId} {onSelect} {onPatch} {rootId} {autoHeight} />
  </div>
</div>
