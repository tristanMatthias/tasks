<!--
  A tiny self-running dependency graph: blockers close, and the tasks they were
  blocking flip from "blocked" to "ready" on their own — the ready-queue idea,
  visualized. Pure SVG + a frame timer.
-->
<script lang="ts">
  import { onMount } from "svelte";

  type State = "blocked" | "ready" | "progress" | "done";
  const NODES = [
    { id: "a", x: 58, y: 52, label: "auth" },
    { id: "b", x: 58, y: 150, label: "schema" },
    { id: "c", x: 176, y: 101, label: "api" },
    { id: "d", x: 274, y: 101, label: "ship" },
  ] as const;
  const EDGES: Array<[string, string]> = [
    ["a", "c"],
    ["b", "c"],
    ["c", "d"],
  ];

  // Scripted frames — the loop of getting real work unblocked.
  const FRAMES: Record<string, State>[] = [
    { a: "ready", b: "ready", c: "blocked", d: "blocked" },
    { a: "progress", b: "ready", c: "blocked", d: "blocked" },
    { a: "done", b: "progress", c: "blocked", d: "blocked" },
    { a: "done", b: "done", c: "ready", d: "blocked" },
    { a: "done", b: "done", c: "progress", d: "blocked" },
    { a: "done", b: "done", c: "done", d: "ready" },
    { a: "done", b: "done", c: "done", d: "progress" },
    { a: "done", b: "done", c: "done", d: "done" },
  ];

  let frame = $state(0);
  const state = $derived(FRAMES[frame]);

  const COLOR: Record<State, string> = {
    blocked: "#565f73",
    ready: "#9ece6a",
    progress: "#7dcfff",
    done: "#9ece6a",
  };
  const pos = (id: string) => NODES.find((n) => n.id === id)!;

  onMount(() => {
    const t = setInterval(() => (frame = (frame + 1) % FRAMES.length), 1150);
    return () => clearInterval(t);
  });
</script>

<div class="relative rounded-xl border bg-card/60 p-4">
  <svg viewBox="0 0 330 200" class="w-full">
    <!-- edges -->
    {#each EDGES as [from, to] (from + to)}
      {@const a = pos(from)}
      {@const b = pos(to)}
      {@const active = state[from] === "done"}
      <line
        x1={a.x} y1={a.y} x2={b.x} y2={b.y}
        stroke={active ? "#9ece6a" : "#2a2f3d"}
        stroke-width="2"
        stroke-dasharray="4 4"
        class="transition-all duration-500"
        opacity={active ? "0.9" : "0.5"}
      />
    {/each}

    <!-- nodes -->
    {#each NODES as n (n.id)}
      {@const s = state[n.id]}
      {@const c = COLOR[s]}
      <g class="transition-all duration-500">
        {#if s === "ready"}
          <circle cx={n.x} cy={n.y} r="20" fill="none" stroke={c} stroke-width="2" opacity="0.5">
            <animate attributeName="r" values="16;24;16" dur="1.6s" repeatCount="indefinite" />
            <animate attributeName="opacity" values="0.6;0;0.6" dur="1.6s" repeatCount="indefinite" />
          </circle>
        {/if}
        <circle
          cx={n.x} cy={n.y} r="16"
          fill={s === "done" ? c : s === "blocked" ? "#161922" : `${c}22`}
          stroke={c}
          stroke-width="2"
          stroke-dasharray={s === "blocked" ? "3 3" : "0"}
          class="transition-all duration-500"
        />
        {#if s === "done"}
          <path
            d="M {n.x - 5} {n.y} l 3.5 3.5 l 6.5 -7"
            fill="none" stroke="#0f1115" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"
          />
        {/if}
        <text x={n.x} y={n.y + 32} text-anchor="middle" class="fill-muted-foreground font-mono" font-size="10">
          {n.label}
        </text>
      </g>
    {/each}
  </svg>

  <div class="mt-1 flex items-center justify-center gap-4 font-mono text-[10px] text-muted-foreground">
    <span class="inline-flex items-center gap-1.5"><span class="size-2 rounded-full border border-dashed" style="border-color:#565f73"></span>blocked</span>
    <span class="inline-flex items-center gap-1.5"><span class="size-2 rounded-full" style="background:#9ece6a"></span>ready</span>
    <span class="inline-flex items-center gap-1.5"><span class="size-2 rounded-full" style="background:#7dcfff"></span>in progress</span>
  </div>
</div>
