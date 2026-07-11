<script>
  import { listKeys, createKey, revokeKey } from "./api.js";

  let { onclose, onauth } = $props();

  let keys = $state([]);
  let label = $state("");
  let secret = $state("");
  let error = $state("");
  let copied = $state(false);

  async function load() {
    error = "";
    try {
      const res = await listKeys();
      if (!res.auth) { onauth?.(); return; }
      keys = res.keys;
    } catch (e) { error = e.message; }
  }

  $effect(() => { load(); });

  async function create(e) {
    e.preventDefault();
    error = "";
    try {
      const k = await createKey(label.trim());
      secret = k.secret;
      label = "";
      await load();
    } catch (e) { error = e.message; }
  }

  async function revoke(id) {
    if (!confirm("Revoke key " + id + "? Anything using it stops working immediately.")) return;
    try { await revokeKey(id); await load(); } catch (e) { error = e.message; }
  }

  async function copy() {
    try { await navigator.clipboard.writeText(secret); copied = true; setTimeout(() => (copied = false), 1200); } catch (_) {}
  }

  const fmt = (ts) => (ts ? ts.slice(0, 10) : "—");
</script>

<div
  class="fixed inset-0 z-[1000] flex items-center justify-center bg-base-100/80 p-4 backdrop-blur-sm"
  onclick={(e) => { if (e.target === e.currentTarget) onclose?.(); }}
  role="presentation"
>
  <div class="card max-h-[86vh] w-[min(620px,94vw)] overflow-auto border border-base-300 bg-base-200 p-6 shadow-2xl">
    <div class="flex items-center justify-between">
      <h2 class="text-lg font-bold text-primary">API keys</h2>
      <button class="btn btn-ghost btn-sm btn-circle text-lg" onclick={() => onclose?.()} aria-label="Close">✕</button>
    </div>
    <p class="mt-1 text-[12.5px] text-base-content/60">
      Keys let bots and agents (Claude Code, the CLI, MCP) authenticate to this board. Treat them like passwords.
    </p>

    <form class="mt-3 flex gap-2" onsubmit={create}>
      <input class="input input-bordered input-sm flex-1" placeholder="Label (e.g. claude-web, ci)" bind:value={label} />
      <button class="btn btn-primary btn-sm" type="submit">Create key</button>
    </form>

    {#if secret}
      <div class="mt-3 rounded-lg border border-primary/40 bg-primary/10 p-3">
        <div class="text-xs">New key — copy it now, it won't be shown again:</div>
        <div class="mt-2 flex items-center gap-2">
          <code class="flex-1 break-all rounded bg-base-100 px-2.5 py-2 text-[12.5px]">{secret}</code>
          <button class="btn btn-primary btn-sm" onclick={copy} type="button">{copied ? "Copied" : "Copy"}</button>
        </div>
      </div>
    {/if}

    {#if error}<div class="mt-2 text-xs text-error">{error}</div>{/if}

    <div class="mt-3 flex flex-col gap-1.5">
      {#if keys.length === 0}
        <div class="py-1.5 text-sm text-base-content/60">No keys yet.</div>
      {:else}
        {#each keys as k (k.id)}
          <div class="flex items-center justify-between gap-2.5 rounded-lg border border-base-300 px-3 py-2" class:opacity-55={k.revoked_at}>
            <div class="flex min-w-0 flex-wrap items-baseline gap-2 text-[12.5px]">
              <span class="font-mono">{k.id}</span>
              <span class="font-semibold">{k.label || "—"}</span>
              {#if k.revoked_at}
                <span class="badge badge-sm badge-error badge-outline">revoked</span>
              {:else}
                <span class="badge badge-sm badge-success badge-outline">active</span>
              {/if}
              <span class="text-[11.5px] text-base-content/50">created {fmt(k.created_at)} · last used {k.last_used_at ? fmt(k.last_used_at) : "never"}</span>
            </div>
            {#if !k.revoked_at}
              <button class="btn btn-ghost btn-xs text-error" onclick={() => revoke(k.id)}>Revoke</button>
            {/if}
          </div>
        {/each}
      {/if}
    </div>
  </div>
</div>
