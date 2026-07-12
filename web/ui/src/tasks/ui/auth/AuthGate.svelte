<!--
  Wraps the app: loads the auth session, then presents the right flow — the app
  when authed (or open mode), a token form for shared-token mode, or the public
  marketing landing page (with a Log in CTA) for a logged-out custom-auth (Clerk)
  visitor.
-->
<script lang="ts">
  import type { Snippet } from "svelte";
  import LoaderIcon from "@lucide/svelte/icons/loader-circle";
  import { session } from "$shared/auth/session.svelte.js";
  import TokenLogin from "./TokenLogin.svelte";
  import LandingPage from "$marketing/LandingPage.svelte";

  let { children }: { children: Snippet } = $props();

  session.load();
</script>

{#if session.loading}
  <div class="flex min-h-screen items-center justify-center text-muted-foreground">
    <LoaderIcon class="size-5 animate-spin" />
  </div>
{:else if session.needsLogin && session.mode === "token"}
  <TokenLogin onSuccess={() => session.load()} />
{:else if session.needsLogin && session.mode === "custom"}
  <LandingPage loginUrl={session.loginUrl} />
{:else}
  {@render children()}
{/if}
