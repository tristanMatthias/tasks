<!-- Members of the active workspace: list + roles + remove, invite, pending invites. -->
<script lang="ts">
  import PageHeader from "$lib/components/PageHeader.svelte";
  import Panel from "$lib/components/Panel.svelte";
  import Field from "$lib/components/Field.svelte";
  import { Input } from "$lib/components/ui/input/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Avatar, AvatarImage, AvatarFallback } from "$lib/components/ui/avatar/index.js";
  import TrashIcon from "@lucide/svelte/icons/trash-2";
  import {
    workspaces,
    roleLabel,
    type Member,
    type Invite,
    type Role,
  } from "$shared/auth/workspaces.svelte.js";
  import { Copy } from "$shared/copy.js";

  let members = $state<Member[]>([]);
  let invites = $state<Invite[]>([]);
  let roles = $state<Role[]>([]);
  let email = $state("");
  let inviteRole = $state("org:member");
  let busy = $state(false);
  let started = false;

  const isOrg = $derived(workspaces.available && !workspaces.active.isPersonal);
  const isAdmin = $derived(workspaces.isAdmin);

  $effect(() => {
    if (started) return;
    started = true;
    void init();
  });

  async function init(): Promise<void> {
    await workspaces.ensureLoaded();
    if (!workspaces.available || workspaces.active.isPersonal) return;
    roles = await workspaces.getRoles();
    inviteRole = roles.find((r) => /member/i.test(r.key))?.key ?? roles[0]?.key ?? "org:member";
    await Promise.all([loadMembers(), loadInvites()]);
  }

  async function loadMembers(): Promise<void> {
    members = await workspaces.getMembers();
  }
  async function loadInvites(): Promise<void> {
    invites = workspaces.isAdmin ? await workspaces.getInvitations() : [];
  }

  function initials(name: string): string {
    return name.trim().slice(0, 2).toUpperCase() || "?";
  }

  async function invite(event: Event): Promise<void> {
    event.preventDefault();
    const addr = email.trim();
    if (!addr || busy) return;
    busy = true;
    const ok = await workspaces.inviteMember(addr, inviteRole);
    busy = false;
    if (ok) {
      email = "";
      await loadInvites();
    }
  }

  async function changeRole(m: Member, role: string): Promise<void> {
    if (role === m.role) return;
    if (await workspaces.updateMemberRole(m.userId, role)) await loadMembers();
  }
  async function remove(m: Member): Promise<void> {
    if (await workspaces.removeMember(m.userId)) await loadMembers();
  }
  async function revoke(i: Invite): Promise<void> {
    await i.revoke();
    await loadInvites();
  }
</script>

<PageHeader title={Copy.Members} description={Copy.MembersDesc} />

{#if !isOrg}
  <Panel>
    <p class="py-6 text-center text-sm text-muted-foreground">{Copy.MembersDesc}</p>
  </Panel>
{:else}
  <div class="space-y-6">
    {#if isAdmin}
      <Panel title={Copy.InviteMember} description={Copy.InviteMemberDesc}>
        <form onsubmit={invite} class="flex flex-wrap items-end gap-2">
          <div class="min-w-48 flex-1">
            <Field label={Copy.EmailAddress} for="invite-email">
              <Input id="invite-email" type="email" bind:value={email} placeholder={Copy.EmailPlaceholder} />
            </Field>
          </div>
          <Field label={Copy.Role} for="invite-role">
            <select
              id="invite-role"
              bind:value={inviteRole}
              class="border-input bg-transparent h-9 rounded-md border px-3 text-sm shadow-xs"
            >
              {#each roles as r (r.key)}
                <option value={r.key}>{r.label}</option>
              {/each}
            </select>
          </Field>
          <Button type="submit" disabled={busy || !email.trim()}>{Copy.SendInvite}</Button>
        </form>
      </Panel>
    {/if}

    <Panel title={Copy.Members} flush>
      {#if members.length === 0}
        <p class="px-5 py-8 text-center text-sm text-muted-foreground">{Copy.NoMembers}</p>
      {:else}
        <ul class="divide-y">
          {#each members as m (m.id)}
            <li class="flex items-center gap-3 px-5 py-3">
              <Avatar class="size-8">
                <AvatarImage src={m.imageUrl} alt="" />
                <AvatarFallback class="text-xs">{initials(m.name)}</AvatarFallback>
              </Avatar>
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm font-medium">
                  {m.name || m.email}
                  {#if m.userId === workspaces.userId}
                    <span class="text-xs font-normal text-muted-foreground">({Copy.You})</span>
                  {/if}
                </div>
                <div class="truncate text-xs text-muted-foreground">{m.email}</div>
              </div>
              {#if isAdmin && m.userId !== workspaces.userId}
                <select
                  value={m.role}
                  onchange={(e) => changeRole(m, e.currentTarget.value)}
                  class="border-input bg-transparent h-8 rounded-md border px-2 text-xs shadow-xs"
                >
                  {#each roles as r (r.key)}
                    <option value={r.key}>{r.label}</option>
                  {/each}
                </select>
                <Button
                  variant="ghost"
                  size="icon"
                  class="size-8 text-muted-foreground hover:text-destructive"
                  title={Copy.Remove}
                  onclick={() => remove(m)}
                >
                  <TrashIcon class="size-4" />
                </Button>
              {:else}
                <span class="text-xs text-muted-foreground">{roleLabel(m.role)}</span>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </Panel>

    {#if isAdmin && invites.length > 0}
      <Panel title={Copy.PendingInvitations} flush>
        <ul class="divide-y">
          {#each invites as i (i.id)}
            <li class="flex items-center gap-3 px-5 py-3">
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm">{i.email}</div>
                <div class="text-xs text-muted-foreground">{roleLabel(i.role)}</div>
              </div>
              <Button variant="ghost" size="sm" class="text-muted-foreground hover:text-destructive" onclick={() => revoke(i)}>
                {Copy.Revoke}
              </Button>
            </li>
          {/each}
        </ul>
      </Panel>
    {/if}
  </div>
{/if}
