---
gsd_state_version: 1.0
milestone: v1.4
milestone_name: Operational catalog UX
current_phase: 10
current_phase_name: Operational catalog UX
status: complete
stopped_at: Phase 10 10-07 share-link admin UI complete 2026-08-21 (server-rendered admin page, create copy-once/list/revoke, admin-only, Phase 9 JSON API unchanged; tests green; make ci green)
last_updated: "2026-08-21T21:30:00.000Z"
last_activity: 2026-08-21
last_activity_desc: Phase 10 complete — 10-07 share-link admin UI (UX-09) landed, closing the operational catalog UX milestone
progress:
  total_phases: 10
  completed_phases: 10
  total_plans: 22
  completed_plans: 22
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-12)

**Core value:** Laptops never hold long-lived bucket credentials; cluster collect can still land multi-GB archives in object storage without hairpinning them through the VPS.
**Current focus:** Phase 9 share links **complete**. **Phase 10 complete** — evidence locker Captures (vanilla HTML) shipped across 10-01..10-07. All 10 phases done.

## Current Position

Phase: 10 of 10 (operational catalog UX) — **complete** (7/7 plans)
Plan: 10-07 complete (share-link admin UI — UX-09, the final plan)
Status: Phase 10 landed 2026-08-21 (10-01..10-07); milestone complete.
Last activity: 2026-08-21 — Phase 10 plans 10-01..10-07 complete; ~45 short functional commits on develop; `make ci` green.

Progress: [██████████] 100% (10/10 phases; 22/22 plans)

## Performance Metrics

**Velocity:**

- Total plans completed: 12
- Average duration: —
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 1 Supply chain | 2 | 2 | — |
| 2 Process | 1 | 1 | — |
| 3 Identity | 1 | 1 | — |
| 4 VPS home | 2 | 2 | — |
| 5 Bucket home | 2 | 2 | — |
| 6 Housekeeping | 1 | 1 | — |
| 7 RBAC | 3 | 3 | — |
| 8 SFTP watcher | 0 | 2 | — |
| 9 Share links | 0 | 2 | — |
| 10 Catalog UX | 7 | 7 | — |

## Accumulated Context

### Decisions

- Phase 7 roles locked: viewer / uploader / admin — **shipped in v0.2.0**
- api_key scopes: upload | read (no admin scope)
- Migration: admin flag → role; non-admins become uploader
- Phase 8 SFTP watcher: poll `GFS_SFTP_INBOX` (groot `remote_dir/inbox`); no SFTP server in gfs; target v0.3.0
- Phase 9 external share links: admin-only time-limited `/s/{token}` for third parties + audit; target v0.4.0
- Phase 10 **locked 2026-08-19**: evidence locker. Steal Captures layout (stats, pin, cluster chips, time window, table/cards). Vanilla HTML, no SPA. Filename parse yes. Manifest peek only 10-06 cheap gzip member. Origin Trigger/Cron/Manual **out** until producer field. Redacted lock **out**. Download always primary. Analyze stays in groot (Q8). Does not replace Phase 8.
- Phase 10 **10-01 complete 2026-08-21**: Captures dashboard — inventory summary strip (count, bytes, cluster count from filename parse, in-transit count, storage topology pill), Upload archive primary CTA card (uploader+ only), per-user pin strip (DB-backed `archive_pins` with FK CASCADE; new `POST/DELETE /v1/pin/archives/{id...}` endpoints with form alias). Path is `/v1/pin/archives/{id...}` (verb-then-id) because Go 1.22 mux requires the wildcard terminal. Filename cluster parser: `store.ParseClusterSlug` is conservative — returns `""` for anything that does not match the `<prefix>-<cluster>-<YYYYMMDD>[<sep>?<HHMMSS>][-since-<slug>].tar.gz` shape.
- **Analyze stays in groot.** gfs does not import `internal/analyze` and does not `exec` groot (GFS-CONSENSUS Q8 locked 2026-08-19). LLM Markdown is producer-side; gfs only stores/serves bytes.
- Phase 10 **10-03 complete 2026-08-21**: inline dropzone on the Captures Upload CTA card. Drag-and-drop + file picker, name/size preview before send, `XMLHttpRequest` upload (fetch has no upload progress), live progress bar, cancel via `xhr.abort()`. XHR sets `Accept: application/json` so `handleUpload` hits the JSON branch (`201` + `{storage}`, `409` + `{existing}`, `413`) instead of the browser-form redirect — confirmed by `isBrowserForm(r) = !wantsJSON(r) && multipart`. No backend change; transit copy (`storage: transit`) shows an inline notice and auto-reloads the page.
- Phase 10 **10-05 complete 2026-08-21**: activity filters (actor substring / action / window) + admin CSV/JSON export (`GET /v1/activity/export`) + typed-name confirm on destructive actions. `AuditFilter` in store with `limit < 0` = no-limit export; indexes idempotent in `schemaSQL`. `data-confirm-require="<text>"` on the shared `confirm-dialog` modal disables confirm until exact match.
- Phase 10 **10-06 complete 2026-08-21**: completeness badge on local (vps) captures from groot `extras/manifest.json` job counters. `peekManifest` = bounding gzip→tar member peek (64 KiB cap, fail-closed: non-gzip/invalid/missing/no-count → unmarked). Badge: `Complete` / `N of M jobs failed` / `Failed`. `map[string]*completenessBadge` (pointer value) so a missing key renders nothing (zero struct is truthy in Go templates). `s3`/`transit` rows unmarked by design (ranged bucket reads deferred).
- Phase 10 **10-07 complete 2026-08-21**: share-link admin UI (UX-09), the operator-facing half of Phase 9 links. `CanShares` flag + "Share" row/card action on Captures (admin-only). New server-rendered page `GET /archives/{id}/shares` lists links (label/created/expires/uses/status pill) and offers a create form (preset `24h`/`7d` + custom `datetime-local`, optional label, optional `max_uses`) and per-link Revoke. `POST /archives/{id}/shares` (form) renders the created URL **once** in the body (raw token in no `Location` header/URL/access-log path; page `GET` never re-emits it); invalid input re-renders with `notice-err` + echoed values. Revoke is `POST /archives/{id}/shares/{share_id}/revoke` → redirect `notice=revoked`/`notice=missing`. Non-admin → 403 on all three routes. Phase 9 JSON API untouched.
- Operator deploy lives in **groot-share-selfhosted** (Compose / systemd / Helm; topologies `vps` / `vps-s3`)
- GitHub issues: public wording + finding IDs only; no local scratch-folder paths

### Pending Todos

- Phase 9 **complete** — next: Phase 10 (10-01 dashboard / evidence locker Captures)
- Phase 10 10-01 **complete** — next: 10-02 cluster chips + search + time window
- Phase 10 10-02 **complete** — Captures facet bar (cluster chips with counts, search, time window 24h/7d/30d/all) with URL-persisted state, distinct empty states ("no captures yet" vs "no matches" with clear-filters link), filter bar hidden when inventory is empty. `store.Filter` and `server.ParseFilter` parse the URL; `applyFilter` is the in-memory filter (cluster via `ParseClusterSlug`; source/storage/since/q as straight comparisons). `FilterURLBuilder` returns `template.URL` so html/template does not escape `=` in hrefs
- Phase 10 10-03 **complete** — inline dropzone + XHR upload on the Captures Upload CTA card (progress, cancel, inline duplicate/too-large notices). Next: 10-04 responsive table → card layout (Download primary on cards)
- Phase 10 10-05 **complete** — activity filters (actor/action/window) + admin CSV/JSON export + typed-name confirm on destructive actions. Next: 10-06 optional manifest peek → partial-capture badge
- Phase 10 10-06 **complete** — completeness badge (Complete / N of M jobs failed / Failed) on local captures via capped manifest peek. Next: 10-07 share-link admin UI
- Phase 10 10-07 **complete** — share-link admin UI (UX-09): server-rendered create/list/revoke behind `CanShares`, copy-once, fail-closed validation. **Phase 10 (operational catalog UX) is now complete — all 10 phases done.**
- Backlog 999.1: B-5, B-6, B-9, B-10, B-13, B-14, I-2, I-3 **done** (shipped v0.4.0); **I-5** (openpgp unreachable false positive) remains — documented, no fix
- After Phase 10: UX-3 role walkthrough (viewer / uploader / admin)
- Phase 9 shipped in **v0.4.0** (CHANGELOG tagged; see release commit)

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-08-21
Stopped at: Phase 10 complete (10-07 share-link admin UI landed); all 10 phases done. Next: UX-3 role walkthrough, then bump version + release.
Resume file: `.planning/ROADMAP.md` or `.planning/phases/10-catalog-ux/10-07-SUMMARY.md`
