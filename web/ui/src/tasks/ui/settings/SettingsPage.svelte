<!-- Renders the settings section matching the current route inside the shell. -->
<script lang="ts">
  import { router } from "$shared/router/router.svelte.js";
  import { settingsSectionFromPath } from "$shared/router/routes.js";
  import SettingsLayout from "./SettingsLayout.svelte";
  import AccountPage from "./AccountPage.svelte";
  import KeysPage from "./KeysPage.svelte";
  import ConnectPage from "./ConnectPage.svelte";
  import MembersPage from "./MembersPage.svelte";
  import WorkspacePage from "./WorkspacePage.svelte";

  const section = $derived(settingsSectionFromPath(router.path));
</script>

<SettingsLayout>
  {#if section === "keys"}
    <KeysPage />
  {:else if section === "connect"}
    <ConnectPage />
  {:else if section === "members"}
    <MembersPage />
  {:else if section === "workspace"}
    <WorkspacePage />
  {:else}
    <AccountPage />
  {/if}
</SettingsLayout>
