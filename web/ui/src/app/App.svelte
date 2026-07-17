<script lang="ts">
  import { ModeWatcher } from "mode-watcher";
  import { Toaster } from "$lib/components/ui/sonner/index.js";
  import AppHeader from "./AppHeader.svelte";
  import CommandPalette from "./CommandPalette.svelte";
  import BoardPage from "$board/BoardPage.svelte";
  import AuthGate from "$tasks/ui/auth/AuthGate.svelte";
  import SettingsPage from "$tasks/ui/settings/SettingsPage.svelte";
  import { router } from "$shared/router/router.svelte.js";
  import { isSettingsPath } from "$shared/router/routes.js";

  const onSettings = $derived(isSettingsPath(router.path));
</script>

<ModeWatcher defaultMode="dark" />
<Toaster />

<AuthGate>
  <!-- Global ⌘K palette — available on the board and in settings. -->
  <CommandPalette />
  {#if onSettings}
    <SettingsPage />
  {:else}
    <div class="flex h-screen flex-col">
      <AppHeader />
      <main class="min-h-0 flex-1">
        <BoardPage />
      </main>
    </div>
  {/if}
</AuthGate>
