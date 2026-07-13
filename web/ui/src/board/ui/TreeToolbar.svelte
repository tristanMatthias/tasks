<script lang="ts">
  import { Button } from "$lib/components/ui/button/index.js";
  import SearchInput from "$lib/components/SearchInput.svelte";
  import ExpandIcon from "@lucide/svelte/icons/chevrons-up-down";
  import CollapseIcon from "@lucide/svelte/icons/chevrons-down-up";
  import FilterMenu from "./FilterMenu.svelte";
  import type { TaskFilter } from "$tasks/model/filter.js";
  import type { PersistedState } from "runed";
  import type { BoardView } from "$board/board-view.svelte.js";
  import type { TaskSort } from "$tasks/model/sort.js";
  import { Copy } from "$shared/copy.js";

  interface Props {
    filter: TaskFilter;
    onExpandAll: () => void;
    onCollapseAll: () => void;
    viewState?: PersistedState<BoardView>;
    sort?: PersistedState<TaskSort>;
    /** Show the tree-only expand/collapse controls (hidden for other views). */
    treeControls?: boolean;
  }
  let { filter, onExpandAll, onCollapseAll, viewState, sort, treeControls = true }: Props = $props();
</script>

<div class="flex items-center gap-1.5 border-b px-2 py-1.5">
  <SearchInput bind:value={filter.query} placeholder={Copy.SearchPlaceholder} class="flex-1" />
  {#if treeControls}
    <Button variant="ghost" size="icon" class="size-8" title={Copy.ExpandAll} onclick={onExpandAll}>
      <ExpandIcon class="size-4" />
    </Button>
    <Button variant="ghost" size="icon" class="size-8" title={Copy.CollapseAll} onclick={onCollapseAll}>
      <CollapseIcon class="size-4" />
    </Button>
  {/if}
  <!-- Filter is pinned last so it stays put on the right edge across views. -->
  <FilterMenu {filter} {viewState} {sort} />
</div>
