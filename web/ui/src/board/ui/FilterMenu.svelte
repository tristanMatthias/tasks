<script lang="ts">
  import ResponsiveMenu from "$lib/components/ResponsiveMenu.svelte";
  import { Checkbox } from "$lib/components/ui/checkbox/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Badge } from "$lib/components/ui/badge/index.js";
  import SlidersIcon from "@lucide/svelte/icons/sliders-horizontal";
  import {
    ALL_STATUSES,
    ALL_TYPES,
    toggleValue,
    type TaskFilter,
  } from "$tasks/model/filter.js";
  import { STATUS_LABEL } from "$tasks/model/appearance.js";
  import StatusDot from "$tasks/ui/StatusDot.svelte";
  import TypeBadge from "$tasks/ui/TypeBadge.svelte";
  import type { IssueType, Status } from "$tasks/model/issue.js";
  import type { PersistedState } from "runed";
  import type { BoardView } from "$board/board-view.svelte.js";
  import type { TaskSort } from "$tasks/model/sort.js";
  import ViewSwitcher from "./ViewSwitcher.svelte";
  import SortControl from "./SortControl.svelte";
  import { Copy } from "$shared/copy.js";

  // `viewState`/`sort` are optional: the main board passes them to surface the
  // view switcher and ordering control; the detail page's embedded subtree omits
  // them (no switcher/sort there).
  let {
    filter,
    viewState,
    sort,
  }: {
    filter: TaskFilter;
    viewState?: PersistedState<BoardView>;
    sort?: PersistedState<TaskSort>;
  } = $props();

  // How many facet options are currently excluded (shown as a badge).
  const hiddenCount = $derived(
    ALL_STATUSES.length - filter.statuses.length + (ALL_TYPES.length - filter.types.length),
  );

  const toggleStatus = (status: Status) => (filter.statuses = toggleValue(filter.statuses, status));
  const toggleType = (type: IssueType) => (filter.types = toggleValue(filter.types, type));
</script>

<ResponsiveMenu title={viewState ? Copy.ViewAndFilters : Copy.Filters} align="end" class="w-72" overlay={false}>
  {#snippet trigger({ props })}
    <Button {...props} variant="ghost" size="sm" class="gap-1.5">
      <SlidersIcon class="size-4" />
      {Copy.Filters}
      {#if hiddenCount > 0}
        <Badge variant="secondary" class="h-4 px-1 text-[10px] tabular-nums">{hiddenCount}</Badge>
      {/if}
    </Button>
  {/snippet}

  {#if viewState}
    <div class="mb-1">
      <div class="mb-2 text-xs font-medium text-muted-foreground">{Copy.View}</div>
      <ViewSwitcher {viewState} />
    </div>
  {/if}

  <div class="grid grid-cols-2 gap-x-4 gap-y-2">
    <fieldset>
      <legend class="text-xs font-medium text-muted-foreground">{Copy.FilterStatus}</legend>
      <div class="mt-2 flex flex-col gap-2">
        {#each ALL_STATUSES as status (status)}
          <label class="flex cursor-pointer items-center gap-2 text-sm">
            <Checkbox checked={filter.statuses.includes(status)} onCheckedChange={() => toggleStatus(status)} />
            <StatusDot {status} />
            {STATUS_LABEL[status]}
          </label>
        {/each}
      </div>
    </fieldset>

    <fieldset>
      <legend class="text-xs font-medium text-muted-foreground">{Copy.FilterType}</legend>
      <div class="mt-2 flex flex-col gap-2">
        {#each ALL_TYPES as type (type)}
          <label class="flex cursor-pointer items-center gap-2 text-sm">
            <Checkbox checked={filter.types.includes(type)} onCheckedChange={() => toggleType(type)} />
            <TypeBadge {type} />
          </label>
        {/each}
      </div>
    </fieldset>
  </div>

  {#if sort}
    <div class="mt-3 border-t pt-3">
      <SortControl {sort} />
    </div>
  {/if}
</ResponsiveMenu>
