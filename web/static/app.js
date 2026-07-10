// Beads UI — collapsible tree + sequence view.

const state = {
  issues: [],
  byId: new Map(),
  children: new Map(),        // parentId -> [childId, ...]
  blockers: new Map(),        // id -> [ids that must finish before this one]
  unblocks: new Map(),        // id -> [ids that this issue unblocks (reverse of blockers)]
  roots: [],
  collapsed: new Set(),       // tree-pane node ids that are collapsed
  detailCollapsed: new Set(), // detail-pane subtree collapse state (independent)
  selected: null,
  query: "",
  statuses: new Set(["open", "in_progress"]),
  types: new Set(["epic", "feature", "task", "bug", "chore"]),
  view: "tree",               // "tree" | "graph"
  edgeTypes: new Set(["blocks"]),  // which edge types feed the graph view
  simpleGraph: false,         // graph: hide (vs dim) nodes outside the selection's subgraph
};

const $tree = document.getElementById("tree");
const $detail = document.getElementById("detail-body");
const $search = document.getElementById("search");
const $status = document.getElementById("status");
const $back = document.getElementById("back");
const $filterRow = document.getElementById("filter-row");
const $toggleFilters = document.getElementById("toggle-filters");
const $searchClear = document.getElementById("search-clear");
const $searchWrap = $search.parentElement;

const isMobile = () => window.matchMedia("(max-width: 760px)").matches;

const STATE_KEY = "beads-ui-state-v2";

function saveState() {
  try {
    localStorage.setItem(STATE_KEY, JSON.stringify({
      query: state.query,
      statuses: [...state.statuses],
      types: [...state.types],
      collapsed: [...state.collapsed],
      detailCollapsed: [...state.detailCollapsed],
      selected: state.selected,
      view: state.view,
      edgeTypes: [...state.edgeTypes],
      simpleGraph: state.simpleGraph,
      showDetail: document.body.classList.contains("show-detail"),
      scrollTree: $tree.scrollTop || 0,
    }));
  } catch (_) { /* quota / private mode — ignore */ }
}

function loadState() {
  try {
    const raw = localStorage.getItem(STATE_KEY);
    if (!raw) return null;
    return JSON.parse(raw);
  } catch (_) { return null; }
}

function syncSearchClear() {
  $searchWrap.classList.toggle("has-value", !!$search.value);
}

// ---------- index ----------
function buildIndex(issues) {
  state.issues = issues;
  state.byId.clear();
  state.children.clear();
  state.blockers.clear();
  state.unblocks.clear();

  for (const it of issues) state.byId.set(it.id, it);

  for (const it of issues) {
    let parent = null;
    for (const d of (it.dependencies || [])) {
      if (d.type === "parent-child" || d.type === "parent") {
        if (d.depends_on_id && d.depends_on_id !== it.id) {
          parent = d.depends_on_id;
        }
      }
      if (d.type === "blocks" && d.depends_on_id) {
        const arr = state.blockers.get(it.id) || [];
        arr.push(d.depends_on_id);
        state.blockers.set(it.id, arr);
        const rev = state.unblocks.get(d.depends_on_id) || [];
        rev.push(it.id);
        state.unblocks.set(d.depends_on_id, rev);
      }
    }
    if (parent && state.byId.has(parent)) {
      const arr = state.children.get(parent) || [];
      arr.push(it.id);
      state.children.set(parent, arr);
    }
  }

  const typeOrder = { epic: 0, feature: 1, task: 2, bug: 3, chore: 4 };
  const sortFn = (a, b) => {
    const ia = state.byId.get(a), ib = state.byId.get(b);
    const ta = typeOrder[ia.issue_type] ?? 9;
    const tb = typeOrder[ib.issue_type] ?? 9;
    if (ta !== tb) return ta - tb;
    const pa = ia.priority ?? 9, pb = ib.priority ?? 9;
    if (pa !== pb) return pa - pb;
    return naturalCompare(ia.id, ib.id);
  };
  for (const arr of state.children.values()) arr.sort(sortFn);

  const childIds = new Set();
  for (const arr of state.children.values()) for (const c of arr) childIds.add(c);
  state.roots = issues.map(i => i.id).filter(id => !childIds.has(id)).sort(sortFn);
}

function computeVisible() {
  const q = state.query.trim().toLowerCase();
  const ownPass = new Map();
  for (const it of state.issues) {
    const statusOk = state.statuses.has(it.status);
    const typeOk = state.types.has(it.issue_type);
    let qOk = true;
    if (q) {
      const hay = (it.id + "\n" + (it.title || "") + "\n" + (it.description || "")).toLowerCase();
      qOk = hay.includes(q);
    }
    ownPass.set(it.id, statusOk && typeOk && qOk);
  }
  const subtree = new Map();
  const visit = (id) => {
    if (subtree.has(id)) return subtree.get(id);
    let any = ownPass.get(id) === true;
    const kids = state.children.get(id) || [];
    for (const k of kids) if (visit(k)) any = true;
    subtree.set(id, any);
    return any;
  };
  for (const r of state.roots) visit(r);
  return { ownPass, subtree };
}

// ---------- helpers ----------
function escapeHtml(s) {
  return String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
}
function escapeAttr(s) {
  return String(s).replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}
function escapeRegex(s) { return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"); }
function shortId(id) {
  const i = id.lastIndexOf("-");
  return i >= 0 ? id.slice(i + 1) : id;
}
// Natural (human) ordering: compare digit runs numerically so ps3t.2 < ps3t.11.
// Splits each string into alternating number / non-number chunks and compares
// chunk-by-chunk — numbers as integers, text lexically.
function naturalCompare(a, b) {
  const ax = String(a).match(/\d+|\D+/g) || [];
  const bx = String(b).match(/\d+|\D+/g) || [];
  const n = Math.min(ax.length, bx.length);
  for (let i = 0; i < n; i++) {
    const as = ax[i], bs = bx[i];
    const aNum = as.charCodeAt(0) >= 48 && as.charCodeAt(0) <= 57;
    const bNum = bs.charCodeAt(0) >= 48 && bs.charCodeAt(0) <= 57;
    if (aNum && bNum) {
      const d = parseInt(as, 10) - parseInt(bs, 10);
      if (d) return d;
    } else if (as !== bs) {
      return as < bs ? -1 : 1;
    }
  }
  return ax.length - bx.length;
}
function highlight(text, q) {
  if (!q) return escapeHtml(text);
  const safe = escapeHtml(text);
  const re = new RegExp(escapeRegex(q), "ig");
  return safe.replace(re, m => `<mark class="hit">${m}</mark>`);
}
function cssEscape(s) {
  return (window.CSS && CSS.escape) ? CSS.escape(s) : s.replace(/"/g, '\\"');
}
function parentOf(id) {
  const it = state.byId.get(id);
  if (!it) return null;
  for (const d of (it.dependencies || [])) {
    if (d.type === "parent-child" || d.type === "parent") return d.depends_on_id;
  }
  return null;
}

// ---------- markdown ----------
// Minimal safe markdown: escapes HTML first, then injects tags.
function renderMarkdown(src) {
  if (!src) return "";
  let s = String(src).replace(/\r\n?/g, "\n");

  const codeBlocks = [];
  s = s.replace(/```([a-zA-Z0-9_+-]*)\n([\s\S]*?)```/g, (_, lang, body) => {
    const idx = codeBlocks.length;
    codeBlocks.push(`<pre><code${lang ? ` class="lang-${escapeHtml(lang)}"` : ""}>${escapeHtml(body.replace(/\n$/, ""))}</code></pre>`);
    return ` CODEBLOCK${idx} `;
  });

  s = escapeHtml(s);

  const inlineCode = [];
  s = s.replace(/`([^`\n]+)`/g, (_, body) => {
    const idx = inlineCode.length;
    inlineCode.push(`<code>${body}</code>`);
    return ` INLINECODE${idx} `;
  });

  s = s.replace(/^####\s+(.+)$/gm, "<h4>$1</h4>");
  s = s.replace(/^###\s+(.+)$/gm, "<h3>$1</h3>");
  s = s.replace(/^##\s+(.+)$/gm, "<h2>$1</h2>");
  s = s.replace(/^#\s+(.+)$/gm, "<h1>$1</h1>");

  s = s.replace(/^(?:-{3,}|\*{3,}|_{3,})\s*$/gm, "<hr>");

  s = s.replace(/(?:^&gt;\s?.*(?:\n|$))+/gm, m => {
    const inner = m.replace(/^&gt;\s?/gm, "").replace(/\n$/, "");
    return `<blockquote>${inner}</blockquote>\n`;
  });

  s = s.replace(/\*\*([^*\n]+)\*\*/g, "<strong>$1</strong>");
  s = s.replace(/(^|[^*])\*([^*\n]+)\*/g, "$1<em>$2</em>");
  s = s.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_, text, href) =>
    `<a href="${escapeAttr(href)}" target="_blank" rel="noopener noreferrer">${text}</a>`);

  const lines = s.split("\n");
  const out = [];
  let i = 0;
  while (i < lines.length) {
    const ul = /^(\s*)[-*+]\s+(.*)$/.exec(lines[i]);
    const ol = /^(\s*)\d+\.\s+(.*)$/.exec(lines[i]);
    if (ul || ol) {
      const tag = ul ? "ul" : "ol";
      const itemRe = ul ? /^(\s*)[-*+]\s+(.*)$/ : /^(\s*)\d+\.\s+(.*)$/;
      out.push(`<${tag}>`);
      while (i < lines.length) {
        const m = itemRe.exec(lines[i]);
        if (!m) break;
        out.push(`<li>${m[2]}</li>`);
        i++;
      }
      out.push(`</${tag}>`);
      continue;
    }
    out.push(lines[i]);
    i++;
  }
  s = out.join("\n");

  const isBlock = chunk => /^\s*<(h[1-6]|ul|ol|li|pre|blockquote|hr|p)\b/i.test(chunk) ||
                           chunk.startsWith(" CODEBLOCK");
  s = s.split(/\n{2,}/).map(chunk => {
    chunk = chunk.replace(/^\n+|\n+$/g, "");
    if (!chunk) return "";
    if (isBlock(chunk)) return chunk;
    return `<p>${chunk.replace(/\n/g, "<br>")}</p>`;
  }).join("\n");

  s = s.replace(/ INLINECODE(\d+) /g, (_, n) => inlineCode[+n]);
  s = s.replace(/ CODEBLOCK(\d+) /g, (_, n) => codeBlocks[+n]);

  return s;
}

// ---------- common row builders ----------
function makeStatusDot(it) {
  const dot = document.createElement("span");
  dot.className = "statusdot " + it.status;
  dot.title = it.status;
  return dot;
}
function makeTypeBadge(it) {
  const t = document.createElement("span");
  t.className = "typebadge " + it.issue_type;
  t.textContent = it.issue_type;
  return t;
}

// ---------- tree view ----------
function renderTree() {
  teardownGraph();
  const { subtree } = computeVisible();
  const q = state.query.trim().toLowerCase();
  $tree.className = "tree";
  $tree.innerHTML = "";

  const frag = document.createDocumentFragment();
  let shown = 0;

  const renderNode = (id, depth) => {
    if (!subtree.get(id)) return null;
    const it = state.byId.get(id);
    const kids = (state.children.get(id) || []).filter(k => subtree.get(k));
    const hasKids = kids.length > 0;
    const collapsed = state.collapsed.has(id);

    const node = document.createElement("div");
    node.className = "node";

    const row = document.createElement("div");
    row.className = "row " + (it.status === "closed" ? "closed " : "") + (state.selected === id ? "selected" : "");
    const indent = isMobile() ? 10 : 14;
    row.style.paddingLeft = (4 + depth * indent) + "px";
    row.dataset.id = id;

    const caret = document.createElement("span");
    caret.className = "caret" + (hasKids ? "" : " empty");
    caret.textContent = hasKids ? (collapsed ? "▶" : "▼") : "•";
    caret.addEventListener("click", (e) => {
      e.stopPropagation();
      if (!hasKids) return;
      if (state.collapsed.has(id)) state.collapsed.delete(id);
      else state.collapsed.add(id);
      renderTree();
      saveState();
    });

    const prio = document.createElement("span");
    prio.className = "prio p" + (it.priority ?? "");
    prio.textContent = "P" + (it.priority ?? "?");

    const idSpan = document.createElement("span");
    idSpan.className = "id";
    idSpan.innerHTML = highlight(shortId(id), q);

    const title = document.createElement("span");
    title.className = "title";
    title.innerHTML = highlight(it.title || "(untitled)", q);

    row.append(caret, makeStatusDot(it), makeTypeBadge(it), prio, idSpan, title);

    if (hasKids) {
      const count = document.createElement("span");
      count.className = "count";
      count.textContent = `(${kids.length})`;
      row.append(count);
    }

    row.addEventListener("click", () => {
      state.selected = id;
      renderDetail(id);
      document.querySelectorAll(".row.selected,.srow.selected").forEach(r => r.classList.remove("selected"));
      row.classList.add("selected");
      if (isMobile()) {
        document.body.classList.add("show-detail");
        $detail.parentElement.scrollTop = 0;
      }
      syncUrl(id);
      saveState();
    });

    node.append(row);
    shown++;

    if (hasKids) {
      const childrenEl = document.createElement("div");
      childrenEl.className = "children" + (collapsed ? " collapsed" : "");
      for (const k of kids) {
        const ch = renderNode(k, depth + 1);
        if (ch) childrenEl.append(ch);
      }
      node.append(childrenEl);
    }

    return node;
  };

  for (const r of state.roots) {
    const n = renderNode(r, 0);
    if (n) frag.append(n);
  }

  $tree.append(frag);
  $status.textContent = `${shown} shown / ${state.issues.length} total`;
}

// ---------- graph view ----------
// Predecessors / successors based on currently selected edge types.
function gPredecessors(id) {
  const out = [];
  if (state.edgeTypes.has("blocks")) {
    for (const b of (state.blockers.get(id) || [])) out.push({ id: b, type: "blocks" });
  }
  if (state.edgeTypes.has("parent-child")) {
    const p = parentOf(id);
    if (p) out.push({ id: p, type: "parent" });
  }
  return out;
}
function gSuccessors(id) {
  const out = [];
  if (state.edgeTypes.has("blocks")) {
    for (const u of (state.unblocks.get(id) || [])) out.push({ id: u, type: "blocks" });
  }
  if (state.edgeTypes.has("parent-child")) {
    for (const k of (state.children.get(id) || [])) out.push({ id: k, type: "parent" });
  }
  return out;
}

function computeLayers(idSet) {
  const memo = new Map();
  const layerOf = (id, stack) => {
    if (memo.has(id)) return memo.get(id);
    if (stack.has(id)) return 0;
    stack.add(id);
    const preds = gPredecessors(id).filter(p => idSet.has(p.id));
    const r = preds.length ? 1 + Math.max(...preds.map(p => layerOf(p.id, stack))) : 0;
    stack.delete(id);
    memo.set(id, r);
    return r;
  };
  for (const id of idSet) layerOf(id, new Set());
  return memo;
}

let _graphCleanup = null;
function teardownGraph() {
  if (_graphCleanup) { _graphCleanup(); _graphCleanup = null; }
}

function renderGraphLegend({ rows = 0, nodes = 0 } = {}) {
  const legend = document.createElement("div");
  legend.className = "graph-legend";
  legend.innerHTML = `
    <span class="lg-item lg-edges">
      <strong style="color:var(--fg-dim)">edges:</strong>
      <label class="chk"><input type="checkbox" data-edge="blocks"> <span style="color:var(--warn)">blocks</span></label>
      <label class="chk"><input type="checkbox" data-edge="parent-child"> <span style="color:var(--accent-2)">parent → child</span></label>
    </span>
    <span class="lg-item">${rows} row${rows === 1 ? "" : "s"}</span>
    <span class="lg-item">${nodes} node${nodes === 1 ? "" : "s"}</span>
    <span class="lg-item" title="Edge/node colors when an issue is selected">
      <span style="color:var(--err)">↑ blocked by</span>
      <span style="color:var(--fg-faint)">·</span>
      <span style="color:var(--accent)">↓ blocks</span>
    </span>
    <span class="lg-item" style="margin-left:auto;color:var(--fg-dim)">drag to pan · scroll / pinch to zoom · dbl-click to fit</span>
  `;
  legend.querySelectorAll("input[data-edge]").forEach(el => {
    el.checked = state.edgeTypes.has(el.dataset.edge);
    el.addEventListener("change", () => {
      if (el.checked) state.edgeTypes.add(el.dataset.edge);
      else state.edgeTypes.delete(el.dataset.edge);
      saveState();
      renderGraph();
    });
  });
  $tree.append(legend);
}

function renderGraph() {
  teardownGraph();
  $tree.className = "tree graph";
  $tree.innerHTML = "";

  const q = state.query.trim().toLowerCase();

  // Same visibility filter as tree.
  const visible = new Set();
  for (const it of state.issues) {
    if (!state.statuses.has(it.status)) continue;
    if (!state.types.has(it.issue_type)) continue;
    if (q) {
      const hay = (it.id + "\n" + (it.title || "") + "\n" + (it.description || "")).toLowerCase();
      if (!hay.includes(q)) continue;
    }
    visible.add(it.id);
  }

  // Only render items that participate in at least one edge of the selected
  // type(s) to/from another visible item.
  const inGraph = new Set();
  for (const id of visible) {
    const hasPred = gPredecessors(id).some(p => visible.has(p.id));
    const hasSucc = gSuccessors(id).some(s => visible.has(s.id));
    if (hasPred || hasSucc) inGraph.add(id);
  }

  if (!inGraph.size) {
    const empty = document.createElement("div");
    empty.className = "graph-empty";
    const types = [...state.edgeTypes].join(" + ") || "(no edge types)";
    empty.textContent = `No ${types} edges among the visible items. Toggle filters or edge types above to widen the set.`;
    renderGraphLegend();
    $tree.append(empty);
    $status.textContent = `0 in graph / ${visible.size} visible / ${state.issues.length} total`;
    return;
  }

  const layers = computeLayers(inGraph);
  const byLayer = new Map();
  for (const id of inGraph) {
    const L = layers.get(id) ?? 0;
    if (!byLayer.has(L)) byLayer.set(L, []);
    byLayer.get(L).push(id);
  }
  const sortedLayers = [...byLayer.keys()].sort((a, b) => a - b);

  // Initial sort: priority then id (stable starting point).
  const initSort = (a, b) => {
    const ia = state.byId.get(a), ib = state.byId.get(b);
    const pa = ia.priority ?? 9, pb = ib.priority ?? 9;
    if (pa !== pb) return pa - pb;
    return naturalCompare(ia.id, ib.id);
  };
  for (const L of sortedLayers) byLayer.get(L).sort(initSort);

  // Barycenter passes to minimize edge crossings.
  const position = new Map();
  const reindex = () => {
    for (const L of sortedLayers) {
      byLayer.get(L).forEach((id, i) => position.set(id, i));
    }
  };
  reindex();
  const baryUp = id => {
    const preds = gPredecessors(id).filter(p => inGraph.has(p.id));
    if (!preds.length) return position.get(id);
    return preds.reduce((s, p) => s + position.get(p.id), 0) / preds.length;
  };
  const baryDown = id => {
    const succs = gSuccessors(id).filter(s => inGraph.has(s.id));
    if (!succs.length) return position.get(id);
    return succs.reduce((s, x) => s + position.get(x.id), 0) / succs.length;
  };
  for (let iter = 0; iter < 4; iter++) {
    for (let i = 1; i < sortedLayers.length; i++) {
      byLayer.get(sortedLayers[i]).sort((a, b) => baryUp(a) - baryUp(b));
      reindex();
    }
    for (let i = sortedLayers.length - 2; i >= 0; i--) {
      byLayer.get(sortedLayers[i]).sort((a, b) => baryDown(a) - baryDown(b));
      reindex();
    }
  }

  // Coordinate layout.
  const NW = 210, NH = 56, XGAP = 28, YGAP = 64, PAD = 24;
  const maxCount = Math.max(...sortedLayers.map(L => byLayer.get(L).length));
  const W = Math.max(420, PAD * 2 + maxCount * (NW + XGAP) - XGAP);
  const H = PAD * 2 + sortedLayers.length * (NH + YGAP) - YGAP;

  const coords = new Map();
  sortedLayers.forEach((L, li) => {
    const arr = byLayer.get(L);
    const rowW = arr.length * (NW + XGAP) - XGAP;
    const startX = (W - rowW) / 2;
    const y = PAD + li * (NH + YGAP);
    arr.forEach((id, i) => coords.set(id, { x: startX + i * (NW + XGAP), y }));
  });

  renderGraphLegend({ rows: sortedLayers.length, nodes: inGraph.size });

  const canvas = document.createElement("div");
  canvas.className = "graph-canvas";
  $tree.append(canvas);

  const NS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("class", "depgraph");
  const viewport = document.createElementNS(NS, "g");
  viewport.setAttribute("class", "viewport");

  const defs = document.createElementNS(NS, "defs");
  defs.innerHTML = `
    <marker id="arrow"        viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
      <path d="M 0 0 L 10 5 L 0 10 z" fill="#5b6378"/>
    </marker>
    <marker id="arrow-blocks" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
      <path d="M 0 0 L 10 5 L 0 10 z" fill="#e0af68"/>
    </marker>
    <marker id="arrow-parent" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
      <path d="M 0 0 L 10 5 L 0 10 z" fill="#bb9af7"/>
    </marker>
    <marker id="arrow-hl"     viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
      <path d="M 0 0 L 10 5 L 0 10 z" fill="#7aa2f7"/>
    </marker>
    <marker id="arrow-up"     viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
      <path d="M 0 0 L 10 5 L 0 10 z" fill="#f7768e"/>
    </marker>
  `;
  svg.append(defs);
  svg.append(viewport);

  const edgeEls = [];
  const nodeEls = new Map();
  const edgeList = []; // {src, dst, el}

  // Bezier from bottom-center of a to top-center of b (a,b are {x,y} of a node).
  const edgePath = (a, b) => {
    const x1 = a.x + NW / 2, y1 = a.y + NH;
    const x2 = b.x + NW / 2, y2 = b.y;
    const mid = (y1 + y2) / 2;
    return `M${x1},${y1} C${x1},${mid} ${x2},${mid} ${x2},${y2}`;
  };

  // Edges first so they render under nodes. Dedup (src, dst) pairs.
  const drawn = new Set();
  for (const src of inGraph) {
    const succs = gSuccessors(src).filter(s => inGraph.has(s.id));
    const a = coords.get(src);
    for (const succ of succs) {
      const t = succ.id;
      const key = `${src} ${t} ${succ.type}`;
      if (drawn.has(key)) continue;
      drawn.add(key);
      const b = coords.get(t);
      const path = document.createElementNS(NS, "path");
      path.setAttribute("d", edgePath(a, b));
      path.setAttribute("class", `edge edge-${succ.type}`);
      path.setAttribute("marker-end", succ.type === "blocks" ? "url(#arrow-blocks)" : "url(#arrow-parent)");
      viewport.append(path);
      edgeEls.push(path);
      edgeList.push({ src, dst: t, type: succ.type, el: path });
    }
  }

  // Nodes.
  for (const id of inGraph) {
    const it = state.byId.get(id);
    const { x, y } = coords.get(id);
    const g = document.createElementNS(NS, "g");
    g.setAttribute("class", `gnode ${it.status} ${it.issue_type}` + (state.selected === id ? " selected" : ""));
    g.setAttribute("transform", `translate(${x},${y})`);
    g.dataset.id = id;

    const rect = document.createElementNS(NS, "rect");
    rect.setAttribute("width", NW);
    rect.setAttribute("height", NH);
    rect.setAttribute("rx", 6);
    g.append(rect);

    const dot = document.createElementNS(NS, "circle");
    dot.setAttribute("cx", 12);
    dot.setAttribute("cy", 14);
    dot.setAttribute("r", 4);
    dot.setAttribute("class", "ndot " + it.status);
    g.append(dot);

    const idText = document.createElementNS(NS, "text");
    idText.setAttribute("x", 24);
    idText.setAttribute("y", 18);
    idText.setAttribute("class", "nid");
    idText.textContent = shortId(id);
    g.append(idText);

    const typeText = document.createElementNS(NS, "text");
    typeText.setAttribute("x", NW - 10);
    typeText.setAttribute("y", 18);
    typeText.setAttribute("text-anchor", "end");
    typeText.setAttribute("class", "ntype " + it.issue_type);
    typeText.textContent = it.issue_type;
    g.append(typeText);

    const titleText = document.createElementNS(NS, "text");
    titleText.setAttribute("x", 12);
    titleText.setAttribute("y", 40);
    titleText.setAttribute("class", "ntitle");
    const t = (it.title || "(untitled)");
    titleText.textContent = t.length > 34 ? t.slice(0, 33) + "…" : t;
    const titleEl = document.createElementNS(NS, "title");
    titleEl.textContent = t;
    g.append(titleEl, titleText);

    g.addEventListener("click", () => {
      state.selected = id;
      renderDetail(id);
      svg.querySelectorAll(".gnode.selected").forEach(n => n.classList.remove("selected"));
      g.classList.add("selected");
      const keep = highlightGraph(id);
      if (state.simpleGraph) { applyLayout(keep); if (keep) fitToNodes(keep); }
      if (isMobile()) {
        document.body.classList.add("show-detail");
        $detail.parentElement.scrollTop = 0;
      }
      syncUrl(id);
      saveState();
    });

    viewport.append(g);
    nodeEls.set(id, g);
  }

  // Select highlight: focus the node's ancestors+descendants subgraph, tagging
  // upstream (what blocks focus — red) apart from downstream (what focus blocks
  // — blue). In normal mode the rest is dimmed; in simple mode it's hidden.
  // Returns the kept id set (or null when nothing is focused).
  function highlightGraph(focus) {
    const clearMarker = () => {
      for (const e of edgeList) {
        e.el.setAttribute("marker-end", e.type === "blocks" ? "url(#arrow-blocks)" : "url(#arrow-parent)");
      }
    };
    if (!focus) {
      nodeEls.forEach(el => el.classList.remove("dimmed", "hidden", "up", "down"));
      edgeEls.forEach(el => el.classList.remove("hl", "hidden", "up", "down"));
      clearMarker();
      lastKeep = null;
      return null;
    }
    const simple = state.simpleGraph;
    // Upstream = transitive blockers (predecessors); downstream = things focus
    // blocks (successors). focus itself is excluded from both.
    const upSet = new Set(), downSet = new Set();
    const up = [focus];
    while (up.length) {
      const n = up.pop();
      for (const p of gPredecessors(n)) {
        if (inGraph.has(p.id) && p.id !== focus && !upSet.has(p.id)) { upSet.add(p.id); up.push(p.id); }
      }
    }
    const down = [focus];
    while (down.length) {
      const n = down.pop();
      for (const s of gSuccessors(n)) {
        if (inGraph.has(s.id) && s.id !== focus && !downSet.has(s.id)) { downSet.add(s.id); down.push(s.id); }
      }
    }
    const keep = new Set([focus, ...upSet, ...downSet]);

    nodeEls.forEach((el, id) => {
      const drop = !keep.has(id);
      el.classList.toggle("dimmed", drop && !simple);
      el.classList.toggle("hidden", drop && simple);
      el.classList.toggle("up", upSet.has(id));
      el.classList.toggle("down", downSet.has(id));
    });
    for (const e of edgeList) {
      const onPath = keep.has(e.src) && keep.has(e.dst);
      // Upstream edge: feeds into the focus's blocker chain (dst is focus or an
      // ancestor). Downstream edge: flows out of focus into what it blocks.
      const isUp = onPath && upSet.has(e.src) && (e.dst === focus || upSet.has(e.dst));
      const isDown = onPath && !isUp && downSet.has(e.dst) && (e.src === focus || downSet.has(e.src));
      e.el.classList.toggle("hl", onPath);
      e.el.classList.toggle("up", isUp);
      e.el.classList.toggle("down", isDown);
      e.el.classList.toggle("hidden", !onPath && simple);
      if (isUp) e.el.setAttribute("marker-end", "url(#arrow-up)");
      else if (onPath) e.el.setAttribute("marker-end", "url(#arrow-hl)");
      else e.el.setAttribute("marker-end", e.type === "blocks" ? "url(#arrow-blocks)" : "url(#arrow-parent)");
    }
    lastKeep = keep;
    return keep;
  }
  let lastKeep = null;

  // ----- layout (base vs compacted subgraph for simple mode) -----
  // curLayout is the positions currently applied to the DOM (base coords, or a
  // compacted layout of the selected subgraph when simple mode is active).
  let curLayout = coords;

  // Left-align + vertically compact just the kept nodes: drop empty layers,
  // pack each remaining row from the left edge, preserving left→right order.
  const compactLayout = (keep) => {
    const rows = new Map(); // layer -> [ids]
    for (const id of keep) {
      const L = layers.get(id) ?? 0;
      if (!rows.has(L)) rows.set(L, []);
      rows.get(L).push(id);
    }
    const occupied = [...rows.keys()].sort((a, b) => a - b);
    const out = new Map();
    occupied.forEach((L, li) => {
      const arr = rows.get(L).sort((a, b) => coords.get(a).x - coords.get(b).x);
      const y = PAD + li * (NH + YGAP);
      arr.forEach((id, i) => out.set(id, { x: PAD + i * (NW + XGAP), y }));
    });
    return out;
  };

  // Reposition every node + edge from a layout. In simple mode with a focus,
  // the kept subgraph is compacted; everyone else falls back to base coords
  // (they're hidden anyway). Records the applied layout in curLayout.
  const applyLayout = (keep) => {
    const layout = new Map(coords);
    if (state.simpleGraph && keep && keep.size) {
      for (const [id, c] of compactLayout(keep)) layout.set(id, c);
    }
    nodeEls.forEach((g, id) => {
      const c = layout.get(id);
      g.setAttribute("transform", `translate(${c.x},${c.y})`);
    });
    for (const e of edgeList) {
      e.el.setAttribute("d", edgePath(layout.get(e.src), layout.get(e.dst)));
    }
    curLayout = layout;
    return layout;
  };

  canvas.append(svg);

  // ----- pan / zoom -----
  const view = { tx: 0, ty: 0, s: 1 };
  const MIN_S = 0.25, MAX_S = 3;

  const applyView = () => {
    viewport.setAttribute("transform", `translate(${view.tx} ${view.ty}) scale(${view.s})`);
    const gs = 24 * view.s;
    canvas.style.backgroundSize = `${gs}px ${gs}px`;
    canvas.style.backgroundPosition = `${view.tx}px ${view.ty}px`;
  };
  const setView = (tx, ty, s) => {
    view.s = Math.max(MIN_S, Math.min(MAX_S, s));
    view.tx = tx; view.ty = ty;
    applyView();
  };
  const zoomAt = (px, py, factor) => {
    const ns = Math.max(MIN_S, Math.min(MAX_S, view.s * factor));
    const wx = (px - view.tx) / view.s;
    const wy = (py - view.ty) / view.s;
    setView(px - ns * wx, py - ns * wy, ns);
  };
  const fitView = () => {
    const r = canvas.getBoundingClientRect();
    if (!r.width || !r.height) return;
    const pad = 24;
    const sx = (r.width - pad * 2) / W;
    const sy = (r.height - pad * 2) / H;
    const s = Math.min(sx, sy, 1.5); // never absurdly upscale on fit
    setView((r.width - W * s) / 2, (r.height - H * s) / 2, s);
  };
  // Fit the view to just the given node ids (simple mode: frame the subgraph).
  const fitToNodes = (ids) => {
    const r = canvas.getBoundingClientRect();
    if (!r.width || !r.height || !ids || !ids.size) return fitView();
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const id of ids) {
      const c = curLayout.get(id);
      if (!c) continue;
      minX = Math.min(minX, c.x); minY = Math.min(minY, c.y);
      maxX = Math.max(maxX, c.x + NW); maxY = Math.max(maxY, c.y + NH);
    }
    if (!isFinite(minX)) return fitView();
    const bw = maxX - minX, bh = maxY - minY, pad = 40;
    const s = Math.max(MIN_S, Math.min(MAX_S,
      Math.min((r.width - pad * 2) / bw, (r.height - pad * 2) / bh, 1.5)));
    setView((r.width - bw * s) / 2 - minX * s, (r.height - bh * s) / 2 - minY * s, s);
  };
  const zoomCentered = (factor) => {
    const r = canvas.getBoundingClientRect();
    zoomAt(r.width / 2, r.height / 2, factor);
  };

  const updateSize = () => {
    const r = canvas.getBoundingClientRect();
    if (!r.width || !r.height) return;
    svg.setAttribute("width", r.width);
    svg.setAttribute("height", r.height);
    svg.setAttribute("viewBox", `0 0 ${r.width} ${r.height}`);
  };

  const ro = new ResizeObserver(updateSize);
  ro.observe(canvas);

  // First fit after layout settles (frame the subgraph if simple mode is on).
  requestAnimationFrame(() => {
    updateSize();
    if (state.simpleGraph && lastKeep) fitToNodes(lastKeep);
    else fitView();
  });

  // ----- pointer events: drag-to-pan and pinch-to-zoom -----
  const pointers = new Map();
  let panAnchor = null;
  let pinchAnchor = null;

  const onDown = (e) => {
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (pointers.size === 1) {
      panAnchor = { x: e.clientX, y: e.clientY, tx: view.tx, ty: view.ty };
      pinchAnchor = null;
      canvas.classList.add("panning");
    } else if (pointers.size === 2) {
      const [a, b] = [...pointers.values()];
      const rect = canvas.getBoundingClientRect();
      pinchAnchor = {
        startDist: Math.hypot(a.x - b.x, a.y - b.y) || 1,
        startMx: (a.x + b.x) / 2 - rect.left,
        startMy: (a.y + b.y) / 2 - rect.top,
        startScale: view.s,
        startTx: view.tx,
        startTy: view.ty,
      };
      panAnchor = null;
    }
  };
  const onMove = (e) => {
    if (!pointers.has(e.pointerId)) return;
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (pointers.size === 1 && panAnchor) {
      setView(panAnchor.tx + (e.clientX - panAnchor.x), panAnchor.ty + (e.clientY - panAnchor.y), view.s);
    } else if (pointers.size === 2 && pinchAnchor) {
      const [a, b] = [...pointers.values()];
      const dist = Math.hypot(a.x - b.x, a.y - b.y) || 1;
      const factor = dist / pinchAnchor.startDist;
      const ns = Math.max(MIN_S, Math.min(MAX_S, pinchAnchor.startScale * factor));
      const rect = canvas.getBoundingClientRect();
      // world point under start midpoint
      const wx = (pinchAnchor.startMx - pinchAnchor.startTx) / pinchAnchor.startScale;
      const wy = (pinchAnchor.startMy - pinchAnchor.startTy) / pinchAnchor.startScale;
      // current midpoint pixel
      const cmx = (a.x + b.x) / 2 - rect.left;
      const cmy = (a.y + b.y) / 2 - rect.top;
      setView(cmx - ns * wx, cmy - ns * wy, ns);
    }
  };
  const onUp = (e) => {
    pointers.delete(e.pointerId);
    if (pointers.size === 0) {
      panAnchor = null; pinchAnchor = null;
      canvas.classList.remove("panning");
    } else if (pointers.size === 1) {
      const [p] = pointers.values();
      panAnchor = { x: p.x, y: p.y, tx: view.tx, ty: view.ty };
      pinchAnchor = null;
    }
  };

  const onWheel = (e) => {
    e.preventDefault();
    const r = canvas.getBoundingClientRect();
    const factor = Math.exp(-e.deltaY * 0.0015);
    zoomAt(e.clientX - r.left, e.clientY - r.top, factor);
  };

  canvas.addEventListener("pointerdown", onDown);
  window.addEventListener("pointermove", onMove);
  window.addEventListener("pointerup", onUp);
  window.addEventListener("pointercancel", onUp);
  canvas.addEventListener("wheel", onWheel, { passive: false });

  // ----- toolbar -----
  const tools = document.createElement("div");
  tools.className = "graph-tools";
  tools.innerHTML = `
    <button data-simple title="Simple mode: hide everything outside the selected subgraph">◎</button>
    <button data-fit title="Fit to view">⤢</button>
    <button data-zin title="Zoom in">+</button>
    <button data-zout title="Zoom out">−</button>
  `;
  const simpleBtn = tools.querySelector("[data-simple]");
  const syncSimpleBtn = () => simpleBtn.classList.toggle("active", state.simpleGraph);
  syncSimpleBtn();
  simpleBtn.addEventListener("click", () => {
    state.simpleGraph = !state.simpleGraph;
    syncSimpleBtn();
    const focus = state.selected && nodeEls.has(state.selected) ? state.selected : null;
    const keep = highlightGraph(focus);
    applyLayout(keep);                       // compact subgraph, or restore base
    if (state.simpleGraph && keep) fitToNodes(keep);
    else fitView();
    saveState();
  });
  tools.querySelector("[data-fit]").addEventListener("click", fitView);
  tools.querySelector("[data-zin]").addEventListener("click", () => zoomCentered(1.25));
  tools.querySelector("[data-zout]").addEventListener("click", () => zoomCentered(0.8));
  canvas.append(tools);

  // Double-tap / double-click to fit
  canvas.addEventListener("dblclick", (e) => {
    if (e.target.closest(".gnode")) return;
    fitView();
  });

  _graphCleanup = () => {
    ro.disconnect();
    window.removeEventListener("pointermove", onMove);
    window.removeEventListener("pointerup", onUp);
    window.removeEventListener("pointercancel", onUp);
  };

  if (state.selected && nodeEls.has(state.selected)) {
    const keep = highlightGraph(state.selected);
    if (state.simpleGraph) applyLayout(keep);
  }

  $status.textContent = `${inGraph.size} in graph / ${visible.size} visible / ${state.issues.length} total`;
}

function renderMain() {
  if (state.view === "graph") renderGraph();
  else if (state.view === "dashboard") renderDashboard();
  else renderTree();
}

// ---------- dashboard: epic progress ----------
// Which progress bucket an issue falls in. Precedence: done → active →
// blocked (open with an unfinished blocker) → deferred → open/ready.
function progressCategory(id) {
  const it = state.byId.get(id);
  if (!it) return "open";
  if (it.status === "closed") return "closed";
  if (it.status === "in_progress") return "in_progress";
  const blocked = (state.blockers.get(id) || []).some(b => {
    const bit = state.byId.get(b);
    return bit && bit.status !== "closed";
  });
  if (blocked) return "blocked";
  if (it.status === "deferred") return "deferred";
  return "open";
}

// Left→right: done first (bar fills from the left), then active/todo states.
const PROGRESS_SEGMENTS = [
  { key: "closed",      label: "done" },
  { key: "in_progress", label: "in progress" },
  { key: "blocked",     label: "blocked" },
  { key: "open",        label: "open" },
  { key: "deferred",    label: "deferred" },
];

function renderDashboard() {
  teardownGraph();
  $tree.className = "tree dashboard";
  $tree.innerHTML = "";

  // Every top-level epic (a root whose type is epic), regardless of filters —
  // the dashboard is a complete overview.
  const epics = state.roots.filter(id => (state.byId.get(id) || {}).issue_type === "epic");

  if (!epics.length) {
    const empty = document.createElement("div");
    empty.className = "dash-empty";
    empty.textContent = "No top-level epics found.";
    $tree.append(empty);
    $status.textContent = `0 epics / ${state.issues.length} total`;
    return;
  }

  const frag = document.createDocumentFragment();
  for (const epicId of epics) {
    const epic = state.byId.get(epicId);
    const kids = allDescendants([epicId]); // COMPLETE recursive child set
    const total = kids.length;
    const counts = { closed: 0, in_progress: 0, blocked: 0, open: 0, deferred: 0 };
    for (const k of kids) counts[progressCategory(k)]++;
    const pct = total ? Math.round((100 * counts.closed) / total) : 0;

    const segs = PROGRESS_SEGMENTS
      .filter(s => counts[s.key] > 0)
      .map(s => `<span class="dseg ${s.key}" style="width:${(100 * counts[s.key]) / total}%" title="${counts[s.key]} ${s.label}"></span>`)
      .join("");
    const legend = PROGRESS_SEGMENTS
      .filter(s => counts[s.key] > 0)
      .map(s => `<span class="dleg"><span class="dswatch ${s.key}"></span>${counts[s.key]} ${s.label}</span>`)
      .join("");

    const card = document.createElement("div");
    card.className = "dash-card" + (state.selected === epicId ? " selected" : "");
    card.dataset.id = epicId;
    card.innerHTML = `
      <div class="dash-head">
        <span class="statusdot ${epic.status}"></span>
        <span class="id">${escapeHtml(shortId(epicId))}</span>
        <span class="dtitle">${escapeHtml(epic.title || "(untitled)")}</span>
        <span class="dpct">${pct}%</span>
      </div>
      <div class="dbar">${total ? segs : '<span class="dseg empty"></span>'}</div>
      <div class="dcounts">
        <span class="dtotal">${counts.closed}/${total} done</span>
        ${total ? legend : '<span class="dleg">no children</span>'}
      </div>
    `;
    card.addEventListener("click", () => {
      state.selected = epicId;
      renderDetail(epicId);
      $tree.querySelectorAll(".dash-card.selected").forEach(c => c.classList.remove("selected"));
      card.classList.add("selected");
      if (isMobile()) { document.body.classList.add("show-detail"); $detail.parentElement.scrollTop = 0; }
      syncUrl(epicId);
      saveState();
    });
    frag.append(card);
  }
  $tree.append(frag);
  $status.textContent = `${epics.length} epic${epics.length === 1 ? "" : "s"} / ${state.issues.length} total`;
}

// ---------- detail pane ----------
// ---------- routing (URL hash <-> selection) ----------
// The selected issue id lives in the URL hash (e.g. #ps3t.1.1) so browser
// back/forward walk the selection history and links are deep-linkable.
function hashId() {
  const h = location.hash.replace(/^#/, "");
  return h ? decodeURIComponent(h) : null;
}

// Reflect a selection in the URL. push (default) adds a history entry;
// replace stamps the current entry without adding one (initial load / refresh).
function syncUrl(id, { replace = false } = {}) {
  const want = id ? "#" + encodeURIComponent(id) : "";
  if (!replace && location.hash === want) return; // already here — no dup entry
  const href = id ? want : location.pathname + location.search;
  const st = { id: id || null };
  if (replace) history.replaceState(st, "", href);
  else history.pushState(st, "", href);
}

function navigateTo(target, { push = true } = {}) {
  // Expand ancestors in tree view so the target is visible.
  let cur = target;
  while (cur) {
    const next = parentOf(cur);
    if (next) state.collapsed.delete(next);
    cur = next;
  }
  state.selected = target;
  renderMain();
  renderDetail(target);
  const row = $tree.querySelector(`[data-id="${cssEscape(target)}"]`);
  if (row) row.scrollIntoView({ block: "center", behavior: "smooth" });
  if (push) syncUrl(target);
  saveState();
}

function allDescendants(rootIds) {
  const out = [];
  const walk = (i) => {
    const kids = state.children.get(i) || [];
    for (const k of kids) { out.push(k); walk(k); }
  };
  for (const r of rootIds) walk(r);
  return out;
}

function renderDetailChildren(host, rootIds) {
  host.innerHTML = "";
  const buildNode = (id, depth) => {
    const it = state.byId.get(id);
    if (!it) return null;
    const kids = state.children.get(id) || [];
    const hasKids = kids.length > 0;
    const collapsed = state.detailCollapsed.has(id);

    const node = document.createElement("div");
    node.className = "dnode";

    const row = document.createElement("div");
    row.className = "drow " + (it.status === "closed" ? "closed" : "");
    row.style.paddingLeft = (depth * 14) + "px";

    const caret = document.createElement("span");
    caret.className = "caret" + (hasKids ? "" : " empty");
    caret.textContent = hasKids ? (collapsed ? "▶" : "▼") : "•";
    caret.addEventListener("click", (e) => {
      e.stopPropagation();
      if (!hasKids) return;
      if (state.detailCollapsed.has(id)) state.detailCollapsed.delete(id);
      else state.detailCollapsed.add(id);
      renderDetailChildren(host, rootIds);
      saveState();
    });

    const link = document.createElement("a");
    link.href = "#";
    link.className = "did";
    link.textContent = shortId(id);
    link.addEventListener("click", (e) => { e.preventDefault(); navigateTo(id); });

    const title = document.createElement("span");
    title.className = "dtitle";
    title.textContent = it.title || "(untitled)";

    row.append(caret, makeStatusDot(it), makeTypeBadge(it), link, title);

    if (hasKids) {
      const count = document.createElement("span");
      count.className = "count";
      count.textContent = `(${kids.length})`;
      row.append(count);
    }

    node.append(row);

    if (hasKids && !collapsed) {
      const childrenEl = document.createElement("div");
      childrenEl.className = "dchildren";
      for (const k of kids) {
        const ch = buildNode(k, depth + 1);
        if (ch) childrenEl.append(ch);
      }
      node.append(childrenEl);
    }
    return node;
  };

  for (const r of rootIds) {
    const n = buildNode(r, 0);
    if (n) host.append(n);
  }
}

function renderDetail(id) {
  const it = state.byId.get(id);
  if (!it) {
    $detail.innerHTML = `<div class="empty">Unknown issue ${escapeHtml(id)}</div>`;
    return;
  }

  const deps = (it.dependencies || []);
  const parent = deps.find(d => d.type === "parent-child" || d.type === "parent");
  const blocks = deps.filter(d => d.type === "blocks");
  const related = deps.filter(d => d.type === "related");

  const dependents = [];
  for (const other of state.issues) {
    for (const d of (other.dependencies || [])) {
      if (d.depends_on_id === id) dependents.push({ id: other.id, type: d.type });
    }
  }

  const childIds = state.children.get(id) || [];

  const countDescendants = (rootId) => {
    let n = 0;
    const walk = (i) => {
      const kids = state.children.get(i) || [];
      for (const k of kids) { n++; walk(k); }
    };
    walk(rootId);
    return n;
  };
  const descendantCount = countDescendants(id);

  const linkList = (ids) => ids.length
    ? `<ul>${ids.map(x => `<li><a href="#" data-go="${escapeHtml(x)}">${escapeHtml(shortId(x))}</a> — ${escapeHtml((state.byId.get(x) || {}).title || "?")}</li>`).join("")}</ul>`
    : `<div class="empty">none</div>`;

  $detail.innerHTML = `
    <h1>${escapeHtml(it.title || "(untitled)")}</h1>
    <div class="sub">
      <span class="typebadge ${it.issue_type}">${it.issue_type}</span>
      <span><span class="statusdot ${it.status}"></span> ${escapeHtml(it.status)}</span>
      <span>P${it.priority ?? "?"}</span>
      <code>${escapeHtml(it.id)}</code>
      ${it.assignee ? `<span>👤 ${escapeHtml(it.assignee)}</span>` : ""}
      ${it.created_at ? `<span>created ${escapeHtml(it.created_at)}</span>` : ""}
      ${it.updated_at ? `<span>updated ${escapeHtml(it.updated_at)}</span>` : ""}
      ${it.closed_at ? `<span>closed ${escapeHtml(it.closed_at)}</span>` : ""}
    </div>

    ${it.description ? `<section><h2>description</h2><div class="desc md">${renderMarkdown(it.description)}</div></section>` : ""}
    ${it.acceptance_criteria ? `<section><h2>acceptance criteria</h2><div class="desc md">${renderMarkdown(it.acceptance_criteria)}</div></section>` : ""}
    ${it.design ? `<section><h2>design</h2><div class="desc md">${renderMarkdown(it.design)}</div></section>` : ""}
    ${it.notes ? `<section><h2>notes</h2><div class="desc md">${renderMarkdown(it.notes)}</div></section>` : ""}
    ${it.close_reason ? `<section><h2>close reason</h2><div class="desc md">${renderMarkdown(it.close_reason)}</div></section>` : ""}

    <section class="deps">
      <h2>parent</h2>
      ${parent ? `<ul><li><a href="#" data-go="${escapeHtml(parent.depends_on_id)}">${escapeHtml(shortId(parent.depends_on_id))}</a> — ${escapeHtml((state.byId.get(parent.depends_on_id) || {}).title || "?")}</li></ul>` : `<div class="empty">none (root)</div>`}

      <div class="children-head">
        <h2>children (${childIds.length}${descendantCount > childIds.length ? ` / ${descendantCount} total` : ""})</h2>
        ${childIds.length ? `<div class="children-actions">
          <button class="mini-btn" data-detail-expand>expand all</button>
          <button class="mini-btn" data-detail-collapse>collapse all</button>
        </div>` : ""}
      </div>
      <div class="children-tree" id="detail-children"></div>

      <h2>blocks (${blocks.length})</h2>
      ${linkList(blocks.map(b => b.depends_on_id))}

      <h2>related (${related.length})</h2>
      ${linkList(related.map(b => b.depends_on_id))}

      <h2>dependents (${dependents.length})</h2>
      ${dependents.length ? `<ul>${dependents.map(d => `<li><a href="#" data-go="${escapeHtml(d.id)}">${escapeHtml(shortId(d.id))}</a> — ${escapeHtml((state.byId.get(d.id) || {}).title || "?")} <span style="color:var(--fg-faint)">(${escapeHtml(d.type)})</span></li>`).join("")}</ul>` : `<div class="empty">none</div>`}
    </section>
  `;

  const $childrenTree = $detail.querySelector("#detail-children");
  if ($childrenTree && childIds.length) renderDetailChildren($childrenTree, childIds);

  $detail.querySelectorAll("[data-detail-expand]").forEach(b => b.addEventListener("click", () => {
    for (const id of allDescendants(childIds)) state.detailCollapsed.delete(id);
    if ($childrenTree) renderDetailChildren($childrenTree, childIds);
    saveState();
  }));
  $detail.querySelectorAll("[data-detail-collapse]").forEach(b => b.addEventListener("click", () => {
    for (const id of allDescendants(childIds)) state.detailCollapsed.add(id);
    if ($childrenTree) renderDetailChildren($childrenTree, childIds);
    saveState();
  }));

  $detail.querySelectorAll("a[data-go]").forEach(a => {
    if (a.dataset.bound === "1") return;
    a.dataset.bound = "1";
    a.addEventListener("click", (e) => {
      e.preventDefault();
      navigateTo(a.dataset.go);
    });
  });
}

// ---------- load / persist / wiring ----------
async function reload({ pull = false } = {}) {
  if (pull) {
    $status.textContent = "pulling…";
    try {
      const pr = await fetch("/api/pull", { method: "POST" });
      const pd = await pr.json().catch(() => ({}));
      if (!pr.ok) $status.textContent = "pull failed: " + (pd.error || pr.status);
    } catch (e) {
      $status.textContent = "pull failed: " + e.message;
    }
  }
  $status.textContent = "loading…";
  const r = await fetch("/api/issues");
  if (r.status === 401) { showLogin(); return; }
  if (!r.ok) { $status.textContent = "load failed: " + r.status; return; }
  const data = await r.json();
  buildIndex(data.issues);

  // URL hash wins over the last-saved selection so deep links open the right issue.
  const routeId = hashId();
  const target = (routeId && state.byId.has(routeId)) ? routeId
    : (state.selected && state.byId.has(state.selected)) ? state.selected
    : null;

  if (target) {
    navigateTo(target, { push: false });   // reveals + renders the row and detail
    syncUrl(target, { replace: true });    // stamp the current history entry
    if (routeId && isMobile()) document.body.classList.add("show-detail");
  } else {
    state.selected = null;
    renderMain();
  }

  const saved = loadState();
  if (!target && saved && typeof saved.scrollTree === "number") {
    requestAnimationFrame(() => { $tree.scrollTop = saved.scrollTree; });
  }
  saveState();
}

const isView = v => v === "tree" || v === "graph" || v === "dashboard";

function restoreFromStorage() {
  const urlView = new URLSearchParams(location.search).get("view");
  const saved = loadState();
  if (!saved) {
    if (isView(urlView)) state.view = urlView;
    setActiveViewBtn();
    return;
  }
  if (typeof saved.query === "string") state.query = saved.query;
  if (Array.isArray(saved.statuses)) state.statuses = new Set(saved.statuses);
  if (Array.isArray(saved.types)) state.types = new Set(saved.types);
  if (Array.isArray(saved.collapsed)) state.collapsed = new Set(saved.collapsed);
  if (Array.isArray(saved.detailCollapsed)) state.detailCollapsed = new Set(saved.detailCollapsed);
  if (typeof saved.selected === "string") state.selected = saved.selected;
  if (isView(saved.view)) state.view = saved.view;
  else if (saved.view === "sequence") state.view = "graph"; // migrate
  if (Array.isArray(saved.edgeTypes)) state.edgeTypes = new Set(saved.edgeTypes);
  if (typeof saved.simpleGraph === "boolean") state.simpleGraph = saved.simpleGraph;
  if (isView(urlView)) state.view = urlView; // URL param wins

  $search.value = state.query || "";
  syncSearchClear();
  setActiveViewBtn();
  if (saved.showDetail && isMobile()) document.body.classList.add("show-detail");
}

function setActiveViewBtn() {
  document.querySelectorAll(".view-btn").forEach(b => {
    b.classList.toggle("active", b.dataset.view === state.view);
  });
}

function bind() {
  $search.addEventListener("input", () => {
    state.query = $search.value;
    if (state.query.trim()) state.collapsed.clear();
    syncSearchClear();
    renderMain();
    saveState();
  });

  $searchClear.addEventListener("click", () => {
    $search.value = "";
    state.query = "";
    syncSearchClear();
    renderMain();
    saveState();
    $search.focus();
  });

  $search.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && $search.value) {
      $searchClear.click();
      e.preventDefault();
    }
  });

  document.querySelectorAll(".filters input[data-status]").forEach(el => {
    el.checked = state.statuses.has(el.dataset.status);
    el.addEventListener("change", () => {
      if (el.checked) state.statuses.add(el.dataset.status);
      else state.statuses.delete(el.dataset.status);
      renderMain();
      saveState();
    });
  });
  document.querySelectorAll(".filters input[data-type]").forEach(el => {
    el.checked = state.types.has(el.dataset.type);
    el.addEventListener("change", () => {
      if (el.checked) state.types.add(el.dataset.type);
      else state.types.delete(el.dataset.type);
      renderMain();
      saveState();
    });
  });

  document.querySelectorAll(".view-btn").forEach(b => {
    b.addEventListener("click", () => {
      state.view = b.dataset.view;
      setActiveViewBtn();
      renderMain();
      saveState();
    });
  });

  document.getElementById("expand-all").addEventListener("click", () => {
    state.collapsed.clear();
    renderMain();
    saveState();
  });
  document.getElementById("collapse-all").addEventListener("click", () => {
    state.collapsed = new Set(state.issues.map(i => i.id));
    renderMain();
    saveState();
  });
  document.getElementById("reload").addEventListener("click", () => reload({ pull: true }));

  $back.addEventListener("click", () => {
    document.body.classList.remove("show-detail");
    saveState();
  });

  // Browser back/forward walk the selection history (deep links).
  window.addEventListener("popstate", (e) => {
    const id = (e.state && e.state.id) || hashId();
    if (id && state.byId.has(id)) {
      navigateTo(id, { push: false });
      if (isMobile()) document.body.classList.add("show-detail");
    } else {
      state.selected = null;
      renderMain();
      $detail.innerHTML = `<div class="empty">Select an issue to see details.</div>`;
      document.body.classList.remove("show-detail");
      saveState();
    }
  });

  $toggleFilters.addEventListener("click", () => {
    const open = $filterRow.classList.toggle("open");
    $toggleFilters.setAttribute("aria-expanded", open ? "true" : "false");
  });

  $tree.addEventListener("scroll", () => {
    if (bind._scrollTimer) clearTimeout(bind._scrollTimer);
    bind._scrollTimer = setTimeout(saveState, 200);
  });

  let lastMobile = isMobile();
  window.addEventListener("resize", () => {
    const m = isMobile();
    if (m !== lastMobile) { lastMobile = m; renderMain(); }
  });
}

// ---------- login ----------
const $loginOverlay = document.getElementById("login-overlay");
const $loginForm = document.getElementById("login-form");
const $loginToken = document.getElementById("login-token");
const $loginError = document.getElementById("login-error");
const $loginMsg = document.getElementById("login-msg");
const $loginSubmit = document.getElementById("login-submit");
let loginCustom = false;

async function showLogin() {
  let mode = "token", loginUrl = "";
  try {
    const r = await fetch("/api/authinfo");
    if (r.ok) { const info = await r.json(); mode = info.mode; loginUrl = info.login_url || ""; }
  } catch (_) { /* offline — assume token */ }
  if (mode === "none") return; // no auth required
  loginCustom = mode === "custom";
  if (loginCustom) {
    // A host (e.g. the hosted SaaS) owns login — send the user to it.
    if (loginUrl) { location.href = loginUrl; return; }
    $loginMsg.textContent = "Authentication required.";
    $loginToken.style.display = "none";
    $loginSubmit.textContent = "Reload";
  } else {
    $loginMsg.textContent = "Enter your access token to continue.";
    $loginToken.style.display = "";
    $loginSubmit.textContent = "Sign in";
  }
  $loginOverlay.hidden = false;
  setTimeout(() => $loginToken.focus(), 40);
}
function hideLogin() { $loginOverlay.hidden = true; $loginError.textContent = ""; }

$loginForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  $loginError.textContent = "";
  if (loginCustom) { location.reload(); return; }
  const token = $loginToken.value.trim();
  if (!token) { $loginError.textContent = "Token required"; return; }
  $loginSubmit.disabled = true;
  try {
    const r = await fetch("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
    });
    if (r.ok) { $loginToken.value = ""; hideLogin(); reload(); }
    else {
      const d = await r.json().catch(() => ({}));
      $loginError.textContent = d.error || ("Sign in failed (" + r.status + ")");
    }
  } catch (err) {
    $loginError.textContent = err.message;
  } finally {
    $loginSubmit.disabled = false;
  }
});

// ---- API keys pane ----
const $keysOverlay = document.getElementById("keys-overlay");
const $keysForm = document.getElementById("keys-form");
const $keysLabel = document.getElementById("keys-label");
const $keysList = document.getElementById("keys-list");
const $keysError = document.getElementById("keys-error");
const $keysNew = document.getElementById("keys-new");
const $keysSecret = document.getElementById("keys-secret");

function fmtKeyDate(ts) { return ts ? ts.slice(0, 10) : "—"; }

async function loadKeys() {
  $keysError.textContent = "";
  try {
    const r = await fetch("/api/v1/keys");
    if (r.status === 401) { hideKeys(); showLogin(); return; }
    if (!r.ok) { $keysError.textContent = "Failed to load keys (" + r.status + ")"; return; }
    const keys = await r.json();
    renderKeys(keys || []);
  } catch (e) { $keysError.textContent = e.message; }
}

function renderKeys(keys) {
  if (!keys.length) { $keysList.innerHTML = '<div class="keys-empty">No keys yet.</div>'; return; }
  $keysList.innerHTML = "";
  for (const k of keys) {
    const row = document.createElement("div");
    row.className = "keys-row" + (k.revoked_at ? " revoked" : "");
    const meta = document.createElement("div");
    meta.className = "keys-meta";
    const state = k.revoked_at ? "revoked" : "active";
    meta.innerHTML =
      '<span class="keys-id">' + escapeHtml(k.id) + "</span>" +
      '<span class="keys-lbl">' + escapeHtml(k.label || "—") + "</span>" +
      '<span class="keys-state ' + state + '">' + state + "</span>" +
      '<span class="keys-dates">created ' + fmtKeyDate(k.created_at) +
      " · last used " + (k.last_used_at ? fmtKeyDate(k.last_used_at) : "never") + "</span>";
    row.appendChild(meta);
    if (!k.revoked_at) {
      const btn = document.createElement("button");
      btn.className = "keys-revoke";
      btn.textContent = "Revoke";
      btn.onclick = () => revokeKey(k.id);
      row.appendChild(btn);
    }
    $keysList.appendChild(row);
  }
}

async function revokeKey(id) {
  if (!confirm("Revoke key " + id + "? Anything using it stops working immediately.")) return;
  try {
    const r = await fetch("/api/v1/keys/" + encodeURIComponent(id) + "/revoke", { method: "POST" });
    if (!r.ok) { $keysError.textContent = "Revoke failed (" + r.status + ")"; return; }
    loadKeys();
  } catch (e) { $keysError.textContent = e.message; }
}

$keysForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  $keysError.textContent = "";
  try {
    const r = await fetch("/api/v1/keys", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ label: $keysLabel.value.trim() }),
    });
    if (!r.ok) { $keysError.textContent = "Create failed (" + r.status + ")"; return; }
    const k = await r.json();
    $keysSecret.textContent = k.secret;
    $keysNew.hidden = false;
    $keysLabel.value = "";
    loadKeys();
  } catch (e) { $keysError.textContent = e.message; }
});

document.getElementById("keys-copy").addEventListener("click", async () => {
  try { await navigator.clipboard.writeText($keysSecret.textContent); } catch (_) {}
  const b = document.getElementById("keys-copy");
  b.textContent = "Copied"; setTimeout(() => (b.textContent = "Copy"), 1200);
});

function openKeys() { $keysNew.hidden = true; $keysOverlay.hidden = false; loadKeys(); }
function hideKeys() { $keysOverlay.hidden = true; }
document.getElementById("settings").addEventListener("click", openKeys);
document.getElementById("keys-close").addEventListener("click", hideKeys);
$keysOverlay.addEventListener("click", (e) => { if (e.target === $keysOverlay) hideKeys(); });

restoreFromStorage();
bind();
reload();
