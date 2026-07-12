<script lang="ts">
  import { cn } from "$lib/utils.js";
  import { isClosed, type Task } from "$tasks/model/issue.js";
  import { Copy } from "$shared/copy.js";
  import { rowPaddingLeft, TreeLayout } from "./tree-layout.js";
  import NodeToggle from "./NodeToggle.svelte";
  import StatusPicker from "$tasks/ui/StatusPicker.svelte";
  import TypePicker from "$tasks/ui/TypePicker.svelte";
  import PriorityPicker from "$tasks/ui/PriorityPicker.svelte";
  import TaskId from "$tasks/ui/TaskId.svelte";

  interface Props {
    task: Task;
    depth: number;
    selected: boolean;
    hasChildren: boolean;
    childCount: number;
    open: boolean;
    onToggle: () => void;
    onSelect: () => void;
    onPatch: (id: string, patch: Partial<Task>) => void;
  }
  let { task, depth, selected, hasChildren, childCount, open, onToggle, onSelect, onPatch }: Props = $props();

  function handleKeydown(event: KeyboardEvent): void {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onSelect();
    }
  }

  // Keep a badge's own menu clicks/keys from selecting/navigating the row.
  const stop = (event: Event) => event.stopPropagation();
</script>

<div
  role="button"
  tabindex="0"
  aria-current={selected}
  class={cn(
    "group flex cursor-pointer select-none items-center gap-1.5 rounded-md pr-2 text-sm",
    selected ? "bg-accent text-accent-foreground" : "hover:bg-accent/50",
  )}
  style="height: {TreeLayout.RowHeightPx}px; padding-left: {rowPaddingLeft(depth)}"
  onclick={onSelect}
  onkeydown={handleKeydown}
>
  <NodeToggle {hasChildren} {open} {onToggle} />
  <!-- Each badge opens its own menu; the wrappers keep those interactions from
       selecting/navigating the row. `contents` keeps the flex layout intact. -->
  <div class="contents" role="presentation" onclick={stop} onkeydown={stop}>
    <StatusPicker
      status={task.status}
      size="md"
      label={false}
      bg={false}
      onSelect={(status) => onPatch(task.id, { status })}
    />
  </div>
  <div class="contents" role="presentation" onclick={stop} onkeydown={stop}>
    <TypePicker type={task.issue_type} onSelect={(issue_type) => onPatch(task.id, { issue_type })} />
  </div>
  <div class="contents" role="presentation" onclick={stop} onkeydown={stop}>
    <PriorityPicker priority={task.priority} onSelect={(priority) => onPatch(task.id, { priority })} />
  </div>
  <div class="contents" role="presentation" onclick={stop} onkeydown={stop}>
    <TaskId id={task.id} short />
  </div>
  <span class={cn("truncate", isClosed(task) && "text-muted-foreground line-through")}>
    {task.title || Copy.UntitledTask}
  </span>
  {#if hasChildren}
    <span class="ml-auto shrink-0 text-[10px] tabular-nums text-muted-foreground">{childCount}</span>
  {/if}
</div>
