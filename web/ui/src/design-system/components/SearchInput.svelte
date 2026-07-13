<!--
  The one search box: an input with a leading magnifier and a trailing clear (×)
  that appears once there's text. Two-way `value`. Used everywhere we search.
-->
<script lang="ts">
  import { Input } from "$lib/components/ui/input/index.js";
  import SearchIcon from "@lucide/svelte/icons/search";
  import XIcon from "@lucide/svelte/icons/x";

  interface Props {
    value: string;
    placeholder?: string;
    class?: string;
  }
  let { value = $bindable(""), placeholder = "", class: className = "" }: Props = $props();
</script>

<div class="relative {className}">
  <SearchIcon class="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
  <Input
    {value}
    oninput={(e) => (value = e.currentTarget.value)}
    {placeholder}
    class="h-8 pl-8 pr-8"
  />
  {#if value}
    <button
      type="button"
      onclick={() => (value = "")}
      aria-label="Clear search"
      class="absolute right-1.5 top-1/2 grid size-5 -translate-y-1/2 place-items-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
    >
      <XIcon class="size-3.5" />
    </button>
  {/if}
</div>
