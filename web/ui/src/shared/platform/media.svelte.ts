/** Responsive breakpoints (matches Tailwind's `md`). */
export const Breakpoint = {
  Desktop: "(min-width: 768px)",
} as const;

/** A reactive `matches` for a media query. */
export function createMediaQuery(query: string) {
  const list = window.matchMedia(query);
  let matches = $state(list.matches);
  list.addEventListener("change", (event) => (matches = event.matches));
  return {
    get matches() {
      return matches;
    },
  };
}

/** Shared desktop-breakpoint state — one listener for the whole app, so the many
 *  per-row menus don't each register their own matchMedia handler. */
export const isDesktop = createMediaQuery(Breakpoint.Desktop);
