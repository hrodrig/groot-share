# Plan 10-04 — Responsive table → card layout — Summary

**Completed:** 2026-08-21
**Branch:** develop
**Commits:** 6 functional commits

## What landed

### 1. Card markup — `internal/server/identity.go` (`homeTmpl`)

A `<ul class="archive-cards">` now renders the same `.Items` range right after
the archive `<table>` (inside the same `{{if .Items}}` block). Each
`<li class="archive-card">` carries:

- `.card-title` (mono, `word-break: break-all`) — the archive key
- `.card-meta` — source pill, storage pill (when present), `humansize` size,
  uploaded UTC timestamp
- `.card-actions` — **Download** full-width primary `.btn` (`/v1/archives/{id}/file`),
  Copy-link button (`data-copy-url`, reuses the existing `.copy-link` handler),
  and a delete `<form data-confirm=...>` gated by `{{if $.CanDelete}}`

No new JS: copy-link and `data-confirm` reuse the handlers already wired in
`homeTmpl`'s inline script.

### 2. CSS — `internal/server/html.go`

- `.archive-cards { display: none; }` by default (desktop uses the table).
- `@media (max-width: 719px)`: `.table-wrap { display: none; }` and the cards
  render as a vertical flex column with `--line` border + `--surface` bg.
- `.card-title` / `.card-meta` / `.card-actions` base rules reuse the existing
  tokens (`--mono`, `--surface`, `--line`, `--radius`) and `.pill` classes.

### 3. Tests — `internal/server/dashboard_test.go`

- `TestHomeArchiveCardsVisibleToUploader` — cards list + card + Download
  primary + copy-link + delete form present for admin.
- `TestHomeArchiveCardsNoDeleteForViewer` — viewer sees cards but no delete
  action (asserted via the `data-confirm` markup, not the `.btn-danger-quiet`
  CSS class string which also appears in the `<style>` block).

## Verification

- `make ci` green: `0 issues.` (lint), gocyclo ≤ 14, `go test ./... -race -count=1`
  green across all packages.
- `gofmt -l` empty on touched files.

## Notes / corrections made while landing

- First attempt at `TestHomeArchiveCardsNoDeleteForViewer` asserted on the
  `btn-danger-quiet` CSS class, which also matched the `<style>` block —
  corrected to assert the `data-confirm="Delete ..."` markup instead.
- `patch` on the table's closing `</table></div>` needed the unique
  `aria-label="Archives pagination"` anchor (the same table/div pattern also
  occurs in the Activity table).

## Out of scope (deferred)

- Share-link admin UI on cards → **10-07** (D-14 top-row slot reserved)
- Activity filters/export + settings safety + typed-confirm → **10-05**
- Manifest peek → **10-06**
