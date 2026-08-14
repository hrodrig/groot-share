# Roadmap: gfs

## Overview

Stand up the groot-trigger supply chain, then a VPS binary that authenticates users, takes HTTP uploads onto local disk, lists and downloads them, then add S3-compatible transit (bucket is home, listing from the prefix so cluster `upload.s3` shows up), then audit and retention.

## Phases

- [x] **Phase 1: Supply chain** — Make/CI/GoReleaser/image identical in spirit to groot-trigger; SPEC is the contract
- [x] **Phase 2: Process** — Config topologies, slog, fail-closed, healthz/readyz (completed 2026-08-12)
- [x] **Phase 3: Identity** — SQLite users, web session, hashed api_key (completed 2026-08-12)
- [x] **Phase 4: VPS home** — HTTP ingest, list, download, vanilla HTML (completed 2026-08-12)
- [x] **Phase 5: Bucket home** — Transit staging → S3; list from prefix; cluster upload.s3 coexist (completed 2026-08-12)
- [x] **Phase 6: Housekeeping** — Audit log + retention job (completed 2026-08-12)
- [x] **Phase 7: Users CRUD + RBAC** — Roles, scoped api_keys, admin user management (completed 2026-08-13, shipped in v0.2.0)
- [ ] **Phase 8: SFTP inbox watcher** — Poll groot SFTP drop dir; `source=sftp`; UI pill (planned 2026-08-13)
- [ ] **Phase 9: External share links** — Admin-only time-limited URLs for third-party download + audit (planned 2026-08-13)

## Phase Details

### Phase 1: Supply chain

**Goal:** A `gfs` module that `make ci` can run, packaged like groot-trigger, implementing against `docs/SPECIFICATIONS.md`.
**Depends on:** Nothing (first phase)
**Requirements:** SUP-01, SUP-02, SUP-03, SUP-04, SPEC-01
**Success Criteria** (what must be TRUE):

  1. `make ci` runs fmt-check + lint + gocyclo + test on `./cmd/gfs` (stub is enough)
  2. `GNUmakefile`, Dockerfiles, `.goreleaser.yaml`, CI workflows exist and name **gfs** / `github.com/hrodrig/groot-share`
  3. `AGENTS.md` tells implementers to follow `docs/SPECIFICATIONS.md` (not invent a second dialect)

**Plans:** 2 plans

Plans:

- [x] 01-01: Copy groot-trigger packaging (Make, Docker, GoReleaser, golangci, CI) renamed to gfs
- [x] 01-02: Stub `cmd/gfs` + VERSION + `make test` / `make ci` green

### Phase 2: Process

**Goal:** Operators can start gfs with a topology; the process refuses bad config and answers probes.
**Depends on:** Phase 1
**Requirements:** AUTH-04, OPS-01, OPS-02, OPS-03
**Success Criteria** (what must be TRUE):

  1. `GFS_TOPOLOGY=s3` (or missing bucket on `vps-s3`) exits non-zero
  2. `GET /healthz` is 200 without auth; `GET /readyz` reflects SQLite (and bucket when configured)
  3. Logs are slog JSON on stdout

**Plans:** 1/1 plans complete

Plans:

- [x] 02-01-PLAN.md
- [x] 02-01: Config, slog, healthz/readyz, fail-closed topology

### Phase 3: Identity

**Goal:** A person can log into the web UI and an uploader can authenticate with an api_key.
**Depends on:** Phase 2
**Requirements:** AUTH-01, AUTH-02, AUTH-03
**Success Criteria** (what must be TRUE):

  1. Username + password yields a session cookie; wrong password is 401
  2. api_key is shown once at creation and stored hashed; Bearer / X-API-Key accepted on upload routes
  3. Audit/log output never contains the raw password or api_key

**Plans:** 1/1 plans complete

Plans:

- [x] 03-01-PLAN.md

- [x] 03-01: SQLite users, password hash, api_key hash, session cookie, bootstrap admin

### Phase 4: VPS home

**Goal:** On topology `vps`, a laptop can upload a `.tar.gz` and a logged-in user can list and download it.
**Depends on:** Phase 3
**Requirements:** ING-01, ING-03, STOR-01, LIST-01, LIST-02, LIST-03
**Success Criteria** (what must be TRUE):

  1. `POST /v1/archives` streams a `.tar.gz` to disk without buffering the whole file in RAM
  2. `GET /` and `GET /v1/archives` list it; `GET /v1/archives/{id}` downloads it
  3. There is no per-upload S3 flag; UI is vanilla HTML

**Plans:** 2/2 plans complete

Plans:

- [x] 04-01-PLAN.md
- [x] 04-02-PLAN.md

- [x] 04-01: Local blob store + upload/list/download API
- [x] 04-02: Vanilla HTML list/download pages

### Phase 5: Bucket home

**Goal:** On topology `vps-s3`, HTTP ingest transits to the bucket and listing is the prefix (including groot `upload.s3` objects).
**Depends on:** Phase 4
**Requirements:** ING-02, STOR-02, STOR-03, STOR-04, STOR-05
**Success Criteria** (what must be TRUE):

  1. HTTP upload writes staging, copies to the bucket, deletes staging; list comes from the prefix
  2. If the copy fails, the client still got 201 and the object remains in transit for retry
  3. An object that only exists in the prefix (cluster `upload.s3`) appears in the list
  4. Path-style custom endpoint works (Contabo/MinIO-style)

**Plans:** 2/2 plans complete

Plans:

- [x] 05-01: S3-compatible client, transit copy, retry
- [x] 05-02: List from prefix; accept foreign keys under `captures/`

### Phase 6: Housekeeping

**Goal:** Operators can see who touched archives and old objects disappear by keep_last **or** max age.
**Depends on:** Phase 5
**Requirements:** AUD-01, RET-01
**Success Criteria** (what must be TRUE):

  1. Upload/download/delete produce audit rows without secrets
  2. Retention deletes home objects when either keep_last or max_age_days fires (defaults 20 / 90)

**Plans:** 1/1 plans complete

Plans:

- [x] 06-01: Audit table + retention job

### Phase 7: Users CRUD + RBAC

**Goal:** Admins manage users; roles enforce permissions on every route; api_keys are scoped to upload or read only.
**Depends on:** Phase 6
**Requirements:** AUTH-05
**Success Criteria** (what must be TRUE):

  1. Roles `viewer`, `uploader`, `admin` enforced on upload/list/download/delete/audit/user routes
  2. api_key scope `upload` cannot download; scope `read` cannot upload; neither can delete or manage users
  3. Admin can CRUD users via `/v1/users`; inactive users cannot authenticate
  4. Last admin cannot be removed or demoted
  5. Admin HTML at `/admin/users`; self-service at `/settings`

**Plans:** 3/3 plans complete

Plans:

- [x] 07-01: RBAC core — schema migration, perm.go, restrict existing routes, SPEC §6.1
- [x] 07-02: Users CRUD API + PATCH `/v1/me` + last-admin guard
- [x] 07-03: api_key list/revoke + scoped create + admin/settings HTML

Context: `.planning/phases/07-rbac/07-CONTEXT.md`

### Phase 8: SFTP inbox watcher

**Goal:** Captures from groot `upload.sftp` appear in gfs with `source=sftp` via an inbox directory watcher (no SFTP server in gfs).
**Depends on:** Phase 7 (recommended — delete/list RBAC applies to SFTP archives too)
**Requirements:** ING-04
**Success Criteria** (what must be TRUE):

  1. With `GFS_SFTP_INBOX` set, a stable `*.tar.gz` in the inbox is ingested, listed, and downloadable
  2. Captures Source column shows **SFTP** (distinct pill); JSON list has `"source":"sftp"`
  3. Duplicate content (SHA256) skips re-ingest and removes the inbox file; audit row uses actor `sftp`
  4. On `vps-s3`, SFTP objects land under `{prefix}sftp/...` and list correctly alongside HTTP and cluster S3 keys
  5. Watcher off when `GFS_SFTP_INBOX` unset — no behavior change for HTTP-only deploys

**Plans:** 0/2 plans complete

Plans:

- [ ] 08-01: Config + watcher loop + ingest with `source=sftp` (vps + vps-s3)
- [ ] 08-02: UI pill + SPEC/README/CHANGELOG

Context: `.planning/phases/08-sftp-watcher/08-CONTEXT.md`

### Phase 9: External share links

**Goal:** An admin can mint a time-limited download URL for one archive so a third party (no gfs account) can fetch it; every fetch is audited; links are revocable.
**Depends on:** Phase 7 (admin role and RBAC)
**Requirements:** SHARE-01, SHARE-02, SHARE-03
**Success Criteria** (what must be TRUE):

  1. Only **admin** session can create, list, or revoke share links; uploader/viewer get `403`
  2. `GET /s/{token}` works without auth until `expires_at`, revocation, or `max_uses` — gfs proxies bytes on `vps` and `vps-s3`
  3. Audit rows: `share_create`, `share_download`, `share_revoke`; raw token never in logs or audit
  4. Admin UI on Captures: create (TTL or until-date), copy URL once, list/revoke active links

**Plans:** 0/2 plans complete

Plans:

- [ ] 09-01: `share_links` schema + admin API + public `/s/{token}` download + audit
- [ ] 09-02: Captures admin UI + SPEC §12 + CHANGELOG

Context: `.planning/phases/09-external-share-links/09-CONTEXT.md`

## Backlog

- [x] **UX-2: Copy capture link** — per-row control in Captures copies the absolute download URL (`/v1/archives/{id}/file`) for pasting into GitLab, Bitbucket, Jira, etc.
- [ ] **999.1: Audit fixes (2026-08-12)** — prioritized findings from the independent audit: packaged docs (man page / README / CHANGELOG), api_key scope decision, login rate limiting, session purge, error mapping, CI gates. Context: `.planning/phases/999.1-audit-fixes/CONTEXT.md`

## Progress

**Execution Order:**
1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Supply chain | 2/2 | Complete | 2026-08-12 |
| 2. Process | 1/1 | Complete   | 2026-08-12 |
| 3. Identity | 1/1 | Complete    | 2026-08-12 |
| 4. VPS home | 2/2 | Complete    | 2026-08-12 |
| 5. Bucket home | 2/2 | Complete    | 2026-08-12 |
| 6. Housekeeping | 1/1 | Complete    | 2026-08-12 |
| 7. RBAC | 3/3 | Complete | 2026-08-13 |
| 8. SFTP watcher | 0/2 | Planned | — |
| 9. External share links | 0/2 | Planned | — |
