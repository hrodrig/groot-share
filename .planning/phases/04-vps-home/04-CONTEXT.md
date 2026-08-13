# Phase 4: VPS home - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning

<domain>
## Phase Boundary

On topology `vps`, HTTP ingest streams a `.tar.gz` to VPS disk (home), and a logged-in user can list and download it. Vanilla HTML (list + upload + download). No per-upload S3 flag. No AWS SDK (Phase 5).

**UI:** usable, shared CSS (login + home). Not a design system / SPA. Product polish (`/gsd-ui-phase`) after the six functional phases.

</domain>

<spec_lock>
## Requirements (locked via SPEC)

AUTH already exists. This phase: ING-01, ING-03, STOR-01, LIST-01, LIST-02, LIST-03.

**In scope:** stream POST `/v1/archives` to `{GFS_DATA_DIR}/home/`; JSON list; download; GET `/` HTML; no S3 flag.

**Out of scope:** S3 transit, HeadBucket, audit table, retention, SPA/shadcn.

</spec_lock>

<decisions>
## Implementation Decisions

- **D-01:** Bytes in `{dataDir}/home/{id}.tar.gz`. Staging `{dataDir}/staging/{id}.partial` then rename. `id` = 32 hex chars. Display name in sqlite `key`.
- **D-02:** SQLite `archives` for id/key/size/sha256/created_at/source=`http`/uploader. Hash while streaming (TeeReader). Do not buffer the whole file in RAM.
- **D-03:** `GFS_MAX_UPLOAD_BYTES` default 32GiB; `http.MaxBytesReader`. Multipart field `file` or raw gzip/octet-stream body.
- **D-04:** Upload auth: session **or** api_key. List/download HTML: session. JSON list: session or api_key.
- **D-05:** Shared vanilla CSS (readable contrast, system fonts, table, forms). Login uses the same sheet. No SPA.
- **D-06:** Topology `vps-s3` still writes local home this phase (transit/S3 is Phase 5). No per-upload flag.

</decisions>

<canonical_refs>
- `docs/SPECIFICATIONS.md` §4 POST `/v1/archives`, list JSON shape
- `docs/GFS-CONSENSUS.md` — VPS is home when topology is vps
- `.planning/REQUIREMENTS.md` — ING-01, ING-03, STOR-01, LIST-01..03
- `.planning/ROADMAP.md` — Phase 4
</canonical_refs>

<code_context>
- `internal/server` authenticate() reused
- `internal/store` migrate + Open; add dir + archives
</code_context>

<deferred>
- S3 transit / list from prefix — Phase 5
- `/gsd-ui-phase` visual contract — after phases 4–6 work
- Audit upload/download — Phase 6
</deferred>

---

*Phase: 4-VPS home*
*Context gathered: 2026-08-12*
