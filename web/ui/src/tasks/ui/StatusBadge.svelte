<script lang="ts">
  import { STATUS_COLOR_VAR, STATUS_LABEL } from "$tasks/model/appearance.js";
  import type { Status } from "$tasks/model/issue.js";
  import Badge, { type BadgeSize } from "$lib/components/Badge.svelte";

  interface Props {
    status: Status;
    size?: BadgeSize;
    /** Show the status label beside the dot. False (e.g. in the tree) = dot only. */
    label?: boolean;
    /** Tinted (neutral) pill background. Independent of `border`. */
    bg?: boolean;
    /** Neutral pill outline. Independent of `bg`. */
    border?: boolean;
  }
  let { status, size = "sm", label = true, bg = true, border = false }: Props = $props();

  // Dot sized to sit comfortably inside the pill at each badge size.
  const DOT: Record<BadgeSize, string> = { sm: "size-1.5", md: "size-2", lg: "size-2.5" };
</script>

<!-- Neutral pill; the dot carries the color, the label is optional. -->
<Badge color="var(--muted-foreground)" {border} {bg} {size}>
  <span
    class="shrink-0 rounded-full {DOT[size]}"
    style="background-color: {STATUS_COLOR_VAR[status]}"
    title={STATUS_LABEL[status]}
    aria-label={STATUS_LABEL[status]}
  ></span>
  {#if label}{STATUS_LABEL[status]}{/if}
</Badge>
