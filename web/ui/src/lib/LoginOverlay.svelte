<script>
  import { authInfo, login } from "./api.js";

  let { onsuccess } = $props();

  let ready = $state(false);
  let custom = $state(false);
  let token = $state("");
  let error = $state("");
  let busy = $state(false);

  $effect(() => {
    (async () => {
      const info = await authInfo();
      if (info.mode === "none") { onsuccess?.(); return; }
      if (info.mode === "custom") {
        if (info.login_url) { location.href = info.login_url; return; }
        custom = true;
      }
      ready = true;
    })();
  });

  async function submit(e) {
    e.preventDefault();
    error = "";
    if (custom) { location.reload(); return; }
    if (!token.trim()) { error = "Token required"; return; }
    busy = true;
    const res = await login(token.trim());
    busy = false;
    if (res.ok) { token = ""; onsuccess?.(); }
    else error = res.error;
  }
</script>

{#if ready}
  <div class="fixed inset-0 z-[1000] flex items-center justify-center bg-base-100/80 backdrop-blur-sm">
    <form
      class="card w-[min(360px,90vw)] gap-3 border border-base-300 bg-base-200 p-7 shadow-2xl"
      onsubmit={submit}
    >
      <div class="text-xl font-bold tracking-wide text-primary">tasks</div>
      {#if custom}
        <p class="text-sm text-base-content/60">Authentication required.</p>
        <button class="btn btn-primary" type="submit">Reload</button>
      {:else}
        <p class="text-sm text-base-content/60">Enter your access token to continue.</p>
        <input
          class="input input-bordered w-full"
          type="password"
          placeholder="Access token"
          autocomplete="current-password"
          bind:value={token}
        />
        <button class="btn btn-primary" type="submit" disabled={busy}>Sign in</button>
      {/if}
      {#if error}<div class="text-xs text-error">{error}</div>{/if}
    </form>
  </div>
{/if}
