---
phase: 04-vps-home
plan: 01
subsystem: storage
tags: [upload, sqlite, blob]
requires:
  - phase: 03-identity
    provides: session and api_key auth
provides:
  - local home blob store
  - POST/GET /v1/archives
affects: [bucket-home]
requirements-completed: [ING-01, ING-03, STOR-01, LIST-02]
coverage:
  - id: D1
    description: Stream upload to disk, list JSON, download bytes
    requirement: ING-01
    verification:
      - kind: unit
        ref: internal/server/archives_test.go#TestUploadListDownload
        status: pass
    human_judgment: false
completed: 2026-08-12
status: complete
---

# Phase 4 Plan 04-01 Summary

Stream ingest to `{dataDir}/home/{id}.tar.gz`; JSON list/download.
