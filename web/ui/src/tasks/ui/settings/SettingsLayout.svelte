<!--
  Settings shell: a back-to-board header and a sidebar of sections (a scrollable
  tab row on mobile), with the active section rendered as `children`.
-->
<script lang="ts">
  import type { Snippet } from "svelte";
  import { Button } from "$lib/components/ui/button/index.js";
  import { cn } from "$lib/utils.js";
  import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
  import UserIcon from "@lucide/svelte/icons/circle-user-round";
  import KeyIcon from "@lucide/svelte/icons/key-round";
  import PlugIcon from "@lucide/svelte/icons/plug";
  import UsersIcon from "@lucide/svelte/icons/users";
  import BuildingIcon from "@lucide/svelte/icons/building-2";
  import { router } from "$shared/router/router.svelte.js";
  import {
    BoardPath,
    settingsPath,
    settingsSectionFromPath,
    type SettingsSection,
  } from "$shared/router/routes.js";
  import { workspaces } from "$shared/auth/workspaces.svelte.js";
  import { Copy } from "$shared/copy.js";

  let { children }: { children: Snippet } = $props();

  const current = $derived(settingsSectionFromPath(router.path));

  $effect(() => {
    workspaces.ensureLoaded();
  });

  // The workspace sections only apply to a shared (Clerk) org, not the personal
  // board or token/none dev mode.
  const showWorkspace = $derived(workspaces.available && !workspaces.active.isPersonal);
  const items = $derived<{ id: SettingsSection; label: string; icon: typeof UserIcon }[]>([
    { id: "account", label: Copy.Account, icon: UserIcon },
    { id: "keys", label: Copy.ApiKeys, icon: KeyIcon },
    { id: "connect", label: Copy.Connect, icon: PlugIcon },
    ...(showWorkspace
      ? ([
          { id: "members", label: Copy.Members, icon: UsersIcon },
          { id: "workspace", label: Copy.Workspace, icon: BuildingIcon },
        ] as const)
      : []),
  ]);
</script>

<div class="flex h-screen flex-col">
  <header class="flex items-center gap-1 border-b px-3 py-2">
    <Button variant="ghost" size="sm" class="gap-1.5" onclick={() => router.navigate(BoardPath)}>
      <ArrowLeftIcon class="size-4" />
      {Copy.Back}
    </Button>
    <div class="font-semibold">{Copy.Settings}</div>
  </header>

  <div
    class="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-6 overflow-y-auto px-4 py-6 md:flex-row md:gap-10 md:px-6 md:py-10"
  >
    <nav class="flex shrink-0 gap-1 overflow-x-auto md:w-48 md:flex-col">
      {#each items as item (item.id)}
        {@const Icon = item.icon}
        <button
          type="button"
          onclick={() => router.navigate(settingsPath(item.id))}
          class={cn(
            "flex shrink-0 items-center gap-2 rounded-md px-3 py-2 text-sm transition",
            current === item.id
              ? "bg-accent font-medium text-accent-foreground"
              : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
          )}
        >
          <Icon class="size-4" />
          {item.label}
        </button>
      {/each}
    </nav>

    <main class="min-w-0 flex-1 pb-10">
      {@render children()}
    </main>
  </div>
</div>
