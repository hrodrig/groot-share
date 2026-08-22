# Plan 10-06 — Manifest peek → partial-capture badge — Summary

**Closed:** 2026-08-21

## What shipped

### UX-08 — Completeness badge (local captures)

- **`peekManifest`** (`internal/server/manifest.go`): bounding
  `gzip.NewReader` → `tar.NewReader` scan for the `extras/manifest.json`
  member (exact or `*/extras/manifest.json` suffix), decoding only the `jobs`
  counters via a 64 KiB `io.LimitReader`. Fail-closed: non-gzip, invalid tar,
  missing member, oversized/malformed JSON, or nonsensical counters
  (`total <= 0`, `failed < 0`, `failed > total`) all return `ok=false` with no
  error.
- **`completenessBadgeOf`**: opens the local blob (`BlobPath`) and maps the
  counters to `Complete` (`failed == 0`, tone `ok`), `N of M jobs failed`
  (`0 < failed < total`, tone `warn`), or `Failed` (`failed >= total`, tone
  `err`). `s3` and `transit` rows return no badge.
- **Rendering** (`identity.go` + `html.go`): the badge renders next to the key
  in both the desktop table and the ≤719px card via
  `{{with index $.Completeness .ID}}`. `Completeness` is a
  `map[string]*completenessBadge` (pointer value so a missing key renders
  nothing, unlike a zero-value struct which is truthy in Go templates).
- **Wiring** (`archives.go` `handleHome`): after pagination, computes the badge
  only for the current page's *local* rows.

## Design decisions

- **Local-only, hot peek.** vps-s3 rows stay unmarked: a manifest peek there
  means a `GetObject` (ranged GET) per row, which defeats the 5s listing cache
  and is not "cheap". See 10-06-CONTEXT for the full rationale.
- **`total == 0` → no badge.** An empty/no-count `jobs` block is
  indistinguishable from a manifest that never tracked jobs, so we fail closed
  rather than label a capture "Complete" on no evidence.
- **Pointer map value.** `index $.Completeness .ID` on a `map[string]struct`
  returns the zero struct (truthy), which rendered an empty badge; switching to
  `map[string]*completenessBadge` makes a missing key falsy (nil) and the badge
  disappears. Caught by the integration test.

## Verification

- `make ci` green — fmt-check, lint 0 issues, gocyclo ≤ 14,
  `go test ./... -race`.
- `TestPeekManifest` (8-case fixture matrix: well-formed partial, all-failed,
  suffix member, missing member, empty jobs, failed>total, malformed JSON,
  non-gzip).
- `TestCompletenessBadgeOnHome` (integration: groot partial renders
  `1 of 4 jobs failed` + `tone-warn`; plain gzip renders no badge).

## Commit order

1. `docs(plan)`: 10-06 CONTEXT + PLAN
2. `feat(server)`: peekManifest gzip-member helper
3. `feat(ui)`: completeness badge on local captures
4. `test(ui)`: peek fixture matrix + home badge render
5. `docs`: SPEC §8 + CHANGELOG + README
6. `docs(plan)`: 10-06 SUMMARY + STATE/ROADMAP sync (6/7)

## Notes / deferrals

- Ranged-GET peek for S3 rows is a deliberate follow-up (documented).
- `jobs` counters are not persisted (no schema change); the peek is computed
  per request for local rows only.
- groot `upstream/upload.s3` direct objects (never staged in gfs) are
  unmarked by design.
