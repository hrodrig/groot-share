---
gsd_state_version: 1.0
milestone: v1.4
milestone_name: Operational catalog UX
current_phase: 10
current_phase_name: Operational catalog UX
status: in_progress
stopped_at: Phase 10 10-01 Captures dashboard complete 2026-08-21 (summary strip + upload CTA + pin strip + filename cluster parser; tests green; make ci green)
last_updated: "2026-08-21T19:25:00.000Z"
last_activity: 2026-08-21
last_activity_desc: Phase 10 plan 10-01 complete (Captures dashboard, inventory summary, pin strip, upload CTA, cluster parser); 6 functional commits on develop
progress:
  total_phases: 10
  completed_phases: 9
  total_plans: 22
  completed_plans: 16
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-12)

**Core value:** Laptops never hold long-lived bucket credentials; cluster collect can still land multi-GB archives in object storage without hairpinning them through the VPS.
**Current focus:** Phase 9 share links **complete**. **Phase 10 locked** — evidence locker Captures (vanilla HTML). Next ship: Phase 10 (10-01/10-02).

## Current Position

Phase: 10 of 10 (operational catalog UX) — **in progress** (1/7 plans)
Plan: 10-01 complete (Captures dashboard: summary strip + upload CTA + pin strip + cluster parser)
Status: 10-01 landed; next: 10-02 (cluster chips + search + time window) or pause.
Last activity: 2026-08-21 — Phase 10 plan 10-01 complete; 6 short functional commits on develop; `make ci` green.

Progress: [█████████░] 90% (9/10 phases; 16/22 plans)

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
| 10 Catalog UX | 0 | 6 | — |

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
- Operator deploy lives in **groot-share-selfhosted** (Compose / systemd / Helm; topologies `vps` / `vps-s3`)
- GitHub issues: public wording + finding IDs only; no local scratch-folder paths

### Pending Todos

- Phase 9 **complete** — next: Phase 10 (10-01 dashboard / evidence locker Captures)
- Phase 10 10-01 **complete** — next: 10-02 cluster chips + search + time window
- Backlog 999.1: B-5, B-6, B-9, B-10, B-13, B-14, I-2, I-3 **done** (shipped v0.4.0); **I-5** (openpgp unreachable false positive) remains — documented, no fix
- After Phase 10: UX-3 role walkthrough (viewer / uploader / admin)
- Phase 9 shipped in **v0.4.0** (CHANGELOG tagged; see release commit)

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-08-19
Stopped at: Phase 9 share links complete; Phase 10 UX locked (no UI implementation yet)
Resume file: `.planning/ROADMAP.md` (Phase 10) or `.planning/phases/10-catalog-ux/10-CONTEXT.md`
