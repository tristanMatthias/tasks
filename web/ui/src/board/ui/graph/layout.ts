/**
 * A lightweight layered (Sugiyama-lite) layout, flowing LEFT → RIGHT: upstream
 * columns on the left, the focus in the middle, downstream on the right. Ranks
 * become columns, so a wide fan-out grows the graph in HEIGHT (vertically
 * scrollable) rather than width — which fits real screens (and portrait phones)
 * far better. A few barycenter sweeps keep edge crossings down.
 */
import type { EdgeKind, Graph } from "$board/model/graph.js";

export const NODE_W = 186;
export const NODE_H = 46;
const COL_GAP = 76; // horizontal gap between ranks
const NODE_GAP = 16; // vertical gap within a column

export interface PositionedNode {
  id: string;
  rank: number;
  x: number;
  y: number;
}
export interface PositionedEdge {
  from: string;
  to: string;
  kind: EdgeKind;
  path: string;
}
export interface Layout {
  nodes: PositionedNode[];
  edges: PositionedEdge[];
  width: number;
  height: number;
}

export function layoutGraph(graph: Graph): Layout {
  const byRank = new Map<number, string[]>();
  for (const n of graph.nodes) {
    const arr = byRank.get(n.rank);
    if (arr) arr.push(n.id);
    else byRank.set(n.rank, [n.id]);
  }
  const ranks = [...byRank.keys()].sort((a, b) => a - b);
  const order = new Map(ranks.map((r) => [r, byRank.get(r)!] as const));

  const adj = new Map<string, string[]>();
  for (const e of graph.edges) {
    (adj.get(e.from) ?? adj.set(e.from, []).get(e.from)!).push(e.to);
    (adj.get(e.to) ?? adj.set(e.to, []).get(e.to)!).push(e.from);
  }
  const idx = new Map<string, number>();
  const reindex = () => {
    for (const r of ranks) order.get(r)!.forEach((id, i) => idx.set(id, i));
  };
  reindex();
  const bary = (id: string): number => {
    const ns = adj.get(id) ?? [];
    if (!ns.length) return idx.get(id) ?? 0;
    return ns.reduce((s, n) => s + (idx.get(n) ?? 0), 0) / ns.length;
  };
  for (let sweep = 0; sweep < 5; sweep++) {
    for (const r of ranks) order.get(r)!.slice().sort((a, b) => bary(a) - bary(b)).forEach((id, i) => order.get(r)![i] = id);
    reindex();
  }

  const colHeight = (r: number) => order.get(r)!.length * (NODE_H + NODE_GAP) - NODE_GAP;
  const maxH = Math.max(0, ...ranks.map(colHeight));

  const xy = new Map<string, { x: number; y: number }>();
  const nodes: PositionedNode[] = [];
  ranks.forEach((r, ci) => {
    const col = order.get(r)!;
    const x = ci * (NODE_W + COL_GAP);
    const start = (maxH - colHeight(r)) / 2;
    col.forEach((id, i) => {
      const y = start + i * (NODE_H + NODE_GAP);
      xy.set(id, { x, y });
      nodes.push({ id, rank: r, x, y });
    });
  });

  const edges: PositionedEdge[] = graph.edges.map((e) => {
    const a = xy.get(e.from)!;
    const b = xy.get(e.to)!;
    const x1 = a.x + NODE_W;
    const y1 = a.y + NODE_H / 2;
    const x2 = b.x;
    const y2 = b.y + NODE_H / 2;
    const mx = (x1 + x2) / 2;
    return { from: e.from, to: e.to, kind: e.kind, path: `M${x1},${y1} C${mx},${y1} ${mx},${y2} ${x2},${y2}` };
  });

  return {
    nodes,
    edges,
    width: ranks.length ? ranks.length * (NODE_W + COL_GAP) - COL_GAP : 0,
    height: maxH,
  };
}
