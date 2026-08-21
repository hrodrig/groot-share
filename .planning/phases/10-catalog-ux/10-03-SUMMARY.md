# Plan 10-03 — Inline dropzone + XHR upload — Summary

**Completed:** 2026-08-21
**Branch:** develop
**Commits:** 6 functional commits

## What landed

### 1. Inline dropzone markup — `internal/server/identity.go` (`homeTmpl`)
- Replaced the "Open upload form" link in the Captures Upload CTA card
  with an inline `<form id="upload-inline">` targeting `POST /v1/archives`.
- The form carries `data-max-upload="{{.MaxUpload}}"` so the client can
  reject oversized files before sending (server still enforces it).
- Contains a `<label class="dropzone">` wrapping the `<input type="file">`,
  a name/size preview row, a `<progress>` bar, a status line (`role=status`),
  and Upload/Cancel actions. The Upload button is `disabled` until a file
  is selected.

### 2. XHR upload JS — `internal/server/identity.go` (`homeTmpl` script)
- Drag-and-drop + file picker; drop assigns `input.files` from
  `dataTransfer.files`, then shows the file name and size before send.
- Upload uses `XMLHttpRequest` (not `fetch` — fetch has no upload
  progress callback), wired to `xhr.upload.onprogress` for the live bar.
- Cancel is `xhr.abort()`, surfaced via `onabort` (calls `resetUI`).
- `Accept: application/json` is set so `handleUpload` takes the JSON
  response branch rather than the browser-form redirect:
  - `201` → `{storage}` — `transit` shows an in-transit notice and
    auto-reloads the page after 800 ms; `local` shows "Capture uploaded."
  - `409` → `{existing.key}` rendered inline so the operator can find the
    earlier capture.
  - `413` → inline too-large notice.
  - `onerror` / unexpected status → generic inline error.
- `humanSize` mirrors the server-side `humansize` for the file preview.

### 3. CSS — `internal/server/html.go`
- `.upload-inline`, `.upload-meta`, `.upload-progress` (with
  `::-webkit-progress-*` / `::-moz-progress-bar` track/value styles),
  `.upload-status` with `.ok` / `.transit` / `.err` variants mapped to the
  existing `--ok` / `--warn` / `--err` + `-soft` tokens, and
  `.upload-actions`. Reuses the existing `.dropzone` / `.drag` rules.

### 4. Tests
- `TestUploadDuplicateMultipartJSON` — multipart + `Accept: application/json`
  returns `201` then `409` with `{"error":"duplicate","existing":…}` (JSON,
  not a redirect). This is the exact path the inline dropzone takes.
- `TestIsBrowserForm` — table over `isBrowserForm(r)` covering multipart
  with/without `Accept: application/json`, charset suffix, `text/html`, and
  raw gzip bodies.
- Updated `TestHomeUploadCTAVisibleToUploader` to assert the inline form
  (`id="upload-inline"` + `id="inline-file"`) instead of the removed
  `/upload` link. `TestHomeUploadCTAHiddenFromViewer` still passes (viewer
  never sees the CTA).

### 5. Docs
- `docs/SPECIFICATIONS.md` §4 — upload row notes the JSON branch for
  `Accept: application/json` clients, plus a paragraph on the inline
  dropzone behavior (XHR, progress, cancel, inline 201/409/413).
- `CHANGELOG.md` — Phase 10 / UX-03 bullet under `[Unreleased]`.
- `README.md` — feature bullet for the inline dropzone upload.
- `.planning/ROADMAP.md` — 10-03 marked `[x]`.
- `.planning/STATE.md` — decision, pending-todo, progress (3/7 plans),
  and next-plan pointer updated.

## Verification

- `go build ./...` exit 0
- `gofmt -l` clean (after `gofmt -w` on the new test file)
- `make ci` exit 0 (fmt-check, golangci-lint/lint 0 issues, gocyclo ≤ 14,
  `go test ./... -race -count=1` green — server 9.2s, store 31.7s)

## Commits

| SHA | Message |
|-----|---------|
| `89a4e10` | docs(plan): 10-03 Inline dropzone + XHR upload — CONTEXT + PLAN |
| `a48756d` | feat(ui): inline dropzone markup in Captures upload CTA |
| `5843d9f` | feat(ui): XHR upload — progress + cancel + duplicate inline |
| `4e0fb5c` | style(ui): inline upload progress + status CSS |
| `0182bae` | test(ui): inline dropzone + XHR upload coverage; isBrowserForm JSON branch |
| `dc63f9f` | docs: Phase 10 10-03 — inline dropzone + XHR upload (SPEC §4, CHANGELOG, README) |

## Not in this plan (deferred, per the PLAN's "Out of scope")

- Responsive table → card layout → **10-04**
- Source/storage pills as toggles → **10-04** if needed
- Activity filters + export + settings safety + destructive confirm → **10-05**
- Manifest peek (partial-capture badge) → **10-06**
- Share-link admin UI → **10-07**

## Notes for the next plan

- The inline dropzone reuses the existing `.dropzone` rules; `uploadTmpl`
  (the standalone `/upload` page) is still intact and untouched this plan.
- `isBrowserForm(r) = !wantsJSON(r) && multipart` is the single place
  deciding redirect vs JSON. Any future inline form must set
  `Accept: application/json` to get JSON errors back.
- `xhr.abort()` triggers `onabort`, not `onerror` — the UI distinguishes
  "canceled" from "network error".
- No backend change ships in 10-03: the transport is entirely client-side
  over the existing `POST /v1/archives` contract.

---

*Plan: 10-03 — Inline dropzone + XHR upload*
*Summary: 2026-08-21*
