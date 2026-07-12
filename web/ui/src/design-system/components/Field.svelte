<!--
  A labeled form field: label + control (the children) + a hint or error line.
  The reusable atom for building forms — pairs with any input/select/etc.
-->
<script lang="ts">
  import type { Snippet } from "svelte";
  import { Label } from "$lib/components/ui/label/index.js";

  interface Props {
    label?: string;
    hint?: string;
    error?: string;
    /** Associate the label with a control id. */
    for?: string;
    children: Snippet;
  }
  let { label, hint, error, for: htmlFor, children }: Props = $props();
</script>

<div class="space-y-1.5">
  {#if label}
    <Label for={htmlFor} class="text-sm font-medium">{label}</Label>
  {/if}
  {@render children()}
  {#if error}
    <p class="text-xs text-destructive">{error}</p>
  {:else if hint}
    <p class="text-xs text-muted-foreground">{hint}</p>
  {/if}
</div>
