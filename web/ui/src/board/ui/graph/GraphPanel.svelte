<!--
  The Graph view: a kind selector (Stack / Blocking / Subtree — a dynamic
  registry) over the pan/zoom canvas, focused on a task and showing its stack.
-->
<script lang="ts">
  import CrosshairIcon from "@lucide/svelte/icons/crosshair";
  import { shortId, type Task } from "$tasks/model/issue.js";
  import type { TaskFilter } from "$tasks/model/filter.js";
  import { GRAPH_KINDS, graphKind } from "$board/model/graph.js";
  import GraphCanvas from "./GraphCanvas.svelte";

  interface Props {
    tasks: readonly Task[];
    filter: TaskFilter;
    focusId: string | null;
    selectedId?: string | null;
    onSelect: (id: string) => void;
    onFocus: (id: string) => void;
  }
  let { tasks, filter, focusId, selectedId = null, onSelect, onFocus }: Props = $props();

  let kindKey = $state("stack");
  const kind = $derived(graphKind(kindKey));
  const byId = $derived(new Map(tasks.map((t) => [t.id, t] as const)));
  const focusTask = $derived(focusId ? byId.get(focusId) : undefined);
  const graph = $derived(focusTask && focusId ? kind.build(tasks, focusId) : null);
</script>

<div class="flex h-full min-h-0 flex-col">
  <div class="flex items-center gap-2 border-b px-2 py-1.5">
    <div class="flex shrink-0 rounded-md border bg-background p-0.5">
      {#each GRAPH_KINDS as k (k.key)}
        <button
          type="button"
          title={k.hint}
          onclick={() => (kindKey = k.key)}
          class="rounded px-2.5 py-1 text-xs font-medium transition-colors {kindKey === k.key
            ? 'bg-accent text-foreground'
            : 'text-muted-foreground hover:text-foreground'}"
        >
          {k.label}
        </button>
      {/each}
    </div>
    {#if focusTask && focusId}
      <div class="ml-1 min-w-0 flex-1 truncate text-xs text-muted-foreground">
        <span class="font-mono">{shortId(focusId)}</span>
        <span class="opacity-60">·</span>
        {focusTask.title}
      </div>
    {/if}
    {#if selectedId && selectedId !== focusId}
      <!-- Re-root the graph on the node you're viewing (double-click a node works too). -->
      <button
        type="button"
        onclick={() => onFocus(selectedId)}
        title="Center the graph on the selected task"
        class="ml-auto flex shrink-0 items-center gap-1.5 rounded-md border px-2 py-1 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
      >
        <CrosshairIcon class="size-3.5" /> Center here
      </button>
    {/if}
  </div>
  <!-- what the selected kind actually shows, so it's never a mystery -->
  <div class="truncate border-b px-3 py-1 text-[11px] text-muted-foreground">{kind.hint}</div>

  <div class="min-h-0 flex-1">
    {#if graph && focusId}
      <GraphCanvas {graph} {byId} {filter} {focusId} {selectedId} {onSelect} {onFocus} />
    {:else}
      <div class="grid h-full place-items-center p-8 text-center text-sm text-muted-foreground">
        Select a task to see it in the context of its stack.
      </div>
    {/if}
  </div>
</div>
