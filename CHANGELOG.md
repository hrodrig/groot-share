# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] — unreleased

### Added

- RBAC: roles `viewer`, `uploader`, `admin` with permission checks on all authenticated routes
- API key scopes `upload` and `read`; list/revoke via `GET/DELETE /v1/me/api-keys`
- Admin HTML user management at `/admin/users`
- Self-service `/settings`: password change, API key create/revoke
- Copy download link button on Captures rows (absolute URL for CI artifacts)

### Changed

- Existing SQLite databases migrate `admin` boolean to `role` + `active`; legacy `admin=1` becomes role `admin`
- Minimum test coverage gate raised to 70% (`COVER_MIN`)

## [0.1.x]

### Added

- Behavior contract: `docs/SPECIFICATIONS.md`
- Packaging scaffold mirrored from groot-trigger (Make, Docker, GoReleaser, CI)
- Stub `cmd/gfs` (`version` only; HTTP is Phase 2)
