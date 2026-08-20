---
phase: 08-sftp-watcher
plan: 01
subsystem: ingest
tags: [sftp, watcher, ingest]
requires:
  - phase: 07-rbac
    provides: delete/list RBAC matrix
provides:
  - SFTP inbox watcher (GFS_SFTP_INBOX / GFS_SFTP_POLL)
  - source=sftp ingest on vps and vps-s3
requirements-completed: [ING-04]
coverage:
  - id: D1
    description: Stable *.tar.gz in inbox ingested, listed, downloadable; source=sftp; duplicate drops inbox file
    requirement: ING-04
    verification:
      - kind: unit
        ref: internal/server/watch_test.go#TestWatchOnceIngestsStableFile
        status: pass
    human_judgment: false
  - id: D2
    description: vps-s3 SFTP objects land under {prefix}sftp/... and list with source=sftp
    requirement: ING-04
    verification:
      - kind: unit
        ref: internal/server/watch_test.go#TestWatchOnceVPSS3UsesSFTPKey
        status: pass
    human_judgment: false
  - id: D3
    description: Watcher off when GFS_SFTP_INBOX unset; dotfiles/non-tar skipped; missing dir warns
    requirement: ING-04
    verification:
      - kind: unit
        ref: internal/server/watch_test.go#TestWatchOnceSkipsDotfilesAndUnset
        status: pass
    human_judgment: false
completed: 2026-08-19
status: complete
---

# Phase 8 Plan 08-01 Summary

SFTP inbox watcher. `GFS_SFTP_INBOX` enables a poll loop (`GFS_SFTP_POLL`, default 30s) that ingests stable `*.tar.gz` with `source=sftp`, SHA-256 dedupe, audit `actor=sftp`, and deletes the inbox file on success. vps-s3 transits via `{prefix}sftp/{yyyy}/{mm}/{dd}/{32hex}.tar.gz`.
