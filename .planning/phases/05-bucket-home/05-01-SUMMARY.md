---
phase: 05-bucket-home
plan: 01
subsystem: storage
tags: [s3, transit, retry]
requires:
  - phase: 04-vps-home
    provides: local ingest and staging dirs
provides:
  - blob.Store interface + memory fake + aws-sdk client
  - transit copy + RetryOnce
  - HeadBucket readyz
affects: [housekeeping]
requirements-completed: [STOR-02, STOR-03, STOR-04, STOR-05]
coverage:
  - id: D1
    description: HTTP ingest Put to fake bucket; fail Put stays transit then RetryOnce
    requirement: STOR-03
    verification:
      - kind: unit
        ref: internal/server/catalog_test.go#TestVPSS3TransitRetry
        status: pass
    human_judgment: false
completed: 2026-08-12
status: complete
---

# Phase 5 Plan 05-01 Summary

S3-compatible client (path-style when endpoint set), staging → Put → delete, `201 storage: transit` + retry on copy fail. HeadBucket on `/readyz`.
