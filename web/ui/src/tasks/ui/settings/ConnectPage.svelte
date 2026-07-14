<script lang="ts">
  import PageHeader from "$lib/components/PageHeader.svelte";
  import Panel from "$lib/components/Panel.svelte";
  import { Button } from "$lib/components/ui/button/index.js";
  import CopyIcon from "@lucide/svelte/icons/copy";
  import CheckIcon from "@lucide/svelte/icons/check";
  import GithubIcon from "@lucide/svelte/icons/git-branch";
  import { mcpUrl } from "$shared/auth/auth.js";
  import { copyText } from "$shared/platform/clipboard.js";
  import { Copy } from "$shared/copy.js";

  const url = mcpUrl();

  // GitHub integration (only rendered when the server has the App configured).
  interface GhIntegration {
    connected: boolean;
    repos: string[];
    connect_url: string;
    app_slug: string;
  }
  let gh = $state<GhIntegration | null>(null);
  $effect(() => {
    fetch("/api/integrations/github")
      .then((r) => (r.ok ? (r.json() as Promise<GhIntegration>) : null))
      .then((d) => (gh = d))
      .catch(() => {});
  });
  const claudeCode = `claude mcp add --transport http tasks ${url} --header "Authorization: Bearer <your-key>"`;

  let copied = $state("");
  async function copy(text: string, id: string): Promise<void> {
    if (await copyText(text)) {
      copied = id;
      setTimeout(() => (copied = copied === id ? "" : copied), 1200);
    }
  }
</script>

{#snippet copyBtn(text: string, id: string)}
  <Button variant="outline" size="icon" class="size-8 shrink-0" onclick={() => copy(text, id)}>
    {#if copied === id}<CheckIcon class="size-3.5" />{:else}<CopyIcon class="size-3.5" />{/if}
  </Button>
{/snippet}

<PageHeader title={Copy.Connect} description={Copy.ConnectPageDesc} />

<div class="space-y-6">
  {#if gh?.app_slug}
    <Panel title="GitHub" description="Link a repo — PRs and commits then auto-update tickets.">
      {#if gh.connected && gh.repos.length}
        <ul data-testid="github-repos" class="mb-3 flex flex-col gap-1.5">
          {#each gh.repos as repo (repo)}
            <li class="flex items-center gap-2 text-sm">
              <GithubIcon class="size-4 text-muted-foreground" />
              <span class="font-mono">{repo}</span>
            </li>
          {/each}
        </ul>
        <Button variant="outline" size="sm" href={gh.connect_url} data-testid="github-connect">
          Add another repo
        </Button>
      {:else}
        <Button href={gh.connect_url} data-testid="github-connect" class="gap-1.5">
          <GithubIcon class="size-4" /> Connect GitHub
        </Button>
        <p class="mt-2 text-xs text-muted-foreground">
          Install the app on a repo; then “Closes &lt;id&gt;” in a PR closes that ticket.
        </p>
      {/if}
    </Panel>
  {/if}

  <Panel title={Copy.McpEndpoint} description="Same endpoint for every client.">
    <div class="flex items-center gap-2">
      <code class="min-w-0 flex-1 truncate rounded bg-muted px-2 py-1.5 font-mono text-xs">{url}</code>
      {@render copyBtn(url, "url")}
    </div>
  </Panel>

  <Panel title="Claude Code" description="Add the board as an MCP server with a key.">
    <div class="flex items-start gap-2">
      <code class="min-w-0 flex-1 whitespace-pre-wrap break-all rounded bg-muted px-2 py-1.5 font-mono text-xs"
        >{claudeCode}</code
      >
      {@render copyBtn(claudeCode, "cli")}
    </div>
    <p class="mt-2 text-xs text-muted-foreground">Mint a key in the API keys tab and drop it in above.</p>
  </Panel>

  <Panel title="claude.ai" description="Web connectors sign in with OAuth — no key needed.">
    <p class="text-sm text-muted-foreground">Add a Connector with the MCP endpoint above.</p>
  </Panel>
</div>
