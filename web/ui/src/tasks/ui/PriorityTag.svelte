<script lang="ts">
  import { PRIORITY_COLOR_VAR } from "$tasks/model/appearance.js";
  import Badge, { type BadgeSize } from "$lib/components/Badge.svelte";

  interface Props {
    priority: number | null;
    /** Render a muted placeholder pill when unset, so it stays a clickable target. */
    placeholder?: boolean;
    size?: BadgeSize;
    /** Independent background / outline toggles (see Badge). */
    bg?: boolean;
    border?: boolean;
  }
  let { priority, placeholder = false, size = "sm", bg = true, border = false }: Props = $props();

  // Colored like beads: P0 urgent → P4 low; unknown falls back to muted.
  const color = $derived(priority !== null ? (PRIORITY_COLOR_VAR[priority] ?? "var(--muted-foreground)") : "");
</script>

{#if priority !== null}
  <Badge {color} {size} {bg} {border}>P{priority}</Badge>
{:else if placeholder}
  <Badge color="var(--muted-foreground)" {size} {bg} {border}>–</Badge>
{/if}
