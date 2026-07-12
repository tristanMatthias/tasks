<!--
  An editable priority tag: clicking it opens a menu (popover on desktop, bottom
  sheet on mobile) to change the task's priority. Picking a value calls `onSelect`.
-->
<script lang="ts">
  import ResponsiveMenu from "$lib/components/ResponsiveMenu.svelte";
  import CheckIcon from "@lucide/svelte/icons/check";
  import PriorityTag from "./PriorityTag.svelte";
  import type { BadgeSize } from "$lib/components/Badge.svelte";
  import { PRIORITIES } from "$tasks/model/issue.js";
  import { Copy } from "$shared/copy.js";

  interface Props {
    priority: number | null;
    onSelect: (priority: number) => void;
    size?: BadgeSize;
  }
  let { priority, onSelect, size = "sm" }: Props = $props();

  let open = $state(false);

  function select(next: number): void {
    if (next !== priority) onSelect(next);
    open = false;
  }
</script>

<ResponsiveMenu bind:open title={Copy.ChangePriority} align="start" class="w-36">
  {#snippet trigger({ props })}
    <button
      {...props}
      type="button"
      title={Copy.ChangePriority}
      class="inline-flex items-center rounded outline-none ring-ring ring-offset-2 ring-offset-background transition hover:opacity-80 focus-visible:ring-2"
    >
      <PriorityTag {priority} {size} placeholder />
    </button>
  {/snippet}

  <div class="flex flex-col gap-0.5">
    {#each PRIORITIES as option (option)}
      <button
        type="button"
        onclick={() => select(option)}
        class="flex items-center justify-between gap-3 rounded-md px-2 py-2 text-left text-base transition hover:bg-accent"
      >
        <PriorityTag priority={option} size="lg" />
        {#if option === priority}
          <CheckIcon class="size-4 text-muted-foreground" />
        {/if}
      </button>
    {/each}
  </div>
</ResponsiveMenu>
