# Plan 10-02 — Captures facets — Summary

**Completed:** 2026-08-21
**Branch:** develop
**Commits:** 7 functional + 1 refactor = 8 commits

## What landed

### 1. `store.Filter` + `ListArchivesFiltered` — `internal/store/filter.go`
- `Filter` struct with `Cluster`, `Query`, `Window`, `Since`, `Source`,
  `Storage`. `IsZero()` reports "no filters".
- `ListArchivesFiltered(ctx, f)` returns the filtered slice; `applyFilter`
  is the in-memory filter (cluster via `ParseClusterSlug`, source/storage
  as exact match, since as `CreatedAt >= Since`, q as case-insensitive
  substring of the key). `archivePasses` is the per-row predicate (kept
  out of the loop so gocyclo stays ≤ 14).
- `ClusterCounts(items)` groups by `ParseClusterSlug`; sorted by count
  desc, slug asc on ties.

### 2. `server.ParseFilter` + URL helpers — `internal/server/filter.go`
- `ParseFilter(r)` reads query params, trims, allowlists Window
  (`""` / `24h` / `7d` / `30d`), Source, Storage. Unknown values are
  silently dropped — stale bookmarks never 400 the page.
- `windowSince(window, now)` derives `Since` from the raw `window` string.
- `applyFilterInMemory` is the server-side mirror of `store.applyFilter`
  (with empty-storage normalized to "local"). `serverArchivePasses` is
  the per-row predicate.
- `FilterURLBuilder.With(key, value)` / `.Without(key)` return
  `template.URL` so `html/template` does not escape `=` to `%3d` in
  href attributes.

### 3. `handleHome` wiring — `internal/server/archives.go`
- Pulls the full inventory first; `Summary` and `ClusterChips` reflect
  the full list (D-06: chips don't disappear when other filters narrow
  the result set).
- Applies `applyFilterInMemory` over the full list to get the page's
  result set; pagination and sort run on that.
- `data["Filter"]`, `data["ClusterChips"]`, `data["FilterURL"]` are
  added so the template can render the bar and the chips.
- Unknown / stale filter values are silently dropped (never 400).

### 4. Captures facet bar — `homeTmpl` + `html.go`
- Renders only when `Summary.Count > 0` (D-09: no useless controls on
  first visit).
- Cluster chips: each is an `<a href="/?{{qswith .FilterURL "cluster" .Slug}}">`
  with an active state when the URL cluster matches; counts come from
  `ClusterCounts` over the full list.
- "All" chip uses `qswithout .FilterURL "cluster"` to clear the
  cluster filter while preserving the others.
- Search input is pre-filled with the current `?q=`; pressing Enter or
  clicking Apply submits the form (the form is `method="get" action="/"`,
  so the URL updates with the new params).
- Time-window chips: `All time` (clears window) / `24h` / `7d` / `30d`;
  active state is per `eq .Filter.Window`.
- `qswith` / `qswithout` are template helpers that call
  `FilterURLBuilder.With` / `.Without`. They return `template.URL` so
  the `=` separator is not percent-encoded inside `href`.

### 5. Empty states
- `{{if .Filter.IsZero}}` → "No captures yet" (already shipped in 10-01)
- `{{else}}` → "No matches" with a "Clear filters" link to `/`
- `<style>.empty-clear { font-weight: 600 }</style>` so the link
  reads as a primary action.

### 6. Refactor — gocyclo gate
- `applyFilter` and `applyFilterInMemory` were both above the project's
  `gocyclo -over 14` limit because of the in-line AND-chain. Extracted
  `archivePasses` / `serverArchivePasses`; `make ci` now green.

## Verification

- `gofmt -l` clean
- `go vet ./...` clean
- `golangci-lint run ./...` 0 issues
- `gocyclo` ≤ 14 across the whole tree
- `go test ./... -race -count=1` green (server 9.6s, store 32.1s)
- `make ci` exits 0
- New tests:
  - `internal/store/filter_test.go` — 8 cases for `applyFilter` (cluster,
    q, since, source, storage, combined) and `ClusterCounts` (excludes
    unparsed keys; alpha tiebreak)
  - `internal/server/filter_test.go` — 9 cases for `ParseFilter`,
    `windowSince`, URL helpers, `FilterURLBuilder`
  - `internal/server/dashboard_test.go` — 7 new cases for the facet bar
    (filter bar visible/hidden, chip counts, cluster/q/window filters
    applied, empty states, clear-filters link)

## Commits

| SHA | Message |
|-----|---------|
| `ad3b8a9` | docs(plan): 10-02 Captures facets — CONTEXT + PLAN |
| `c028f6b` | feat(server,store): Filter struct + parseFilter + ListArchivesFiltered + ClusterCounts |
| `074a1e1` | feat(server): handleHome applies filter + cluster chip counts from full list |
| `93fddc3` | feat(ui): Captures filter bar — cluster chips + search + time window (query params) |
| `078aa4f` | feat(ui): empty states — no captures yet vs no match (with clear filters) |
| `d01b880` | refactor(filter): split applyFilter/applyFilterInMemory to keep gocyclo under 14 |

Docs sync (SPEC §4, CHANGELOG, README, ROADMAP, STATE) follows in the next
commit; this SUMMARY is written first so the diff that ships is mechanical.

## Not in this plan (deferred, per the PLAN's "Out of scope")

- Source/storage pills as toggles in the facet bar (they stay in the
  table for now; 10-04 if needed)
- Inline dropzone + XHR upload progress on Captures → **10-03**
- Card layout for narrow viewports → **10-04**
- Color-token audit pass (D-07 sweep) → **10-05**
- Activity export + destructive confirm → **10-05**
- Manifest peek (incomplete count remains the *transit* proxy) → **10-06**
- Share-link admin UI on each row → **10-07**

## Notes for the next plan

- The filter state is in the URL — `handleHome` already receives it via
  `r.URL.Query()`. Plan 10-04 can read the same values for the
  responsive card layout.
- `applyFilter` does a `strings.ToLower` per row; for the evidence
  locker scale (hundreds of archives) this is fine. We can push it to
  SQL once the table grows past a few thousand rows.
- `FilterURLBuilder` returns `template.URL`. Any new template helper
  that builds query strings must do the same or `=` will be encoded.

---

*Plan: 10-02 — Captures facets*
*Summary: 2026-08-21*
