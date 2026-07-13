<!--
  A segmented control for the board view (tree / graph / dashboard). Lives inside
  the filter menu; the standard Tabs list gives the muted-track / raised-active
  look. It writes straight to the persisted view state.
-->
<script lang="ts">
  import type { PersistedState } from "runed";
  import { Tabs, TabsList, TabsTrigger } from "$lib/components/ui/tabs/index.js";
  import ListTreeIcon from "@lucide/svelte/icons/list-tree";
  import NetworkIcon from "@lucide/svelte/icons/network";
  import LayoutDashboardIcon from "@lucide/svelte/icons/layout-dashboard";
  import { BoardView } from "$board/board-view.svelte.js";
  import { Copy } from "$shared/copy.js";

  let { viewState }: { viewState: PersistedState<BoardView> } = $props();

  const VIEWS = [
    { value: BoardView.Tree, label: Copy.ViewTree, icon: ListTreeIcon },
    { value: BoardView.Graph, label: Copy.ViewGraph, icon: NetworkIcon },
    { value: BoardView.Dashboard, label: Copy.ViewDashboard, icon: LayoutDashboardIcon },
  ];
</script>

<Tabs
  value={viewState.current}
  onValueChange={(v) => {
    if (v) viewState.current = v as BoardView;
  }}
  class="w-full"
>
  <!-- Recessed track + a raised, shadowed active pill. The vendored Tabs styles
       active via `data-active:`, but bits-ui emits `data-state="active"` — so we
       target that here, and pick surfaces that contrast in both themes (the dark
       popover has muted == popover, which would otherwise hide the track). -->
  <TabsList class="w-full border bg-background">
    {#each VIEWS as v (v.value)}
      {@const Icon = v.icon}
      <TabsTrigger
        value={v.value}
        data-testid="view-{v.value}"
        class="text-xs text-muted-foreground data-[state=active]:bg-accent data-[state=active]:text-foreground data-[state=active]:shadow-sm"
      >
        <Icon />
        {v.label}
      </TabsTrigger>
    {/each}
  </TabsList>
</Tabs>
