/**
 * Route paths + builders/parsers, in one place. Components never hand-assemble
 * URLs — they call these, so the route shape has a single definition.
 */

/** The board (task list). */
export const BoardPath = "/";

const TASK_PATH_PREFIX = "/tasks/";

/** URL for a task's detail page. */
export function taskPath(id: string): string {
  return TASK_PATH_PREFIX + encodeURIComponent(id);
}

/** The task id encoded in a `/tasks/:id` path, or null if the path isn't one. */
export function taskIdFromPath(path: string): string | null {
  if (!path.startsWith(TASK_PATH_PREFIX)) return null;
  const rest = path.slice(TASK_PATH_PREFIX.length);
  if (rest.length === 0 || rest.includes("/")) return null;
  return decodeURIComponent(rest);
}

// ---- settings ----

const SETTINGS_PREFIX = "/settings";

/** The ordered settings sections (also their url slugs). The workspace sections
 * (members / workspace) only render when the board is Clerk-backed and an org is
 * active; the route slugs always exist so links can be built unconditionally. */
export const SETTINGS_SECTIONS = ["account", "keys", "connect", "members", "workspace"] as const;
export type SettingsSection = (typeof SETTINGS_SECTIONS)[number];

/** URL for a settings section (or the settings root). */
export function settingsPath(section?: SettingsSection): string {
  return section ? `${SETTINGS_PREFIX}/${section}` : SETTINGS_PREFIX;
}

/** Whether a path is within settings. */
export function isSettingsPath(path: string): boolean {
  return path === SETTINGS_PREFIX || path.startsWith(SETTINGS_PREFIX + "/");
}

/** The section a settings path targets, defaulting to the first. */
export function settingsSectionFromPath(path: string): SettingsSection {
  const slug = path.slice(SETTINGS_PREFIX.length + 1);
  return (SETTINGS_SECTIONS as readonly string[]).includes(slug)
    ? (slug as SettingsSection)
    : SETTINGS_SECTIONS[0];
}
