# 10-07 CONTEXT — Share-link admin UI (UX-09)

**Phase:** 10 (operational catalog UX) — final plan.
**Backs onto:** Phase 9 external share links (already shipped in v0.4.0, JSON API only).

## Requirement (exact)

> **UX-09 — Share-link admin UI.** On a Captures row, an admin can create a
> time-limited share link (preset TTLs `24h`/`7d` + custom until-date, optional
> label, optional `max_uses`), copy the URL once, and list/revoke active links
> per archive. Backs onto the Phase 9 API (`POST/GET/DELETE
> /v1/archives/{id}/shares`); no raw token shown again after create.

## What already exists (do not rebuild)

Phase 9 shipped the full backend, **JSON-only**:

- `internal/store/share.go` — `ShareLink` model, `CreateShareLink`,
  `ListShareLinks`, `ShareByTokenHash`, `RevokeShareLink`, `IncrementShareUse`.
- `internal/server/share.go` — three JSON handlers:
  - `POST /v1/archives/{id}/shares` → `201` `{id, url, expires_at, max_uses, label}`
    (raw token composed into `url`, returned exactly once).
  - `GET /v1/archives/{id}/shares` → `200` `{items: [{id, label, max_uses,
    use_count, created_at, active, expires_at?, revoked_at?}]}`.
  - `DELETE /v1/archives/{id}/shares/{share_id}` → `200` `{revoked: true}`.
- `GET /s/{token}` — unauthenticated public download (Phase 9 `SHARE-02`).
- Authz gating: all three `/v1/.../shares` routes run under
  `requirePermission(auth.PermSharesManage, ...)` (admin-only).

The token is stored **hashed only** (`auth.HashSecret`); the raw token exists in
memory only at create time. `CreateShareLink` response returns the raw token once
via `url`, and gfs never persists or re-emits it. This is the security invariant
the UI must preserve.

## What 10-07 adds (scope, honest)

A **server-rendered** admin surface, in the same vanilla-HTML style as the rest of
Phase 10 (no SPA, no JS framework), so an operator can manage share links without
`curl`. Four concerns:

1. **Entry point** — a "Share" action on each Captures row (table + card), admin
   only (`CanShares`), between Copy-link and Delete. Links to the per-archive
   shares page.
2. **List + revoke** — a shares page (`GET /archives/{id}/shares`, HTML) listing
   every link for that archive (label, created, expiry, use count / max, active
   state) with a revoke button per link. Revoke is destructive → typed-name
   confirm reuses the existing confirm dialog.
3. **Create** — a form with preset TTLs `24h`/`7d`, a custom until-date
   (datetime-local → RFC3339), optional label, optional `max_uses`.
4. **Copy URL once** — after create, the full share URL is shown **once** in a
   flash notice on the shares page with a copy button. Subsequent loads never
   render the raw token again (only `{id, url}` is ever server-side; `url` plus
   the raw token die with the create response).

## Decisions

- **No new JSON routes.** The UI is a thin HTML layer over the existing Phase 9
  store calls (`CreateShareLink`, `ListShareLinks`, `RevokeShareLink`), reusing the
  exact same validation the JSON `POST` does (`expires_in`/`expires_at` exactly one,
  `max_uses >= 0`, expiry in the future). We do **not** duplicate the JSON handlers;
  we factor the already-small parse/validate logic so both paths share it, or the
  HTML create handler re-implements the identical minimal validation.
- **Raw token once.** The create form is `POST` form-encoded → server creates the
  link → renders the shares page with a `notice=created` flash carrying the full
  URL **and** the raw token embedded **only in that response** (as a copyable
  `data-copy-url` + visible text). We redirect after create with a one-shot query
  param? **No** — a redirect would need to re-inject the raw token via query string,
  which leaks it into `AccessLog` (path is logged) and history. Instead the create
  POST renders the page **directly** (200, not 303) with the token in that single
  HTML body. No token ever touches a URL query or the access log.
- **custom until-date** uses `<input type="datetime-local">`; server converts the
  user's local wall-clock to UTC RFC3339. If the value is in the past or unparsable,
  fail closed with a notice.
- **Presets** are client-side shortcuts that fill `expires_in` (`24h`, `168h`) and
  clear the custom field; the server still validates exactly-one.
- **Revoke** is `DELETE /v1/archives/{id}/shares/{share_id}` but the HTML form can
  only reliably emit `POST`. Mirror the existing delete-alias pattern: add a
  `POST /v1/archives/{id}/shares/{share_id}/revoke` form alias (or accept `POST`
  with a trailing `/revoke` suffix) that calls the same revoke logic and redirects.
  (Confirm against the existing `handleDelete` `/delete` suffix convention.)
- **No pagination** on the shares list — a capture has at most a handful of links.
- **Nav** `Nav = "captures"` (the shares page is a sub-view of Captures, not a new
  top-level item). Page shows a "Back to captures" link.

## Out of scope (unchanged from Phase 10 lock)

- No producer origin (`source` is not a proxy for trigger/cron/manual) — UX-4.
- No redaction, no analyze, no full manifest unpack — those stay in groot (Q8).
- Share download-landing customization / branding on `/s/{token}` — out; the
  public page already streams the blob.

## Files likely touched

- `internal/server/share.go` — add HTML handlers (`sharesPage`, create/revoke form
  handlers) + helper for form validation; reuses store calls.
- `internal/server/server.go` — register HTML routes (or a `/archives/{id}/shares`
  HTML GET + POST alias).
- `internal/server/identity.go` / `html.go` — "Share" row action (table + card) +
  `CanShares` flag; shares template + CSS reusing `.btn`, `.notice`, `.card`,
  `.field`, `.grid` primitives already in `layoutCSS`.
- `internal/server/actor.go` — `shellUserData` adds `"CanShares"` from
  `ac.Can(auth.PermSharesManage)`.
- Tests: `internal/server/share_test.go` (existing) extended for the HTML surface;
  happy-path create shows URL once, revoke redirects, non-admin gets 403/redirect,
  invalid TTL fails closed.

## Risks / honesty

- `datetime-local` value is browser-local; an operator in a different TZ than the
  VPS may set a slightly-off "until" time. Documented; the server validates
  `> now` and rejects past.
- The raw token must not leak into `AccessLog`. The create POST uses form body
  (not query), renders directly (no 303), and the access log only records
  method+path, so the token never appears there. Verified in tests (no token in
  response headers or path).
- The Phase 9 JSON `POST` already returns the token in a JSON body — that stays
  (API contract). The UI is additive; it does not change the API.
