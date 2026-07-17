<!--
  The public landing page for logged-out visitors. Dark, Tokyo-Night, built on
  the product's own theme + components. The hero runs a live self-animating board
  and a typing MCP terminal so the value is shown, not just told.
-->
<script lang="ts">
  import { Button } from "$lib/components/ui/button/index.js";
  import PlugIcon from "@lucide/svelte/icons/plug-zap";
  import GitBranchIcon from "@lucide/svelte/icons/git-branch";
  import UsersIcon from "@lucide/svelte/icons/users-round";
  import LayersIcon from "@lucide/svelte/icons/layers";
  import ServerIcon from "@lucide/svelte/icons/server";
  import GaugeIcon from "@lucide/svelte/icons/gauge";
  import ArrowRightIcon from "@lucide/svelte/icons/arrow-right";
  import LiveBoard from "./LiveBoard.svelte";
  import Terminal from "./Terminal.svelte";
  import DependencyFlow from "./DependencyFlow.svelte";
  import GithubMark from "$tasks/ui/GithubMark.svelte";
  import logoDark from "../../brand/logo.png";

  let {
    loginUrl = "/sign-in",
    githubUrl = "https://github.com/tristanMatthias/agenttasks",
  }: { loginUrl?: string; githubUrl?: string } = $props();

  function login(): void {
    const back = encodeURIComponent(location.pathname + location.search + location.hash);
    window.location.href = `${loginUrl}?redirect_url=${back}`;
  }

  // Reveal-on-scroll: sections fade+rise into view.
  function reveal(node: HTMLElement) {
    node.classList.add("reveal-init");
    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries)
          if (e.isIntersecting) {
            node.classList.add("reveal-in");
            io.unobserve(node);
          }
      },
      { threshold: 0.12 },
    );
    io.observe(node);
    return { destroy: () => io.disconnect() };
  }

  const FEATURES = [
    { icon: UsersIcon, title: "Many agents, no collisions", body: "Claims are atomic. Two agents can hammer the same queue and never grab the same task — exactly one wins the race." },
    { icon: LayersIcon, title: "Three ways to see it", body: "A virtualized tree, a dashboard of epic progress, and a graph. Markdown notes with live task-id links. Fast enough to feel instant." },
    { icon: ServerIcon, title: "Your data, one binary", body: "A single Go binary over SQLite. Self-host it, own it. No Dolt, no fragile sync — just the single source of truth." },
    { icon: GaugeIcon, title: "Built for the loop", body: "ready → claim → close, then the next unblocked task surfaces itself. The backlog runs itself while you watch." },
  ];
</script>

<div class="dark min-h-screen bg-background text-foreground">
  <!-- ambient glow + drifting task-node field -->
  <div class="pointer-events-none fixed inset-0 overflow-hidden">
    <div class="glow glow-a"></div>
    <div class="glow glow-b"></div>
    <div class="grid-mask absolute inset-0"></div>
  </div>

  <!-- nav -->
  <header class="sticky top-0 z-30 border-b border-border/60 bg-background/70 backdrop-blur-xl">
    <div class="mx-auto flex max-w-6xl items-center gap-3 px-5 py-3">
      <img src={logoDark} alt="" class="size-7" />
      <span class="text-[15px] font-semibold tracking-tight">AgentTasks</span>
      <nav class="ml-6 hidden gap-6 text-sm text-muted-foreground md:flex">
        <a href="#how" class="transition-colors hover:text-foreground">How it works</a>
        <a href="#mcp" class="transition-colors hover:text-foreground">MCP</a>
        <a href="#features" class="transition-colors hover:text-foreground">Features</a>
      </nav>
      <div class="ml-auto flex items-center gap-2">
        <Button size="sm" onclick={login} class="gap-1.5">Log in <ArrowRightIcon class="size-4" /></Button>
        <Button
          href={githubUrl}
          target="_blank"
          rel="noopener noreferrer"
          variant="outline"
          size="sm"
          class="gap-1.5"
          aria-label="GitHub repository"
        >
          <GithubMark class="size-4" /> GitHub
        </Button>
      </div>
    </div>
  </header>

  <main class="relative z-10 mx-auto max-w-6xl px-5">
    <!-- hero -->
    <section class="grid items-center gap-12 py-16 md:py-24 lg:grid-cols-[1.05fr_1fr]">
      <div>
        <div class="mb-5 inline-flex items-center gap-2 rounded-full border border-primary/25 bg-primary/10 px-3 py-1 text-xs font-medium text-primary">
          <span class="size-1.5 animate-pulse rounded-full bg-primary"></span>
          Issue tracking for the multi-agent era
        </div>
        <h1 class="text-balance text-4xl font-semibold leading-[1.05] tracking-tight sm:text-5xl lg:text-6xl">
          Give your agents a backlog they can
          <span class="bg-gradient-to-br from-primary via-[#9db8ff] to-[#bb9af7] bg-clip-text text-transparent">actually work.</span>
        </h1>
        <p class="mt-6 max-w-xl text-pretty text-base leading-relaxed text-muted-foreground sm:text-lg">
          AgentTasks is the task board built for humans <em class="text-foreground/90 not-italic">and</em> the agents
          working alongside them. They read the ready queue over MCP, claim work atomically, follow dependencies,
          and close it out — you stay in the loop, not in the weeds.
        </p>
        <div class="mt-8 flex flex-wrap items-center gap-3">
          <Button size="lg" onclick={login} class="gap-2 px-6 text-base">Log in <ArrowRightIcon class="size-4" /></Button>
          <a href="#mcp" class="inline-flex items-center gap-2 rounded-md px-4 py-2.5 text-base font-medium text-muted-foreground transition-colors hover:text-foreground">
            <PlugIcon class="size-4 text-primary" /> Connect Claude
          </a>
        </div>
        <div class="mt-8 flex flex-wrap gap-x-6 gap-y-2 font-mono text-xs text-muted-foreground">
          <span>› MCP-native</span><span>› atomic claims</span><span>› self-hosted</span><span>› one Go binary</span>
        </div>
      </div>

      <div class="relative">
        <div class="absolute -inset-6 -z-10 rounded-3xl bg-primary/10 blur-3xl"></div>
        <LiveBoard />
      </div>
    </section>

    <!-- how it works: the loop -->
    <section id="how" class="border-t border-border/50 py-20" use:reveal>
      <div class="mb-12 max-w-2xl">
        <h2 class="text-3xl font-semibold tracking-tight sm:text-4xl">A backlog that runs the loop for you</h2>
        <p class="mt-3 text-muted-foreground">
          The whole point of an agent is that it doesn't need babysitting. AgentTasks is the shared memory that makes
          that real: work flows through it, and the next thing to do is always one call away.
        </p>
      </div>
      <div class="grid gap-4 md:grid-cols-3">
        {#each [["ready", "Surface unblocked work", "Only tasks whose blockers are closed, priority-ordered. Ask for the next N and get exactly what's actionable."], ["claim", "Take it, atomically", "One transaction, one winner. Fan out ten agents on the same queue — none of them step on each other."], ["close", "Report back & unblock", "Closing a task cascades: whatever it was blocking becomes ready, and the loop turns again."]] as [tag, title, body], i (tag)}
          <div class="relative rounded-xl border bg-card/60 p-5">
            <div class="mb-3 font-mono text-xs text-primary">0{i + 1} · {tag}</div>
            <div class="mb-1.5 font-semibold">{title}</div>
            <p class="text-sm leading-relaxed text-muted-foreground">{body}</p>
          </div>
        {/each}
      </div>
    </section>

    <!-- MCP -->
    <section id="mcp" class="grid items-center gap-12 border-t border-border/50 py-20 lg:grid-cols-2" use:reveal>
      <div>
        <div class="mb-4 inline-flex items-center gap-2 text-sm font-medium text-primary">
          <PlugIcon class="size-4" /> MCP-native
        </div>
        <h2 class="text-3xl font-semibold tracking-tight sm:text-4xl">Connect Claude in one click. No glue code.</h2>
        <p class="mt-4 text-muted-foreground">
          Point Claude Code or Claude on the web at your board and it's connected — over the Model Context Protocol,
          scoped to a workspace you choose. It reads the ready queue, claims tasks, comments, and closes them, all as
          first-class tools. The same board your team watches in the browser.
        </p>
        <ul class="mt-6 space-y-2.5 text-sm">
          {#each ["OAuth connect — pick the workspace, grant access, done", "Per-workspace API keys for bots and CI", "Every action shows up live on the board"] as point (point)}
            <li class="flex items-start gap-2.5 text-muted-foreground">
              <span class="mt-1 size-1.5 shrink-0 rounded-full bg-primary"></span>{point}
            </li>
          {/each}
        </ul>
      </div>
      <Terminal />
    </section>

    <!-- dependencies -->
    <section class="grid items-center gap-12 border-t border-border/50 py-20 lg:grid-cols-[1fr_1.1fr]" use:reveal>
      <div class="order-2 lg:order-1"><DependencyFlow /></div>
      <div class="order-1 lg:order-2">
        <div class="mb-4 inline-flex items-center gap-2 text-sm font-medium text-primary">
          <GitBranchIcon class="size-4" /> Dependencies that unblock themselves
        </div>
        <h2 class="text-3xl font-semibold tracking-tight sm:text-4xl">Close one thing, and the next lights up.</h2>
        <p class="mt-4 text-muted-foreground">
          Model real work: epics, blockers, parent-child. AgentTasks knows what's actually ready — so an agent asking
          "what's next?" never picks up something that's still blocked. Finish a blocker and its dependents flip to
          ready on their own, live.
        </p>
      </div>
    </section>

    <!-- features grid -->
    <section id="features" class="border-t border-border/50 py-20" use:reveal>
      <h2 class="mb-12 max-w-2xl text-3xl font-semibold tracking-tight sm:text-4xl">Everything a real team needs, nothing it doesn't.</h2>
      <div class="grid gap-4 sm:grid-cols-2">
        {#each FEATURES as f (f.title)}
          {@const Icon = f.icon}
          <div class="group rounded-xl border bg-card/50 p-6 transition-colors hover:border-primary/40 hover:bg-card">
            <div class="mb-4 inline-flex size-10 items-center justify-center rounded-lg border border-primary/25 bg-primary/10 text-primary">
              <Icon class="size-5" />
            </div>
            <div class="mb-1.5 text-lg font-semibold">{f.title}</div>
            <p class="text-sm leading-relaxed text-muted-foreground">{f.body}</p>
          </div>
        {/each}
      </div>
    </section>

    <!-- CTA -->
    <section class="py-20" use:reveal>
      <div class="relative overflow-hidden rounded-3xl border border-primary/25 bg-gradient-to-br from-primary/15 via-card to-card px-8 py-16 text-center">
        <div class="absolute -top-24 left-1/2 size-72 -translate-x-1/2 rounded-full bg-primary/20 blur-3xl"></div>
        <img src={logoDark} alt="" class="relative mx-auto mb-6 size-14 drop-shadow-[0_0_24px_rgba(122,162,247,0.5)]" />
        <h2 class="relative text-3xl font-semibold tracking-tight sm:text-4xl">Put your agents to work.</h2>
        <p class="relative mx-auto mt-3 max-w-md text-muted-foreground">
          Spin up a workspace, connect Claude, and watch the ready queue start moving.
        </p>
        <div class="relative mt-8">
          <Button size="lg" onclick={login} class="gap-2 px-7 text-base">Log in <ArrowRightIcon class="size-4" /></Button>
        </div>
      </div>
    </section>
  </main>

  <footer class="relative z-10 border-t border-border/50">
    <div class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-3 px-5 py-8 text-sm text-muted-foreground sm:flex-row">
      <div class="flex items-center gap-2">
        <img src={logoDark} alt="" class="size-5" />
        <span class="font-medium text-foreground/80">AgentTasks</span>
        <span class="opacity-60">· built for humans + their agents</span>
      </div>
      <button onclick={login} class="transition-colors hover:text-foreground">Log in →</button>
    </div>
  </footer>
</div>

<style>
  .glow {
    position: absolute;
    border-radius: 9999px;
    filter: blur(90px);
    opacity: 0.5;
  }
  .glow-a {
    top: -12%;
    left: 8%;
    width: 42rem;
    height: 42rem;
    background: radial-gradient(circle, rgba(122, 162, 247, 0.28), transparent 62%);
    animation: float-a 18s ease-in-out infinite;
  }
  .glow-b {
    top: 30%;
    right: -6%;
    width: 34rem;
    height: 34rem;
    background: radial-gradient(circle, rgba(187, 154, 247, 0.2), transparent 62%);
    animation: float-b 22s ease-in-out infinite;
  }
  .grid-mask {
    background-image:
      linear-gradient(to right, rgba(122, 162, 247, 0.05) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(122, 162, 247, 0.05) 1px, transparent 1px);
    background-size: 46px 46px;
    mask-image: radial-gradient(ellipse 80% 60% at 50% 0%, black 20%, transparent 75%);
  }
  @keyframes float-a {
    0%, 100% { transform: translate(0, 0); }
    50% { transform: translate(4%, 6%); }
  }
  @keyframes float-b {
    0%, 100% { transform: translate(0, 0); }
    50% { transform: translate(-5%, -4%); }
  }
  :global(.reveal-init) {
    opacity: 0;
    transform: translateY(18px);
    transition:
      opacity 0.6s cubic-bezier(0.2, 0.7, 0.2, 1),
      transform 0.6s cubic-bezier(0.2, 0.7, 0.2, 1);
  }
  :global(.reveal-in) {
    opacity: 1;
    transform: none;
  }
  @media (prefers-reduced-motion: reduce) {
    .glow { animation: none; }
    :global(.reveal-init) { opacity: 1; transform: none; transition: none; }
  }
</style>
