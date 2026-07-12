<!--
  The task id as a badge. Same geometry as the other badges (via Badge), shown in
  monospace, and clicking it copies the full id to the clipboard with a toast.
-->
<script lang="ts">
  import { toast } from "svelte-sonner";
  import CheckIcon from "@lucide/svelte/icons/check";
  import Badge, { type BadgeSize } from "$lib/components/Badge.svelte";
  import { shortId } from "$tasks/model/issue.js";
  import { copyText } from "$shared/platform/clipboard.js";
  import { Copy } from "$shared/copy.js";

  interface Props {
    id: string;
    size?: BadgeSize;
    /** Display the short, prefix-free id (the full id is still what gets copied). */
    short?: boolean;
    /** Tinted pill background. */
    bg?: boolean;
  }
  let { id, size = "sm", short = false, bg = false }: Props = $props();

  let copied = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copyId(event: MouseEvent): Promise<void> {
    event.stopPropagation();
    if (!(await copyText(id))) {
      toast.error(Copy.CopyFailed);
      return;
    }
    toast.success(`${Copy.CopiedId} ${id}`);
    copied = true;
    clearTimeout(timer);
    timer = setTimeout(() => (copied = false), 1200);
  }
</script>

<button
  type="button"
  onclick={copyId}
  title={Copy.CopyId}
  class="inline-flex rounded-[3px] outline-none ring-ring ring-offset-2 ring-offset-background transition hover:opacity-80 focus-visible:ring-2"
>
  <Badge color="var(--muted-foreground)" border={false} {bg} {size}>
    <span class="font-mono normal-case">{short ? shortId(id) : id}</span>
    {#if copied}<CheckIcon class="size-3" />{/if}
  </Badge>
</button>
