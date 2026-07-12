<script lang="ts">
  import PageHeader from "$lib/components/PageHeader.svelte";
  import Panel from "$lib/components/Panel.svelte";
  import Field from "$lib/components/Field.svelte";
  import { Input } from "$lib/components/ui/input/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import TrashIcon from "@lucide/svelte/icons/trash-2";
  import CopyIcon from "@lucide/svelte/icons/copy";
  import CheckIcon from "@lucide/svelte/icons/check";
  import { listKeys, createKey, revokeKey, type ApiKey } from "$shared/auth/auth.js";
  import { copyText } from "$shared/platform/clipboard.js";
  import { Copy } from "$shared/copy.js";

  let keys = $state<ApiKey[]>([]);
  let label = $state("");
  let busy = $state(false);
  // The most recently minted key — its secret is shown once, then never again.
  let minted = $state<ApiKey | null>(null);
  let copied = $state(false);

  const active = $derived(keys.filter((k) => !k.revoked_at));

  async function reload(): Promise<void> {
    keys = await listKeys();
  }
  reload();

  async function create(event: Event): Promise<void> {
    event.preventDefault();
    if (busy) return;
    busy = true;
    const key = await createKey(label.trim());
    busy = false;
    if (key) {
      minted = key;
      copied = false;
      label = "";
      await reload();
    }
  }

  async function revoke(id: string): Promise<void> {
    if (await revokeKey(id)) {
      if (minted?.id === id) minted = null;
      await reload();
    }
  }

  async function copySecret(): Promise<void> {
    if (minted?.secret && (await copyText(minted.secret))) copied = true;
  }

  function fmtDate(iso?: string): string {
    return iso ? new Date(iso).toLocaleDateString() : Copy.Never;
  }
</script>

<PageHeader title={Copy.ApiKeys} description={Copy.KeysPageDesc} />

<div class="space-y-6">
  <Panel title={Copy.CreateKey} description={Copy.ApiKeysBlurb}>
    <form onsubmit={create} class="flex items-end gap-2">
      <div class="flex-1">
        <Field label="Label" for="key-label">
          <Input id="key-label" bind:value={label} placeholder={Copy.KeyLabelPlaceholder} />
        </Field>
      </div>
      <Button type="submit" disabled={busy}>{Copy.Create}</Button>
    </form>

    {#if minted?.secret}
      <div class="mt-4 space-y-1.5 rounded-md border border-primary/40 bg-primary/5 p-3">
        <p class="text-xs text-muted-foreground">{Copy.CopyTokenOnce}</p>
        <div class="flex items-center gap-2">
          <code class="min-w-0 flex-1 truncate font-mono text-xs">{minted.secret}</code>
          <Button variant="outline" size="sm" class="shrink-0 gap-1.5" onclick={copySecret}>
            {#if copied}<CheckIcon class="size-3.5" />{:else}<CopyIcon class="size-3.5" />{/if}
            {copied ? Copy.Copied : Copy.Copy}
          </Button>
        </div>
      </div>
    {/if}
  </Panel>

  <Panel title={Copy.YourKeys} flush>
    {#if active.length === 0}
      <p class="px-5 py-8 text-center text-sm text-muted-foreground">{Copy.NoKeys}</p>
    {:else}
      <ul class="divide-y">
        {#each active as key (key.id)}
          <li class="flex items-center gap-3 px-5 py-3">
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium">{key.label || key.id}</div>
              <div class="text-xs text-muted-foreground">
                <code class="font-mono">{key.id}</code> · {Copy.LastUsed}: {fmtDate(key.last_used_at)}
              </div>
            </div>
            <Button
              variant="ghost"
              size="icon"
              class="size-8 text-muted-foreground hover:text-destructive"
              title={Copy.Revoke}
              onclick={() => revoke(key.id)}
            >
              <TrashIcon class="size-4" />
            </Button>
          </li>
        {/each}
      </ul>
    {/if}
  </Panel>
</div>
