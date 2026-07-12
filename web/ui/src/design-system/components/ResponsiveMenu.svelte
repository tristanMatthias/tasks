<!--
  A menu whose shell adapts to the viewport: a popover on desktop, a bottom-sheet
  modal on mobile. The trigger and body are passed in as snippets and rendered
  once — only the surrounding shell differs, so nothing is defined twice.

  The mobile sheet is animated with a Svelte transition (not the utility-class
  keyframes) so the slide distance, duration, and easing are fully under our
  control and glide up like Apple's rather than a short library nudge.
-->
<script lang="ts" module>
  const SHEET_MS = 600;
  // The iOS "sheet" curve: quick to start, gentle settle at the top.
  const iosEase = cubicBezier(0.32, 0.72, 0, 1);

  /** Slide the sheet its own full height, in/out along the bottom edge. */
  function sheetSlide(_node: Element, { duration }: { duration: number }) {
    return {
      duration,
      easing: iosEase,
      css: (t: number) => `transform: translateY(${(1 - t) * 100}%)`,
    };
  }
</script>

<script lang="ts">
  import type { Snippet } from "svelte";
  import { fade } from "svelte/transition";
  import { Dialog } from "bits-ui";
  import * as Popover from "$lib/components/ui/popover/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { cubicBezier } from "$lib/motion/easing.js";
  import XIcon from "@lucide/svelte/icons/x";
  import { isDesktop } from "$shared/platform/media.svelte.js";
  import { router } from "$shared/router/router.svelte.js";
  import { Copy } from "$shared/copy.js";

  interface Props {
    /** The control that opens the menu; spread `props` onto your element. */
    trigger: Snippet<[{ props: Record<string, unknown> }]>;
    /** The menu body, rendered identically in both layouts. */
    children: Snippet;
    /** Heading shown on the mobile sheet (popover has no title bar). */
    title?: string;
    /** Desktop popover alignment relative to the trigger. */
    align?: "start" | "center" | "end";
    /** Desktop popover surface classes (e.g. a fixed width). The mobile sheet is
     * always full-width along the bottom, so these don't apply to it. */
    class?: string;
    /** Dim the background behind the mobile sheet. Defaults to true. */
    overlay?: boolean;
    /** Open state; bindable so a consumer can close it (e.g. after a pick). */
    open?: boolean;
  }
  let {
    trigger,
    children,
    title,
    align = "end",
    class: className,
    overlay = true,
    open = $bindable(false),
  }: Props = $props();

  // Defer the (heavy) popover/dialog machinery until first opened. A page full of
  // these — one per tree row × three fields — then costs only a button each until
  // actually used, which keeps the virtualized tree smooth while scrolling.
  let activated = $state(false);
  $effect(() => {
    if (open) activated = true;
  });
  const activate = () => {
    activated = true;
    open = true;
  };

  // Dismiss on navigation — e.g. opening a task from behind a non-modal sheet.
  // Depend ONLY on router.path (don't read `open`, or opening would re-trigger
  // this and immediately close it).
  $effect(() => {
    void router.path;
    open = false;
  });
</script>

{#if !activated}
  {@render trigger({ props: { type: "button", onclick: activate } })}
{:else if isDesktop.matches}
  <Popover.Root bind:open>
    <Popover.Trigger>
      {#snippet child({ props })}{@render trigger({ props })}{/snippet}
    </Popover.Trigger>
    <Popover.Content {align} class={className}>
      {@render children()}
    </Popover.Content>
  </Popover.Root>
{:else}
  <Dialog.Root bind:open>
    <Dialog.Trigger>
      {#snippet child({ props })}{@render trigger({ props })}{/snippet}
    </Dialog.Trigger>
    <Dialog.Portal>
      {#if overlay}
        <Dialog.Overlay forceMount>
          {#snippet child({ props, open: isOpen })}
            {#if isOpen}
              <div {...props} transition:fade={{ duration: SHEET_MS }} class="fixed inset-0 z-50 bg-black/60"></div>
            {/if}
          {/snippet}
        </Dialog.Overlay>
      {/if}
      <!-- Without a backdrop, stay non-modal so the screen behind keeps
           scrolling and stays interactive; close via the X or Escape only. -->
      <Dialog.Content
        forceMount
        preventScroll={overlay}
        trapFocus={overlay}
        interactOutsideBehavior={overlay ? "close" : "ignore"}
      >
        {#snippet child({ props, open: isOpen })}
          {#if isOpen}
            <div
              {...props}
              transition:sheetSlide={{ duration: SHEET_MS }}
              class="bg-popover text-popover-foreground fixed inset-x-3 bottom-0 z-50 flex flex-col gap-2.5 rounded-t-2xl border p-4 pb-8 text-sm shadow-lg"
            >
              <div class="flex items-center justify-between">
                <Dialog.Title class={title ? "text-base font-semibold" : "sr-only"}>
                  {title ?? Copy.Filters}
                </Dialog.Title>
                <Dialog.Close>
                  {#snippet child({ props: closeProps })}
                    <Button {...closeProps} variant="ghost" size="icon" class="size-8">
                      <XIcon class="size-4" />
                      <span class="sr-only">{Copy.Close}</span>
                    </Button>
                  {/snippet}
                </Dialog.Close>
              </div>
              {@render children()}
            </div>
          {/if}
        {/snippet}
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>
{/if}
