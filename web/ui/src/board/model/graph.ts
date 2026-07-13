/**
 * Graphs of a task in the context of its "stack": everything upstream (its
 * blockers + ancestors) and everything downstream (what it blocks + its
 * subtree), as a directed graph ranked by signed distance from the focus.
 *
 * It's a small registry so we can add more graph kinds later — each kind is just
 * a pure `(tasks, focusId) -> Graph`. Edges always point downstream (blocker →
 * blocked, parent → child), so the layout flows top (upstream) to bottom.
 */
import { buildHierarchy, parentId } from "$tasks/model/hierarchy.js";
import { blockedByIds, blockingTasks, type Task } from "$tasks/model/issue.js";

export type EdgeKind = "blocks" | "parent";

/** A node id + its signed rank (<0 upstream, 0 = focus, >0 downstream). */
export interface GraphNode {
  id: string;
  rank: number;
}
export interface GraphEdge {
  from: string;
  to: string;
  kind: EdgeKind;
}
export interface Graph {
  focusId: string;
  nodes: GraphNode[];
  edges: GraphEdge[];
}

/** A selectable way to build the graph. Add one = one function. */
export interface GraphKind {
  key: string;
  label: string;
  hint: string;
  build: (tasks: readonly Task[], focusId: string, visible?: (t: Task) => boolean) => Graph;
}

/** Everything shows unless a filter says otherwise. */
const ALL_VISIBLE = () => true;

interface Opts {
  blocks: boolean;
  parent: boolean;
  maxDepth?: number;
  maxNodes?: number;
  maxFanout?: number;
}

function buildDirected(
  tasks: readonly Task[],
  focusId: string,
  opts: Opts,
  visible: (t: Task) => boolean = ALL_VISIBLE,
): Graph {
  const byId = new Map(tasks.map((t) => [t.id, t] as const));
  const h = buildHierarchy(tasks);
  const maxDepth = opts.maxDepth ?? 4;
  const maxNodes = opts.maxNodes ?? 60;
  const maxFanout = opts.maxFanout ?? 12; // cap one node's neighbors per direction

  const ranks = new Map<string, number>([[focusId, 0]]);
  const edges: GraphEdge[] = [];
  const edgeKeys = new Set<string>();
  const addEdge = (from: string, to: string, kind: EdgeKind) => {
    const k = `${from}>${to}`;
    if (!edgeKeys.has(k)) {
      edgeKeys.add(k);
      edges.push({ from, to, kind });
    }
  };

  // Walk one direction (dir = -1 up, +1 down), assigning ranks + edges.
  const walk = (dir: -1 | 1) => {
    let frontier = [focusId];
    const seen = new Set([focusId]);
    for (let depth = 0; depth < maxDepth && frontier.length; depth++) {
      const next: string[] = [];
      for (const id of frontier) {
        const t = byId.get(id);
        if (!t) continue;
        const neighbors: Array<[string, EdgeKind]> = [];
        if (dir === -1) {
          if (opts.blocks) for (const b of blockedByIds(t)) if (byId.has(b)) neighbors.push([b, "blocks"]);
          if (opts.parent) {
            const p = parentId(h, id);
            if (p && byId.has(p)) neighbors.push([p, "parent"]);
          }
        } else {
          if (opts.blocks) for (const b of blockingTasks(id, tasks)) neighbors.push([b.id, "blocks"]);
          if (opts.parent) for (const c of h.children.get(id) ?? []) neighbors.push([c, "parent"]);
        }
        // Cap a single node's fan-out so one huge parent can't explode a column.
        for (const [nid, kind] of neighbors.slice(0, maxFanout)) {
          // A filtered-out node is a WALL: skip it and don't traverse through it,
          // so everything reachable only via it (its "children") drops out too.
          const nt = byId.get(nid);
          if (nt && !visible(nt)) continue;
          // Edge always points downstream so arrows read the same everywhere.
          if (dir === -1) addEdge(nid, id, kind);
          else addEdge(id, nid, kind);
          if (!seen.has(nid) && ranks.size < maxNodes) {
            seen.add(nid);
            ranks.set(nid, dir * (depth + 1));
            next.push(nid);
          }
        }
      }
      frontier = next;
    }
  };
  walk(-1);
  walk(1);

  const nodes = [...ranks].map(([id, rank]) => ({ id, rank }));
  return { focusId, nodes, edges: edges.filter((e) => ranks.has(e.from) && ranks.has(e.to)) };
}

export const GRAPH_KINDS: readonly GraphKind[] = [
  {
    key: "stack",
    label: "All",
    hint: "Dependencies + hierarchy — what blocks it and what it blocks, plus the epic breakdown",
    build: (tasks, focus, visible) => buildDirected(tasks, focus, { blocks: true, parent: true }, visible),
  },
  {
    key: "blocking",
    label: "Blocks",
    hint: "Just the dependency chain — what must finish first (left) and what this unblocks (right)",
    build: (tasks, focus, visible) => buildDirected(tasks, focus, { blocks: true, parent: false }, visible),
  },
  {
    key: "subtree",
    label: "Hierarchy",
    hint: "Just the breakdown — the parent/epic (left) and its children (right)",
    build: (tasks, focus, visible) => buildDirected(tasks, focus, { blocks: false, parent: true }, visible),
  },
];

export function graphKind(key: string): GraphKind {
  return GRAPH_KINDS.find((k) => k.key === key) ?? GRAPH_KINDS[0];
}
