---
phase: 09-external-share-links
plan: 01
subsystem: share
tags: [share, store, auth, audit, handlers]
requires:
  - phase: 07-rbac
    plan: 01
    provides: admin role + requirePermission gate
provides:
  - share_links store (create/list/lookup/revoke/increment)
  - admin-only share API (create/list/revoke)
  - public /s/{token} streaming download proxy
  - share_create / share_download / share_revoke audit
requirements-completed: [SHARE-01, SHARE-02, SHARE-03]
coverage:
  - id: S1
    description: Admin creates a time-limited share link; full URL returned once
    requirement: SHARE-01
    verification:
      - kind: unit
        ref: internal/server/share_test.go#TestShareCreateAndDownload
        status: pass
  - id: S2
    description: List returns metadata only, never the raw token
    requirement: SHARE-03
    verification:
      - kind: unit
        ref: internal/server/share_test.go#TestShareListNoTokenLeak
        status: pass
  - id: S3
    description: Public /s/{token} streams archive; 404 on unknown/expired/revoked/exhausted
    requirement: SHARE-02
    verification:
      - kind: unit
        ref: internal/server/share_test.go#TestShareDownload_UnknownNotFound
        status: pass
  - id: S4
    description: max_uses=1 one-shot exhausts after first download
    requirement: SHARE-01
    verification:
      - kind: unit
        ref: internal/server/share_test.go#TestShareOneShotExhausts
        status: pass
  - id: S5
    description: Non-admin (uploader/viewer) is rejected with 403
    requirement: SHARE-01
    verification:
      - kind: unit
        ref: internal/server/share_test.go#TestShareNonAdminForbidden
        status: pass
completed: 2026-08-19
status: complete
---

# Phase 9 Plan 09-01 Summary

External share links end-to-end: `share_links` table (token_hash, expires_at, max_uses, use_count, revoked_at, created_by), `NewShareToken` (32 random bytes, SHA-256 stored), `PermSharesManage` (admin session only), handlers for create/list/revoke + public `/s/{token}` proxy download, and `share_create`/`share_download`/`share_revoke` audit. Token is shown once and never persisted or logged in the clear.

No admin UI in Captures — that is Phase 10 (UX-01..08); SHARE-01..03 are API-level requirements only.
