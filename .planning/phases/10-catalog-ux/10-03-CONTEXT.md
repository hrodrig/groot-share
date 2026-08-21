# Phase 10-03: Inline dropzone + XHR upload UX — Context

**Gathered:** 2026-08-21 (from 10-CONTEXT lock D-10 + 10-02 carried decisions)
**Status:** Ready for planning
**Target release:** v0.5.0 (with 10-01/10-02)

<domain>
## Phase Boundary

Make the **Upload archive** CTA on Captures a first-class inline surface
instead of a dead link to a separate page. Operators should drop or pick a
`.tar.gz`, see its **name + size before send**, watch a **progress bar**,
see **transit vs. complete** status, and **cancel** mid-flight — all without
leaving the Captures page. A **duplicate** upload must be called out inline
(the same-content check already returns 409), not buried in a redirect.

10-01 shipped the CTA *card* (`upload-cta`) with an "Open upload form" link
to `/upload`. 10-03 replaces that link's target surface: the dropzone moves
**inline into the CTA card itself**, and the upload goes over
`XMLHttpRequest` (not `fetch` — fetch has no upload progress) straight to the
existing `POST /v1/archives`. The separate `/upload` page stays (it is a
valid alternative for API-less operators and is already covered by tests),
but the primary path from Captures is now inline.

</domain>

<spec_lock>
## Requirements (carried from 10-CONTEXT.md / REQUIREMENTS.md)

**In scope (this plan):**
- **UX-03** — HTTP upload UX: `.tar.gz` + size limit visible (already in the
  CTA hint), **name + size before send**, **progress**, **transit copy**,
  **cancel**; **duplicate called out**.
- **D-10** (10-CONTEXT) — progress uses `XMLHttpRequest` `upload` `progress`
  events (no `fetch`); cancel = `AbortController` on the XHR; the listing page
  re-renders on completion (no SSE).
- **D-13** echoes: no raw token/key shown; nothing secret in the UI.

**Out of scope (deferred):**
- Manifest peek / partial-capture badge → **10-06**.
- Responsive card layout → **10-04**.
- Activity export / destructive confirm / color-token sweep → **10-05**.
- Share-link admin UI → **10-07**.
- Multiple simultaneous uploads in one dropzone (one file at a time matches
  the existing single-file `uploadTmpl`).

</spec_lock>

<decisions>
## Implementation Decisions

- **D-01:** The inline dropzone lives **inside the existing `upload-cta`
  card** in `homeTmpl` (shown only when `.CanUpload`). The card becomes a
  `form` (or hosts the dropzone label) that posts over XHR — no page nav.
- **D-02:** Upload uses `XMLHttpRequest` + `FormData.append("file", …)` +
  `xhr.setRequestHeader("Accept", "application/json")`. The explicit
  `Accept: application/json` makes `isBrowserForm` (which requires
  `!wantsJSON(r)`) return `false`, so `handleUpload` takes the **JSON
  branch** (`201` + `archiveJSON`, `409` + `{"error":"duplicate",…}`,
  `413` + `{"error":"too_large"}`) instead of the redirect branch. No
  backend change is required for the happy path; the XHR must also set
  `xhr.upload.onprogress` for the bar.
- **D-03:** Progress is the raw `event.loaded / event.total` (bytes) over the
  `xhr.upload` stream; the bar is an `<progress>` element or a styled div.
  Because `MaxUploadBytes` is enforced by `http.MaxBytesReader`, the browser
  may hit a `413` before `event.total` stabilizes on oversized files — the UI
  shows the size limit up front and treats a `413` as the too-large state.
- **D-04:** **Transit copy** is surfaced from the JSON response's
  `storage` field: `"transit"` (vps-s3, copy to bucket still pending) renders
  an amber "in transit — will appear in Captures once the bucket copy
  completes" line; anything else renders the green success line. This mirrors
  the existing `Storage` pill logic without a second request.
- **D-05:** **Cancel** = `AbortController` wired to `xhr.abort()` on a
  "Cancel" button that appears only while a transfer is in flight. After
  cancel, the dropzone resets to the pre-send state; an aborted transfer may
  leave a partial staging file that the server's normal error path already
  discards.
- **D-06:** **Duplicate inline** — on `409`, the UI shows an inline error
  "already uploaded (same content)" with the existing archive's key and a
  "View in Captures" link (to `/`) rather than a redirect flash. This keeps
  the operator on the page.
- **D-07:** Name + size **before send** — on file selection (`change`/`drop`),
  the dropzone text shows `filename · {humanSize}`. The size is computed
  client-side from `file.size`; if it exceeds the displayed limit, the UI
  blocks send and shows "exceeds the X limit" without firing the XHR.
- **D-08:** The existing `uploadTmpl` and `/upload` route are **unchanged**
  (still the standalone form for API-less operators). Only `homeTmpl` +
  its inline script + `html.go` CSS grow. `handleUpload` needs **no** server
  change; `handleUploadGET` stays.

</decisions>

<canonical_refs>
- `docs/GFS-CONSENSUS.md` — upload is a first-class operator action; transit
  vs. complete distinction.
- `docs/SPECIFICATIONS.md` §4/§6 — list/upload contract (`POST /v1/archives`).
- `.planning/phases/10-catalog-ux/10-CONTEXT.md` — D-10 (XHR progress), D-16
  (plan order).
- `.planning/phases/10-catalog-ux/10-02-PLAN.md` — carried CTA card + filter
  bar structure this plan sits alongside.
- `internal/server/archives.go` — `handleUpload` (JSON 201/409/413 branches),
  `isBrowserForm` (in `identity.go`).
- `internal/server/identity.go` — `homeTmpl` `upload-cta` card + `uploadTmpl`
  (dropzone pattern to inline).
- `internal/server/html.go` — `.upload-cta`, `.dropzone`, `.upload` CSS
  (lines ~282 and ~490).

</canonical_refs>

<code_context>
- `internal/server/identity.go` — `homeTmpl` (lines ~369-377: the
  `upload-cta` card to convert inline); inline `<script>` block
  (lines ~502-532: add the XHR upload JS).
- `internal/server/html.go` — add `.upload-progress`, `.upload-progress-bar`,
  `.upload-status`, `.upload-status.err` styles near `.upload` (~line 490).
- `internal/server/archives.go` — `handleUpload` already returns JSON on the
  non-browser-form path; the XHR sets `Accept: application/json` to reach it.
- `internal/server/dashboard_test.go` / `html_test.go` — add assertions that
  the inline dropzone + script markers render when `CanUpload`.
- No new store/blob changes; no new schema.

</code_context>

<operator_notes>
## Use sketch

1. Uploader lands on Captures, sees the **Upload archive** card with an
   inline dropzone (not just a link).
2. Picks (or drops) `prod-eks-1-collect.tar.gz`; dropzone shows
   `prod-eks-1-collect.tar.gz · 4.2 GB`.
3. Clicks **Upload**. Progress bar fills; "Cancel" is available.
4. On completion the card shows **"Capture uploaded"** (green) — or **"in
   transit — bucket copy pending"** (amber) on vps-s3 — and the Captures
   table refreshes to show the new row at the top.
5. If the same content already exists, the card shows an inline **"already
   uploaded (same content)"** with a "View in Captures" link.
6. Mid-transfer, **Cancel** aborts and resets the dropzone.

## Visual lock

- Dropzone: reuses the existing `.dropzone` dashed-surface look.
- Progress bar: `var(--accent)` fill on a `var(--surface-2)` track.
- Success: `var(--green)`; transit: `var(--amber)`; error/duplicate:
  `var(--red)` — consistent with D-07 color tokens.
- Filename/size line: `mono`.

</operator_notes>

<deferred>
- Multiple simultaneous uploads → backlog.
- Resume of interrupted transfers (chunked / TUS) → backlog.
- Client-side SHA-256 pre-check before send → backlog (server still
  dedupes authoritatively).

</deferred>

---

*Phase: 10-03 — Inline dropzone + XHR upload*
*Context gathered: 2026-08-21*
