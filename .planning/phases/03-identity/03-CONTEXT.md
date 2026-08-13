# Phase 3: Identity - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning

<domain>
## Phase Boundary

A person can log into the web UI with username + password (session cookie). An uploader can create an api_key (shown once, stored hashed) and present it as Bearer / X-API-Key. Empty user table bootstraps one admin from `GFS_BOOTSTRAP_*` or the process refuses to start. No archive upload/list/download (Phase 4). No audit table (Phase 6).

</domain>

<spec_lock>
## Requirements (locked via SPEC)

**3 requirements are locked.** See `docs/SPECIFICATIONS.md` §4 `/login` `/logout`, §5 bootstrap env, §6 auth and `.planning/REQUIREMENTS.md` AUTH-01, AUTH-02, AUTH-03.

**In scope:**
- SQLite users / sessions / api_keys schema
- Bootstrap admin (env once, fail closed if empty table)
- GET+POST `/login`, POST `/logout`, session cookie
- Create api_key (shown once); Bearer / X-API-Key (not query string)
- Never log password or raw api_key

**Out of scope:**
- Archive upload/list/download HTML inventory (Phase 4)
- AWS SDK / HeadBucket (Phase 5)
- Audit table / retention (Phase 6)
- OIDC, richer roles/scopes (v2 AUTH-05)
- Well-known default password

</spec_lock>

<decisions>
## Implementation Decisions

### Bootstrap
- **D-01:** After migrate, if `COUNT(users)=0`, require `GFS_BOOTSTRAP_ADMIN` + `GFS_BOOTSTRAP_PASSWORD` (min 8 chars), create `admin=1`. Missing/blank → exit 1. If users exist, ignore env. — **Reversibility:** costly — operators will set systemd from this contract.

### Crypto
- **D-02:** Passwords: `bcrypt` (`golang.org/x/crypto/bcrypt`, DefaultCost). SPEC allows argon2id or bcrypt; bcrypt needs no salt packing. Dummy hash on unknown user so Compare always runs.
- **D-03:** api_key: `gfs_` + hex(32 random bytes). Store SHA-256. Lookup by hash. Prefix (first 12 chars) for display only. No pepper env this phase.
- **D-04:** Session token: 32 random bytes hex in cookie `gfs_session`; store SHA-256; 24h absolute expiry; delete row on logout.

### HTTP / cookie
- **D-05:** Cookie: HttpOnly, Path=/, SameSite=Lax, Secure when `GFS_COOKIE_SECURE` true (default false for HTTP lab).
- **D-06:** Extract api_key from `X-API-Key` or `Authorization: Bearer` only. **Not** query string, **not** form `api_key` (SPEC; tighter than trigger).
- **D-07:** Routes this phase: `GET|POST /login` (unauth), `POST /logout` (session), `GET /` session HTML stub (not the archive list), `POST /v1/api-keys` (session, return raw key once), `GET /v1/me` (session **or** api_key). `POST /v1/users` admin+session so later humans can be added without another bootstrap.
- **D-08:** Wrong password → 401. Unauthenticated HTML → 302 `/login`. JSON clients with Accept/Content-Type json get JSON errors.

### Claude's Discretion
- Exact HTML wording of the login form (vanilla, English).
- Whether `/v1/users` body is JSON-only this phase (yes unless tests need form).

</decisions>

<canonical_refs>
## Canonical References

- `docs/SPECIFICATIONS.md` — §4 login/logout; §5 `GFS_BOOTSTRAP_*`; §6 auth
- `docs/GFS-CONSENSUS.md` — first admin env-once (locked 2026-08-12)
- `AGENTS.md` — no well-known default password
- `.planning/REQUIREMENTS.md` — AUTH-01, AUTH-02, AUTH-03
- `.planning/ROADMAP.md` — Phase 3
- `/Volumes/Data/addlink/github/groot-trigger/internal/auth/auth.go` — header extract shape (do not copy form `api_key`)
- `.planning/phases/02-process/02-CONTEXT.md` — store ping, slog, mux

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/store` — open/ping; add migrate + user/session/key repos
- `internal/server` — mux + access log; add login and authn
- `internal/config` — add bootstrap fields (not required at LoadFromEnv)

### Established Patterns
- Fail closed on stderr then exit 1
- httptest + `t.Setenv` + `t.TempDir`
- stdlib mux, html/template, no chi

### Integration Points
- Phase 4 upload will call the same authenticate() (session or api_key)
- Phase 6 audit will insert on login/logout/key create

</code_context>

<specifics>
## Specific Ideas

User locked env-once bootstrap (not admin/changeme). Continue GSD (`ok, sigamos`).

</specifics>

<deferred>
## Deferred Ideas

- Archive list on `GET /` — Phase 4
- CSRF token beyond SameSite=Lax
- Argon2id (bcrypt is enough)
- api_key pepper / rotation UI
- Trusted proxies
- Audit rows for login — Phase 6

</deferred>

---

*Phase: 3-Identity*
*Context gathered: 2026-08-12*
