<!-- Header menu: go to Settings (keys/connect/account) and, when signed in, log out. -->
<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { toggleMode } from "mode-watcher";
  import SettingsIcon from "@lucide/svelte/icons/settings";
  import LogOutIcon from "@lucide/svelte/icons/log-out";
  import SunIcon from "@lucide/svelte/icons/sun";
  import MoonIcon from "@lucide/svelte/icons/moon";
  import { session } from "$shared/auth/session.svelte.js";
  import { router } from "$shared/router/router.svelte.js";
  import { settingsPath } from "$shared/router/routes.js";
  import { Copy } from "$shared/copy.js";
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger>
    {#snippet child({ props })}
      <Button {...props} variant="ghost" size="icon" class="size-8" title={Copy.Settings}>
        <SettingsIcon class="size-4" />
      </Button>
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="end" class="w-44">
    <DropdownMenu.Item onSelect={() => router.navigate(settingsPath())}>
      <SettingsIcon class="size-4" />
      {Copy.Settings}
    </DropdownMenu.Item>
    <DropdownMenu.Item onSelect={(e) => (e.preventDefault(), toggleMode())}>
      <SunIcon class="size-4 dark:hidden" />
      <MoonIcon class="hidden size-4 dark:block" />
      {Copy.ToggleTheme}
    </DropdownMenu.Item>
    {#if session.canLogout}
      <DropdownMenu.Separator />
      <DropdownMenu.Item variant="destructive" onSelect={() => session.logout()}>
        <LogOutIcon class="size-4" />
        {Copy.LogOut}
      </DropdownMenu.Item>
    {/if}
  </DropdownMenu.Content>
</DropdownMenu.Root>
