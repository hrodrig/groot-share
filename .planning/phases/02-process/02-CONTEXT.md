# Phase 2: Process - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Operators can start **gfs** with a topology. The process refuses bad config (fail closed), logs slog JSON, listens on `GFS_LISTEN`, and answers unauthenticated `GET /healthz` and `GET /readyz`. No login, no upload, no archive list, no AWS SDK / HeadBucket (Phase 5), no user schema (Phase 3).

</domain>

<spec_lock>
## Requirements (locked via SPEC)

**4 requirements are locked.** See `docs/SPECIFICATIONS.md` (HTTP §4 probes, config §5, observability §8) and `.planning/REQUIREMENTS.md` AUTH-04, OPS-01, OPS-02, OPS-03.

Downstream agents MUST read those files before planning or implementing. Requirement text is not duplicated here.

**In scope:**
- Env config `GFS_*` with fail-closed topology
- slog JSON process logs + HTTP access line (skip probes)
- `GET /healthz` 200 without auth
- `GET /readyz` SQLite ping; vps-s3 also requires S3 env still present
- stdlib HTTP listen + SIGTERM shutdown (trigger shape)

**Out of scope:**
- `/login`, sessions, api_key, bootstrap admin (Phase 3)
- Upload / list / download / HTML (Phase 4)
- AWS SDK, HeadBucket, transit copy, prefix listing (Phase 5)
- Audit table, retention job (Phase 6)
- Topology **S3 only** as a gfs process (never)

</spec_lock>

<decisions>
## Implementation Decisions

### Config
- **D-01:** Env-only `internal/config.LoadFromEnv()` like groot-trigger. No Viper. Prefix `GFS_*`. `GFS_LISTEN` default `:8080` (SPEC; do **not** reuse trigger's unprefixed `LISTEN_ADDR`). — **Reversibility:** costly — every later phase reads this struct.
- **D-02:** `GFS_TOPOLOGY` required. Allowed: `vps`, `vps-s3`. Missing, empty, `s3`, or anything else → exit non-zero (fail closed). Tests set `GFS_TOPOLOGY=vps`.
- **D-03:** `GFS_DATA_DIR` required, no default. `MkdirAll` 0750; if not writable → exit. Holds `gfs.db` now; staging/home dirs later.
- **D-04:** Topology `vps-s3` also requires `GFS_S3_BUCKET`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`. Optional: `GFS_S3_ENDPOINT` (empty = AWS default), `GFS_S3_REGION` default `us-east-1`, `GFS_S3_PREFIX` default `captures/`, `GFS_S3_PATH_STYLE` default `true` when endpoint set else `false`. Store on Config; **do not** open an S3 client this phase.
- **D-05:** `GFS_LOG_FORMAT` default `json`, `GFS_LOG_LEVEL` default `info` (trigger names with `GFS_` prefix). Retention env (`GFS_KEEP_LAST` / `GFS_MAX_AGE_DAYS`) is **not** parsed until Phase 6.

### Process / HTTP
- **D-06:** Copy trigger `cmd` shape: `version`/`--version`/`-V` still prints and exits 0; **bare invoke starts HTTP** (replace the Phase-2 stub message). `internal/server` stdlib `ServeMux` only — no chi/gin. Routes this phase: `GET /healthz`, `GET /readyz`. Other paths 404.
- **D-07:** `GET /healthz` → `200` + `ok\n`, no auth, no dependency checks.
- **D-08:** `GET /readyz` → `200` + `ok\n` if SQLite ping succeeds; `503` otherwise. For `vps-s3`, also 503 if required S3 env fields are empty (should not happen after D-04). **No HeadBucket** this phase.
- **D-09:** Listen + graceful shutdown copied from trigger (`ReadHeaderTimeout` 5s, SIGINT/SIGTERM, 10s `Shutdown`).
- **D-10:** Access log like trigger: skip `/healthz` and `/readyz`; slog `http` with method, path, status, duration. IP = `RemoteAddr` only (trusted proxies later). Never log secrets.

### SQLite / slog
- **D-11:** Open `{GFS_DATA_DIR}/gfs.db` with **pure-Go** `modernc.org/sqlite` so `CGO_ENABLED=0` / distroless stay valid. Ping `SELECT 1`. No user/session schema (Phase 3).
- **D-12:** Copy trigger `internal/logging` (gghstats-style prefix writer). Line prefix `gfs `. **Writer is stdout** (SPEC §8), not trigger's stderr. `SetupWriter` for tests.

### Claude's Discretion
- Exact `GFS_S3_*` env key spellings as long as they match D-04.
- SQLite DSN / `_pragma` as long as ping works and CGO stays off.
- Whether `internal/store` is named `store` or `db`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Contract
- `docs/SPECIFICATIONS.md` — §4 `/healthz` `/readyz`; §5 config; §8 slog stdout
- `docs/GFS-CONSENSUS.md` — three topologies; S3-only is not a gfs process
- `AGENTS.md` — do/don't
- `.planning/REQUIREMENTS.md` — AUTH-04, OPS-01, OPS-02, OPS-03
- `.planning/ROADMAP.md` — Phase 2 success criteria

### Template (copy shape, rename)
- `/Volumes/Data/addlink/github/groot-trigger/internal/config/config.go`
- `/Volumes/Data/addlink/github/groot-trigger/internal/logging/logging.go`
- `/Volumes/Data/addlink/github/groot-trigger/internal/server/server.go` (probes + access log only)
- `/Volumes/Data/addlink/github/groot-trigger/cmd/groot-trigger/main.go` (listenAndServe)

### Prior phase
- `.planning/phases/01-supply-chain/01-CONTEXT.md` — COVER_MIN=60; HTTP was deferred here; pure-Go SQLite

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/gfs/main.go` — version/ldflags; replace bare-invoke stub with config+listen.
- `GNUmakefile` / CI — already `make ci` on `./...`; new packages must stay green and ≥ COVER_MIN 60.

### Established Patterns
- groot-trigger: fail-closed `LoadFromEnv`; slog JSON; stdlib mux `GET /healthz` `/readyz`; skip probes in access log; SIGTERM shutdown 10s; tests via `httptest` + `t.Setenv`.
- Deny-all gitignore whitelist — new files under `internal/` are source and are already allowed if `*.go` is whitelisted; confirm `.gitignore` if adding non-Go files.

### Integration Points
- Phase 3 will add schema + auth middleware on this mux.
- Phase 5 will add S3 client and HeadBucket into the existing `Ready` func.

</code_context>

<specifics>
## Specific Ideas

User: continue GSD (`sigue`); yolo — lock from SPEC + trigger, do not invent a second HTTP/config dialect. Local commits only.

</specifics>

<deferred>
## Deferred Ideas

- Live HeadBucket / list prefix on `/readyz` — Phase 5
- User/session schema, `/login`, api_key — Phase 3
- Trusted proxies / `X-Forwarded-For` — later (SPEC: v0.1 may use RemoteAddr)
- Retention env parsing — Phase 6
- Helm / k8s deploy — not until operators ask

</deferred>

---

*Phase: 2-Process*
*Context gathered: 2026-08-12*
