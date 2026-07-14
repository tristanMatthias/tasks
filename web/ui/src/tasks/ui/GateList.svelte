<!--
  Read-only list of a task's acceptance gates. Each gate is a check that must
  pass (verified via the CLI) before the task can close. Gates are defined and
  verified from the CLI/API — this view only reflects their state.
-->
<script lang="ts">
  import CircleCheckIcon from "@lucide/svelte/icons/circle-check";
  import CircleDashedIcon from "@lucide/svelte/icons/circle-dashed";
  import Badge from "$lib/components/Badge.svelte";
  import { GateStatus, type Gate } from "$tasks/model/issue.js";
  import { GATE_STATUS_COLOR_VAR, GATE_STATUS_LABEL } from "$tasks/model/appearance.js";

  interface Props {
    gates: readonly Gate[];
  }
  let { gates }: Props = $props();

  const isVerified = (g: Gate) => g.status === GateStatus.Verified;
</script>

<ul data-testid="gates" class="flex flex-col gap-1.5">
  {#each gates as gate (gate.id)}
    {@const verified = isVerified(gate)}
    <li
      data-testid="gate-item"
      data-gate-status={gate.status}
      class="rounded-md border bg-card/50 px-2.5 py-2"
    >
      <div class="flex items-start gap-2">
        {#if verified}
          <CircleCheckIcon class="mt-0.5 size-4 shrink-0" style="color: {GATE_STATUS_COLOR_VAR[gate.status]}" />
        {:else}
          <CircleDashedIcon class="mt-0.5 size-4 shrink-0" style="color: {GATE_STATUS_COLOR_VAR[gate.status]}" />
        {/if}

        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <span class="min-w-0 truncate text-sm font-medium">{gate.description || gate.command}</span>
            <span class="ml-auto shrink-0">
              <Badge color={GATE_STATUS_COLOR_VAR[gate.status]} size="md">
                {GATE_STATUS_LABEL[gate.status]}
              </Badge>
            </span>
          </div>

          {#if gate.description && gate.command}
            <code class="mt-1 block truncate font-mono text-xs text-muted-foreground">$ {gate.command}</code>
          {/if}

          {#if verified && (gate.verified_by || gate.exit_code !== undefined)}
            <div class="mt-1 text-[11px] text-muted-foreground">
              Verified{gate.verified_by ? ` by ${gate.verified_by}` : ""}{gate.exit_code !== undefined
                ? ` · exit ${gate.exit_code}`
                : ""}
            </div>
          {/if}
        </div>
      </div>
    </li>
  {/each}
</ul>
