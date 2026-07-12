<script lang="ts">
  import { Copy } from "$shared/copy.js";
  import AccountMenu from "$tasks/ui/auth/AccountMenu.svelte";
  import WorkspaceSwitcher from "$tasks/ui/auth/WorkspaceSwitcher.svelte";
  import logoDark from "../../brand/logo.png";
  import logoLight from "../../brand/logo-light.png";
</script>

<header class="flex items-center gap-2 border-b px-3 py-2">
  <!-- white-bodied robot on dark, dark-bodied on light -->
  <img src={logoLight} alt="" class="size-6 shrink-0 dark:hidden" />
  <img src={logoDark} alt="" class="hidden size-6 shrink-0 dark:block" />
  <div class="font-semibold">{Copy.AppName}</div>

  <!-- Workspace switcher (renders only when the board is Clerk-backed). -->
  <WorkspaceSwitcher />

  <div class="ml-auto">
    <AccountMenu />
  </div>
</header>
