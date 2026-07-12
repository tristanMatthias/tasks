<!--
  Linear-style ordering control: a set of field chips plus an asc/desc toggle.
  Writes straight to the persisted sort. Lives inside the filter menu.
-->
<script lang="ts">
  import type { PersistedState } from "runed";
  import { Button } from "$lib/components/ui/button/index.js";
  import ArrowUpNarrowWide from "@lucide/svelte/icons/arrow-up-narrow-wide";
  import ArrowDownWideNarrow from "@lucide/svelte/icons/arrow-down-wide-narrow";
  import { SortField, SORT_FIELDS, type TaskSort } from "$tasks/model/sort.js";
  import { Copy } from "$shared/copy.js";

  let { sort }: { sort: PersistedState<TaskSort> } = $props();

  const LABEL: Record<SortField, string> = {
    [SortField.Default]: Copy.SortDefault,
    [SortField.Priority]: Copy.SortPriority,
    [SortField.Status]: Copy.SortStatus,
    [SortField.Type]: Copy.SortType,
    [SortField.Title]: Copy.SortTitle,
    [SortField.Id]: Copy.SortId,
  };

  const setField = (field: SortField) => (sort.current = { ...sort.current, field });
  const toggleDir = () =>
    (sort.current = { ...sort.current, dir: sort.current.dir === "asc" ? "desc" : "asc" });
</script>

<div class="flex items-center justify-between">
  <div class="text-xs font-medium text-muted-foreground">{Copy.Sort}</div>
  <Button
    variant="ghost"
    size="icon"
    class="size-6"
    title={sort.current.dir === "asc" ? Copy.SortAscending : Copy.SortDescending}
    onclick={toggleDir}
  >
    {#if sort.current.dir === "asc"}
      <ArrowUpNarrowWide class="size-3.5" />
    {:else}
      <ArrowDownWideNarrow class="size-3.5" />
    {/if}
  </Button>
</div>
<div class="mt-1.5 flex flex-wrap gap-1">
  {#each SORT_FIELDS as field (field)}
    <button
      type="button"
      onclick={() => setField(field)}
      class="rounded-md border px-2 py-0.5 text-xs transition-colors {sort.current.field === field
        ? 'border-input bg-accent text-foreground'
        : 'border-transparent text-muted-foreground hover:bg-accent/50'}"
    >
      {LABEL[field]}
    </button>
  {/each}
</div>
