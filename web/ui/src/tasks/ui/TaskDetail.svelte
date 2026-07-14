<script lang="ts">
  import { ScrollArea } from "$lib/components/ui/scroll-area/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import * as Dialog from "$lib/components/ui/dialog/index.js";
  import WaypointsIcon from "@lucide/svelte/icons/waypoints";
  import TrashIcon from "@lucide/svelte/icons/trash-2";
  import { Copy } from "$shared/copy.js";
  import { buildHierarchy, parentId } from "$tasks/model/hierarchy.js";
  import { blockingTasks, type Task } from "$tasks/model/issue.js";
  import { newFilter } from "$tasks/model/filter.js";
  import TaskMeta from "$tasks/ui/TaskMeta.svelte";
  import TaskSection from "$tasks/ui/TaskSection.svelte";
  import TaskMarkdown from "$tasks/ui/TaskMarkdown.svelte";
  import GithubMark from "$tasks/ui/GithubMark.svelte";
  import GateList from "$tasks/ui/GateList.svelte";
  import StatusBadge from "$tasks/ui/StatusBadge.svelte";
  import TypeBadge from "$tasks/ui/TypeBadge.svelte";
  import TreePanel from "$board/ui/TreePanel.svelte";

  interface Props {
    task: Task | null;
    /** The full task list, for the children subtree + blocking lookup. */
    tasks: readonly Task[];
    /** Edit any task by id (the current one, or a child in the subtree). */
    onPatch?: (id: string, patch: Partial<Task>) => void;
    /** Navigate to another task (child / blocked task). */
    onSelect?: (id: string) => void;
    /** Open the Graph view rooted on this task (shown when provided). */
    onViewGraph?: (id: string) => void;
    /** Permanently delete this task (shows a Delete action + confirm when set). */
    onDelete?: (id: string) => void;
    /** Show the meta line (type/status/priority/id). Off when a header shows it. */
    meta?: boolean;
  }
  let { task, tasks, onPatch, onSelect, onViewGraph, onDelete, meta = true }: Props = $props();

  let confirmDelete = $state(false);

  const hierarchy = $derived(buildHierarchy(tasks));
  const childCount = $derived(task ? (hierarchy.children.get(task.id)?.length ?? 0) : 0);
  const blocking = $derived(task ? blockingTasks(task.id, tasks) : []);
  const parent = $derived.by(() => {
    if (!task) return null;
    const pid = parentId(hierarchy, task.id);
    return pid ? (hierarchy.byId.get(pid) ?? null) : null;
  });

  // The children subtree has its own independent search/filter.
  const childFilter = $state(newFilter());

  // A GitHub activity comment is "<prefix>[<link text>](<url>)". Split it so the
  // whole row becomes one link to the PR. Returns null for non-link activity.
  function parseGithubLink(text: string): { prefix: string; title: string; url: string } | null {
    const m = text.match(/^(.*?)\[([^\]]+)\]\(([^)]+)\)\s*$/);
    return m ? { prefix: m[1], title: m[2], url: m[3] } : null;
  }
</script>

{#snippet taskLink(t: Task)}
  <button
    type="button"
    onclick={() => onSelect?.(t.id)}
    class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition hover:bg-accent"
  >
    <StatusBadge status={t.status} />
    <TypeBadge type={t.issue_type} />
    <span class="truncate">{t.title || Copy.UntitledTask}</span>
  </button>
{/snippet}

{#if task}
  <ScrollArea class="h-full">
    <div class="mx-auto max-w-2xl space-y-6 px-6 py-6">
      <div>
        <div class="mb-3 flex items-start justify-between gap-3">
          {#if meta}
            <TaskMeta {task} onPatch={(patch) => onPatch?.(task.id, patch)} />
          {:else}
            <span></span>
          {/if}
          <div class="flex shrink-0 items-center gap-1.5">
            {#if onViewGraph}
              <Button
                variant="outline"
                size="sm"
                data-testid="view-in-graph"
                class="gap-1.5"
                onclick={() => onViewGraph?.(task.id)}
                title="See this task in the context of its stack"
              >
                <WaypointsIcon class="size-4" /> View in graph
              </Button>
            {/if}
            {#if onDelete}
              <Button
                variant="ghost"
                size="icon"
                data-testid="delete-task"
                class="size-8 text-muted-foreground hover:text-destructive"
                onclick={() => (confirmDelete = true)}
                title="Delete task"
              >
                <TrashIcon class="size-4" />
              </Button>
            {/if}
          </div>
        </div>
        <h1 class="text-xl font-semibold leading-snug">{task.title || Copy.UntitledTask}</h1>
      </div>

      {#if task.comments?.length}
        <TaskSection title="Activity">
          <ul data-testid="activity" class="flex flex-col gap-1.5">
            {#each task.comments as c (c.id)}
              {@const link = c.author === "github" ? parseGithubLink(c.text) : null}
              {#if link}
                <li>
                  <a
                    href={link.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    data-testid="activity-item"
                    class="flex items-center gap-2 rounded-md border bg-card/50 px-2.5 py-1.5 text-sm transition-colors hover:bg-accent/50"
                  >
                    <GithubMark class="size-4 shrink-0 text-muted-foreground" />
                    <span class="min-w-0 truncate">
                      <span class="text-muted-foreground">{link.prefix}</span><span class="font-medium">{link.title}</span>
                    </span>
                  </a>
                </li>
              {:else}
                <li class="flex items-start gap-2 rounded-md border bg-card/50 px-2.5 py-1.5 text-sm">
                  {#if c.author === "github"}
                    <GithubMark class="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                  {/if}
                  <div class="min-w-0 flex-1">
                    <TaskMarkdown text={c.text} />
                    {#if c.author && c.author !== "github"}
                      <div class="mt-0.5 text-[11px] text-muted-foreground">{c.author}</div>
                    {/if}
                  </div>
                </li>
              {/if}
            {/each}
          </ul>
        </TaskSection>
      {/if}

      <TaskSection title={Copy.Description} text={task.description} />
      <TaskSection title={Copy.AcceptanceCriteria} text={task.acceptance_criteria} />

      {#if task.gates?.length}
        <TaskSection title="{Copy.AcceptanceGates} ({task.gates.length})">
          <GateList gates={task.gates} />
        </TaskSection>
      {/if}
      <TaskSection title={Copy.Design} text={task.design} />
      <TaskSection title={Copy.Notes} text={task.notes} />

      {#if task.labels?.length}
        <TaskSection title={Copy.Labels}>
          <div class="flex flex-wrap gap-1.5">
            {#each task.labels as label (label)}
              <span class="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">{label}</span>
            {/each}
          </div>
        </TaskSection>
      {/if}

      {#if parent}
        <TaskSection title={Copy.Parent}>
          {@render taskLink(parent)}
        </TaskSection>
      {/if}

      {#if blocking.length}
        <TaskSection title="{Copy.Blocking} ({blocking.length})">
          <div class="flex flex-col gap-0.5">
            {#each blocking as b (b.id)}
              {@render taskLink(b)}
            {/each}
          </div>
        </TaskSection>
      {/if}

      {#if childCount > 0}
        <TaskSection title="{Copy.Children} ({childCount})">
          <div class="overflow-hidden rounded-md border">
            <TreePanel
              {tasks}
              rootId={task.id}
              autoHeight
              filter={childFilter}
              selectedId={task.id}
              {onSelect}
              onPatch={(id, patch) => onPatch?.(id, patch)}
            />
          </div>
        </TaskSection>
      {/if}
    </div>
  </ScrollArea>

  <Dialog.Root bind:open={confirmDelete}>
    <Dialog.Content class="max-w-sm">
      <Dialog.Header>
        <Dialog.Title>Delete this task?</Dialog.Title>
        <Dialog.Description>
          “{task.title || Copy.UntitledTask}” and its comments will be permanently removed. This
          can’t be undone.
        </Dialog.Description>
      </Dialog.Header>
      <Dialog.Footer>
        <Button variant="outline" onclick={() => (confirmDelete = false)}>Cancel</Button>
        <Button
          variant="destructive"
          data-testid="delete-confirm"
          onclick={() => {
            confirmDelete = false;
            onDelete?.(task.id);
          }}
        >
          Delete
        </Button>
      </Dialog.Footer>
    </Dialog.Content>
  </Dialog.Root>
{:else}
  <div class="flex h-full items-center justify-center px-6 text-center text-sm text-muted-foreground">
    {Copy.NoSelection}
  </div>
{/if}
