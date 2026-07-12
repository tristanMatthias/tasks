<!--
  A terminal that types itself out: the CLI surfacing ready work, then Claude
  claiming and closing it over MCP. Loops. Pure CSS/JS, no deps.
-->
<script lang="ts">
  import { onMount } from "svelte";

  interface Line {
    t: string;
    c: string;
  }
  const LINES: Line[] = [
    { t: "$ tasks ready -n 3", c: "text-primary" },
    { t: "● 3 tasks ready to claim", c: "text-[#9ece6a]" },
    { t: "  forge-ps3t   AST as the single source of truth", c: "text-muted-foreground" },
    { t: "  forge-9x2k   fix flaky cache test", c: "text-muted-foreground" },
    { t: "", c: "" },
    { t: "# Claude, connected over MCP", c: "text-[#7dcfff]" },
    { t: "✓ claimed forge-ps3t  →  claude-code", c: "text-[#9ece6a]" },
    { t: "✓ closed  forge-9x2k  →  done in 42s", c: "text-[#9ece6a]" },
  ];

  let done = $state<Line[]>([]);
  let cur = $state<Line>({ t: "", c: "" });
  let typed = $state("");
  let showCursor = $state(true);

  onMount(() => {
    let li = 0;
    let ci = 0;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;

    const step = () => {
      if (stopped) return;
      if (li >= LINES.length) {
        // hold the finished frame, then restart
        timer = setTimeout(() => {
          done = [];
          cur = { t: "", c: "" };
          typed = "";
          li = 0;
          ci = 0;
          step();
        }, 2600);
        return;
      }
      const line = LINES[li];
      cur = line;
      if (ci <= line.t.length) {
        typed = line.t.slice(0, ci);
        ci++;
        timer = setTimeout(step, line.t === "" ? 120 : 22 + Math.random() * 30);
      } else {
        done = [...done, line];
        li++;
        ci = 0;
        typed = "";
        cur = { t: "", c: "" };
        timer = setTimeout(step, line.t.startsWith("$") || line.t.startsWith("#") ? 320 : 160);
      }
    };
    step();
    const blink = setInterval(() => (showCursor = !showCursor), 550);
    return () => {
      stopped = true;
      clearTimeout(timer);
      clearInterval(blink);
    };
  });
</script>

<div class="w-full overflow-hidden rounded-xl border bg-[#0b0d12] shadow-2xl shadow-black/40">
  <div class="flex items-center gap-2 border-b border-white/5 px-3 py-2">
    <span class="size-2.5 rounded-full bg-[#f7768e]/70"></span>
    <span class="size-2.5 rounded-full bg-[#e0af68]/70"></span>
    <span class="size-2.5 rounded-full bg-[#9ece6a]/70"></span>
    <span class="ml-2 font-mono text-[11px] text-muted-foreground">claude ~ agenttasks</span>
  </div>
  <div class="min-h-[188px] p-4 font-mono text-[12.5px] leading-[1.7]">
    {#each done as line, i (i)}
      <div class={line.c || "text-foreground"}>{line.t || " "}</div>
    {/each}
    <div class={cur.c || "text-foreground"}>
      {typed}<span class="inline-block w-[7px] {showCursor ? 'bg-primary' : 'bg-transparent'}">&nbsp;</span>
    </div>
  </div>
</div>
