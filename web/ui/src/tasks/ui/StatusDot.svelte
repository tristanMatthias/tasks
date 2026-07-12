<script lang="ts">
  import { STATUS_COLOR_VAR, STATUS_LABEL } from "$tasks/model/appearance";
  import type { Status } from "$tasks/model/issue";
  import type { BadgeSize } from "$lib/components/Badge.svelte";

  let { status, size = "sm" }: { status: Status; size?: BadgeSize } = $props();

  const SIZE: Record<BadgeSize, string> = {
    sm: "size-2",
    md: "size-2.5",
    lg: "size-3",
  };
</script>

<span
  class="block shrink-0 rounded-full {SIZE[size]}"
  style="background-color: {STATUS_COLOR_VAR[status]}"
  title={STATUS_LABEL[status]}
  aria-label={STATUS_LABEL[status]}
></span>
