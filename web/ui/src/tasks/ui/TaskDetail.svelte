<script lang="ts">
  import { ScrollArea } from "$lib/components/ui/scroll-area/index.js";
  import { Copy } from "$shared/copy.js";
  import { buildHierarchy, parentId } from "$tasks/model/hierarchy.js";
  import { blockingTasks, type Task } from "$tasks/model/issue.js";
  import { newFilter } from "$tasks/model/filter.js";
  import TaskMeta from "$tasks/ui/TaskMeta.svelte";
  import TaskSection from "$tasks/ui/TaskSection.svelte";
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
    /** Show the meta line (type/status/priority/id). Off when a header shows it. */
    meta?: boolean;
  }
  let { task, tasks, onPatch, onSelect, meta = true }: Props = $props();

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
        {#if meta}
          <div class="mb-3">
            <TaskMeta {task} onPatch={(patch) => onPatch?.(task.id, patch)} />
          </div>
        {/if}
        <h1 class="text-xl font-semibold leading-snug">{task.title || Copy.UntitledTask}</h1>
      </div>

      <TaskSection title={Copy.Description} text={task.description} />
      <TaskSection title={Copy.AcceptanceCriteria} text={task.acceptance_criteria} />
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
{:else}
  <div class="flex h-full items-center justify-center px-6 text-center text-sm text-muted-foreground">
    {Copy.NoSelection}
  </div>
{/if}
