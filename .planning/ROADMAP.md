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

## Progress

**Execution Order:**
1 → 2 → 3 → 4 → 5 → 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Supply chain | 2/2 | Complete | 2026-08-12 |
| 2. Process | 1/1 | Complete   | 2026-08-12 |
| 3. Identity | 1/1 | Complete    | 2026-08-12 |
| 4. VPS home | 2/2 | Complete    | 2026-08-12 |
| 5. Bucket home | 2/2 | Complete    | 2026-08-12 |
| 6. Housekeeping | 1/1 | Complete    | 2026-08-12 |
