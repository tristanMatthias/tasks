<!--
  A pan/zoom canvas for a laid-out graph. Edges are one SVG layer; nodes are real
  task chips on top. One-finger / mouse drag pans, wheel / pinch zooms, a tap on a
  node selects it, double-tap/dbl-click re-roots the graph on it. Fits to view when
  the graph changes.
-->
<script lang="ts">
  import { untrack } from "svelte";
  import ZoomInIcon from "@lucide/svelte/icons/plus";
  import ZoomOutIcon from "@lucide/svelte/icons/minus";
  import FitIcon from "@lucide/svelte/icons/scan";
  import ExpandIcon from "@lucide/svelte/icons/maximize-2";
  import ShrinkIcon from "@lucide/svelte/icons/minimize-2";
  import { shortId, Status, type Task } from "$tasks/model/issue.js";
  import type { TaskFilter } from "$tasks/model/filter.js";
  import StatusDot from "$tasks/ui/StatusDot.svelte";
  import TypeBadge from "$tasks/ui/TypeBadge.svelte";
  import type { Graph } from "$board/model/graph.js";
  import { layoutGraph, NODE_W, NODE_H } from "./layout.js";

  interface Props {
    graph: Graph;
    byId: ReadonlyMap<string, Task>;
    filter: TaskFilter;
    focusId: string;
    selectedId?: string | null;
    onSelect: (id: string) => void;
    onFocus: (id: string) => void;
    isFullscreen?: boolean;
    onToggleFullscreen?: () => void;
  }
  let {
    graph,
    byId,
    filter,
    focusId,
    selectedId = null,
    onSelect,
    onFocus,
    isFullscreen = false,
    onToggleFullscreen,
  }: Props = $props();

  const layout = $derived(layoutGraph(graph));

  // Facet filtering HIDES nodes upstream in the builder, so here the filter is
  // only the search query, which HIGHLIGHTS matches.
  const query = $derived(filter.query.trim().toLowerCase());
  const isHit = (id: string, t: Task | undefined): boolean =>
    query.length > 0 && `${id} ${t?.title ?? ""}`.toLowerCase().includes(query);

  // Emphasis by direction so the line up to the focus is easy to thread: the
  // focus + everything UPSTREAM stays full; DOWNSTREAM (what it blocks/contains)
  // is dimmed.
  const nodeOpacity = (rank: number): number => (rank > 0 ? 0.6 : 1);

  let viewport = $state<HTMLDivElement | null>(null);
  let tx = $state(0);
  let ty = $state(0);
  let scale = $state(1);

  const MIN = 0.35;
  const MAX = 2;
  const clamp = (s: number) => Math.min(MAX, Math.max(MIN, s));

  function fit(): void {
    if (!viewport) return;
    const vw = viewport.clientWidth;
    const vh = viewport.clientHeight;
    const pad = 48;
    const w = layout.width || 1;
    const h = layout.height || 1;
    const s = clamp(Math.min((vw - pad) / w, (vh - pad) / h, 1.1));
    scale = s;
    tx = (vw - w * s) / 2;
    ty = (vh - h * s) / 2;
  }

  // Re-fit whenever the graph (its focus) changes.
  $effect(() => {
    void layout;
    untrack(() => fit());
  });

  // Re-fit after entering/leaving full-page, once the viewport has resized.
  $effect(() => {
    void isFullscreen;
    const id = requestAnimationFrame(() => untrack(() => fit()));
    return () => cancelAnimationFrame(id);
  });

  function zoomAt(cx: number, cy: number, factor: number): void {
    const s2 = clamp(scale * factor);
    if (s2 === scale) return;
    tx = cx - ((cx - tx) / scale) * s2;
    ty = cy - ((cy - ty) / scale) * s2;
    scale = s2;
  }
  function zoomButton(factor: number): void {
    if (!viewport) return;
    zoomAt(viewport.clientWidth / 2, viewport.clientHeight / 2, factor);
  }

  // ---- pointer pan + pinch + tap detection ----
  interface P {
    x: number;
    y: number;
    node: string | null;
  }
  const pointers = new Map<number, P>();
  let downX = 0;
  let downY = 0;
  let downNode: string | null = null;
  let moved = false;
  let pinchDist = 0;

  // Apple-Maps one-finger zoom: double-tap, then (without lifting on the 2nd tap)
  // drag vertically — up zooms in, down zooms out, about the tap point.
  let lastUpTime = 0;
  let lastUpX = 0;
  let lastUpY = 0;
  let zoomArmed = false; // 2nd tap is down; a vertical drag becomes a zoom
  let zoomActive = false;
  let zStartY = 0;
  let zStartScale = 1;
  let zAnchorX = 0;
  let zAnchorY = 0;

  function localXY(e: PointerEvent): { x: number; y: number } {
    const r = viewport!.getBoundingClientRect();
    return { x: e.clientX - r.left, y: e.clientY - r.top };
  }
  function nodeOf(target: EventTarget | null): string | null {
    const el = (target as HTMLElement | null)?.closest?.("[data-node]");
    return el?.getAttribute("data-node") ?? null;
  }

  function onPointerDown(e: PointerEvent): void {
    viewport?.setPointerCapture(e.pointerId);
    const { x, y } = localXY(e);
    pointers.set(e.pointerId, { x, y, node: nodeOf(e.target) });
    if (pointers.size === 1) {
      downX = x;
      downY = y;
      downNode = nodeOf(e.target);
      moved = false;
      // A second tap landing quickly near the first arms the zoom-drag.
      const now = performance.now();
      zoomArmed = now - lastUpTime < 300 && Math.hypot(x - lastUpX, y - lastUpY) < 32;
      zoomActive = false;
      if (zoomArmed) {
        zStartY = y;
        zStartScale = scale;
        zAnchorX = x;
        zAnchorY = y;
      }
    } else if (pointers.size === 2) {
      zoomArmed = false;
      const [a, b] = [...pointers.values()];
      pinchDist = Math.hypot(a.x - b.x, a.y - b.y);
    }
  }

  function onPointerMove(e: PointerEvent): void {
    const p = pointers.get(e.pointerId);
    if (!p) return;
    const { x, y } = localXY(e);
    if (pointers.size >= 2) {
      const prev = { x: p.x, y: p.y };
      p.x = x;
      p.y = y;
      const [a, b] = [...pointers.values()];
      const dist = Math.hypot(a.x - b.x, a.y - b.y);
      const mid = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
      if (pinchDist > 0) zoomAt(mid.x, mid.y, dist / pinchDist);
      pinchDist = dist;
      // also pan by this pointer's movement (halved, both contribute)
      tx += (x - prev.x) / 2;
      ty += (y - prev.y) / 2;
      moved = true;
      return;
    }
    // double-tap-drag zoom (one finger)
    if (zoomArmed) {
      if (!zoomActive && Math.abs(y - zStartY) > 4) zoomActive = true;
      if (zoomActive) {
        const target = clamp(zStartScale * Math.pow(2, (zStartY - y) / 180));
        zoomAt(zAnchorX, zAnchorY, target / scale);
        moved = true;
      }
      p.x = x;
      p.y = y;
      return;
    }
    // single-pointer pan
    tx += x - p.x;
    ty += y - p.y;
    p.x = x;
    p.y = y;
    if (Math.abs(x - downX) + Math.abs(y - downY) > 6) moved = true;
  }

  function onPointerUp(e: PointerEvent): void {
    const wasTap = !moved && pointers.size === 1 && !zoomActive;
    pointers.delete(e.pointerId);
    if (pointers.size < 2) pinchDist = 0;
    zoomArmed = false;
    zoomActive = false;
    if (wasTap) {
      lastUpTime = performance.now();
      lastUpX = downX;
      lastUpY = downY;
      if (downNode) onSelect(downNode);
    }
  }

  function onWheel(e: WheelEvent): void {
    e.preventDefault();
    const { x, y } = localXY(e as unknown as PointerEvent);
    zoomAt(x, y, e.deltaY < 0 ? 1.1 : 1 / 1.1);
  }

</script>

<div class="relative h-full min-h-0 w-full overflow-hidden">
  <div
    bind:this={viewport}
    class="graph-vp h-full w-full touch-none select-none"
    role="application"
    aria-label="Task graph"
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerup={onPointerUp}
    onpointercancel={onPointerUp}
    onwheel={onWheel}
  >
    <div class="absolute left-0 top-0 origin-top-left will-change-transform" style="transform: translate({tx}px, {ty}px) scale({scale})">
      <!-- edges -->
      <svg width={layout.width} height={layout.height} class="pointer-events-none absolute left-0 top-0 overflow-visible">
        <defs>
          <!-- context-stroke → the arrow inherits its line's colour; orient=auto
               rotates it to the direction the line arrives from. -->
          <marker id="gh-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto">
            <path d="M0.5,0.5 L9,5 L0.5,9.5 Z" fill="context-stroke" />
          </marker>
        </defs>
        {#each layout.edges as e (e.from + e.to)}
          <path
            d={e.path}
            fill="none"
            stroke={e.kind === "blocks" ? "var(--muted-foreground)" : "var(--border)"}
            stroke-width="1.5"
            stroke-dasharray={e.kind === "parent" ? "5 5" : "0"}
            marker-end="url(#gh-arrow)"
            opacity="0.8"
          />
        {/each}
      </svg>

      <!-- nodes -->
      {#each layout.nodes as n (n.id)}
        {@const t = byId.get(n.id)}
        <div
          data-node={n.id}
          class={`absolute cursor-pointer rounded-lg border bg-card px-2.5 py-1.5 shadow-sm transition-all hover:border-primary/50 ${
            n.id !== focusId && isHit(n.id, t) ? "ring-4 ring-[#e0af68]/30" : ""
          }`}
          class:ring-2={n.id === focusId}
          class:ring-primary={n.id === focusId}
          class:border-primary={n.id === selectedId && n.id !== focusId}
          style="left:{n.x}px; top:{n.y}px; width:{NODE_W}px; height:{NODE_H}px; opacity:{nodeOpacity(n.rank)}"
          ondblclick={() => onFocus(n.id)}
          role="button"
          tabindex="-1"
        >
          <div class="flex items-center gap-1.5" class:opacity-55={t?.status === Status.Closed}>
            {#if t}<StatusDot status={t.status} size="sm" />{/if}
            {#if t}<TypeBadge type={t.issue_type} />{/if}
            <code class="ml-auto shrink-0 font-mono text-[10px] text-muted-foreground">{shortId(n.id)}</code>
          </div>
          <div
            class="mt-0.5 truncate text-[12px] leading-tight"
            class:line-through={t?.status === Status.Closed}
            class:text-muted-foreground={t?.status === Status.Closed}
          >
            {t?.title ?? n.id}
          </div>
        </div>
      {/each}
    </div>
  </div>

  <!-- controls -->
  <div class="absolute bottom-3 right-3 flex flex-col gap-1 rounded-lg border bg-card/90 p-1 backdrop-blur">
    <button class="grid size-7 place-items-center rounded text-muted-foreground hover:bg-accent hover:text-foreground" onclick={() => zoomButton(1.2)} aria-label="Zoom in"><ZoomInIcon class="size-4" /></button>
    <button class="grid size-7 place-items-center rounded text-muted-foreground hover:bg-accent hover:text-foreground" onclick={() => zoomButton(1 / 1.2)} aria-label="Zoom out"><ZoomOutIcon class="size-4" /></button>
    <button class="grid size-7 place-items-center rounded text-muted-foreground hover:bg-accent hover:text-foreground" onclick={fit} aria-label="Fit to view"><FitIcon class="size-4" /></button>
    {#if onToggleFullscreen}
      <button
        class="grid size-7 place-items-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
        onclick={onToggleFullscreen}
        aria-label={isFullscreen ? "Exit full page" : "Full page"}
      >
        {#if isFullscreen}<ShrinkIcon class="size-4" />{:else}<ExpandIcon class="size-4" />{/if}
      </button>
    {/if}
  </div>
</div>

<style>
  .graph-vp {
    background-image: radial-gradient(circle, var(--border) 1px, transparent 1px);
    background-size: 22px 22px;
  }
</style>
