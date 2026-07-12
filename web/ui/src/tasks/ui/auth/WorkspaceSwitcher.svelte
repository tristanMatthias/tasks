<!--
  Workspace switcher (Linear/Slack-style): the active workspace name with a
  dropdown to switch, create a new workspace, or jump to member management.
  Renders nothing when Clerk isn't available (local token/none dev mode).
-->
<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
  import * as Dialog from "$lib/components/ui/dialog/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Input } from "$lib/components/ui/input/index.js";
  import Field from "$lib/components/Field.svelte";
  import ChevronsUpDownIcon from "@lucide/svelte/icons/chevrons-up-down";
  import CheckIcon from "@lucide/svelte/icons/check";
  import PlusIcon from "@lucide/svelte/icons/plus";
  import UsersIcon from "@lucide/svelte/icons/users";
  import { workspaces } from "$shared/auth/workspaces.svelte.js";
  import { router } from "$shared/router/router.svelte.js";
  import { settingsPath } from "$shared/router/routes.js";
  import { Copy } from "$shared/copy.js";

  // Load Clerk once and keep the list live.
  $effect(() => {
    workspaces.ensureLoaded();
  });

  let creating = $state(false);
  let name = $state("");
  let busy = $state(false);

  async function submit(event: Event): Promise<void> {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed || busy) return;
    busy = true;
    await workspaces.create(trimmed); // switches + reloads into the new workspace
    // (unreachable after reload, but keep state sane if create throws)
    busy = false;
  }

  const select = (id: string) => {
    if (id !== workspaces.activeId) workspaces.switchTo(id);
  };
</script>

{#if workspaces.available}
  <DropdownMenu.Root>
    <DropdownMenu.Trigger>
      {#snippet child({ props })}
        <Button {...props} variant="ghost" size="sm" class="max-w-44 gap-1.5 px-2">
          <span class="truncate font-medium">{workspaces.active.name}</span>
          <ChevronsUpDownIcon class="size-3.5 shrink-0 text-muted-foreground" />
        </Button>
      {/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content align="start" class="w-56">
      <DropdownMenu.Label class="text-xs text-muted-foreground">{Copy.Workspaces}</DropdownMenu.Label>
      {#each workspaces.workspaces as ws (ws.id)}
        <DropdownMenu.Item onSelect={() => select(ws.id)}>
          <span class="truncate">{ws.name}</span>
          {#if ws.id === workspaces.activeId}
            <CheckIcon class="ml-auto size-4" />
          {/if}
        </DropdownMenu.Item>
      {/each}
      <DropdownMenu.Separator />
      {#if !workspaces.active.isPersonal}
        <DropdownMenu.Item onSelect={() => router.navigate(settingsPath("members"))}>
          <UsersIcon class="size-4" />
          {Copy.ManageMembers}
        </DropdownMenu.Item>
      {/if}
      <DropdownMenu.Item onSelect={() => (creating = true)}>
        <PlusIcon class="size-4" />
        {Copy.CreateWorkspace}
      </DropdownMenu.Item>
    </DropdownMenu.Content>
  </DropdownMenu.Root>

  <Dialog.Root bind:open={creating}>
    <Dialog.Content class="sm:max-w-md">
      <Dialog.Header>
        <Dialog.Title>{Copy.CreateWorkspace}</Dialog.Title>
        <Dialog.Description>{Copy.CreateWorkspaceDesc}</Dialog.Description>
      </Dialog.Header>
      <form onsubmit={submit} class="space-y-4">
        <Field label={Copy.WorkspaceName} for="ws-name">
          <Input id="ws-name" bind:value={name} placeholder={Copy.WorkspaceNamePlaceholder} autocomplete="off" />
        </Field>
        <Dialog.Footer>
          <Button type="button" variant="outline" onclick={() => (creating = false)}>{Copy.Cancel}</Button>
          <Button type="submit" disabled={busy || !name.trim()}>{Copy.Create}</Button>
        </Dialog.Footer>
      </form>
    </Dialog.Content>
  </Dialog.Root>
{/if}
