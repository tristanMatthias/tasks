<!--
  Renders task text as markdown, with references to other tasks linked. Clicks on
  those links are delegated (one listener, not one per link) to the SPA router;
  cmd/ctrl-click and hover fall through to the real href.
-->
<script lang="ts">
  import { renderTaskMarkdown, TASK_REF_ATTR } from "$tasks/markdown/taskref.js";
  import { indexVersion } from "$tasks/markdown/task-index.svelte.js";
  import { router } from "$shared/router/router.svelte.js";

  let { text, class: className = "" }: { text: string; class?: string } = $props();

  // Re-render when the id index fills (the task list can arrive after first paint).
  const html = $derived((indexVersion(), renderTaskMarkdown(text)));

  // One delegated listener for every task-ref link in this block.
  function intercept(node: HTMLElement) {
    const onClick = (event: MouseEvent) => {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const link = (event.target as HTMLElement).closest(`a[${TASK_REF_ATTR}]`);
      const id = link?.getAttribute(TASK_REF_ATTR);
      if (!id) return;
      event.preventDefault();
      router.navigate(link!.getAttribute("href") ?? "/");
      void id;
    };
    node.addEventListener("click", onClick);
    return { destroy: () => node.removeEventListener("click", onClick) };
  }
</script>

<div
  use:intercept
  class="prose prose-sm max-w-none dark:prose-invert prose-pre:bg-muted prose-pre:text-foreground prose-code:before:content-none prose-code:after:content-none {className}"
>
  {@html html}
</div>
