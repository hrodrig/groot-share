---
phase: 05-bucket-home
plan: 02
subsystem: storage
tags: [list, prefix, foreign-keys]
requires:
  - phase: 05-bucket-home
    provides: blob.Store
provides:
  - list from prefix
  - source=s3 for groot upload.s3 keys
  - download by slashed object key
affects: [housekeeping]
requirements-completed: [ING-02]
coverage:
  - id: D1
    description: Foreign prefix object appears in list with source=s3 and downloads
    requirement: ING-02
    verification:
      - kind: unit
        ref: internal/server/catalog_test.go#TestVPSS3ForeignKeyList
        status: pass
    human_judgment: false
completed: 2026-08-12
status: complete
---

# Phase 5 Plan 05-02 Summary

`vps-s3` listing is ListObjectsV2 under the prefix. Dated gfs keys are `source=http`; other keys (cluster `upload.s3`) are `source=s3`. Mux `{id...}`.
