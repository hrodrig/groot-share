---
phase: 06-housekeeping
plan: 01
subsystem: storage
tags: [audit, retention]
requires:
  - phase: 05-bucket-home
    provides: blob home and listing
provides:
  - audit table
  - retention sweep
requirements-completed: [AUD-01, RET-01]
coverage:
  - id: D1
    description: Upload/download/delete audit without secrets; keep_last and max_age delete home
    requirement: AUD-01
    verification:
      - kind: unit
        ref: internal/server/audit_test.go#TestAuditUploadDownload
        status: pass
    human_judgment: false
completed: 2026-08-12
status: complete
---

# Phase 6 Plan 06-01 Summary

Audit rows for upload/download/delete. Retention deletes by keep_last **or** max_age (defaults 20/90). vps-s3 deletes bucket home.
