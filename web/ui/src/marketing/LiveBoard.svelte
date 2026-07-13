<!--
  A living demo of the board that runs itself: agents steadily claim ready work,
  push it in-progress, and close it — a new task slides in to take its place.
  Reuses the REAL StatusDot / TypeBadge / PriorityTag so it looks exactly like
  the product, not a mockup.
-->
<script lang="ts">
  import { onMount } from "svelte";
  import { fade } from "svelte/transition";
  import { Status, IssueType } from "$tasks/model/issue.js";
  import StatusDot from "$tasks/ui/StatusDot.svelte";
  import TypeBadge from "$tasks/ui/TypeBadge.svelte";
  import PriorityTag from "$tasks/ui/PriorityTag.svelte";

  interface Row {
    key: number;
    id: string;
    title: string;
    type: IssueType;
    priority: number;
    status: Status;
    agent?: string;
  }

  const AGENTS = ["claude-code", "claude-web", "agent-2", "sonnet-4", "opus-planner"];
  const SEEDS: Array<[string, IssueType, number]> = [
    ["render the dependency graph", IssueType.Feature, 1],
    ["atomic claim race guard", IssueType.Task, 0],
    ["stream MCP tool results", IssueType.Feature, 1],
    ["fix flaky cache test", IssueType.Bug, 0],
    ["markdown task-id linkify", IssueType.Task, 2],
    ["export → jsonl backup", IssueType.Chore, 3],
    ["ready-queue priority sort", IssueType.Task, 1],
    ["per-workspace prefixes", IssueType.Feature, 2],
  ];

  let counter = 0;
  let agentIdx = 0;
  const shortId = () => Math.random().toString(36).slice(2, 6);

  // Recycle a row IN PLACE into a fresh ready task. Keeping the list a fixed
  // length (never splicing/pushing) means its height never changes — so the
  // page below it doesn't jump as tasks flow through.
  function recycle(row: Row): void {
    const [title, type, priority] = SEEDS[counter % SEEDS.length];
    counter++;
    row.id = shortId();
    row.title = title;
    row.type = type;
    row.priority = priority;
    row.status = Status.Open;
    row.agent = undefined;
  }

  let rows = $state<Row[]>(
    Array.from({ length: 5 }, (_, i) => {
      const [title, type, priority] = SEEDS[i % SEEDS.length];
      return { key: i, id: shortId(), title, type, priority, status: Status.Open };
    }),
  );

  function tick(): void {
    // Snapshot the pipeline BEFORE mutating so each row advances one stage.
    const closed = rows.find((r) => r.status === Status.Closed);
    const inProgress = rows.find((r) => r.status === Status.InProgress);
    const ready = rows.find((r) => r.status === Status.Open);

    if (closed) recycle(closed); // a finished task frees up as fresh ready work
    if (inProgress) inProgress.status = Status.Closed; // finish the in-flight one
    if (ready && ready !== closed) {
      ready.status = Status.InProgress; // an agent claims the next ready task
      ready.agent = AGENTS[agentIdx++ % AGENTS.length];
    }
  }

  onMount(() => {
    const t = setInterval(tick, 1700);
    return () => clearInterval(t);
  });
</script>

<div class="w-full overflow-hidden rounded-xl border bg-card shadow-2xl shadow-black/40">
  <!-- window chrome -->
  <div class="flex items-center gap-2 border-b px-3 py-2">
    <span class="size-2.5 rounded-full bg-[#f7768e]/70"></span>
    <span class="size-2.5 rounded-full bg-[#e0af68]/70"></span>
    <span class="size-2.5 rounded-full bg-[#9ece6a]/70"></span>
    <span class="ml-2 font-mono text-[11px] text-muted-foreground">agenttasks · ready queue</span>
    <span class="ml-auto flex items-center gap-1.5 text-[11px] text-muted-foreground">
      <span class="size-1.5 animate-pulse rounded-full bg-[#9ece6a]"></span> live
    </span>
  </div>

  <div class="divide-y">
    {#each rows as row (row.key)}
      <div class="flex h-[42px] items-center gap-2.5 px-3">
        <StatusDot status={row.status} size="md" />
        <TypeBadge type={row.type} />
        <PriorityTag priority={row.priority} />
        <code class="shrink-0 font-mono text-[11px] text-muted-foreground">{row.id}</code>
        <!-- keyed on id so a recycled task's title cross-fades instead of snapping -->
        {#key row.id}
          <span
            in:fade={{ duration: 300 }}
            class="min-w-0 flex-1 truncate text-sm transition-opacity duration-300"
            class:line-through={row.status === Status.Closed}
            class:opacity-40={row.status === Status.Closed}
          >
            {row.title}
          </span>
        {/key}
        {#if row.agent}
          <span
            in:fade={{ duration: 250 }}
            out:fade={{ duration: 200 }}
            class="shrink-0 rounded-full border border-primary/30 bg-primary/10 px-2 py-0.5 font-mono text-[10px] text-primary"
          >
            {row.agent}
          </span>
        {/if}
      </div>
    {/each}
  </div>
</div>
