<script lang="ts">
  import { untrack } from "svelte";
  import { fly } from "svelte/transition";
  import {
    ResizablePaneGroup,
    ResizablePane,
    ResizableHandle,
  } from "$lib/components/ui/resizable/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
  import XIcon from "@lucide/svelte/icons/x";
  import CrosshairIcon from "@lucide/svelte/icons/crosshair";
  import MainPanel from "./ui/MainPanel.svelte";
  import TaskDetail from "$tasks/ui/TaskDetail.svelte";
  import TaskMeta from "$tasks/ui/TaskMeta.svelte";
  import { IssueType, type Task } from "$tasks/model/issue.js";
  import { initialTasks, loadTaskList, fetchTaskDetail, updateTask } from "$tasks/data.js";
  import { indexTasks } from "$tasks/markdown/task-index.svelte.js";
  import { createPersistedFilter } from "./board-filter.svelte.js";
  import { createPersistedView, BoardView } from "./board-view.svelte.js";
  import { createPersistedSort } from "./board-sort.svelte.js";
  import { router } from "$shared/router/router.svelte.js";
  import { BoardPath, taskPath, taskIdFromPath } from "$shared/router/routes.js";
  import { Breakpoint, createMediaQuery } from "$shared/platform/media.svelte.js";
  import { StorageKey } from "$shared/platform/storage.js";
  import { Copy } from "$shared/copy.js";

  const TREE_PANE_DEFAULT_SIZE = 40;
  const TREE_PANE_MIN_SIZE = 24;

  const isDesktop = createMediaQuery(Breakpoint.Desktop);

  // Seed synchronously from the preload so the tree paints with data on the first
  // frame; fall back to awaiting the fetch if it wasn't ready yet.
  let tasks = $state<Task[]>(initialTasks());
  // Keep the id index (for linkifying task refs in markdown) in sync with the
  // list — seeded synchronously, then refreshed when the full list loads below.
  indexTasks(tasks);
  // Search + facet filter, persisted across refreshes.
  const filter = createPersistedFilter();
  // Which view is active (tree / graph / dashboard), also persisted.
  const view = createPersistedView();
  // Sort order (field + direction), also persisted.
  const sort = createPersistedSort();
  // The full record for the open task (the list is slim — no description, etc.).
  let detailTask = $state<Task | null>(null);

  // The selected task lives in the URL, so a refresh lands on the same issue.
  const selectedId = $derived(taskIdFromPath(router.path));

  // Refresh in the background; the tree already showed cached data instantly.
  // A null result means the fetch failed (keep what's shown); an empty list is
  // a real state (a workspace with no tasks) and must replace the cache.
  loadTaskList().then((list) => {
    if (list !== null) {
      tasks = list;
      indexTasks(list);
    }
  });

  // Detail-on-demand: show the slim row instantly, then fetch the full record.
  $effect(() => {
    const id = selectedId;
    if (id === null) {
      detailTask = null;
      return;
    }
    detailTask = untrack(() => tasks.find((task) => task.id === id) ?? null);
    let cancelled = false;
    fetchTaskDetail(id).then((full) => {
      if (!cancelled && full) detailTask = full;
    });
    return () => {
      cancelled = true;
    };
  });

  // Edit a task field (type, status, priority, …): update the tree list and the
  // open detail optimistically, then persist and reconcile with the server.
  async function patchTask(id: string, patch: Partial<Task>): Promise<void> {
    tasks = tasks.map((task) => (task.id === id ? { ...task, ...patch } : task));
    if (detailTask?.id === id) detailTask = { ...detailTask, ...patch };
    const saved = await updateTask(id, patch);
    if (saved) {
      // Reconcile just the edited fields into the slim tree row; take the full
      // record for the detail pane.
      const keys = Object.keys(patch) as (keyof Task)[];
      const reconciled = (task: Task): Task => {
        const next = { ...task };
        for (const key of keys) (next[key] as Task[keyof Task]) = saved[key];
        return next;
      };
      tasks = tasks.map((task) => (task.id === id ? reconciled(task) : task));
      if (detailTask?.id === id) detailTask = saved;
    }
  }

  const openTask = (id: string) => router.navigate(taskPath(id));
  const backToList = () => router.navigate(BoardPath);

  // The Graph view is ROOTED on a pinned task (not the transient selection), so
  // clicking a node updates the detail pane without the graph jumping. Root it on
  // the open task when you enter the view; re-root explicitly via double-click
  // (desktop) or "Center here" (mobile).
  let graphFocusId = $state<string | null>(null);
  $effect(() => {
    const activeGraph = view.current === BoardView.Graph;
    const list = tasks;
    const sel = selectedId;
    if (!activeGraph || !list.length) return;
    untrack(() => {
      const valid = graphFocusId && list.some((t) => t.id === graphFocusId);
      if (!valid) graphFocusId = sel ?? list.find((t) => t.issue_type === IssueType.Epic)?.id ?? list[0]?.id ?? null;
    });
  });
  const refocus = (id: string) => {
    graphFocusId = id;
    openTask(id);
  };
  const centerHere = () => {
    if (selectedId) graphFocusId = selectedId;
  };
</script>

{#snippet mainPanel()}
  <MainPanel
    {tasks}
    filter={filter.current}
    {view}
    {sort}
    {selectedId}
    onSelect={openTask}
    onPatch={patchTask}
    {graphFocusId}
    onGraphFocus={refocus}
  />
{/snippet}

{#if isDesktop.matches}
  <!-- Desktop: resizable two-pane. The left pane swaps view (tree/dashboard/
       graph); the right detail pane persists and just updates on selection. -->
  <ResizablePaneGroup direction="horizontal" autoSaveId={StorageKey.PaneLayout} class="h-full">
    <ResizablePane id="tree" order={1} defaultSize={TREE_PANE_DEFAULT_SIZE} minSize={TREE_PANE_MIN_SIZE}>
      {@render mainPanel()}
    </ResizablePane>
    <ResizableHandle withHandle />
    <ResizablePane id="detail" order={2}>
      <TaskDetail task={detailTask} {tasks} onPatch={patchTask} onSelect={openTask} />
    </ResizablePane>
  </ResizablePaneGroup>
{:else}
  <!-- Mobile: the view is full-width. It stays MOUNTED and laid out (only made
       invisible, not display:none) under the opaque detail overlay — so the
       virtualizer keeps its rows measured and returning to the list is instant. -->
  <div class="relative h-full min-h-0">
    {#if view.current === BoardView.Graph}
      <!-- Graph stays full-screen + interactive; a non-modal peek sheet shows the
           tapped node so you can keep exploring the graph above it. -->
      <div class="h-full min-h-0">
        {@render mainPanel()}
      </div>
      {#if selectedId !== null && detailTask}
        <div
          class="absolute inset-x-0 bottom-0 z-20 flex max-h-[64%] flex-col rounded-t-2xl border bg-background shadow-2xl shadow-black/50"
          transition:fly={{ y: 500, duration: 300 }}
        >
          <div class="flex flex-col items-center pt-2">
            <div class="h-1 w-9 rounded-full bg-border"></div>
          </div>
          <div class="mt-1 flex items-center gap-2 border-b px-3 py-2">
            <Button variant="outline" size="sm" class="gap-1.5" onclick={centerHere}>
              <CrosshairIcon class="size-4" /> Center here
            </Button>
            <Button variant="ghost" size="icon" class="ml-auto size-8" onclick={backToList}>
              <XIcon class="size-4" />
            </Button>
          </div>
          <div class="min-h-0 flex-1 overflow-y-auto">
            <TaskDetail task={detailTask} {tasks} onPatch={patchTask} onSelect={openTask} />
          </div>
        </div>
      {/if}
    {:else}
      <div class="h-full min-h-0" class:invisible={selectedId !== null}>
        {@render mainPanel()}
      </div>
      {#if selectedId !== null}
        <div class="absolute inset-0 flex min-h-0 flex-col bg-background">
          <div class="flex items-center gap-2 border-b px-2 py-1.5">
            <Button variant="ghost" size="sm" class="shrink-0 gap-1.5" onclick={backToList}>
              <ArrowLeftIcon class="size-4" />
              {Copy.Back}
            </Button>
            {#if detailTask}
              <div
                class="ml-auto flex min-w-0 items-center justify-end overflow-x-auto [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
              >
                <TaskMeta task={detailTask} onPatch={(patch) => patchTask(detailTask.id, patch)} />
              </div>
            {/if}
          </div>
          <div class="min-h-0 flex-1">
            <TaskDetail task={detailTask} {tasks} meta={false} onPatch={patchTask} onSelect={openTask} />
          </div>
        </div>
      {/if}
    {/if}
  </div>
{/if}
