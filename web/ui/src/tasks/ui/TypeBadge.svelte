<script lang="ts">
  import { TYPE_COLOR_VAR } from "$tasks/model/appearance.js";
  import type { IssueType } from "$tasks/model/issue.js";
  import Badge, { type BadgeSize } from "$lib/components/Badge.svelte";

  let { type, size = "sm" }: { type: IssueType; size?: BadgeSize } = $props();
</script>

<Badge color={TYPE_COLOR_VAR[type]} {size}>{type}</Badge>
