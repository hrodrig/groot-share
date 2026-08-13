# Phase 2 Research — Process

**Date:** 2026-08-12
**Sources:** groot-trigger internals (validated v0.1.x), `docs/SPECIFICATIONS.md` §4–§8, Phase 2 CONTEXT.

## Copy from groot-trigger

| Piece | Path | gfs mapping |
|-------|------|-------------|
| Fail-closed env | `internal/config.LoadFromEnv` | `GFS_*`; topology + data dir required |
| slog JSON + prefix writer | `internal/logging` | prefix `gfs `; **stdout** (SPEC), not stderr |
| Probes | `GET /healthz` `200 ok\n`; `/readyz` 200/503 | same bodies |
| Access log | skip probes; slog `http` | `RemoteAddr` only |
| Listen | `http.Server` + SIGINT/SIGTERM 10s Shutdown | copy `listenAndServe` |
| Tests | `t.Setenv`, `httptest`, no testify | same |

Do **not** copy: k8s jobs, rate limit, API key, HTML collect form, `LISTEN_ADDR` unprefixed.

## SQLite

- Driver: `modernc.org/sqlite` (pure Go). Phase 1 already locked CGO=0 / distroless.
- Phase 2: open `{GFS_DATA_DIR}/gfs.db`, `SELECT 1`. No migrations/schema.
- `/readyz` 503 if ping fails (dir gone, locked, etc.).

## S3

- Phase 2 validates env presence only (`vps-s3`).
- HeadBucket is Phase 5. Do not add `aws-sdk-go-v2` now.

## Risks

- COVER_MIN=60: new packages need tests (fail-closed table, healthz 200, readyz 503 without db).
- `GFS_DATA_DIR` required: every test must `t.Setenv` a `t.TempDir()`.
- JSON logs on stdout: `version` still uses `fmt` stdout; HTTP slog must not break version tests.
