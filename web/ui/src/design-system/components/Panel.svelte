<!--
  A titled card section for settings/forms: an optional header (title +
  description + actions) over a content body. Compose these down a page.
-->
<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    title?: string;
    description?: string;
    /** Header-right controls (e.g. a primary action button). */
    actions?: Snippet;
    /** Remove the body padding (e.g. when embedding a full-bleed list). */
    flush?: boolean;
    children: Snippet;
  }
  let { title, description, actions, flush = false, children }: Props = $props();
</script>

<section class="overflow-hidden rounded-lg border bg-card">
  {#if title || actions}
    <div class="flex items-start justify-between gap-4 border-b px-5 py-4">
      <div class="space-y-0.5">
        {#if title}<h3 class="text-sm font-semibold">{title}</h3>{/if}
        {#if description}<p class="text-sm text-muted-foreground">{description}</p>{/if}
      </div>
      {#if actions}<div class="shrink-0">{@render actions()}</div>{/if}
    </div>
  {/if}
  <div class={flush ? "" : "p-5"}>
    {@render children()}
  </div>
</section>
