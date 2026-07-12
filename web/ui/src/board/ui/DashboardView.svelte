<!--
  The dashboard view: one card per epic with a stacked progress bar summarizing
  its whole subtree by status, the completion percentage, and a per-status
  breakdown. Clicking a card opens that epic's detail.
-->
<script lang="ts">
  import type { Task } from "$tasks/model/issue.js";
  import { IssueType, Status } from "$tasks/model/issue.js";
  import { STATUS_COLOR_VAR, STATUS_LABEL } from "$tasks/model/appearance.js";
  import { epicSummaries, type EpicSummary } from "$board/model/dashboard.js";
  import { DEFAULT_SORT, SortField, makeTaskComparator, type TaskSort } from "$tasks/model/sort.js";
  import TypeBadge from "$tasks/ui/TypeBadge.svelte";
  import StatusDot from "$tasks/ui/StatusDot.svelte";
  import { Copy } from "$shared/copy.js";

  interface Props {
    tasks: readonly Task[];
    /** Search text from the board filter — narrows epics by id/title. */
    query?: string;
    /** Epic order; the default keeps the "needs-attention-first" ordering. */
    sort?: TaskSort;
    onSelect: (id: string) => void;
  }
  let { tasks, query = "", sort = DEFAULT_SORT, onSelect }: Props = $props();

  // Left→right fill order: done first, then work-in-flight, then not-started.
  const SEGMENT_ORDER: Status[] = [Status.Closed, Status.InProgress, Status.Open, Status.Deferred];

  const summaries = $derived(epicSummaries(tasks));
  const filtered = $derived.by(() => {
    const q = query.trim().toLowerCase();
    if (!q) return summaries;
    return summaries.filter((s) => `${s.epic.id} ${s.epic.title ?? ""}`.toLowerCase().includes(q));
  });
  // Default keeps the model's attention-first order; any explicit sort reorders
  // the epics by that field/direction.
  const visible = $derived.by(() => {
    if (sort.field === SortField.Default) return filtered;
    const cmp = makeTaskComparator(sort);
    return [...filtered].sort((a, b) => cmp(a.epic, b.epic));
  });

  /** Non-empty, proportionally-sized segments for the progress bar. */
  function segments(s: EpicSummary) {
    return SEGMENT_ORDER.filter((status) => s.counts[status] > 0).map((status) => ({
      status,
      count: s.counts[status],
      pct: (s.counts[status] / s.total) * 100,
      color: STATUS_COLOR_VAR[status],
    }));
  }
</script>

<div class="h-full overflow-y-auto">
  {#if visible.length === 0}
    <div class="flex h-full items-center justify-center p-8 text-sm text-muted-foreground">
      {Copy.NoEpics}
    </div>
  {:else}
    <div class="flex flex-col gap-3 p-3">
      {#each visible as s (s.epic.id)}
        <button
          type="button"
          onclick={() => onSelect(s.epic.id)}
          class="group flex w-full flex-col gap-2.5 rounded-lg border bg-card p-3.5 text-left transition-colors hover:bg-accent/40"
        >
          <div class="flex items-center gap-2">
            <TypeBadge type={IssueType.Epic} />
            <span class="min-w-0 flex-1 truncate font-medium">{s.epic.title || Copy.UntitledTask}</span>
            <span class="shrink-0 text-sm font-semibold tabular-nums">{s.percent}%</span>
          </div>

          <div class="flex h-2 w-full overflow-hidden rounded-full bg-muted">
            {#each segments(s) as seg (seg.status)}
              <div
                class="h-full"
                style="width:{seg.pct}%; background-color:{seg.color}"
                title="{STATUS_LABEL[seg.status]}: {seg.count}"
              ></div>
            {/each}
          </div>

          <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            {#if s.total === 0}
              <span>{Copy.NoChildTasks}</span>
            {:else}
              <span class="font-medium text-foreground">{s.done}/{s.total} {Copy.Done}</span>
              {#each SEGMENT_ORDER as status (status)}
                {#if s.counts[status] > 0}
                  <span class="inline-flex items-center gap-1.5">
                    <StatusDot {status} />
                    <span class="tabular-nums">{s.counts[status]}</span>
                    <span>{STATUS_LABEL[status]}</span>
                  </span>
                {/if}
              {/each}
            {/if}
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>
