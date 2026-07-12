/**
 * A runes-native binding for TanStack Virtual (`@tanstack/virtual-core`). The
 * core engine does the real work — measuring, range calculation, scroll/resize
 * observation — this wrapper just exposes its output as reactive `$state`, the
 * same job the official framework adapters do.
 */
import {
  Virtualizer,
  elementScroll,
  observeElementOffset,
  observeElementRect,
  type VirtualItem,
} from "@tanstack/virtual-core";

export interface VirtualListOptions {
  /** Reactive row count (call inside for reactivity). */
  count: () => number;
  /** The scrolling element (available after mount). */
  getScrollElement: () => HTMLElement | null;
  /** Row height in pixels. */
  estimateSize: () => number;
  /** Rows rendered beyond the viewport edges. */
  overscan: number;
}

export interface VirtualList {
  readonly items: VirtualItem[];
  readonly totalSize: number;
}

export function createVirtualList(options: VirtualListOptions): VirtualList {
  let items = $state<VirtualItem[]>([]);
  let totalSize = $state(0);

  const sync = (instance: Virtualizer<HTMLElement, Element>) => {
    items = instance.getVirtualItems();
    totalSize = instance.getTotalSize();
  };

  const instance = new Virtualizer<HTMLElement, Element>({
    count: options.count(),
    getScrollElement: options.getScrollElement,
    estimateSize: options.estimateSize,
    overscan: options.overscan,
    observeElementRect,
    observeElementOffset,
    scrollToFn: elementScroll,
    // Seed a viewport height so the first render already has rows (before the
    // real measurement lands), avoiding an empty first frame.
    initialRect: { width: 0, height: typeof window !== "undefined" ? window.innerHeight : 800 },
    onChange: (self) => sync(self),
  });

  // Compute the initial window synchronously (with initialRect) so `items` isn't
  // empty on the very first render.
  instance._willUpdate();
  sync(instance);

  // Mount once: attach scroll/resize observers; detach on destroy.
  $effect(() => instance._didMount());

  // Re-sync whenever the reactive row count changes.
  $effect(() => {
    instance.setOptions({
      ...instance.options,
      count: options.count(),
      getScrollElement: options.getScrollElement,
      estimateSize: options.estimateSize,
      overscan: options.overscan,
    });
    instance._willUpdate();
    sync(instance);
  });

  return {
    get items() {
      return items;
    },
    get totalSize() {
      return totalSize;
    },
  };
}
