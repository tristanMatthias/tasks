<!--
  The task's meta line: editable type / status / priority badges plus its id.
  Rendered at the top of the detail body on desktop, and inline on the mobile
  header row (beside the Back button).
-->
<script lang="ts">
  import type { Task } from "$tasks/model/issue.js";
  import StatusPicker from "./StatusPicker.svelte";
  import TypePicker from "./TypePicker.svelte";
  import PriorityPicker from "./PriorityPicker.svelte";
  import TaskId from "./TaskId.svelte";

  interface Props {
    task: Task;
    onPatch?: (patch: Partial<Task>) => void;
  }
  let { task, onPatch }: Props = $props();
</script>

<div class="flex items-center gap-2.5">
  <TypePicker type={task.issue_type} size="md" onSelect={(issue_type) => onPatch?.({ issue_type })} />
  <StatusPicker size="md" status={task.status} onSelect={(status) => onPatch?.({ status })} />
  <PriorityPicker priority={task.priority} size="md" onSelect={(priority) => onPatch?.({ priority })} />
  <TaskId id={task.id} size="md" bg />
</div>
