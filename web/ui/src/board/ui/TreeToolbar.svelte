<script lang="ts">
  import { Input } from "$lib/components/ui/input/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import SearchIcon from "@lucide/svelte/icons/search";
  import ExpandIcon from "@lucide/svelte/icons/chevrons-up-down";
  import CollapseIcon from "@lucide/svelte/icons/chevrons-down-up";
  import FilterMenu from "./FilterMenu.svelte";
  import type { TaskFilter } from "$tasks/model/filter.js";
  import { Copy } from "$shared/copy.js";

  interface Props {
    filter: TaskFilter;
    onExpandAll: () => void;
    onCollapseAll: () => void;
  }
  let { filter, onExpandAll, onCollapseAll }: Props = $props();
</script>

<div class="flex items-center gap-1.5 border-b px-2 py-1.5">
  <div class="relative flex-1">
    <SearchIcon class="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
    <Input
      value={filter.query}
      oninput={(e) => (filter.query = e.currentTarget.value)}
      placeholder={Copy.SearchPlaceholder}
      class="h-8 pl-8"
    />
  </div>
  <FilterMenu {filter} />
  <Button variant="ghost" size="icon" class="size-8" title={Copy.ExpandAll} onclick={onExpandAll}>
    <ExpandIcon class="size-4" />
  </Button>
  <Button variant="ghost" size="icon" class="size-8" title={Copy.CollapseAll} onclick={onCollapseAll}>
    <CollapseIcon class="size-4" />
  </Button>
</div>
