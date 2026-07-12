<!-- Full-screen token entry, shown when the server runs in shared-token mode. -->
<script lang="ts">
  import { Input } from "$lib/components/ui/input/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import ListChecksIcon from "@lucide/svelte/icons/list-checks";
  import KeyIcon from "@lucide/svelte/icons/key-round";
  import { login } from "$shared/auth/auth.js";
  import { Copy } from "$shared/copy.js";

  let { onSuccess }: { onSuccess: () => void } = $props();

  let token = $state("");
  let error = $state(false);
  let busy = $state(false);

  async function submit(event: Event): Promise<void> {
    event.preventDefault();
    if (!token.trim() || busy) return;
    busy = true;
    error = false;
    const ok = await login(token.trim());
    busy = false;
    if (ok) onSuccess();
    else error = true;
  }
</script>

<div class="relative flex min-h-screen items-center justify-center overflow-hidden px-6">
  <!-- Subtle depth behind the form, à la Linear. -->
  <div
    class="pointer-events-none absolute left-1/2 top-[42%] -z-10 size-[560px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/15 blur-[130px]"
  ></div>

  <div class="w-full max-w-[320px]">
    <div class="flex flex-col items-center text-center">
      <div
        class="flex size-12 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg shadow-primary/25"
      >
        <ListChecksIcon class="size-6" />
      </div>
      <h1 class="mt-5 text-xl font-semibold tracking-tight">
        {Copy.SignIn} to {Copy.AppName}
      </h1>
      <p class="mt-1.5 text-sm text-muted-foreground">{Copy.SignInPrompt}</p>
    </div>

    <form onsubmit={submit} class="mt-7 space-y-3">
      <div class="relative">
        <KeyIcon class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          type="password"
          bind:value={token}
          placeholder={Copy.AccessToken}
          autocomplete="current-password"
          aria-invalid={error}
          class="h-11 pl-9 {error ? 'border-destructive focus-visible:ring-destructive/30' : ''}"
        />
      </div>
      {#if error}
        <p class="text-xs text-destructive">{Copy.InvalidToken}</p>
      {/if}
      <Button
        type="submit"
        size="lg"
        class="h-11 w-full font-medium shadow-lg shadow-primary/25"
        disabled={busy || !token.trim()}
      >
        {Copy.SignIn}
      </Button>
    </form>
  </div>
</div>
