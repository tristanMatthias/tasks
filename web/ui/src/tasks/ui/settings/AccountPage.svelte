<script lang="ts">
  import PageHeader from "$lib/components/PageHeader.svelte";
  import Panel from "$lib/components/Panel.svelte";
  import { Button } from "$lib/components/ui/button/index.js";
  import LogOutIcon from "@lucide/svelte/icons/log-out";
  import { session } from "$shared/auth/session.svelte.js";
  import { Copy } from "$shared/copy.js";

  const open = $derived(session.mode === "none");
</script>

<PageHeader title={Copy.Account} description={Copy.AccountDesc} />

<Panel title={Copy.Session}>
  {#snippet actions()}
    {#if session.canLogout}
      <Button variant="outline" size="sm" class="gap-1.5" onclick={() => session.logout()}>
        <LogOutIcon class="size-4" />
        {Copy.LogOut}
      </Button>
    {/if}
  {/snippet}

  <div class="flex items-center gap-3">
    <span class="size-2 shrink-0 rounded-full {open ? 'bg-muted-foreground' : 'bg-[var(--status-open)]'}"></span>
    <div class="min-w-0">
      <div class="text-sm font-medium">{open ? Copy.OpenAccess : Copy.SignedIn}</div>
      <div class="text-xs text-muted-foreground">
        {open ? Copy.OpenAccessDesc : `${Copy.AuthModeLabel}: ${session.mode}`}
      </div>
    </div>
  </div>
</Panel>
