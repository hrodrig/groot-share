# Requirements: gfs

**Defined:** 2026-08-12  
**Core Value:** Laptops never hold long-lived bucket credentials; cluster collect can still land multi-GB archives in object storage without hairpinning them through the VPS.

Canonical product freeze: `docs/GFS-CONSENSUS.md`.  
Canonical behavior contract: `docs/SPECIFICATIONS.md`.  
Supply-chain reference: `/Volumes/Data/addlink/github/groot-trigger`.

## v1 Requirements

### Supply chain

- [x] **SUP-01**: Repo builds with GNU Make targets matching groot-trigger (`build`, `test`, `cover`, `fmt-check`, `lint`, `gocyclo`, `ci`, `govulncheck`, `grype`, `docker-build-amd64`, `goreleaser-check`, `release-check`)
- [x] **SUP-02**: GoReleaser publishes `v`-prefixed tags; CGO_ENABLED=0; linux/darwin/freebsd/openbsd amd64+arm64; distroless image
- [x] **SUP-03**: CI on `main`/`develop` runs fmt-check + golangci-lint + gocyclo + test (trigger workflow shape)
- [x] **SUP-04**: English-only artifacts; `AGENTS.md` / project guide points implementers at `docs/SPECIFICATIONS.md`

### Contract

- [x] **SPEC-01**: `docs/SPECIFICATIONS.md` is the approved behavior contract; application code implements it (trigger model)

### Authentication

- [x] **AUTH-01**: User can log in to the web UI with username + password and get a session cookie. Empty user table: create one admin from `GFS_BOOTSTRAP_ADMIN` + `GFS_BOOTSTRAP_PASSWORD` or refuse start; no well-known default password; ignore bootstrap env once users exist
- [x] **AUTH-02**: User can upload a `.tar.gz` with username + api_key (header or equivalent; full secret shown only at creation; stored hashed)
- [x] **AUTH-03**: Passwords are hashed in SQLite; audit rows never contain secrets
- [x] **AUTH-04**: Process fails closed if required operator secrets for the configured topology are missing (same spirit as trigger empty API key)

### Ingest

- [x] **ING-01**: User can HTTP-upload a groot `.tar.gz` to gfs (laptops; optional cluster path)
- [x] **ING-02**: In topology VPS + S3, an in-cluster groot/trigger `upload.s3` to the same prefix is accepted as a first-class ingest (preferred for multi-GB); listing includes those objects
- [x] **ING-03**: No per-upload “also S3” flag; topology is deploy-time config

### Ingest (v1.2 — Phase 8)

- [x] **ING-04**: SFTP inbox watcher — groot `upload.sftp` drops into `GFS_SFTP_INBOX`; gfs ingests with `source=sftp`; Captures shows SFTP pill; dedupe + audit; vps and vps-s3

### Storage

- [x] **STOR-01**: Topology VPS only: uploaded bytes live on VPS disk (home); listing is local
- [x] **STOR-02**: Topology VPS + S3: HTTP ingest lands on local staging, copies to the bucket, deletes staging; listing is the bucket; in-flight staging is not a listed groot file
- [x] **STOR-03**: If the bucket copy fails, the HTTP upload still succeeded (object stays in transit) and gfs retries; local-only is not the happy state
- [x] **STOR-04**: Staging disk is for in-flight objects only, not the retention set
- [x] **STOR-05**: S3-compatible endpoint + path-style when required (Contabo, MinIO, …); AWS virtual-hosted also works

### List and download

- [x] **LIST-01**: Authenticated web user can list archives (respect visibility config; admin sees all)
- [x] **LIST-02**: Authenticated web user can download an archive
- [x] **LIST-03**: MVP UI is server-rendered / vanilla HTML (no SPA framework)

### Audit and retention

- [x] **AUD-01**: gfs records who uploaded, downloaded, or deleted, with timestamps and useful request metadata; never secrets
- [x] **RET-01**: Retention job deletes when **either** keep_last=N **or** max_age_days=D fires (defaults 20 / 90); in VPS + S3 it deletes **home** (bucket)

### Operations

- [x] **OPS-01**: `GET /healthz` liveness without auth
- [x] **OPS-02**: `GET /readyz` readiness (SQLite reachable; bucket reachable when S3 configured)
- [x] **OPS-03**: slog JSON logs (groot-trigger / gghstats style)

## v1.1 Requirements (Phase 7 — RBAC)

- [x] **AUTH-05**: Roles `viewer` / `uploader` / `admin` enforced on all authenticated routes; api_key scopes `upload` / `read`; admin CRUD users via `/v1/users`; last-admin guard; self-service password + api_key revoke

## Operator UX (Phase 10)

Catalog polish on shipped list/upload HTML. **gfs is the incident evidence locker**, not a generic Dropbox. Not v2 analyze. **LIST-03** still applies (no SPA). May interleave with Phases 8–9.

Primary facets are **cluster, capture timestamp, since-window, completeness** — not generic source/storage (those stay secondary pills).

- [ ] **UX-01**: Captures dashboard — totals (count, bytes), cluster count, incomplete count when known, storage topology, primary **Upload archive**; default sort newest first; optional per-user pin strip
- [ ] **UX-02**: Search/filters via query params; empty **no archives yet** vs **no match** + clear. Primary: cluster chips with counts, name/`since`/message search, capture window 24h/7d/30d/all. Secondary: source/storage. No Trigger/Cron/Manual origin filter until a producer field exists
- [ ] **UX-03**: HTTP upload shows `.tar.gz`/size limit (32 GiB copy unless SPEC differs), name+size before send, progress, transit copy, cancel; duplicate called out
- [ ] **UX-04**: Narrow viewports: table row → compact card; **Download** primary on the card; desktop table also exposes Download (not kebab-only)
- [ ] **UX-05**: Activity is compliance-grade — filter by user/action/date; **download** events first-class; admin CSV/JSON export required
- [ ] **UX-06**: Typed-name confirm on destructive actions; API key shown once with copy feedback; four-color state tokens (blue primary, green ready, amber transit/partial, red error/failed); `mono` for IDs/hashes/filenames/cluster
- [ ] **UX-07**: Filename facets from groot basename (`<prefix>-<ts>[-since-<slug>]-<cluster>[-message].tar.gz`) — show cluster / capture time / since as columns or pills; optional per-user pin
- [ ] **UX-08**: Partial-capture badge from `extras/manifest.json` job failed counts — only via cheap gzip-member peek (SPEC §11 still Open); never full unpack; unmarked if peek missing; labels Complete / `N of M jobs failed` / Failed

## v2 Requirements

Deferred. Tracked in roadmap Phase 9+ and backlog.

### External share links (Phase 9)

- **SHARE-01**: **Admin only** can create a time-limited download link for one archive (`expires_at` absolute or `expires_in` TTL); high-entropy token shown once; optional label and `max_uses`; token stored hashed only
- **SHARE-02**: Unauthenticated `GET /s/{token}` streams the archive via gfs (proxy) until expired, revoked, or max uses exhausted; works on `vps` and `vps-s3`
- **SHARE-03**: Audit `share_create`, `share_download`, and `share_revoke`; admin can list and revoke links per archive; audit never contains the raw token

### Other v2

- **ANLZ-01**: gfs does **not** run `groot analyze`. If a producer uploaded an LLM-ready sidecar (e.g. `--output llm` Markdown next to the `.tar.gz`), gfs may list/download it like any other object — dumb locker
- **ANLZ-02**: Per-user BYOK LLM credentials, encrypted at rest — still phase 2; still not “gfs calls groot”
- **CLI-01**: `groot upload --gfs` with api_key, never AWS keys
- **AUTH-06**: OIDC / SSO
- **STOR-06**: Presigned PUT laptop / bastion → bucket (only after a spike against that provider)
- **LIST-04**: Presigned GET download for **authenticated** users (spike per endpoint) — not a substitute for SHARE links to third parties

## Out of Scope

| Feature | Reason |
|---------|--------|
| gfs in topology S3 only | Humans use S3 clients; groot `upload.s3` already works |
| Mass-distribute `AWS_*` to laptops | The problem gfs exists to avoid |
| HTTP server inside groot CLI | One-shot collector philosophy |
| WebDAV as gfs substitute | groot #97 is a different sink |
| Download proxy / status API in groot-trigger | Trigger SPEC non-goal |
| Replace CronJob collect | groot-selfhosted |

## Traceability

Filled by roadmap.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SUP-01 | Phase 1 | Complete |
| SUP-02 | Phase 1 | Complete |
| SUP-03 | Phase 1 | Complete |
| SUP-04 | Phase 1 | Complete |
| SPEC-01 | Phase 1 | Complete |
| AUTH-04 | Phase 2 | Complete |
| OPS-01 | Phase 2 | Complete |
| OPS-02 | Phase 2 | Complete |
| OPS-03 | Phase 2 | Complete |
| AUTH-01 | Phase 3 | Complete |
| AUTH-02 | Phase 3 | Complete |
| AUTH-03 | Phase 3 | Complete |
| ING-01 | Phase 4 | Complete |
| ING-03 | Phase 4 | Complete |
| STOR-01 | Phase 4 | Complete |
| LIST-01 | Phase 4 | Complete |
| LIST-02 | Phase 4 | Complete |
| LIST-03 | Phase 4 | Complete |
| ING-02 | Phase 5 | Complete |
| STOR-02 | Phase 5 | Complete |
| STOR-03 | Phase 5 | Complete |
| STOR-04 | Phase 5 | Complete |
| STOR-05 | Phase 5 | Complete |
| AUD-01 | Phase 6 | Complete |
| RET-01 | Phase 6 | Complete |
| AUTH-05 | Phase 7 | Complete |
| ING-04 | Phase 8 | Complete |
| SHARE-01 | Phase 9 | Complete |
| SHARE-02 | Phase 9 | Complete |
| SHARE-03 | Phase 9 | Complete |
| UX-01 | Phase 10 | Planned |
| UX-02 | Phase 10 | Planned |
| UX-03 | Phase 10 | Planned |
| UX-04 | Phase 10 | Planned |
| UX-05 | Phase 10 | Planned |
| UX-06 | Phase 10 | Planned |
| UX-07 | Phase 10 | Planned |
| UX-08 | Phase 10 | Planned |

**Coverage:**

- v1 requirements: 25 total
- v1.1 requirements: 1 (AUTH-05)
- v1.2 requirements: 1 (ING-04)
- v2 mapped (Phase 9): 3 (SHARE-01..03)
- Operator UX (Phase 10): 8 (UX-01..08)
- Mapped to phases: 38
- Unmapped v2: ANLZ, CLI, AUTH-06, STOR-06, LIST-04 (backlog)

---
*Requirements defined: 2026-08-12*  
*Last updated: 2026-08-19 — Phase 10 UX lock (Captures mock layout; no SPA; no analyze; no origin-from-source)*
