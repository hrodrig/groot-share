---
phase: 08-sftp-watcher
plan: 02
subsystem: ui
tags: [ui, docs, changelog]
requires:
  - phase: 08-sftp-watcher
    plan: 01
    provides: source=sftp ingest + watcher
provides:
  - SFTP pill in Captures
  - SPEC/README/CHANGELOG SFTP docs
requirements-completed: [ING-04]
coverage:
  - id: D1
    description: Captures Source column renders pill-sftp for source=sftp
    requirement: ING-04
    verification:
      - kind: unit
        ref: internal/server/watch_test.go#TestWatchOnceIngestsStableFile
        status: pass
    human_judgment: false
  - id: D2
    description: JSON list exposes source=sftp
    requirement: ING-04
    verification:
      - kind: unit
        ref: internal/server/watch_test.go#TestWatchOnceVPSS3UsesSFTPKey
        status: pass
    human_judgment: false
completed: 2026-08-19
status: complete
---

# Phase 8 Plan 08-02 Summary

Captures pill `pill-sftp` (teal), JSON list `source=sftp`, SPEC §4/§5 config rows, README feature + env table, ALTERNATIVES wording, CHANGELOG entry. Sort stays lexical (http/sftp/s3) — no change needed.
