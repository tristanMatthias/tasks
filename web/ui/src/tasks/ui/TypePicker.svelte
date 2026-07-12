<!--
  An editable type badge: the badge is the trigger, and clicking it opens a menu
  (popover on desktop, bottom sheet on mobile) to change the task's type. Picking
  a value calls `onSelect` and closes the menu.
-->
<script lang="ts">
  import ResponsiveMenu from "$lib/components/ResponsiveMenu.svelte";
  import CheckIcon from "@lucide/svelte/icons/check";
  import TypeBadge from "./TypeBadge.svelte";
  import type { BadgeSize } from "$lib/components/Badge.svelte";
  import { ALL_TYPES } from "$tasks/model/filter.js";
  import type { IssueType } from "$tasks/model/issue.js";
  import { Copy } from "$shared/copy.js";

  interface Props {
    type: IssueType;
    onSelect: (type: IssueType) => void;
    size?: BadgeSize;
  }
  let { type, onSelect, size = "sm" }: Props = $props();

  let open = $state(false);

  function select(next: IssueType): void {
    if (next !== type) onSelect(next);
    open = false;
  }
</script>

<ResponsiveMenu bind:open title={Copy.ChangeType} align="start" class="w-44">
  {#snippet trigger({ props })}
    <button
      {...props}
      type="button"
      title={Copy.ChangeType}
      class="inline-flex items-center rounded-[3px] outline-none ring-ring ring-offset-2 ring-offset-background transition hover:opacity-80 focus-visible:ring-2"
    >
      <TypeBadge {type} {size} />
    </button>
  {/snippet}

  <div class="flex flex-col gap-0.5">
    {#each ALL_TYPES as option (option)}
      <button
        type="button"
        onclick={() => select(option)}
        class="flex items-center justify-between gap-3 rounded-md px-2 py-2 text-left text-base transition hover:bg-accent"
      >
        <TypeBadge type={option} size="lg" />
        {#if option === type}
          <CheckIcon class="size-4 text-muted-foreground" />
        {/if}
      </button>
    {/each}
  </div>
</ResponsiveMenu>
