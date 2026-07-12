<!-- A page's title + subtitle. -->
<script lang="ts">
  let { title, description }: { title: string; description?: string } = $props();
</script>

<div class="mb-6 space-y-1">
  <h1 class="text-lg font-semibold tracking-tight">{title}</h1>
  {#if description}
    <p class="text-sm text-muted-foreground">{description}</p>
  {/if}
</div>
