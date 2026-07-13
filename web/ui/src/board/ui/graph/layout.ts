/**
 * A lightweight layered (Sugiyama-lite) layout: group nodes into rows by rank,
 * order each row to reduce edge crossings (barycenter sweeps), then assign
 * coordinates and cubic-bezier edge paths. Small graphs (a task's stack), so a
 * few sweeps are plenty and it stays fast on mobile.
 */
import type { EdgeKind, Graph } from "$board/model/graph.js";

export const NODE_W = 180;
export const NODE_H = 48;
const RANK_GAP = 66;
const NODE_GAP = 22;

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

  // adjacency for crossing reduction
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
    for (const r of ranks) order.set(r, order.get(r)!.slice().sort((a, b) => bary(a) - bary(b)));
    reindex();
  }

  const rowWidth = (r: number) => order.get(r)!.length * (NODE_W + NODE_GAP) - NODE_GAP;
  const maxW = Math.max(0, ...ranks.map(rowWidth));

  const xy = new Map<string, { x: number; y: number }>();
  const nodes: PositionedNode[] = [];
  ranks.forEach((r, ri) => {
    const row = order.get(r)!;
    const start = (maxW - rowWidth(r)) / 2;
    const y = ri * (NODE_H + RANK_GAP);
    row.forEach((id, i) => {
      const x = start + i * (NODE_W + NODE_GAP);
      xy.set(id, { x, y });
      nodes.push({ id, rank: r, x, y });
    });
  });

  const edges: PositionedEdge[] = graph.edges.map((e) => {
    const a = xy.get(e.from)!;
    const b = xy.get(e.to)!;
    const x1 = a.x + NODE_W / 2;
    const y1 = a.y + NODE_H;
    const x2 = b.x + NODE_W / 2;
    const y2 = b.y;
    const my = (y1 + y2) / 2;
    return { from: e.from, to: e.to, kind: e.kind, path: `M${x1},${y1} C${x1},${my} ${x2},${my} ${x2},${y2}` };
  });

  return {
    nodes,
    edges,
    width: maxW,
    height: ranks.length ? ranks.length * (NODE_H + RANK_GAP) - RANK_GAP : 0,
  };
}
