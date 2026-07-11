// Thin fetch helpers for the auth + API-keys surfaces (same-origin, cookie auth).

export async function authInfo() {
  try {
    const r = await fetch("/api/authinfo");
    if (r.ok) return await r.json();
  } catch (_) { /* offline — assume token mode */ }
  return { mode: "token" };
}

export async function login(token) {
  const r = await fetch("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
  if (r.ok) return { ok: true };
  const d = await r.json().catch(() => ({}));
  return { ok: false, error: d.error || "Sign in failed (" + r.status + ")" };
}

export async function listKeys() {
  const r = await fetch("/api/v1/keys");
  if (r.status === 401) return { auth: false, keys: [] };
  if (!r.ok) throw new Error("Failed to load keys (" + r.status + ")");
  return { auth: true, keys: (await r.json()) || [] };
}

export async function createKey(label) {
  const r = await fetch("/api/v1/keys", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ label }),
  });
  if (!r.ok) throw new Error("Create failed (" + r.status + ")");
  return await r.json();
}

export async function revokeKey(id) {
  const r = await fetch("/api/v1/keys/" + encodeURIComponent(id) + "/revoke", { method: "POST" });
  if (!r.ok) throw new Error("Revoke failed (" + r.status + ")");
}
