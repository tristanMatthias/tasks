<!--
  The global ⌘K / Ctrl+K command palette: fuzzy-search every task and jump to it,
  switch views, or hop to settings. Built entirely from the standard Command
  primitives (cmdk/bits-ui) so it matches the rest of the design system.
-->
<script lang="ts">
  import * as Command from "$lib/components/ui/command/index.js";
  import { toggleMode } from "mode-watcher";
  import ListTreeIcon from "@lucide/svelte/icons/list-tree";
  import WaypointsIcon from "@lucide/svelte/icons/waypoints";
  import LayoutDashboardIcon from "@lucide/svelte/icons/layout-dashboard";
  import SettingsIcon from "@lucide/svelte/icons/settings";
  import SunMoonIcon from "@lucide/svelte/icons/sun-moon";
  import { initialTasks, loadTaskList } from "$tasks/data.js";
  import { shortId, type Task } from "$tasks/model/issue.js";
  import StatusBadge from "$tasks/ui/StatusBadge.svelte";
  import TypeBadge from "$tasks/ui/TypeBadge.svelte";
  import { router } from "$shared/router/router.svelte.js";
  import { BoardPath, taskPath, settingsPath } from "$shared/router/routes.js";
  import { boardView, BoardView } from "$board/board-view.svelte.js";
  import { Copy } from "$shared/copy.js";

  let open = $state(false);
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

  // Pull the freshest list each time it opens (cheap; the list is slim).
  $effect(() => {
    if (!open) return;
    loadTaskList().then((list) => {
      if (list) tasks = list;
    });
  });

  function run(action: () => void): void {
    open = false;
    action();
  }
  const goTask = (id: string) => run(() => router.navigate(taskPath(id)));
  const setView = (v: BoardView) =>
    run(() => {
      boardView.current = v;
      router.navigate(BoardPath);
    });
</script>

<Command.Dialog
  bind:open
  title="Command palette"
  description="Search tasks and jump anywhere"
  class="max-w-xl"
>
  <Command.Input placeholder={Copy.CommandPlaceholder} />
  <Command.List>
    <Command.Empty>{Copy.CommandNoResults}</Command.Empty>

    <Command.Group heading={Copy.CommandTasks}>
      {#each tasks as task (task.id)}
        <Command.Item value={`${shortId(task.id)} ${task.title ?? ""}`} onSelect={() => goTask(task.id)}>
          <StatusBadge status={task.status} label={false} />
          <TypeBadge type={task.issue_type} />
          <span class="truncate">{task.title || Copy.UntitledTask}</span>
          <Command.Shortcut class="font-mono tracking-normal">{shortId(task.id)}</Command.Shortcut>
        </Command.Item>
      {/each}
    </Command.Group>

    <Command.Separator />

    <Command.Group heading={Copy.CommandViews}>
      <Command.Item value="tree list board" onSelect={() => setView(BoardView.Tree)}>
        <ListTreeIcon /> Tree
      </Command.Item>
      <Command.Item value="graph stack dependency" onSelect={() => setView(BoardView.Graph)}>
        <WaypointsIcon /> Graph
      </Command.Item>
      <Command.Item value="dashboard epics overview" onSelect={() => setView(BoardView.Dashboard)}>
        <LayoutDashboardIcon /> Dashboard
      </Command.Item>
    </Command.Group>

    <Command.Group heading={Copy.CommandGoTo}>
      <Command.Item value="settings account keys" onSelect={() => run(() => router.navigate(settingsPath()))}>
        <SettingsIcon /> {Copy.Settings}
      </Command.Item>
      <Command.Item value="toggle theme dark light" onSelect={() => run(toggleMode)}>
        <SunMoonIcon /> {Copy.CommandToggleTheme}
      </Command.Item>
    </Command.Group>
  </Command.List>
</Command.Dialog>
