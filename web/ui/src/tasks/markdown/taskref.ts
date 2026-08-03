/**
 * Markdown → sanitized HTML with task-id references turned into links, done in a
 * single parse pass: a marked inline extension matches id-shaped tokens and only
 * emits a link for ones that resolve to a real task (see resolveTaskRef). The
 * link is a real `/tasks/<id>` anchor carrying data-taskref, so the renderer can
 * intercept clicks for SPA navigation while cmd-click / hover still work.
 */
import { Marked, type Tokens } from "marked";
import DOMPurify from "dompurify";
import { shortId } from "$tasks/model/issue.js";
import { taskPath } from "$shared/router/routes.js";
import { resolveTaskRef, taskRefInfo } from "./task-index.svelte.js";

/** Attribute the click handler reads to route via the SPA router. */
export const TASK_REF_ATTR = "data-taskref";

// One id-ish token: a word (optionally hyphenated, for a full id) with an
// optional dotted numeric suffix — "proj-ps3t.2.1", "ps3t.2", "ps3t", or "6w6v".
// Selectors are base62 and may start with a digit, so allow a leading digit;
// pure numbers just fail the id lookup and stay plain text.
const TOKEN = /^[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*(?:\.\d+)*/;

interface TaskRefToken extends Tokens.Generic {
  type: "taskref";
  id: string;
  display: string;
}

// A task reference renders as a status pill: a status-colored dot + the task's
// title, as a link. It mirrors StatusBadge (which can't be used here — this is
// sanitized HTML injected into markdown, not a Svelte component). Status drives
// the dot color via a data-attr + global CSS (.taskref-pill in app.css), and the
// title falls back to the short id when a task has none. When the id isn't in
// the index yet (list still loading) we fall back to a plain id link.
function pill(id: string): string {
  const info = taskRefInfo(id);
  const href = `${taskPath(id)}`;
  if (!info) {
    return `<a href="${href}" ${TASK_REF_ATTR}="${id}" class="taskref-pill" data-s="">${escapeHtml(shortId(id))}</a>`;
  }
  const label = info.title.trim() || shortId(id);
  return (
    `<a href="${href}" ${TASK_REF_ATTR}="${id}" class="taskref-pill" data-s="${info.status}">` +
    `<span class="taskref-dot"></span>${escapeHtml(label)}</a>`
  );
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

const marked = new Marked({
  extensions: [
    {
      name: "taskref",
      level: "inline",
      start(src: string) {
        const i = src.search(/[A-Za-z0-9]/);
        return i < 0 ? undefined : i;
      },
      tokenizer(src: string): TaskRefToken | undefined {
        const m = TOKEN.exec(src);
        if (!m) return undefined;
        const id = resolveTaskRef(m[0]);
        if (!id) return undefined;
        return { type: "taskref", raw: m[0], id, display: shortId(id) };
      },
      renderer(token) {
        return pill((token as TaskRefToken).id);
      },
    },
  ],
  renderer: {
    // Ids are often written in `backticks`; a code span whose whole content is a
    // real task becomes a status pill, otherwise it stays plain code.
    codespan(token) {
      const id = resolveTaskRef(token.text);
      return id ? pill(id) : `<code>${escapeHtml(token.text)}</code>`;
    },
  },
});

export function renderTaskMarkdown(text: string): string {
  return DOMPurify.sanitize(marked.parse(text, { async: false }) as string);
}
