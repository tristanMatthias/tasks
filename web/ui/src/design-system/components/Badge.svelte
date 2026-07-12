<!--
  The one badge primitive: an outlined/tinted pill. Every domain badge (type,
  status, priority) is a thin adapter that maps its value to a `color` and some
  content, then renders this. Geometry is identical per `size`; `border` and `bg`
  toggle the tinted outline and fill, and both tints derive from `color`.
-->
<script lang="ts" module>
  export type BadgeSize = "sm" | "md" | "lg";
</script>

<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    /** A CSS color (usually a themeable var) that tints text, border, and fill. */
    color: string;
    size?: BadgeSize;
    /** Tinted outline (default true). */
    border?: boolean;
    /** Tinted background fill (default true). */
    bg?: boolean;
    children: Snippet;
  }
  let { color, size = "sm", border = true, bg = true, children }: Props = $props();

  const SIZE: Record<BadgeSize, string> = {
    sm: "h-[15px] gap-1 text-[9px]",
    md: "h-[18px] gap-1.5 text-[10px]",
    lg: "h-6 gap-2 text-xs",
  };

  // Pill padding. gap == px at each size, so the space between children matches
  // the space to the edge (status dot → label reads equidistant to dot → edge).
  const PAD: Record<BadgeSize, string> = {
    sm: "min-w-[15px] px-1",
    md: "min-w-[18px] px-1.5",
    lg: "min-w-6 px-2",
  };

  // No surface (no border, no fill) → no pill padding; it's just the content.
  const bare = $derived(!border && !bg);

  const style = $derived(
    [
      `color: ${color}`,
      border ? `border-color: color-mix(in oklab, ${color} 45%, transparent)` : null,
      bg ? `background-color: color-mix(in oklab, ${color} 12%, transparent)` : null,
    ]
      .filter(Boolean)
      .join(";"),
  );
</script>

<span
  class="inline-flex shrink-0 items-center justify-center rounded-[3px] font-semibold uppercase leading-none tracking-wide {SIZE[size]} {bare ? '' : PAD[size]}"
  class:border
  {style}
>
  {@render children()}
</span>
