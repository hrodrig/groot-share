# Backlog 999.1 — Audit fixes (2026-08-12)

**Source:** independent repo audit (agent kimi-k3), report at `.no-va-al-repo/auditoria-agent-kimik3-2026-08-12.md` (untracked).
**Captured:** 2026-08-13
**Status:** backlog — promote items into a milestone via `/gsd-review-backlog`.

All findings verified against `develop@dda902c`. Severity: M = medium, B = low, I = info.

## P0 — before tagging v0.1.0

| ID | Finding | Evidence | Status |
|----|---------|----------|--------|
| M-1 | Packaged man page describes **groot-trigger** (Kubernetes Jobs, `/v1/collect`, `GROOT_TRIGGER_*`) and wrong version `v0.1.1` vs `VERSION=0.1.0`. Ships in GoReleaser tarballs and BSD dists. | `contrib/man/man1/gfs.1:3-55` | open |
| M-4 | README says "Design only. No application code yet." with all 6 phases complete. | `README.md:16-20` | open |
| M-3 | api_key acts as a full session (download, delete, create-user, audit). SPEC §4 scopes it to the upload API. Decide: restrict code to `POST /v1/archives` or amend the SPEC. | `internal/server/identity.go:165-185` | open |
| M-2 | No rate limiting / lockout on `/login` (bcrypt cost is the only brake). | `internal/server/identity.go:32-71` | open |
| B-3 | BSD Make stub header reads "groot-trigger". | `Makefile:1` | open |

## P1 — hardening

| ID | Finding | Evidence | Status |
|----|---------|----------|--------|
| B-7 | Expired sessions are never purged (table grows unbounded). | `internal/store/identity.go` | open |
| B-4 | `POST /v1/users` returns 401 instead of 403 for non-admin. | `internal/server/identity.go:119-124` | open |
| B-5 | Internal ingest errors (disk full, staging) map to 400 `bad_request`. | `internal/server/archives.go:89-97` | open |
| B-6 | No Content-Type check on raw body, no gzip magic-byte validation. | `internal/server/archives.go:162-177` | open |
| B-1 | Dead condition in `uniqueHTTPKey`: `errors.Is(err, blob.ErrNotFound) \|\| err != nil` ≡ `err != nil` — Head network errors fail open the collision check. | `internal/server/catalog.go:51-53` | open |
| B-10 | `GFS_COOKIE_SECURE` defaults false; no TLS/HSTS guidance; SPEC §9 asked for a systemd unit or `deploy/` note — missing. | `internal/config/config.go:63` | open |
| M-5 | JSON logs carry a `gfs ` line prefix → stdout is not parseable JSON (jq/Vector/Fluent Bit). Deliberate (gghstats style) but undocumented trade-off. | `internal/logging/logging.go:44-69` | open |

## P2 — process / debt

| ID | Finding | Evidence | Status |
|----|---------|----------|--------|
| B-11 | CI runs no coverage gate and no govulncheck/grype (only `release-check` on tags does). | `.github/workflows/ci.yml` | open |
| B-12 | Coverage declining per phase: 82.2 → 74.7 → 72.7 → 67.2 → 67.0 (measured 66.9). Raise `COVER_MIN` gradually. | phase VERIFICATION files | open |
| B-2 | CHANGELOG stuck at "Stub cmd/gfs" under Unreleased. | `CHANGELOG.md:8-14` | open |
| B-8 | SQLite DSN without `foreign_keys=on` / WAL / `busy_timeout`. | `internal/store/store.go:26-31` | open |
| B-13 | `RetryLoop` / `SweepLoop` run on `context.Background()`; not cancelled on shutdown. | `cmd/gfs/main.go:76-79` | open |
| B-9 | `auth.EqualHash` is dead code in production. | `internal/auth/auth.go:79-85` | open |
| B-14 | (a) orphan home blobs on crash between rename and INSERT; (b) `DeleteArchive` removes row before file; (c) `GET .../delete` serves bytes; (d) `/v1/audit` readable by any authenticated user. | `internal/store/archives.go`; `internal/server/catalog.go:214-219` | open |
| I-1 | `gfs.db` inherits umask (typically 0644); consider 0600. | `internal/store/store.go` | open |
| I-2 | Release workflow: `goreleaser-action` `version: latest`; base images not digest-pinned. | `.github/workflows/release.yml`; `Dockerfile` | open |
| I-3 | `manager.Uploader` deprecated (SA1019, `//nolint`); migrate when SDK major lands. | `internal/blob/s3.go:95-97` | open |
| I-4 | Stuck transit (> `GFS_STAGING_GRACE`, default 24 h) is swept with only a Warn log — silent data loss window; needs operator alert (consensus open Q10). | `internal/server/sweep.go:120-151` | open |
| I-5 | govulncheck: 1 CVE in a required module, not reachable from gfs code — watch on dep bumps. | audit run 2026-08-12 | open |

## Done in this session

| ID | Finding | Resolution |
|----|---------|------------|
| UX-1 | UI did not read "enterprise" | Refactor: design-token CSS, app shell, data-grid tables, flash notices, drag-and-drop upload, native `<dialog>` delete confirm (2026-08-13) |
