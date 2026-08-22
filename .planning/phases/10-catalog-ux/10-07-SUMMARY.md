# Plan 10-07 — Share-link admin UI (UX-09) — Summary

**Closed:** 2026-08-21

## What shipped

The operator-facing half of external share links. Phase 9 shipped the JSON API
(`POST/GET/DELETE /v1/archives/{id}/shares`, `/s/{token}` download); 10-07 puts
a server-rendered admin UI on top of it — no `curl`, no raw-token re-emission.

### Entry point
- `CanShares` flag on the shell user data (`ac.Can(auth.PermSharesManage)`).
- "Share" action on Captures in both the table row (`<td class="actions">`) and
  the card (`<div class="card-actions">`), between copy-link and Delete, guarded
  by `{{if $.CanShares}}` — admin session only; viewer/uploader never see it.

### Page (`GET /archives/{id}/shares`, admin session)
- Resolves the archive (404 if unknown), lists links via `ListShareLinks`, and
  renders `sharesTmpl` (reuses `pageShell` + `mergeActorData`, `Nav=captures`).
- Three regions: a **create form** (preset TTL buttons `24h`/`7d`, custom
  `datetime-local`, optional label, optional `max_uses`), an **active-links
  table** (label, created/expires UTC, `use_count`/`max_uses` with `∞`, status
  pill, per-link Revoke), and the standard notice flash.

### Create (`POST /archives/{id}/shares`, form-encoded)
- Parses `expires_in` / `expires_at_local` / `label` / `max_uses` with the **same
  validation semantics** as the JSON path (exactly one expiry, future, `max_uses
  >= 0`). On success it renders the page directly (200) with the full URL **once**
  in the body under a copy-once block — the raw token appears in no `Location`
  header, no URL, and no access-log path. On validation failure it re-renders
  with `notice-err` and echoes the submitted values.
- `share_create` audit row recorded.

### Revoke (`POST /archives/{id}/shares/{share_id}/revoke`)
- HTML alias that calls the same `RevokeShareLink` as the JSON `DELETE`, then
  redirects (`303`) to the page with `notice=revoked` — the redirect carries no
  token. Unknown id → `notice=missing`. `share_revoke` audit row recorded.
- Revoke is gated behind the confirm dialog (mirrors the existing
  `data-confirm` pattern).

### CSS
- `.share-url`, `.share-warn`, `.ttl-fieldset`, `.ttl-presets`, and `pill-active`
  / `pill-expired` / `pill-exhausted` / `pill-revoked` tones added to `layoutCSS`
  (US spelling, `misspell` clean).

## Fail-closed / safety notes
- Non-admin on any of the three routes → `403` (via `requirePermission`).
- The raw token is generated with `auth.NewShareToken()` (32 random bytes, hex);
  only its SHA-256 hash is stored — the UI never Persist the raw value.
- `GET` of the page never re-emits a token; only the create POST response does,
  exactly once.
- The Phase 9 JSON API is untouched — clients that already `curl` it keep working.

## Tests
- `TestSharesPageRenders` — create form + empty "No share links" state render.
- `TestSharesPageUnknownArchive404`.
- `TestSharesPageNonAdminForbidden` — uploader gets 403 on the page.
- `TestSharesCreateFormShowsURLOnce` — 200 with `/s/` URL in body, no `Location`,
  and a later list `GET` does not leak it.
- `TestSharesCreateFormValidationFailsClosed` — table of bad forms (none/both/
  bogus/past/negative max) all re-render 200 with `notice-err` and no URL.
- `TestSharesCreateFormBadArchive404`.
- `TestSharesRevokeForm` — 303 `notice=revoked`, link shows `revoked` pill, no
  `active` pill.
- `TestSharesRevokeUnknown404` — 303 `notice=missing`.

`go build ./...`, `go vet`, `gofmt -s`, and `make ci` (fmt, lint, gocyclo ≤14,
`go test ./... -race`) all green.

## Files
- `internal/server/shares_ui.go` (new) — `shareView`, `shareViews`, `shareStatus`,
  handlers (`handleSharesPage`, `handleSharesCreate`, `handleSharesRevoke`),
  form parsers (`parseMaxUses`, `parseShareExpiry`), notice copy, `sharesTmpl`.
- `internal/server/server.go` — three HTML routes registered alongside the JSON
  share routes.
- `internal/server/actor.go` — `CanShares` flag.
- `internal/server/identity.go` — "Share" row + card action.
- `internal/server/html.go` — shares CSS.
- `internal/server/shares_ui_test.go` (new) — test matrix above.
- `docs/SPECIFICATIONS.md` §12, `CHANGELOG.md`, `README.md`.
