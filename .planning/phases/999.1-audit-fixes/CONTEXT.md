# Backlog 999.1 — Audit fixes (2026-08-12)

**Source:** independent repo audits (2026-08-12, 2026-08-18).
**Captured:** 2026-08-13
**Last reviewed:** 2026-08-19
**Status:** remaining items still backlog — promote via `/gsd-review-backlog`.

Severity: M = medium, B = low, I = info.

GitHub: public wording only. Finding IDs (B-1, M-2, …) are OK in issues.

## Closed

| ID | Finding | Resolution |
|----|---------|------------|
| M-1 | Packaged man page described groot-trigger | Done v0.2.0 — `contrib/man` describes gfs |
| M-4 | README said "Design only" | Done — README is the product page |
| M-3 | api_key acted as a full session | Done v0.2.0 — scopes `upload` / `read` (Phase 7) |
| M-2 | No rate limiting on `/login` | Done v0.2.2 — [#4](https://github.com/hrodrig/groot-share/issues/4) |
| M-5 | JSON logs prefixed with `gfs ` | Done v0.2.2 — [#6](https://github.com/hrodrig/groot-share/issues/6) |
| B-2 | CHANGELOG stuck at stub | Done — Keep a Changelog from v0.2.0 |
| B-3 | BSD Make stub header said groot-trigger | Done v0.2.4 — [#17](https://github.com/hrodrig/groot-share/issues/17) |
| B-4 | `POST /v1/users` 401 instead of 403 | Done v0.2.0 — RBAC returns 403 |
| B-12 | Coverage declining / gate too low | Done v0.2.0 — `COVER_MIN=80` |
| A1 | vps-s3 download/delete outside prefix | Done v0.2.2 — [#5](https://github.com/hrodrig/groot-share/issues/5) |
| A3 | Password change did not invalidate sessions | Done v0.2.3 — [#9](https://github.com/hrodrig/groot-share/issues/9) |
| A4 | SECURITY.md stale | Done v0.2.3 — [#10](https://github.com/hrodrig/groot-share/issues/10) |
| A5 | `.env.example` weak password | Done v0.2.3 — [#11](https://github.com/hrodrig/groot-share/issues/11) |
| B-1 | `uniqueHTTPKey` Head errors fail-open | Done v0.2.4 — [#16](https://github.com/hrodrig/groot-share/issues/16) |
| B-7 | Expired sessions never purged | Done v0.2.4 — [#19](https://github.com/hrodrig/groot-share/issues/19) |
| B-8 | SQLite without foreign_keys / WAL / busy_timeout | Done v0.2.4 — [#18](https://github.com/hrodrig/groot-share/issues/18) |
| B-11 | CI had no cover / govulncheck / grype on PRs | Done v0.2.4 — [#20](https://github.com/hrodrig/groot-share/issues/20) |
| I-1 | `gfs.db` inherited umask | Done v0.2.4 — [#18](https://github.com/hrodrig/groot-share/issues/18) |
| I-4 | Stuck transit swept with Warn only | Done v0.2.4 — [#21](https://github.com/hrodrig/groot-share/issues/21) |
| UX-1 | UI did not read "enterprise" | Done 2026-08-13 — design-token CSS / tickets |

## Still open

| ID | Finding | Evidence | Status |
|----|---------|----------|--------|
| B-5 | Internal ingest errors map to 400 `bad_request` | `internal/server/archives.go` | open |
| B-6 | No Content-Type / gzip magic-byte check on raw body | `internal/server/archives.go` | open |
| B-10 | `GFS_COOKIE_SECURE` defaults false | `internal/config/config.go` | open |
| B-13 | `RetryLoop` / `SweepLoop` use `context.Background()` | `cmd/gfs/main.go` | open |
| B-9 | `auth.EqualHash` dead in production | `internal/auth/auth.go` | open |
| B-14 | (a) orphan home blobs on crash; (b) `DeleteArchive` row before file; (c) `GET .../delete` serves bytes | store + catalog | open (14d audit-for-any-auth is SPEC) |
| I-2 | `goreleaser-action` `version: latest`; images not digest-pinned | `.github/workflows/release.yml` | open |
| I-3 | `manager.Uploader` deprecated (`//nolint`) | `internal/blob/s3.go` | open |
| I-5 | govulncheck: CVE in required module, not reachable | watch dep bumps | open |
| A2 | Reverse-proxy trust for Host / X-Forwarded-Proto | docs landed [#8](https://github.com/hrodrig/groot-share/issues/8); cookie Secure default still B-10 | partial |
