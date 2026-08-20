# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.0] — 2026-08-19

### Added

- External share links (Phase 9 / SHARE-01..03): admin-only time-limited download links for a single archive
  - `POST /v1/archives/{id}/shares` — create with `expires_at` **or** `expires_in` (mutually exclusive), optional `label` and `max_uses` (`1` = one-shot); full URL returned once
  - `GET /v1/archives/{id}/shares` — list active links (token never returned)
  - `DELETE /v1/archives/{id}/shares/{share_id}` — revoke before expiry
  - `GET /s/{token}` — unauthenticated streaming download proxied through gfs (works on `vps` and `vps-s3`); `404` on unknown/expired/revoked/exhausted
  - Token is 32 random bytes (hex); only its SHA-256 hash is stored; audit logs never contain the raw token

### Security

- Reject raw upload bodies that are not gzip by magic-byte check (B-6)
- Cookie `Secure` defaults to `true` (B-10)

### Changed

- Lower bcrypt work factor to `MinCost` in tests via `auth.UseTestCost()`; production stays `DefaultCost` ([#13](https://github.com/hrodrig/groot-share/issues/13))
- Cache vps-s3 object listings (5s TTL, invalidated on upload/delete/retry) to avoid a full `ListObjects` per page ([#12](https://github.com/hrodrig/groot-share/issues/12))
- PR/push CI runs govulncheck and directory Grype in the `security` job ([#20](https://github.com/hrodrig/groot-share/issues/20))
- Pin GoReleaser to `~2.16` instead of `latest` (I-2)
- Ingest internal errors return `500` (not `400`) (B-5)
- Background loops use signal-aware context instead of `context.Background()` (B-13)

### Fixed

- Delete removes the blob before the DB row; `GET .../delete` returns `404` instead of failing mid-way (B-14)

### Removed

- Dead `EqualHash` helper (B-9)

## [0.3.0] — 2026-08-19

### Added

- SFTP inbox watcher: `GFS_SFTP_INBOX` + `GFS_SFTP_POLL` (default 30s) ingest stable `*.tar.gz` with `source=sftp` (Phase 8 / ING-04)
- Captures Source pill **SFTP**; **vps-s3** object keys `{prefix}sftp/{yyyy}/{mm}/{dd}/{id}.tar.gz`

## [0.2.4] — 2026-08-19

### Security

- `vps-s3` HTTP ingest treats object-store `Head` errors as fatal when allocating a key ([#16](https://github.com/hrodrig/groot-share/issues/16))
- SQLite opens with `foreign_keys=ON`, WAL, `busy_timeout`; `gfs.db` is `chmod 0600` ([#18](https://github.com/hrodrig/groot-share/issues/18))

### Changed

- Retention sweep deletes expired session rows ([#19](https://github.com/hrodrig/groot-share/issues/19))
- Stuck-transit leftovers log at ERROR with `last_error` ([#21](https://github.com/hrodrig/groot-share/issues/21))
- PR/push CI runs the coverage gate; Security workflow runs govulncheck and directory Grype ([#20](https://github.com/hrodrig/groot-share/issues/20))
- BSD Make stub header names gfs ([#17](https://github.com/hrodrig/groot-share/issues/17))

## [0.2.3] — 2026-08-16

### Security

- Password change invalidates all sessions for that user; self-service also clears the cookie ([#9](https://github.com/hrodrig/groot-share/issues/9))

### Changed

- Document trusted reverse-proxy expectations for `Host` / `X-Forwarded-Proto` ([#8](https://github.com/hrodrig/groot-share/issues/8))
- Refresh `SECURITY.md` supported-versions wording ([#10](https://github.com/hrodrig/groot-share/issues/10))
- Strengthen `.env.example` bootstrap password placeholder and warning ([#11](https://github.com/hrodrig/groot-share/issues/11))

## [0.2.2] — 2026-08-16

### Security

- `vps-s3` download/delete reject object keys outside `GFS_S3_PREFIX` ([#5](https://github.com/hrodrig/groot-share/issues/5))
- Rate-limit `POST /login` per client IP and per username (default `GFS_LOGIN_RATE_LIMIT=20/1m` → `429`) ([#4](https://github.com/hrodrig/groot-share/issues/4))

### Changed

- Process logs emit pure slog JSON/text lines (no `gfs ` prefix) so jq and log shippers can parse stdout ([#6](https://github.com/hrodrig/groot-share/issues/6))

### Fixed

- Mark `github.com/aws/smithy-go` as a direct `go.mod` require (imported by `internal/blob`)

## [0.2.1] — 2026-08-13

### Fixed

- Build on Go 1.26.6 (stdlib CVEs reported against 1.26.5)
- Clamp `GFS_KEEP_LAST` / `GFS_MAX_AGE_DAYS` to `int` (CodeQL integer conversion)

### Added

- Operator chrome: `GFS_LOGIN_SIMPLE` (white `/login`, no product marks), `GFS_BRAND_SUB` (app-bar tag, default `archive door`), `GFS_FOOTER` (default family links; `-` hides)
- User **Name** (required). Header shows it, truncated at 30 runes (`Juan ...egro`). First admin defaults to `Administrator` or `GFS_BOOTSTRAP_ADMIN_NAME`. Login (`username`) is unique; only admin can change it (Users page / `PATCH /v1/users/{id}`). Own Settings: Name only.

### Changed

- Login gate uses the cropped crate + wordmark hero (~46 KiB JPEG) with a glass card; brand copy is not repeated above the form
- Users/Settings forms: field hints, specific password/username errors, and card padding so controls are not flush to the edge
- Admin Users: Activate and Remove on inactive accounts (JSON `DELETE` stays soft-deactivate)
- API keys record `last_used_at` on successful auth; Settings shows Last used (UTC) or `never`

## [0.2.0] — 2026-08-13

### Fixed

- Release CI: extra tests so merged coverage stays ≥80% on Linux runners (was 79.9% at the gate)

### Added

- RBAC: roles `viewer`, `uploader`, `admin` with permission checks on all authenticated routes
- API key scopes `upload` and `read`; list/revoke via `GET/DELETE /v1/me/api-keys`
- Admin HTML user management at `/admin/users`
- Self-service `/settings`: password change, API key create/revoke
- Copy download link button on Captures rows (absolute URL for CI artifacts)

### Changed

- Existing SQLite databases migrate `admin` boolean to `role` + `active`; legacy `admin=1` becomes role `admin`
- Minimum test coverage gate raised to 80% (`COVER_MIN`)
- Man page describes **gfs** (was incorrectly copied from groot-trigger)

## [0.1.x]

### Added

- Behavior contract: `docs/SPECIFICATIONS.md`
- Packaging scaffold mirrored from groot-trigger (Make, Docker, GoReleaser, CI)
- Stub `cmd/gfs` (`version` only; HTTP is Phase 2)
