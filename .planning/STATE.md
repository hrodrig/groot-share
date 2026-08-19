---
gsd_state_version: 1.0
milestone: v1.2
milestone_name: SFTP watcher
current_phase: 8
current_phase_name: SFTP inbox watcher
status: planned
stopped_at: Phase 10 UX locked 2026-08-19; GSD current remains Phase 8; product v0.2.4
last_updated: "2026-08-19T23:40:00.000Z"
last_activity: 2026-08-19
last_activity_desc: Phase 10 locked from Captures mock (vanilla HTML, steal layout, reject SPA/analyze/origin-from-source)
progress:
  total_phases: 10
  completed_phases: 7
  total_plans: 22
  completed_plans: 12
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-12)

**Core value:** Laptops never hold long-lived bucket credentials; cluster collect can still land multi-GB archives in object storage without hairpinning them through the VPS.
**Current focus:** Phase 8 — SFTP inbox watcher (GSD). **Phase 10 locked** — evidence locker Captures (vanilla HTML). May run 10-01/10-02 before 08-01.

## Current Position

Phase: 8 of 10 (SFTP inbox watcher)
Plan: 08-01 not started
Status: Planned — ready for `/gsd-execute-phase` or manual execution. Phase 10 on ROADMAP (no UI code yet).
Last activity: 2026-08-19 — Phase 10 UX **locked**. Product still **v0.2.4**. Operator repo [groot-share-selfhosted](https://github.com/hrodrig/groot-share-selfhosted).

Progress: [███████░░░] 70% (7/10 phases; 12/22 plans)

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
- **Analyze stays in groot.** gfs does not import `internal/analyze` and does not `exec` groot (GFS-CONSENSUS Q8 locked 2026-08-19). LLM Markdown is producer-side; gfs only stores/serves bytes.
- Operator deploy lives in **groot-share-selfhosted** (Compose / systemd / Helm; topologies `vps` / `vps-s3`)
- GitHub issues: public wording + finding IDs only; no local scratch-folder paths

### Pending Todos

- Execute `.planning/phases/08-sftp-watcher/08-01-PLAN.md` **or** start Phase 10 (10-01 dashboard) if catalog UX is the next ship
- Backlog 999.1 remaining: B-5, B-6, B-9, B-10, B-13, B-14, I-2, I-3, I-5
- After Phase 10: UX-3 role walkthrough (viewer / uploader / admin)

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-08-19
Stopped at: Phase 10 UX locked (no UI implementation yet)
Resume file: `.planning/ROADMAP.md` (Phase 10) or `.planning/phases/08-sftp-watcher/08-01-PLAN.md`
