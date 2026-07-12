<!--
  Wraps the app: loads the auth session, then presents the right flow —
  the app when authed (or open mode), a token form for shared-token mode, or a
  redirect to the embedder's sign-in page for custom (e.g. Clerk) mode.
-->
<script lang="ts">
  import type { Snippet } from "svelte";
  import LoaderIcon from "@lucide/svelte/icons/loader-circle";
  import { session } from "$shared/auth/session.svelte.js";
  import { Copy } from "$shared/copy.js";
  import TokenLogin from "./TokenLogin.svelte";

  let { children }: { children: Snippet } = $props();

  session.load();

  // Custom mode (embedder auth) can't be satisfied in-app — hand off to its page.
  $effect(() => {
    if (session.needsLogin && session.mode === "custom" && session.loginUrl) {
      window.location.href = session.loginUrl;
    }
  });
</script>

{#if session.loading}
  <div class="flex min-h-screen items-center justify-center text-muted-foreground">
    <LoaderIcon class="size-5 animate-spin" />
  </div>
{:else if session.needsLogin && session.mode === "token"}
  <TokenLogin onSuccess={() => session.load()} />
{:else if session.needsLogin && session.mode === "custom"}
  <div class="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
    {Copy.RedirectingToSignIn}
  </div>
{:else}
  {@render children()}
{/if}
