<!--
  An editable status badge: clicking it opens a menu (popover on desktop, bottom
  sheet on mobile) to change the task's status. Picking a value calls `onSelect`.
-->
<script lang="ts">
  import ResponsiveMenu from "$lib/components/ResponsiveMenu.svelte";
  import CheckIcon from "@lucide/svelte/icons/check";
  import StatusBadge from "./StatusBadge.svelte";
  import type { BadgeSize } from "$lib/components/Badge.svelte";
  import { ALL_STATUSES } from "$tasks/model/filter.js";
  import type { Status } from "$tasks/model/issue.js";
  import { Copy } from "$shared/copy.js";

  interface Props {
    status: Status;
    onSelect: (status: Status) => void;
    size?: BadgeSize;
    /** Show the status label in the trigger. False (e.g. in the tree) = dot only. */
    label?: boolean;
    /** Tinted pill background on the trigger. Independent of `border`. */
    bg?: boolean;
    /** Pill outline on the trigger. Independent of `bg`. */
    border?: boolean;
  }
  let { status, onSelect, size = "sm", label = true, bg = true, border = false }: Props = $props();

  let open = $state(false);

  function select(next: Status): void {
    if (next !== status) onSelect(next);
    open = false;
  }
</script>

<ResponsiveMenu bind:open title={Copy.ChangeStatus} align="start" class="w-48">
  {#snippet trigger({ props })}
    <button
      {...props}
      type="button"
      title={Copy.ChangeStatus}
      class="inline-flex items-center rounded-[3px] outline-none ring-ring ring-offset-2 ring-offset-background transition hover:opacity-80 focus-visible:ring-2"
    >
      <StatusBadge {status} {size} {label} {bg} {border} />
    </button>
  {/snippet}

  <div class="flex flex-col gap-0.5">
    {#each ALL_STATUSES as option (option)}
      <button
        type="button"
        onclick={() => select(option)}
        class="flex items-center justify-between gap-3 rounded-md px-2 py-2 text-left text-base transition hover:bg-accent"
      >
        <StatusBadge status={option} size="lg" />
        {#if option === status}
          <CheckIcon class="size-4 text-muted-foreground" />
        {/if}
      </button>
    {/each}
  </div>
</ResponsiveMenu>
