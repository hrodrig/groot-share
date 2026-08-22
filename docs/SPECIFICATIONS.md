# gfs — specifications

**Status:** Approved (2026-08-12) for MVP shape. HTTP paths below are the contract unless a later SPEC revision says otherwise.  
**Repo:** `groot-share` (product / binary **gfs**)  
**Product freeze:** [GFS-CONSENSUS.md](GFS-CONSENSUS.md)  
**Supply chain reference:** [groot-trigger](https://github.com/hrodrig/groot-trigger) (`GNUmakefile`, GoReleaser, CI, distroless, `v`-tags)  
**Not in scope:** groot CLI collect behavior; CronJob packaging (**groot** / **groot-selfhosted**); topology **S3 only** (no gfs process).

---

## 1. Problem

A ~20-person team on a shared cluster wants one place for groot `.tar.gz` archives without putting S3-compatible keys on every laptop. Archives can be several GB. Producers include laptops, **bastion hosts** (groot-selfhosted Docker/cron/systemd), and in-cluster jobs — **groot-trigger** can `upload.s3` (multipart). gfs is the authenticated door when a VPS exists.

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
- OIDC, quotas, presigned PUT
- External share links for third parties (Phase 9)
- HTTP inside the groot binary
- Download proxy inside groot-trigger
- Multi-tenant SaaS

## 3. Architecture

```
laptop / bastion                 cluster (trigger / CronJob)
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

Object key (both writers): `{key_prefix}{yyyy}/{mm}/{dd}/{id}.tar.gz` with `id` unique (ULID or sha256 prefix). groot `upload.s3` today keys from filename + prefix — gfs listing must accept **whatever keys already land** under the configured prefix (do not require groot to change in v0.1). HTTP ingest from gfs uses the dated-id scheme (`{prefix}{yyyy}/{mm}/{dd}/{32hex}.tar.gz`). SFTP inbox ingest on **vps-s3** uses `{prefix}sftp/{yyyy}/{mm}/{dd}/{32hex}.tar.gz`. Collision: last writer wins is unacceptable; if a key exists, ingest picks a new id. A Head error other than not-found is fatal (not treated as a free key).

## 4. HTTP contract

Unauthenticated:

| Method | Path | Behavior |
|--------|------|----------|
| GET | `/healthz` | Liveness |
| GET | `/readyz` | SQLite OK; if S3 configured, HeadBucket/list prefix must succeed |
| GET | `/login` | Login form |
| POST | `/login` | username + password → session cookie; fail `401`; rate limit → `429` |

Authenticated (session cookie **or** api_key for upload API):

| Method | Path | Behavior |
|--------|------|----------|
| POST | `/logout` | Clear session |
| GET | `/` | Vanilla HTML list (session) |
| GET | `/?cluster=&q=&window=&source=&storage=` | Same as `/` with facet filters applied (session) |
| GET | `/v1/archives` | JSON list (session) |
| POST | `/v1/archives` | Upload body `.tar.gz` (api_key **or** session); `201` + metadata. With `Accept: application/json` (and no browser form), errors are JSON (`409`/`413`) for inline clients |
| GET | `/v1/archives/{id}` | Download (session); `404` if unknown |
| GET | `/v1/archives/{id}/file` | Same bytes (HTML “download” link may use this) |
| POST | `/v1/pin/archives/{id...}` | Pin an archive for the calling user (idempotent) |
| DELETE | `/v1/pin/archives/{id...}` | Unpin (idempotent) |
| POST | `/v1/pin/archives/{id...}/delete` | Unpin form alias (redirects to `/` on success) |

On **vps-s3**, `{id}` is the object key. Download and delete accept only keys under `GFS_S3_PREFIX` (after normalize); keys outside the prefix → `404` (no raw bucket Get/Delete).

Upload auth: `Authorization: Bearer <api_key>` or `X-API-Key` (trigger-style). Do not accept api_key on the query string.

`POST /v1/archives`:

- `Content-Type: application/gzip` or `application/octet-stream` (raw body) or `multipart/form-data` field `file`
- Max size configurable (default high enough for several GB; stream to staging, **do not** buffer whole archive in RAM)
- On VPS + S3: write staging, copy to bucket, delete staging; if copy fails → `201` with `storage: transit` + retry (do not `5xx` the laptop)
- On VPS only: write home dir, `201` with `storage: local`

**SFTP inbox (optional):** when `GFS_SFTP_INBOX` is an absolute directory, gfs polls it (`GFS_SFTP_POLL`, default 30s) and ingests stable `*.tar.gz` files (size unchanged across two polls). Success or SHA-256 duplicate → delete the inbox file. Audit `action=upload`, `actor=sftp`, `uploaded_by` empty. gfs does **not** run an SFTP server. Unset inbox → watcher off.

List JSON (shape): `{ "items": [ { "id", "key", "size", "etag_or_sha256", "created_at", "source": "http"|"s3"|"sftp" } ] }`  
In VPS + S3, `source=s3` includes objects groot wrote that gfs never saw over HTTP. `source=sftp` is a gfs inbox ingest (object key `{prefix}sftp/{yyyy}/{mm}/{dd}/{id}.tar.gz`).

Captures page (`GET /`, session required) renders, in order: inventory summary
strip (count, bytes on disk, distinct cluster slugs from the filename,
in-transit count, storage topology); "Upload archive" CTA card (uploader and
admin only); per-user pin strip (only when the user has at least one pin);
facet bar (cluster chips with counts, search box, time-window chips, hidden
when the inventory is empty); table of archives. Cluster slugs come from
`store.ParseClusterSlug` which is deliberately conservative: anything that
does not match the
`<prefix>-<cluster>-<YYYYMMDD>[<sep>?<HHMMSS>][-since-<slug>].tar.gz` shape
returns `""` and is excluded from the cluster count rather than guessed.

Facet query params: `cluster` (exact slug, empty = no filter), `q`
(case-insensitive substring of the archive key, empty = no filter), `window`
(`24h` | `7d` | `30d` | empty = all), `source` (`http` | `s3` | `sftp` |
empty = no filter), `storage` (`local` | `s3` | `transit` | empty = no
filter). Unknown values are silently dropped, never 400. Cluster filter
applies in Go via `ParseClusterSlug`; the other four apply in Go against
the same in-memory list (cheap for an evidence locker of hundreds of
archives, no extra SQL round-trips). Filter state is encoded in the URL
so it is shareable. The "no matches" empty state shows when the inventory
is non-empty but every row is filtered out; a "Clear filters" link goes
to `/`.

The "Upload archive" CTA card (uploader and admin) hosts an inline dropzone:
pick or drag-and-drop a `.tar.gz`, see the file name + size before sending,
and upload via `XMLHttpRequest` with a live progress bar and a cancel button.
The XHR sets `Accept: application/json` so `POST /v1/archives` answers `201`
with `{storage}`, `409` duplicate, or `413` too-large as JSON rendered inline
(no page navigation); `409` shows the existing key so the operator can find
the earlier capture.

The archive list is responsive: a sortable table on desktop, replaced by a
card layout at ≤ 719px (each card: key, source/storage pills, size,
timestamp, and **Download** as a primary button plus copy-link and, when
authorized, delete). Card and row actions are identical; the breakpoint only
restructures the layout — no mobile-only data loss and no horizontal scroll.

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
| `GFS_BOOTSTRAP_ADMIN_NAME` | first admin display name (default `Administrator`) |
| `GFS_MAX_UPLOAD_BYTES` | default 32GiB; stream cap (`http.MaxBytesReader`) |
| `GFS_LOGIN_SIMPLE` | `true`: `/login` is a white form only (no hero, no gfs title/favicon). Default off |
| `GFS_LOGIN_RATE_LIMIT` | Cap on `POST /login` per client IP **and** per username (`N/duration`, default `20/1m`). `0` / `off` disables |
| `GFS_BRAND_SUB` | App-bar tag after the wordmark. Default `archive door`. Short string (e.g. `ACME CORP`). `-` hides |
| `GFS_FOOTER` | Authenticated footer. Default `gfs vX · groot · groot-share`. Plain text replaces it. `-` hides |
| `GFS_SFTP_INBOX` | Absolute directory for groot `upload.sftp` drops. Empty/unset → watcher off. gfs does **not** run an SFTP server |
| `GFS_SFTP_POLL` | Inbox poll interval (default `30s`) |

Fail closed: `vps-s3` without bucket/creds → exit. Empty data dir permissions → exit. Empty user table without bootstrap env → exit. `gfs.db` is `chmod 0600` on open. The SQLite connection enables `foreign_keys`, WAL, and `busy_timeout`.

## 6. Auth detail

- Password: argon2id or bcrypt; never log
- api_key: opaque, hashed at rest (SHA-256 of key + pepper, or equivalent); show once; `last_used_at` updated on successful key auth (list via Settings / `GET /v1/me/api-keys`)
- Session: httpOnly cookie; Secure when TLS. The retention sweep deletes session rows whose `expires_at` is in the past.
- Password change (Settings, `PATCH /v1/me`, or admin password patch) deletes **all** sessions for that user; self-service change also clears the browser cookie and requires re-login
- `POST /login` rate limit: in-process sliding window per client IP and per username (default `20/1m` via `GFS_LOGIN_RATE_LIMIT`); exceeded → `429` + `Retry-After`
- Bootstrap of first admin (locked): if the user table is **empty**, require `GFS_BOOTSTRAP_ADMIN` + `GFS_BOOTSTRAP_PASSWORD`, create one **admin**, hash the password, log that bootstrap ran (**never** log the password). Display name from `GFS_BOOTSTRAP_ADMIN_NAME` (default `Administrator`). If users already exist, **ignore** those env vars (do not reset the password, do not create a second bootstrap user). Empty table + missing/blank env → refuse start (fail closed). **No** well-known default password (`admin`/`changeme` or similar). Operator should drop the env from the unit after first start; gfs does not require that.
- **Reverse proxy:** gfs expects a trusted proxy for TLS. Absolute links use `Host` and (when not on TLS) `X-Forwarded-Proto`. The proxy must overwrite those headers; do not expose gfs to clients that can set them.

### 6.1 Roles and api_key scopes (v0.2)

Roles: `viewer`, `uploader`, `admin`. Session auth inherits the user's role.

| Permission | viewer | uploader | admin |
|------------|--------|----------|-------|
| List / download archives | ✓ | ✓ | ✓ |
| Upload archives | — | ✓ | ✓ |
| Delete archives | — | — | ✓ (session only) |
| Read audit log | ✓ | ✓ | ✓ |
| Manage users | — | — | ✓ (session only) |
| Manage own api_keys | — | ✓ | ✓ (session only) |
| Create / revoke external share links | — | — | ✓ (session only; Phase 9) |

api_key scopes (never grant delete or user management):

- **`upload`:** `POST /v1/archives` only (default on create).
- **`read`:** `GET /v1/archives`, download, `GET /v1/audit`, `GET /v1/me`.

Missing auth → `401`. Authenticated but forbidden → `403`.

Inactive users cannot log in or use api_keys.

User management (admin session only):

| Method | Path | Behavior |
|--------|------|----------|
| GET | `/v1/users` | List `{items:[{id,username,name,role,active,created_at}]}` |
| POST | `/v1/users` | Create `{username,name,password,role?}` — `name` required; default role `uploader` |
| GET | `/v1/users/{id}` | One user |
| PATCH | `/v1/users/{id}` | `{role?,active?,password?,name?,username?}` — last active admin cannot be demoted/deactivated (`409 last_admin`); `username` must be unique (`409 conflict`) |
| DELETE | `/v1/users/{id}` | Soft deactivate (`active=0`) |
| POST | `/admin/users/{id}/activate` | HTML: set `active=1` |
| POST | `/admin/users/{id}/username` | HTML: admin changes login id (unique) |
| POST | `/admin/users/{id}/remove` | HTML: hard-delete an **inactive** user (sessions/keys go; username is freed) |
| PATCH | `/v1/me` | Session only: `{password}` — change own password (invalidates all sessions; clears cookie) |
| POST | `/settings/name` | HTML: change own display name (not login) |

Display **Name** is required (max 80). The app bar shows it truncated at 30 runes, keeping the last 4 (`Juan ...egro`). Existing rows without a name are backfilled from `username`. **Username** (login) is unique; only an admin can change it. Users change their own Name in Settings.

## 7. Retention

Job (timer in-process or cron): delete objects that violate **either** keep_last **or** max_age_days.  
VPS only: delete files + sqlite rows if any.  
VPS + S3: delete bucket objects (home). Staging leftovers older than a grace period are swept as incidents (ERROR log including `last_error`), not as the retention set.

## 8. Observability

- slog JSON (or text) lines to stdout — one JSON object per line when `GFS_LOG_FORMAT=json` (no process-name prefix; parseable by jq / Vector / Fluent Bit)
- HTTP access line with status; never api_key/password
- Audit table: actor, action, object key/id, ts, remote IP (respect trusted proxies later; v0.1 may use RemoteAddr only)
- Activity page filter bar: actor (case-insensitive substring), action
  (exact), time window (`24h`/`7d`/`30d`/all) via `actor`/`action`/`window`
  query params. Admin CSV/JSON export at
  `GET /v1/activity/export?format=csv|json` (admin-only, honors the same
  filters, streams the full unpaginated log). Downloads are audited as
  `action=download` alongside uploads and deletes.

## 9. Supply chain (must match groot-trigger)

Copy patterns from [`groot-trigger`](https://github.com/hrodrig/groot-trigger), renaming `groot-trigger` → `gfs` / module `github.com/hrodrig/groot-share`:

- `Makefile` BSD stub + `GNUmakefile`
- `Dockerfile` + `Dockerfile.release` (distroless, UID 65532)
- `.goreleaser.yaml` (v-prefixed tags, CGO=0, freebsd/openbsd)
- `.github/workflows/ci.yml` (fmt, lint, gocyclo, coverage gate) + `security.yml` (govulncheck, grype directory) + `release.yml`
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

## 12. External share links (implemented — v0.4.0)

**Problem:** Hand one archive to a third party (vendor, external auditor) without a gfs account; know if they downloaded it; link must expire.

**Locked:** Only **admin** (session) may create, list, or revoke share links. Uploader and viewer → `403`. No api_key scope for share management.

| Method | Path | Auth | Behavior |
|--------|------|------|----------|
| POST | `/v1/archives/{id}/shares` | admin session | Create link; body `{ "expires_at" }` **or** `{ "expires_in" }`; optional `label`, `max_uses`. Response includes full URL **once**. |
| GET | `/v1/archives/{id}/shares` | admin session | List active and historical links (no raw token) |
| DELETE | `/v1/archives/{id}/shares/{share_id}` | admin session | Revoke (`share_revoke` audit) |
| GET | `/s/{token}` | none | Stream archive until expired, revoked, or uses exhausted; `share_download` audit |

- Token: high entropy (32 random bytes, hex); store SHA-256 hash only (same spirit as api_key). Raw token shown once in the create response.
- `expires_at` / `expires_in` are mutually exclusive; exactly one is required. `max_uses` defaults to `0` (unlimited); `1` is one-shot.
- Download path **proxies through gfs** (local or GetObject) — do not hand third parties a presigned S3 URL (audit + revocation).
- Team “copy download link” (`/v1/archives/{id}/file`, session required) stays separate from external share URLs.
- `share_download` audit actor is the literal `share` (public); the raw token is never logged or stored.

Requirements: **SHARE-01..03** in `.planning/REQUIREMENTS.md`. Context: `.planning/phases/09-external-share-links/09-CONTEXT.md`.

---

*SPEC approved 2026-08-12 from GFS-CONSENSUS.md + groot-trigger supply-chain reference.*  
*§12 added 2026-08-13 — external share links (Phase 9, admin-only).*
