<script lang="ts">
  import ChevronRight from "@lucide/svelte/icons/chevron-right";
  import { cn } from "$lib/utils.js";
  import { Copy } from "$shared/copy.js";

  interface Props {
    hasChildren: boolean;
    open: boolean;
    onToggle: () => void;
  }
  let { hasChildren, open, onToggle }: Props = $props();

  function handleClick(event: MouseEvent): void {
    event.stopPropagation(); // toggle only; don't also select the row
    onToggle();
  }
</script>

{#if hasChildren}
  <button
    type="button"
    class="flex size-4 shrink-0 items-center justify-center rounded text-muted-foreground transition-colors hover:text-foreground"
    aria-label={open ? Copy.Collapse : Copy.Expand}
    onclick={handleClick}
  >
    <ChevronRight class={cn("size-3.5 transition-transform duration-150", open && "rotate-90")} />
  </button>
{:else}
  <span class="size-4 shrink-0" aria-hidden="true"></span>
{/if}
