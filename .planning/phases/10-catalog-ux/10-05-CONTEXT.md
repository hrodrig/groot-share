# Plan 10-05 — Activity filters + admin export + typed-name confirm — Context

**Locked against:** `10-CONTEXT.md` D-11, D-12, D-13; `REQUIREMENTS.md` UX-05, UX-06.
**Prepared:** 2026-08-21
**Status:** in progress

## Goal

Make the Activity page compliance-grade (filter by actor/action/window, admin
CSV/JSON export), and harden destructive actions with a typed-name confirm
dialog instead of a bare OK/Cancel modal.

## What already exists (verified in-repo)

- Activity page (`GET /activity` → `handleActivityGET`, `activityTmpl`) with a
  paginated "Recent events" table (When/Who/Action/Object/IP) — no filters yet.
- JSON list (`GET /v1/audit` → `handleListAudit`) — paginated, no filters.
- Downloads are already first-class in audit: `recordAudit(r, "download", a)`
  in `archives.go:185`.
- Settings page already shows a new api_key **once** with a "Copy key" button
  (`key-once` + `copy-new-key`) — D-13 is satisfied.
- A generic `data-confirm` modal (`confirm-dialog`) gates delete/unpin/revoke
  forms — but it is OK/Cancel only, no typed name (D-12 not yet satisfied).
- Four-state color tokens already exist: `--accent` (blue), `--ok` (green),
  `--warn` (amber), `--err` (red) — UX-06 token portion satisfied.
- Store `audit` schema has no indexes on `actor`/`action`/`created_at`;
  `ListAuditPage`/`CountAudit` take no filter.

## Locked decisions

### Activity filters (UX-05)

Query params on `/activity` and `/v1/audit` (and export):

- `actor` — case-insensitive **substring** of the actor username (empty = all).
- `action` — exact action string (empty = all). Examples already emitted:
  `upload`, `download`, `delete`, `apikey.revoke`, `share.create`,
  `share.revoke`, `user.*`. UI offers a dropdown of known actions + "All".
- `window` — `24h` | `7d` | `30d` | empty (all). Same semantics as the Captures
  facet window.

Filter state lives in the URL so the page is shareable; unknown `action`
values are **not** 400'd — they either match nothing (exact) or are dropped
(substring/empty). Keep the existing "filter applies in Go against the list"
spirit but push filtering into SQL (`WHERE`) since activity can grow larger
than the in-memory Captures inventory. Add indexes: `audit(actor)`,
`audit(action)`, `audit(created_at)`.

### Admin export (UX-05 / D-11)

`GET /v1/activity/export?format=csv|json&actor=&action=&window=`
— **admin-only** (`PermAuditRead` covers viewer/uploader too, so gate on admin
role explicitly), honors the same filters, returns **all** matching rows
(no pagination). `format` defaults to `json`. CSV = one row per event with a
header. No scheduled reports. `Content-Disposition: attachment`.

### Typed-name confirm (UX-06 / D-12)

Replace the OK/Cancel modal with a typed-name modal:

- `data-confirm` → keep as the prompt text.
- New `data-confirm-require="<value>"` — the exact value the user must type
  (archive key, username, or API-key prefix) before the confirm button
  enables. When absent, fall back to a plain OK (not every confirm needs a
  typed name — pin/unpin, name changes).
- The title stays `data-confirm`'s sentence; the dialog adds a mono input and
  a hint "Type <value> to confirm".
- Apply `data-confirm-require` to: archive delete (key), user remove
  (username), api-key revoke (prefix), share-link revoke (label/token), and
  `user.deactivate`.

### Tokens (UX-06)

Already satisfied — no work.

## Non-goals (deferred)

- Activity nav grouping / compliance export to external SIEM → out of scope.
- Manifest peek → **10-06**.
- Share-link admin UI → **10-07**.

## Verification (binary)

- `make ci` exits 0 (fmt-check, lint, gocyclo ≤ 14, `go test ./... -race`).
- New tests: store filter methods, activity filter handler (param plumbing +
  window cutoff), export handler (admin gate + csv/json shapes), and the
  typed-confirm markup renders `data-confirm-require` where required.

## Commit order (short, per-thing-functional)

1. `docs(plan): 10-05 Activity filters + admin export + typed confirm — CONTEXT + PLAN`
2. `feat(store): audit filter (actor/action/window) + indexes`
3. `feat(server): activity + audit filters via query params`
4. `feat(server): admin activity export (csv/json)`
5. `feat(ui): activity filter bar + export button`
6. `feat(ui): typed-name confirm dialog`
7. `test(server,store): audit filters + export + typed confirm`
8. `docs: Phase 10 10-05 — SPEC §4 + CHANGELOG + README`
9. `docs(plan): 10-05 SUMMARY + STATE/ROADMAP sync`
