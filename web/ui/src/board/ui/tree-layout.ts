/** Layout tokens for the tree. Keeps pixel math out of the components. */
export const TreeLayout = {
  /** Left padding of a depth-0 row, in pixels. */
  BasePaddingPx: 6,
  /** Extra left padding added per hierarchy level, in pixels. */
  IndentPerLevelPx: 14,
  /** Fixed row height, in pixels — the basis for virtualized scrolling. */
  RowHeightPx: 28,
  /** Rows rendered above/below the viewport to avoid blank edges while scrolling. */
  Overscan: 8,
} as const;

/** The CSS `padding-left` value for a row at the given depth. */
export function rowPaddingLeft(depth: number): string {
  return `${TreeLayout.BasePaddingPx + depth * TreeLayout.IndentPerLevelPx}px`;
}
