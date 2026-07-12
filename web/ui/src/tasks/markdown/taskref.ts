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
import { resolveTaskRef } from "./task-index.svelte.js";

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

function link(id: string, display: string, extra = ""): string {
  return `<a href="${taskPath(id)}" ${TASK_REF_ATTR}="${id}" class="font-medium text-primary underline decoration-primary/40 underline-offset-2 hover:decoration-primary${extra ? " " + extra : ""}">${display}</a>`;
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
        const { id, display } = token as TaskRefToken;
        return link(id, display);
      },
    },
  ],
  renderer: {
    // Ids are usually written in `backticks`; a code span whose whole content is
    // a real task becomes a (monospace) link, otherwise it stays plain code.
    codespan(token) {
      const id = resolveTaskRef(token.text);
      return id ? link(id, shortId(id), "font-mono") : `<code>${escapeHtml(token.text)}</code>`;
    },
  },
});

export function renderTaskMarkdown(text: string): string {
  return DOMPurify.sanitize(marked.parse(text, { async: false }) as string);
}
