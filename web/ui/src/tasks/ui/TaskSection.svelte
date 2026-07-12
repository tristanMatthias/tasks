<!-- A titled detail section. Renders nothing when there's no content. -->
<script lang="ts">
  import type { Snippet } from "svelte";
  import TaskMarkdown from "$tasks/ui/TaskMarkdown.svelte";

  interface Props {
    title: string;
    /** Markdown body; when omitted, render `children` instead. */
    text?: string;
    children?: Snippet;
  }
  let { title, text, children }: Props = $props();
</script>

{#if text?.trim() || children}
  <section>
    <h2 class="mb-1.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h2>
    {#if text?.trim()}
      <TaskMarkdown {text} />
    {:else if children}
      {@render children()}
    {/if}
  </section>
{/if}
