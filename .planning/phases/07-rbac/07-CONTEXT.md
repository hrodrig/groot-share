# Phase 7: Users CRUD + RBAC — Context

**Gathered:** 2026-08-13
**Status:** Ready for planning
**Target release:** v0.2.0

<domain>
## Phase Boundary

Admins manage users (CRUD). Every authenticated route enforces role permissions. api_keys carry scope (`upload` | `read`) and cannot act as full sessions. Session auth inherits the user's role. Self-service password change and api_key revoke. Minimal admin HTML at `/admin/users`.

Does **not** include: OIDC, per-archive visibility, quotas, analyze/LLM, presigned URLs.

</domain>

<spec_lock>
## Requirements

Promote **AUTH-05** from v2 deferred → implement in this phase. Amend `docs/SPECIFICATIONS.md` §6 (remove RBAC from §2 non-goals for v0.1.x; add §6.1 roles).

**In scope:**
- Roles: `viewer`, `uploader`, `admin` (locked — from GFS-CONSENSUS parked ideas)
- Permission matrix on all existing archive/audit/user routes
- api_key scope: `upload` (POST `/v1/archives` only) or `read` (list/download/audit GET)
- Users CRUD API (`GET|POST|PATCH|DELETE /v1/users`) — admin + session only
- Self-service: `PATCH /v1/me` (password), `GET|DELETE /v1/me/api-keys`
- Schema migration: `admin` INTEGER → `role` TEXT; `api_keys.scope`; `users.active`
- Bootstrap still creates one `admin` role user
- Guard: cannot delete/deactivate last admin
- HTML: `/admin/users` (admin), `/settings` (password + keys list)
- Close backlog **999.1 M-3** (api_key full-session leak)

**Out of scope:**
- OIDC / SSO
- Per-archive visibility (private / team / hybrid)
- Quotas (GB/user, uploads/day)
- api_key scope `admin` or user-management via api_key
- CSRF tokens beyond SameSite=Lax (unchanged)
- Rate limiting (stay in 999.1 M-2)

</spec_lock>

<decisions>
## Implementation Decisions

### Roles & permissions (locked)

| Permission | viewer | uploader | admin |
|------------|--------|----------|-------|
| `archives:read` (list, download) | ✓ | ✓ | ✓ |
| `archives:write` (upload) | — | ✓ | ✓ |
| `archives:delete` | — | — | ✓ session only |
| `audit:read` | ✓ | ✓ | ✓ |
| `users:manage` | — | — | ✓ session only |
| `apikeys:manage` (own keys) | — | ✓ | ✓ |

Retention sweeps remain system actor `retention` — not a human role.

### api_key scopes (locked)

- **`upload`:** `POST /v1/archives` only.
- **`read`:** `GET /v1/archives`, `GET /v1/archives/{id}`, `GET /v1/audit`, `GET /v1/me`.
- Default scope on create: `upload`.
- **uploader** role may create keys with scope `upload` only.
- **admin** may create keys with scope `upload` or `read`.
- **viewer** cannot create api_keys.
- api_key auth **never** grants `archives:delete` or `users:manage`.

### Schema migration (locked)

- **D-01:** Replace `users.admin INTEGER` with `users.role TEXT NOT NULL CHECK(role IN ('viewer','uploader','admin'))`.
- **D-02:** Add `users.active INTEGER NOT NULL DEFAULT 1`. Inactive users cannot log in or use keys.
- **D-03:** Add `api_keys.scope TEXT NOT NULL DEFAULT 'upload' CHECK(scope IN ('upload','read'))`.
- **D-04:** Migration on Open: if `admin` column exists, `UPDATE users SET role='admin' WHERE admin=1`, `role='uploader' WHERE admin=0`; then drop `admin`. Document in CHANGELOG (non-admins gain upload-only delete restriction).
- **D-05:** SQLite migrate via `PRAGMA user_version` or rebuild-table pattern in `store.migrate()`.

### HTTP errors (locked)

- **D-06:** Missing auth → 401. Authenticated but forbidden → **403** (fix 999.1 B-4).
- **D-07:** Last-admin guard → 409 on DELETE/PATCH deactivate self or demote last admin.

### Auth middleware (locked)

- **D-08:** New `internal/auth/perm.go`: `Permission` constants + `Can(user, perm, authMethod)`.
- **D-09:** `requirePermission(perm)` wraps handlers; session vs api_key passed from `actorFromRequest`.
- **D-10:** Extend `actorFromRequest` to return `(user, method, keyScope)`.

### Bootstrap (unchanged spirit)

- Empty table → `EnsureAdmin` creates user with `role=admin` (not `admin=1` column).

### Claude's Discretion

- Exact HTML layout for `/admin/users` (match existing design tokens in `html.go`).
- Whether DELETE user is hard delete or soft (`active=0`) — prefer **soft** (locked above).

</decisions>

<canonical_refs>
## Canonical References

- `docs/GFS-CONSENSUS.md` — parked roles/scopes (lines 133–135)
- `docs/SPECIFICATIONS.md` — §4 HTTP, §6 auth (amend in 07-01)
- `.planning/REQUIREMENTS.md` — AUTH-05 (promote to v1.1)
- `.planning/phases/999.1-audit-fixes/CONTEXT.md` — M-3, B-4
- `internal/server/identity.go` — current auth surface
- `internal/store/schema.go` — current tables
- Independent audits: `.no-va-al-repo/auditoria-cursor-composer-25-2026-08-12.md`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable
- `POST /v1/users` — extend, do not rewrite
- `requireAuth` — replace with `requirePermission` per route
- `handleCreateAPIKey` — add scope body field
- Home template — delete button only for admin (template conditional)

### Gaps to close
- Delete archive: today any authed user — restrict to admin
- api_key: today full session — restrict by scope
- No user list/update/delete
- No api_key revoke

### Integration
- Phase 6 audit: add actions `user.create`, `user.update`, `user.deactivate`, `apikey.revoke` (optional this phase; minimum: keep existing upload/download/delete)
- Tests: table-driven role × route matrix

</code_context>

<execution_order>
## Plans (execute in order)

1. **07-01** — RBAC core: schema migration, perm.go, restrict existing routes, SPEC amend, close M-3
2. **07-02** — Users CRUD API + self-service password + last-admin guard
3. **07-03** — api_key scope create/list/revoke + admin HTML + `/settings`

Each plan = one PR. `make ci` green after each. Raise `COVER_MIN` to 70 in 07-03.

</execution_order>

<deferred>
## Deferred (Phase 8+)

- OIDC
- Visibility enum per archive
- Quotas
- Rate limiting (999.1)
- api_key rotation automation
- Audit rows for every user admin action (nice-to-have in 07-02)

</deferred>

---

*Phase: 7-RBAC*
*Context gathered: 2026-08-13*
