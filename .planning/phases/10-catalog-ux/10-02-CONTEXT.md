# Phase 10-02: Captures facets — Context

**Gathered:** 2026-08-21 (transcribed from 2026-08-19 lock + 10-01 follow-ons)
**Status:** Ready for planning
**Target release:** v0.5.0 (with 10-01)

<domain>
## Phase Boundary

Make Captures answer "find this archive, fast" without leaving the page.
Operators land on Captures with a vague notion of what they want — a
cluster, a since-window, a substring of the filename — and the page must
narrow the result set in one click, with a clear empty state when nothing
matches and a clear-filters escape hatch.

10-01 already added the inventory summary strip and the per-user pin
strip; 10-02 adds the **facet bar** between them and the table:

1. **Cluster chips** with counts (primary)
2. **Search box** (substring of filename; matches the `name`, `since-*`
   marker, and `-message` segments per groot archive basename)
3. **Time-window chips** (`24h` / `7d` / `30d` / `all`; primary)
4. **Source / storage pills** (secondary, already shipped in 10-01 table
   — 10-02 just makes them clickable as filters)

All filter state lives in the URL as query params so it is shareable and
survives refresh.

</domain>

<spec_lock>
## Requirements (carried from REQUIREMENTS.md / 10-CONTEXT.md / 10-01)

**In scope (this plan):**
- **UX-02** — Search/filters via query params; primary: cluster chips with
  counts, name/`since`/message search, capture window 24h/7d/30d/all;
  secondary: source/storage pills.
- **UX-01** continuation — distinct empty states: **no archives yet**
  (zero data) vs **no match** (filters exclude everything) with a
  **clear filters** link in the second case.
- **D-06** (10-CONTEXT) — Query params persist for cluster, search, time
  window, source, storage. Shareable URL.
- Filename parse already in 10-01 (`store.ParseClusterSlug`).

**Out of scope (deferred):**
- Origin pills (Trigger / Scheduled / Manual) — locked out until a
  producer field exists (D-11, 10-CONTEXT).
- Server-side full-text search (we use `LIKE` on the basename).
- Saved filter sets per user (deferred to backlog).
- Inline dropzone / XHR progress (10-03).

</spec_lock>

<decisions>
## Implementation Decisions

- **D-01:** Filter parsing lives in `internal/server/filter.go`; one small
  struct (`Filter`) with `Cluster`, `Query`, `Window`, `Source`,
  `Storage` fields and a parse/validate function `parseFilter(r)`.
- **D-02:** Filter application lives in the store layer as a single
  `Store.ListArchivesFiltered(ctx, filter)` method that returns the same
  `[]store.Archive` shape `ListArchives` already returns. We keep the
  unfiltered `ListArchives` for internal callers (Sweep, audit hooks).
- **D-03:** On `vps-s3`, the bucket listing happens **once**; filtering
  is applied in Go after the list. We do not push filters into the S3
  ListObjects call — that would require knowing the key scheme and
  complicate the future split between S3-direct and S3-via-VPS keys.
- **D-04:** Time window is applied in UTC, comparing `archive.CreatedAt`
  against `time.Now().UTC()`. Snapshots: `24h`, `7d`, `30d`. `all` = no
  filter.
- **D-05:** Search is case-insensitive `LIKE %q%` on the archive `key`.
  `q` may contain spaces (URL-encoded). Empty `q` = no filter. We do not
  tokenize; the operator types "what they remember".
- **D-06:** Cluster chips are derived from the *unfiltered* list so the
  counts reflect the full inventory, not just what the other filters
  show. This matches how GitHub / Linear facet chips behave.
- **D-07:** Source/storage pills are wired as toggles in the same facet
  bar; clicking a pill adds it as a filter; clicking the active one
  removes it. They are visually separated from the primary cluster chips
  with a "More filters" toggle so the default view stays focused.
- **D-08:** Empty states:
  - **no archives yet** — when zero archives exist regardless of filters
    (operator has never uploaded or all were deleted). CTA: "Upload
    archive" (uploader+ only).
  - **no match** — when archives exist but filters exclude all. CTA:
    "Clear filters" link that goes to the same page with no query params.
- **D-09:** The Captures filter bar sits between the per-user pin strip
  (or summary strip, if no pins) and the table. Hidden entirely when the
  page is empty (count == 0 and no filters); operators should not see
  useless filter controls on first visit.

</decisions>

<canonical_refs>
- `docs/GFS-CONSENSUS.md` — facets locked to cluster / capture window /
  completeness (NOT source/storage as primary)
- `docs/SPECIFICATIONS.md` §4 — list JSON + filter params (TBD update)
- `.planning/REQUIREMENTS.md` — UX-01, UX-02
- `.planning/phases/10-catalog-ux/10-CONTEXT.md` — D-06 (query param
  persistence), D-09 (filter bar position)
- `.planning/phases/10-catalog-ux/10-01-CONTEXT.md` — `ParseClusterSlug`
  parser, `archive_pins` snapshots
- groot `internal/uploader/` — filename grammar (SPEC §5)
- `internal/store/cluster.go` — existing cluster parser

</canonical_refs>

<code_context>
- `internal/server/archives.go` — `handleHome` (will call
  `parseFilter` + `ListArchivesFiltered`)
- `internal/server/html.go` — `.summary`, `.pin-strip`, `.grid` styles;
  add `.filter-bar`, `.chip`, `.chip.is-active`, `.search`, `.window`
- `internal/server/identity.go` — `homeTmpl` (insert filter bar; new
  empty-state branches)
- `internal/store/archives.go` — `ListArchives(ctx)` (unfiltered; keep
  for callers that need the full inventory)
- `internal/server/filter.go` — new file: `Filter` struct +
  `parseFilter(r) Filter` + `Window` constants + `ApplyFilter` helper
- `internal/store/filter.go` — new file: `ListArchivesFiltered(ctx, f)`
  + filter SQL

</code_context>

<operator_notes>
## Use sketch

1. Operator opens Captures. The facet bar shows cluster chips (e.g.
   `prod-eks-1 (3)`, `stage (1)`) and the time-window chip `All (4)` is
   active.
2. Operator clicks `prod-eks-1` chip. The page reloads with
   `?cluster=prod-eks-1`; only 3 archives from that cluster show.
3. Operator types "since-2h" in the search box. URL becomes
   `?cluster=prod-eks-1&q=since-2h`; only the matching archive shows.
4. Operator clicks the `7d` window chip. URL becomes
   `?cluster=prod-eks-1&q=since-2h&window=7d`; the window narrows the
   result further.
5. If nothing matches, the page shows **"No matches"** with a
   "Clear filters" link that goes to `/`.

## Visual lock

- Active filter chip: filled with `var(--accent)`, white text.
- Inactive chip: hairline border, `var(--ink)` text, no background fill.
- Counts in chips: `tabular-nums`, `var(--muted)`, smaller font.
- Search input: 240px max width on desktop; full width on mobile.
- Time window chips: pill style, smaller padding.

</operator_notes>

<deferred>
- Server-side full-text search (currently `LIKE %q%`)
- Saved filter sets per user
- Recent searches per user
- Filter chips that combine (e.g. cluster + time window in one chip)
- Filter bar collapse/expand for very long cluster lists
- "Quick filter" keyboard shortcuts (`/` to focus search)

</deferred>

---

*Phase: 10-02 — Captures facets*
*Context gathered: 2026-08-21*
