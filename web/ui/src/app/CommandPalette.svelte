<!--
  The global ⌘K / Ctrl+K command palette: fuzzy-search every task and jump to it,
  switch views, or hop to settings. Built from the standard Command primitives
  (cmdk/bits-ui) so it matches the design system.

  Performance: boards can hold thousands of tasks, so we DON'T hand cmdk every
  task and let it re-filter on each keystroke (that renders thousands of DOM
  rows and lags). Instead we filter ourselves and render only the top matches,
  with cmdk's own filtering turned off (`shouldFilter={false}`).
-->
<script lang="ts">
  import * as Command from "$lib/components/ui/command/index.js";
  import { toggleMode } from "mode-watcher";
  import ListTreeIcon from "@lucide/svelte/icons/list-tree";
  import WaypointsIcon from "@lucide/svelte/icons/waypoints";
  import LayoutDashboardIcon from "@lucide/svelte/icons/layout-dashboard";
  import SettingsIcon from "@lucide/svelte/icons/settings";
  import SunMoonIcon from "@lucide/svelte/icons/sun-moon";
  import type { Component } from "svelte";
  import { initialTasks, loadTaskList } from "$tasks/data.js";
  import { shortId, type Task } from "$tasks/model/issue.js";
  import StatusBadge from "$tasks/ui/StatusBadge.svelte";
  import TypeBadge from "$tasks/ui/TypeBadge.svelte";
  import { router } from "$shared/router/router.svelte.js";
  import { BoardPath, taskPath, settingsPath } from "$shared/router/routes.js";
  import { boardView, BoardView } from "$board/board-view.svelte.js";
  import { Copy } from "$shared/copy.js";

  // How many task rows to render at once (keeps the DOM + keystrokes fast).
  const MAX_TASKS = 20;

  let open = $state(false);
  let query = $state("");
  // Seed instantly from cache; refresh whenever the palette opens.
  let tasks = $state<Task[]>(initialTasks());

  // ⌘K / Ctrl+K toggles the palette from anywhere.
  function onKeydown(e: KeyboardEvent): void {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      open = !open;
    }
  }
  $effect(() => {
    window.addEventListener("keydown", onKeydown);
    return () => window.removeEventListener("keydown", onKeydown);
  });

  // Reset the query each time it opens, and pull the freshest list.
  $effect(() => {
    if (!open) return;
    query = "";
    loadTaskList().then((list) => {
      if (list) tasks = list;
    });
  });

  const q = $derived(query.trim().toLowerCase());

  // Our own cheap substring filter over id + title, capped to MAX_TASKS. Empty
  // query shows the first N (so the palette isn't blank on open).
  const matches = $derived.by(() => {
    if (!q) return tasks.slice(0, MAX_TASKS);
    const out: Task[] = [];
    for (const t of tasks) {
      if (`${shortId(t.id)} ${t.title ?? ""}`.toLowerCase().includes(q)) {
        out.push(t);
        if (out.length >= MAX_TASKS) break;
      }
    }
    return out;
  });
  const truncated = $derived(!!q && matches.length >= MAX_TASKS);

  interface Action {
    label: string;
    keywords: string;
    icon: Component;
    run: () => void;
  }
  const viewActions: Action[] = [
    { label: "Tree", keywords: "tree list board", icon: ListTreeIcon, run: () => setView(BoardView.Tree) },
    { label: "Graph", keywords: "graph stack dependency", icon: WaypointsIcon, run: () => setView(BoardView.Graph) },
    { label: "Dashboard", keywords: "dashboard epics overview", icon: LayoutDashboardIcon, run: () => setView(BoardView.Dashboard) },
  ];
  const gotoActions: Action[] = [
    { label: Copy.Settings, keywords: "settings account keys", icon: SettingsIcon, run: () => router.navigate(settingsPath()) },
    { label: Copy.CommandToggleTheme, keywords: "toggle theme dark light", icon: SunMoonIcon, run: toggleMode },
  ];
  const matchAction = (a: Action) => !q || `${a.label} ${a.keywords}`.toLowerCase().includes(q);
  const views = $derived(viewActions.filter(matchAction));
  const gotos = $derived(gotoActions.filter(matchAction));

  const empty = $derived(matches.length === 0 && views.length === 0 && gotos.length === 0);

  function act(run: () => void): void {
    open = false;
    run();
  }
  const goTask = (id: string) => act(() => router.navigate(taskPath(id)));
  function setView(v: BoardView): void {
    boardView.current = v;
    router.navigate(BoardPath);
  }
</script>

<Command.Dialog
  bind:open
  shouldFilter={false}
  title="Command palette"
  description="Search tasks and jump anywhere"
  class="max-w-xl"
>
  <Command.Input bind:value={query} placeholder={Copy.CommandPlaceholder} />
  <Command.List>
    {#if empty}
      <Command.Empty>{Copy.CommandNoResults}</Command.Empty>
    {/if}

    {#if matches.length}
      <Command.Group heading={Copy.CommandTasks}>
        {#each matches as task (task.id)}
          <Command.Item value={task.id} onSelect={() => goTask(task.id)}>
            <StatusBadge status={task.status} label={false} />
            <TypeBadge type={task.issue_type} />
            <span class="truncate">{task.title || Copy.UntitledTask}</span>
            <Command.Shortcut class="font-mono tracking-normal">{shortId(task.id)}</Command.Shortcut>
          </Command.Item>
        {/each}
        {#if truncated}
          <div class="px-2 py-1.5 text-xs text-muted-foreground">Keep typing to narrow results…</div>
        {/if}
      </Command.Group>
    {/if}

    {#if views.length}
      <Command.Separator />
      <Command.Group heading={Copy.CommandViews}>
        {#each views as a (a.label)}
          <Command.Item value={a.label} onSelect={() => act(a.run)}>
            <a.icon /> {a.label}
          </Command.Item>
        {/each}
      </Command.Group>
    {/if}

    {#if gotos.length}
      <Command.Group heading={Copy.CommandGoTo}>
        {#each gotos as a (a.label)}
          <Command.Item value={a.label} onSelect={() => act(a.run)}>
            <a.icon /> {a.label}
          </Command.Item>
        {/each}
      </Command.Group>
    {/if}
  </Command.List>
</Command.Dialog>
