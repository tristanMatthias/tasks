<script>
  import { onMount } from "svelte";
  import { createBoard } from "./lib/board.js";
  import LoginOverlay from "./lib/LoginOverlay.svelte";
  import KeysModal from "./lib/KeysModal.svelte";

  const STATUS_OPTS = ["open", "in_progress", "deferred", "closed"];
  const TYPE_OPTS = ["epic", "feature", "task", "bug", "chore"];
  const VIEWS = [
    ["tree", "tree", "Hierarchy / parent-child tree"],
    ["graph", "graph", "Dependency graph by 'blocks' edges"],
    ["dashboard", "dashboard", "Epic progress dashboard"],
  ];

  let treeEl, detailEl;
  let board;

  let status = $state("");
  let query = $state("");
  let view = $state("tree");
  let statuses = $state(new Set(["open", "in_progress"]));
  let types = $state(new Set(TYPE_OPTS));
  let showDetail = $state(false);
  let filtersOpen = $state(false);
  let theme = $state("tokyonight");
  let loginOpen = $state(false);
  let keysOpen = $state(false);

  onMount(() => {
    theme = localStorage.getItem("tasks-theme") || "tokyonight";
    document.documentElement.dataset.theme = theme;

    board = createBoard({
      treeEl,
      detailEl,
      onStatus: (t) => (status = t),
      onAuthRequired: () => (loginOpen = true),
      onDetailVisible: (v) => (showDetail = v),
    });
    const s = board.getState();
    query = s.query;
    statuses = new Set(s.statuses);
    types = new Set(s.types);
    view = s.view;
    showDetail = s.showDetail;
    board.reload();
    return () => board?.destroy();
  });

  const onSearch = (e) => { query = e.target.value; board.setQuery(query); };
  const clearSearch = () => { query = ""; board.setQuery(""); };
  function toggleStatus(s) {
    const on = !statuses.has(s);
    const n = new Set(statuses); on ? n.add(s) : n.delete(s); statuses = n;
    board.setStatus(s, on);
  }
  function toggleType(t) {
    const on = !types.has(t);
    const n = new Set(types); on ? n.add(t) : n.delete(t); types = n;
    board.setType(t, on);
  }
  const setView = (v) => { view = v; board.setView(v); };
  function cycleTheme() {
    theme = theme === "tokyonight" ? "daylight" : "tokyonight";
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("tasks-theme", theme);
  }
</script>

<header class="flex flex-col gap-1.5 border-b border-base-300 bg-base-200 px-3 py-2">
  <div class="flex flex-wrap items-center gap-2.5">
    <div class="text-xs font-bold uppercase tracking-wider text-secondary">tasks</div>

    <label class="input input-bordered input-sm flex max-w-[480px] flex-1 items-center gap-2">
      <input class="grow" type="search" placeholder="filter…" autocomplete="off" value={query} oninput={onSearch} />
      {#if query}
        <button class="text-base-content/50 hover:text-base-content" onclick={clearSearch} aria-label="Clear" title="Clear">✕</button>
      {/if}
    </label>

    <div class="join" role="tablist">
      {#each VIEWS as [id, label, title]}
        <button
          class="btn join-item btn-sm {view === id ? 'btn-primary' : 'btn-ghost'}"
          title={title}
          onclick={() => setView(id)}
        >{label}</button>
      {/each}
    </div>

    <div class="ml-auto flex items-center gap-1">
      <button class="btn btn-ghost btn-sm btn-square md:hidden" title="Filters" onclick={() => (filtersOpen = !filtersOpen)} aria-expanded={filtersOpen}>☰</button>
      <button class="btn btn-ghost btn-sm btn-square" title="Expand all" onclick={() => board.expandAll()}>⊕</button>
      <button class="btn btn-ghost btn-sm btn-square" title="Collapse all" onclick={() => board.collapseAll()}>⊖</button>
      <button class="btn btn-ghost btn-sm btn-square" title="Reload" onclick={() => board.reload({ pull: true })}>⟳</button>
      <button class="btn btn-ghost btn-sm btn-square" title="Theme" onclick={cycleTheme}>{theme === "tokyonight" ? "☾" : "☀"}</button>
      <button class="btn btn-ghost btn-sm btn-square" title="API keys" onclick={() => (keysOpen = true)}>⚙</button>
    </div>
  </div>

  <div class="flex-wrap items-center gap-x-4 gap-y-1.5 text-xs {filtersOpen ? 'flex' : 'hidden'} md:flex">
    <div class="flex flex-wrap gap-x-3 gap-y-1">
      {#each STATUS_OPTS as s}
        <label class="flex cursor-pointer items-center gap-1.5 text-base-content/70">
          <input type="checkbox" class="checkbox checkbox-xs checkbox-primary" checked={statuses.has(s)} onchange={() => toggleStatus(s)} />
          {s}
        </label>
      {/each}
    </div>
    <div class="flex flex-wrap gap-x-3 gap-y-1">
      {#each TYPE_OPTS as t}
        <label class="flex cursor-pointer items-center gap-1.5 text-base-content/70">
          <input type="checkbox" class="checkbox checkbox-xs checkbox-primary" checked={types.has(t)} onchange={() => toggleType(t)} />
          {t}
        </label>
      {/each}
    </div>
    <span class="text-[11px] text-base-content/40">{status}</span>
  </div>
</header>

<main class:show-detail={showDetail}>
  <aside class="tree" bind:this={treeEl}></aside>
  <section class="detail">
    {#if showDetail}
      <button class="btn btn-ghost btn-sm mb-2 md:hidden" onclick={() => board.back()}>← back</button>
    {/if}
    <div bind:this={detailEl}><div class="empty">Select an issue to see details.</div></div>
  </section>
</main>

{#if loginOpen}
  <LoginOverlay onsuccess={() => { loginOpen = false; board.reload(); }} />
{/if}
{#if keysOpen}
  <KeysModal onclose={() => (keysOpen = false)} onauth={() => { keysOpen = false; loginOpen = true; }} />
{/if}
