---
gsd_state_version: 1.0
milestone: v1.2
milestone_name: SFTP watcher
current_phase: 8
current_phase_name: SFTP inbox watcher
status: planned
stopped_at: Phase 7 shipped in v0.2.0; product at v0.2.4 — next is Phase 8
last_updated: "2026-08-19T05:00:00.000Z"
last_activity: 2026-08-19
last_activity_desc: Release v0.2.4 (audit hardening #16–#21)
progress:
  total_phases: 9
  completed_phases: 7
  total_plans: 14
  completed_plans: 12
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-12)

**Core value:** Laptops never hold long-lived bucket credentials; cluster collect can still land multi-GB archives in object storage without hairpinning them through the VPS.
**Current focus:** Phase 8 — SFTP inbox watcher

## Current Position

Phase: 8 of 9 (SFTP inbox watcher)
Plan: 08-01 not started
Status: Planned — ready for `/gsd-execute-phase` or manual execution
Last activity: 2026-08-19 — gfs **v0.2.4** (Head fail-closed, SQLite pragmas, session purge, PR CI cover/govulncheck/grype). Operator repo [groot-share-selfhosted](https://github.com/hrodrig/groot-share-selfhosted).

Progress: [███████░░░] 78% (7/9 phases; 12/14 plans)

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
| 9 Share links | 0 | — | — |

## Accumulated Context

### Decisions

- Phase 7 roles locked: viewer / uploader / admin — **shipped in v0.2.0**
- api_key scopes: upload | read (no admin scope)
- Migration: admin flag → role; non-admins become uploader
- Phase 8 SFTP watcher: poll `GFS_SFTP_INBOX` (groot `remote_dir/inbox`); no SFTP server in gfs; target v0.3.0
- Phase 9 external share links: admin-only time-limited `/s/{token}` for third parties + audit; target v0.4.0
- Operator deploy lives in **groot-share-selfhosted** (Compose / systemd / Helm; topologies `vps` / `vps-s3`)
- GitHub issues: public wording + finding IDs only; no local scratch-folder paths

### Pending Todos

- Execute `.planning/phases/08-sftp-watcher/08-01-PLAN.md`
- Backlog 999.1 remaining: B-5, B-6, B-9, B-10, B-13, B-14, I-2, I-3, I-5

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-08-19
Stopped at: v0.2.4 release (PR develop→main + tag)
Resume file: `.planning/phases/08-sftp-watcher/08-01-PLAN.md`
