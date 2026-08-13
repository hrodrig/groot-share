# gfs — specifications

**Status:** Approved (2026-08-12) for MVP shape. HTTP paths below are the contract unless a later SPEC revision says otherwise.  
**Repo:** `groot-share` (product / binary **gfs**)  
**Product freeze:** [GFS-CONSENSUS.md](GFS-CONSENSUS.md)  
**Supply chain reference:** [groot-trigger](https://github.com/hrodrig/groot-trigger) (`GNUmakefile`, GoReleaser, CI, distroless, `v`-tags)  
**Not in scope:** groot CLI collect behavior; CronJob packaging (**groot** / **groot-selfhosted**); topology **S3 only** (no gfs process).

---

## 1. Problem

A ~20-person team on a shared cluster wants one place for groot `.tar.gz` archives without putting S3-compatible keys on every laptop. Archives can be several GB. An in-cluster **groot-trigger** Job can already `upload.s3` (multipart). gfs is the authenticated door when a VPS exists.

## 2. Goals / non-goals

### Goals

- Three operator topologies (deploy-time, **no per-upload S3 flag**):
  - **VPS only** — gfs HTTP ingest; disk is home; listing is local
  - **S3 only** — **not this binary**; groot `upload.s3` + S3 client
  - **VPS + S3** — gfs HTTP ingest transits staging → bucket; listing **from the bucket**; cluster `upload.s3` to the same prefix is preferred
- Per-user web login (password + session) and upload api_key
- Audit without secrets; retention keep_last **and** max_age_days
- Vanilla / server-rendered HTML for list + download (English UI strings)
- Packaging identical in spirit to **groot-trigger** (Make, GoReleaser, distroless, CI)
- English-only repo artifacts

### Non-goals (v0.1.x)

- Analyze / compare / BYOK LLM
- `groot upload --gfs`
- OIDC, RBAC roles, quotas, presigned PUT
- HTTP inside the groot binary
- Download proxy inside groot-trigger
- Multi-tenant SaaS

## 3. Architecture

```
laptop                         cluster (trigger / CronJob)
  │ HTTP + api_key                    │ upload.s3 (preferred when bucket exists)
  ▼                                   ▼
┌─────────────────┐              ┌─────────────┐
│ gfs (VPS)       │              │ S3-compatible│
│ auth, SQLite,   │   transit    │ bucket      │
│ staging, UI     │ ───────────► │ prefix      │
│ list/download   │◄── list ──── │ captures/   │
└─────────────────┘              └─────────────┘
```

| Topology | Bytes home | List source | Cluster ingest |
|----------|------------|-------------|----------------|
| VPS only | VPS disk | SQLite + files | HTTP to gfs |
| VPS + S3 | bucket | bucket | **`upload.s3` preferred**; HTTP to gfs also valid (transit) |
| S3 only | bucket | S3 client | `upload.s3` — **no gfs** |

**Model:** one gfs process on a VPS. SQLite = users, sessions, api_keys, audit. Inventory of files in VPS + S3 = `ListObjects` on the prefix, not a parallel catalog.

Object key (both writers): `{key_prefix}{yyyy}/{mm}/{dd}/{id}.tar.gz` with `id` unique (ULID or sha256 prefix). groot `upload.s3` today keys from filename + prefix — gfs listing must accept **whatever keys already land** under the configured prefix (do not require groot to change in v0.1). HTTP ingest from gfs uses the dated-id scheme. Collision: last writer wins is unacceptable; if a key exists, HTTP ingest picks a new id.

## 4. HTTP contract

Unauthenticated:

| Method | Path | Behavior |
|--------|------|----------|
| GET | `/healthz` | Liveness |
| GET | `/readyz` | SQLite OK; if S3 configured, HeadBucket/list prefix must succeed |
| GET | `/login` | Login form |
| POST | `/login` | username + password → session cookie; fail 401 |

Authenticated (session cookie **or** api_key for upload API):

| Method | Path | Behavior |
|--------|------|----------|
| POST | `/logout` | Clear session |
| GET | `/` | Vanilla HTML list (session) |
| GET | `/v1/archives` | JSON list (session) |
| POST | `/v1/archives` | Upload body `.tar.gz` (api_key **or** session); `201` + metadata |
| GET | `/v1/archives/{id}` | Download (session); `404` if unknown |
| GET | `/v1/archives/{id}/file` | Same bytes (HTML “download” link may use this) |

Upload auth: `Authorization: Bearer <api_key>` or `X-API-Key` (trigger-style). Do not accept api_key on the query string.

`POST /v1/archives`:

- `Content-Type: application/gzip` or `application/octet-stream` (raw body) or `multipart/form-data` field `file`
- Max size configurable (default high enough for several GB; stream to staging, **do not** buffer whole archive in RAM)
- On VPS + S3: write staging, copy to bucket, delete staging; if copy fails → `201` with `storage: transit` + retry (do not `5xx` the laptop)
- On VPS only: write home dir, `201` with `storage: local`

List JSON (shape): `{ "items": [ { "id", "key", "size", "etag_or_sha256", "created_at", "source": "http"|"s3" } ] }`  
In VPS + S3, `source=s3` includes objects groot wrote that gfs never saw over HTTP.

## 5. Config (operator)

Environment / file (names may match trigger `GROOT_*` style with `GFS_` prefix):

| Setting | Purpose |
|---------|---------|
| `GFS_LISTEN` | default `:8080` |
| `GFS_DATA_DIR` | SQLite + staging/home root (e.g. `/var/lib/gfs`) |
| `GFS_TOPOLOGY` | `vps` \| `vps-s3` (`s3` alone is invalid — refuse start) |
| `GFS_S3_*` | bucket, region, endpoint, prefix (`captures/`), path-style |
| AWS creds | env `AWS_*` on the VPS only |
| `GFS_KEEP_LAST` / `GFS_MAX_AGE_DAYS` | retention defaults 20 / 90 |
| `GFS_BOOTSTRAP_ADMIN` / `GFS_BOOTSTRAP_PASSWORD` | first admin only; required when the user table is empty |
| `GFS_MAX_UPLOAD_BYTES` | default 32GiB; stream cap (`http.MaxBytesReader`) |

Fail closed: `vps-s3` without bucket/creds → exit. Empty data dir permissions → exit. Empty user table without bootstrap env → exit.

## 6. Auth detail

- Password: argon2id or bcrypt; never log
- api_key: opaque, hashed at rest (SHA-256 of key + pepper, or equivalent); show once
- Session: httpOnly cookie; Secure when TLS
- Bootstrap of first admin (locked): if the user table is **empty**, require `GFS_BOOTSTRAP_ADMIN` + `GFS_BOOTSTRAP_PASSWORD`, create one **admin**, hash the password, log that bootstrap ran (**never** log the password). If users already exist, **ignore** those env vars (do not reset the password, do not create a second bootstrap user). Empty table + missing/blank env → refuse start (fail closed). **No** well-known default password (`admin`/`changeme` or similar). Operator should drop the env from the unit after first start; gfs does not require that.

## 7. Retention

Job (timer in-process or cron): delete objects that violate **either** keep_last **or** max_age_days.  
VPS only: delete files + sqlite rows if any.  
VPS + S3: delete bucket objects (home). Staging leftovers older than a grace period are swept as incidents, not as the retention set.

## 8. Observability

- slog JSON to stdout (trigger/gghstats style)
- HTTP access line with status; never api_key/password
- Audit table: actor, action, object key/id, ts, remote IP (respect trusted proxies later; v0.1 may use RemoteAddr only)

## 9. Supply chain (must match groot-trigger)

Copy patterns from `/Volumes/Data/addlink/github/groot-trigger`, renaming `groot-trigger` → `gfs` / module `github.com/hrodrig/groot-share`:

- `Makefile` BSD stub + `GNUmakefile`
- `Dockerfile` + `Dockerfile.release` (distroless, UID 65532)
- `.goreleaser.yaml` (v-prefixed tags, CGO=0, freebsd/openbsd)
- `.github/workflows/ci.yml` + `release.yml`
- `.golangci.yml`, `COVER_MIN` (start lower than 80 if needed; trigger raised to 80 after tests existed — gfs may start `COVER_MIN=60` and raise)
- `govulncheck` / `gocyclo` (threshold 14) / `grype`
- `VERSION`, `cmd/gfs/main.go`
- English README / CHANGELOG / SECURITY / CONTRIBUTING as they appear in trigger (adapt names)

**Do not** invent Helm until operators ask (trigger deferred Helm). Flat systemd unit or a short `deploy/` note is enough for a VPS.

## 10. Testing

- Unit: topology refuse, auth 401, hashed keys, transit retry state machine, list merges prefix objects
- `make ci` must pass
- No live bucket required for unit tests (fake S3 / fake fs)

## 11. Open (do not block v0.1 code)

- Visibility enum (private / team / hybrid) — MVP: all authenticated users see all; admin flag reserved
- Presigned GET vs proxy download — MVP: gfs streams (local or GetObject)
- Manifest peek (`extras/manifest.json`)

---

*SPEC approved 2026-08-12 from GFS-CONSENSUS.md + groot-trigger supply-chain reference.*
