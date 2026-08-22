# Plan 10-06 — Manifest peek → partial-capture badge — Context

**Phase:** 10 (Operational catalog UX)
**Requirement:** UX-08 (partial-capture badge from `extras/manifest.json` job failed counts)
**Design anchor:** D-09 in `10-CONTEXT.md`

## Requirement (UX-08)

Partial-capture badge from `extras/manifest.json` job failed counts — **only via
cheap gzip-member peek** (SPEC §11 still Open); never full unpack; unmarked if
peek missing; labels `Complete` / `N of M jobs failed` / `Failed`.

## Manifest contract (source of truth: groot ≥ 0.4.x / archive_layout_version 1)

groot writes `extras/manifest.json` inside every `.tar.gz` with:

```json
{
  "groot_version": "...",
  "archive_layout_version": 1,
  "run_id": "...",
  "jobs": { "total": <int>, "success": <int>, "failed": <int> },
  "paths": [ ... ]
}
```

Only the `jobs` counters matter for the badge. The member name is either
`extras/manifest.json` or `*/extras/manifest.json` (groot matches both in
`LookupSuffix`). There is a single manifest member per archive.

## Topology reality (why the scope is what it is)

gfs lists archives from two sources (`internal/server/catalog.go`):

- **vps** (`store.ListArchives`): local files on disk → opening and
  streaming the gzip header + tar members is cheap (bounded read).
- **vps-s3** (`listItemsBucket`): `blob.List` over S3 → the file bytes live
  in the bucket. Peeking the manifest here means a `GetObject` (or ranged
  GET) **per row**, which is not "cheap" and defeats the 5s listing cache.
  groot `upload.s3` objects never pass through gfs staging, so there is no
  local copy to peek.

**Decision:** peek in the list endpoint **only for local (vps) archives**;
vps-s3 rows stay **unmarked** (which UX-08 explicitly allows — "unmarked if
peek missing"). No SQLite persistence, no migration, no list-cache
invalidation. This keeps 10-06 genuinely cheap and fail-closed.

## Fail-closed rules (D-09)

Open the `.tar.gz` as `gzip.NewReader` → `tar.NewReader`, scan members for the
manifest by name, and decode only the `jobs` block (cap ~64 KiB). On **any** of:

- not a gzip stream (bad magic),
- not a valid tar header,
- manifest member absent,
- JSON too large or malformed,
- `jobs` counters missing or nonsensical (`total < 0`, `failed < 0`, `failed > total`),

…return no badge, no error, no log spam. The row just renders **unmarked**.
Never decompress the whole archive; the manifest is read with
`io.LimitReader` once the member is found.

## Badge labels (ROADMAP success criterion #8)

| Condition | Label |
|-----------|-------|
| `failed == 0` | `Complete` (green) |
| `0 < failed < total` | `N of M jobs failed` (amber) |
| `failed == total` (all failed) | `Failed` (red) |
| peek absent / non-groot archive | unmarked |

Reuse the four-color tokens already in `html.go` (`--ok` / `--warn` / `--err`).

## Not in scope

- No SQLite column, no migration, no persistence of `jobs` counters.
- No peek over S3 (ranged GET) — follow-up if the operator needs it.
- No job-level list, no failure reasons — counter-only badge.
- No marking of `transit` rows (they are in-flight by definition).

## References

- groot `internal/arcread/manifest.go` — `ManifestJobs{Total,Success,Failed}` (do
  **not** import groot; stdlib `archive/tar` + `compress/gzip` only, per
  GFS-CONSENSUS Q8).
- `docs/SPECIFICATIONS.md` §8 + §11 (manifest peek, open).
- `.planning/REQUIREMENTS.md` UX-08; `.planning/ROADMAP.md` success criterion #8.
