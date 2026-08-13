# Phase 5: Bucket home - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning

<domain>
## Phase Boundary

On topology `vps-s3`, HTTP ingest writes staging, copies to the S3-compatible bucket (home), then deletes staging. Listing is the bucket prefix (including groot `upload.s3` objects gfs never saw). Copy failure still returns `201` with `storage: transit` and retries. `/readyz` calls HeadBucket. No live bucket in unit tests.

</domain>

<spec_lock>
## Requirements (locked via SPEC)

ING-02, STOR-02, STOR-03, STOR-04, STOR-05.

**In scope:** AWS SDK client (path-style when custom endpoint), transit ingest + retry, list from prefix, HeadBucket on `/readyz`, `{id...}` download of slashed keys.

**Out of scope:** S3-only as a gfs process, audit table, retention deletes, SPA, live-bucket CI.

</spec_lock>

<decisions>
## Implementation Decisions

- **D-01:** `internal/blob.Store` interface (Put/Get/List/Head/HeadBucket). Memory fake for unit tests. Real client wraps aws-sdk-go-v2 like groot (`BaseEndpoint` + `UsePathStyle` when endpoint set; checksums `WhenRequired` on custom endpoint).
- **D-02:** HTTP object key `{prefix}{yyyy}/{mm}/{dd}/{32hex}.tar.gz`. If Head says the key exists, allocate a new id (no last-writer-wins). Listing accepts **any** key under the prefix (groot `objectKey(prefix, filename)` unchanged).
- **D-03:** `source=http` when the key matches the dated 32-hex scheme; otherwise `source=s3` (cluster `upload.s3`). Inventory on `vps-s3` is ListObjectsV2, not sqlite `archives`.
- **D-04:** Copy fail → keep staging, sqlite `transit` row, `201` `{storage: transit, id: <s3 key>}`. In-process retry (default 30s). Staging is not listed. Download may serve staging by that id until the copy lands.
- **D-05:** `GET /v1/archives/{id...}` (strip trailing `/file`). Local 32-hex still works on topology `vps`.
- **D-06:** Topology `vps` unchanged (local home). No per-upload S3 flag. No give-up policy this phase (SPEC open Q).

</decisions>

<canonical_refs>
- `docs/SPECIFICATIONS.md` §4 POST `/v1/archives` transit `201`, list `source`
- `docs/GFS-CONSENSUS.md` — VPS is transit; bucket is home
- `.planning/REQUIREMENTS.md` — ING-02, STOR-02..05
- `.planning/ROADMAP.md` — Phase 5
- groot `internal/uploader/s3.go` — client dialect

</canonical_refs>

<code_context>
- `internal/config` already has S3 fields + path-style default
- `internal/store` Stage/home ingest stays for `vps`
- `internal/server` list/download/upload branch on topology + `Blobs`

</code_context>

<deferred>
- Retention deletes of bucket home — Phase 6
- Audit rows — Phase 6
- Transit give-up / operator alert — SPEC open Q
- `/gsd-ui-phase` visual contract — after phases 4–6 work
</deferred>

---

*Phase: 5-Bucket home*
*Context gathered: 2026-08-12*
