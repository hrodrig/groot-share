# Phase 9: External share links — Context

**Gathered:** 2026-08-13
**Status:** Ready for planning
**Target release:** v0.4.0 (after Phase 8 SFTP watcher)

<domain>
## Phase Boundary

Teams sometimes must hand a **single archive** to a **third party** (vendor, external auditor, cloud support) **without** provisioning a gfs account.

This phase adds **admin-only**, **time-limited** download links that gfs mints and serves (proxy stream). Each use is **audited**. Links are revocable before expiry.

**Not** the same as **LIST-04** (presigned S3 GET for authenticated users): third parties must not receive bucket presigned URLs — gfs loses audit and revocation.

Works on topology **`vps`** and **`vps-s3`** (gfs streams local home or GetObject).

</domain>

<spec_lock>
## Requirements

New **SHARE-01..03** (v2 / Phase 9). Amend `docs/SPECIFICATIONS.md` §12 when implementing.

**In scope:**
- SQLite `share_links` table: archive_id, token_hash, created_by (admin user id), expires_at, optional label, optional max_uses, use_count, revoked_at, created_at
- **Admin session only** — create, list, revoke (no api_key; uploader/viewer → `403`)
- Create: `POST /v1/archives/{id}/shares` with `{ "expires_at": "RFC3339" }` **or** `{ "expires_in": "24h" }` (mutually exclusive); optional `label`, `max_uses` (default unlimited; `1` = one-shot)
- Response shows full URL **once**: `{ "url": "https://…/s/{token}", "expires_at", "id", … }` — store SHA-256(token + pepper) only
- Public download: `GET /s/{token}` — no auth; stream bytes; `404` if unknown/expired/revoked/exhausted
- Audit: `share_create` (admin actor), `share_download` (actor `share:<prefix>` or equivalent), `share_revoke` (admin)
- Admin UI: action on Captures row — create link (preset TTLs + custom date), list active links for archive, revoke
- Expired/revoked links cleaned by optional background sweep (or lazy check on GET only for MVP)

**Out of scope:**
- Uploader/viewer creating share links (locked: **admin only**)
- Presigned S3 URL to third party (LIST-04 stays separate)
- Password-protected links, email delivery, branded landing pages
- Share links for audit log export or bulk zip of many archives
- Rate limiting dedicated middleware (defer to 999.1 backlog unless trivial)

</spec_lock>

<decisions>
## Implementation Decisions

- **D-01:** **Admin only** for create/list/revoke share links — reduces leak surface; external handoff is an operator-controlled action.
- **D-02:** Token ≥ 32 random bytes, URL-safe encoding; path `/s/{token}` (short public route, distinct from `/v1/`).
- **D-03:** gfs **proxies** download (same as authenticated `/file`) so audit and revocation stay authoritative on vps-s3.
- **D-04:** `share_download` audit includes remote IP, User-Agent when present, share link id, archive id/key, `created_by` username in metadata or object_key suffix if needed.
- **D-05:** Copy-link button for team members (`/v1/archives/{id}/file`, session required) **unchanged** — share link is a separate admin control (“Share externally”).
- **D-06:** Retention job does not delete share link rows automatically; expired links remain for audit history (or soft-delete with `revoked_at` / expiry only blocking GET).

</decisions>

<canonical_refs>
- `docs/GFS-CONSENSUS.md` — external share links (admin-only, locked)
- `docs/SPECIFICATIONS.md` §12 — planned HTTP contract
- `.planning/REQUIREMENTS.md` — SHARE-01..03
- `internal/server/audit.go` — extend actions
- Phase 7 RBAC — admin gate on new routes

</canonical_refs>

<code_context>
- `internal/store/schema.go` — new table + migration
- `internal/server/archives.go` — download stream reuse for `/s/{token}`
- `internal/server/html.go` / `identity.go` — Captures admin share UI
- `internal/auth/perm.go` — admin-only permission for share routes

</code_context>

<operator_notes>
## Use sketch

1. Admin opens Captures → **Share externally** on one row.
2. Chooses **24h** / **7d** / **until date**; optional note (“Acme vendor ticket #123”).
3. Copies URL once; sends via ticket/email.
4. Third party downloads without login; Activity shows `share_download`.
5. Admin revokes early if ticket closes.

</operator_notes>

<deferred>
- Per-link download count cap UI polish
- Metrics: `gfs_share_download_total`
- Optional rate limit on `GET /s/*`
</deferred>

---

*Phase: 9-External share links*
*Context gathered: 2026-08-13*
