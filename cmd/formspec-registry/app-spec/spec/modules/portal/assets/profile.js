// ── Portal Profile Page (mode: custom asset) ──
//
// Menampilkan identitas user yang sedang sign-in: avatar inisial,
// username, roles, dan permissions. Data diambil dari /_meta/me
// (session identity) — bukan entity API, sehingga tidak butuh grant.
//
// Kontrak asset (07-component-kinds.md §4): default export dengan
// mount(el, props, formspec) / unmount(el).
//
// Styling memakai CSS variables tema FormSpec agar mengikuti light/dark.

function initialsOf(name) {
  const parts = String(name)
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .split(/[\s._@-]+/)
    .filter(Boolean)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
  return String(name).slice(0, 2).toUpperCase()
}

function esc(s) {
  return String(s ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  )
}

export function mount(el, _props, formspec) {
  el.innerHTML =
    '<div style="padding:2rem;color:var(--muted-foreground)">Loading…</div>'

  // formspec.api memakai prefixUrl /{ws}/_ui/entity — "../_meta/me"
  // menormalisasi ke /{ws}/_ui/_meta/me (endpoint identitas session).
  formspec.api
    .get("../_meta/me")
    .json()
    .then((res) => {
      const me = res?.data ?? {}
      const username = me.username || me.user_id || "Signed in"
      const roles = me.roles ?? []
      const perms = me.permissions ?? []

      el.innerHTML = `
        <div style="max-width:32rem;margin:0 auto;display:flex;flex-direction:column;gap:1rem">
          <div style="display:flex;align-items:center;gap:1rem;padding:1.5rem;border:1px solid var(--border);border-radius:var(--radius,0.75rem);background:var(--background)">
            <div style="width:4rem;height:4rem;border-radius:9999px;display:flex;align-items:center;justify-content:center;background:var(--primary);color:var(--primary-foreground);font-size:1.25rem;font-weight:600;flex-shrink:0">
              ${esc(initialsOf(username))}
            </div>
            <div style="min-width:0">
              <div style="font-size:1.125rem;font-weight:600;color:var(--foreground)">${esc(username)}</div>
              <div style="font-size:0.8125rem;color:var(--muted-foreground)">User ID: ${esc(me.user_id)}</div>
              <div style="font-size:0.8125rem;color:var(--muted-foreground)">Workspace: ${esc(me.workspace)}</div>
            </div>
          </div>

          <div style="padding:1.25rem;border:1px solid var(--border);border-radius:var(--radius,0.75rem);background:var(--background)">
            <div style="font-size:0.875rem;font-weight:600;color:var(--foreground);margin-bottom:0.5rem">Roles</div>
            ${
              roles.length
                ? `<div style="display:flex;flex-wrap:wrap;gap:0.375rem">${roles
                    .map(
                      (r) =>
                        `<span style="font-size:0.75rem;padding:0.125rem 0.5rem;border-radius:9999px;border:1px solid var(--border);color:var(--foreground)">${esc(r)}</span>`,
                    )
                    .join("")}</div>`
                : '<div style="font-size:0.8125rem;color:var(--muted-foreground)">No roles assigned.</div>'
            }
          </div>

          <div style="padding:1.25rem;border:1px solid var(--border);border-radius:var(--radius,0.75rem);background:var(--background)">
            <div style="font-size:0.875rem;font-weight:600;color:var(--foreground);margin-bottom:0.5rem">Permissions (${perms.length})</div>
            ${
              perms.length
                ? `<div style="display:flex;flex-wrap:wrap;gap:0.375rem;max-height:12rem;overflow-y:auto">${perms
                    .map(
                      (p) =>
                        `<span style="font-size:0.75rem;padding:0.125rem 0.5rem;border-radius:9999px;border:1px solid var(--border);color:var(--muted-foreground)">${esc(p)}</span>`,
                    )
                    .join("")}</div>`
                : '<div style="font-size:0.8125rem;color:var(--muted-foreground)">No permissions granted.</div>'
            }
          </div>
        </div>
      `
    })
    .catch(() => {
      el.innerHTML =
        '<div style="padding:2rem;color:var(--destructive)">Failed to load profile.</div>'
    })
}

export function unmount(el) {
  el.innerHTML = ""
}
