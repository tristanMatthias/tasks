<!-- Active workspace settings: rename (admin) + leave/delete (danger zone). -->
<script lang="ts">
  import PageHeader from "$lib/components/PageHeader.svelte";
  import Panel from "$lib/components/Panel.svelte";
  import Field from "$lib/components/Field.svelte";
  import * as Dialog from "$lib/components/ui/dialog/index.js";
  import { Input } from "$lib/components/ui/input/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { workspaces } from "$shared/auth/workspaces.svelte.js";
  import { Copy } from "$shared/copy.js";

  let name = $state("");
  let busy = $state(false);
  let confirmDelete = $state(false);
  let confirmLeave = $state(false);
  let started = false;

  const isOrg = $derived(workspaces.available && !workspaces.active.isPersonal);
  const isAdmin = $derived(workspaces.isAdmin);

  $effect(() => {
    if (started) return;
    started = true;
    workspaces.ensureLoaded().then(() => {
      name = workspaces.active.name;
    });
  });

  async function rename(event: Event): Promise<void> {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed || busy || trimmed === workspaces.active.name) return;
    busy = true;
    await workspaces.updateName(trimmed);
    busy = false;
  }
</script>

<PageHeader title={Copy.WorkspaceSettings} description={Copy.WorkspaceSettingsDesc} />

{#if !isOrg}
  <Panel>
    <p class="py-6 text-center text-sm text-muted-foreground">{Copy.WorkspaceSettingsDesc}</p>
  </Panel>
{:else}
  <div class="space-y-6">
    <Panel title={Copy.Workspace}>
      <form onsubmit={rename} class="flex items-end gap-2">
        <div class="flex-1">
          <Field label={Copy.WorkspaceName} for="ws-rename">
            <Input id="ws-rename" bind:value={name} disabled={!isAdmin} />
          </Field>
        </div>
        {#if isAdmin}
          <Button type="submit" disabled={busy || !name.trim() || name.trim() === workspaces.active.name}>
            {Copy.Save}
          </Button>
        {/if}
      </form>
    </Panel>

    <Panel title={Copy.DangerZone}>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="min-w-0">
          <div class="text-sm font-medium">{Copy.LeaveWorkspace}</div>
          <div class="text-xs text-muted-foreground">{Copy.LeaveWorkspaceDesc}</div>
        </div>
        <Button variant="outline" onclick={() => (confirmLeave = true)}>{Copy.Leave}</Button>
      </div>

      {#if isAdmin}
        <div class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t pt-4">
          <div class="min-w-0">
            <div class="text-sm font-medium">{Copy.DeleteWorkspace}</div>
            <div class="text-xs text-muted-foreground">{Copy.DeleteWorkspaceDesc}</div>
          </div>
          <Button variant="destructive" onclick={() => (confirmDelete = true)}>{Copy.Delete}</Button>
        </div>
      {/if}
    </Panel>
  </div>

  <Dialog.Root bind:open={confirmDelete}>
    <Dialog.Content class="sm:max-w-md">
      <Dialog.Header>
        <Dialog.Title>{Copy.DeleteWorkspace}</Dialog.Title>
        <Dialog.Description>{Copy.DeleteWorkspaceDesc}</Dialog.Description>
      </Dialog.Header>
      <Dialog.Footer>
        <Button variant="outline" onclick={() => (confirmDelete = false)}>{Copy.Cancel}</Button>
        <Button variant="destructive" onclick={() => workspaces.destroy()}>{Copy.Delete}</Button>
      </Dialog.Footer>
    </Dialog.Content>
  </Dialog.Root>

  <Dialog.Root bind:open={confirmLeave}>
    <Dialog.Content class="sm:max-w-md">
      <Dialog.Header>
        <Dialog.Title>{Copy.LeaveWorkspace}</Dialog.Title>
        <Dialog.Description>{Copy.LeaveWorkspaceDesc}</Dialog.Description>
      </Dialog.Header>
      <Dialog.Footer>
        <Button variant="outline" onclick={() => (confirmLeave = false)}>{Copy.Cancel}</Button>
        <Button variant="destructive" onclick={() => workspaces.leave()}>{Copy.Leave}</Button>
      </Dialog.Footer>
    </Dialog.Content>
  </Dialog.Root>
{/if}
