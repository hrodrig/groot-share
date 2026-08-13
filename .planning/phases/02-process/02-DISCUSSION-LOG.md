# Phase 2: Process - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-12
**Phase:** 2-Process
**Areas discussed:** Config fail-closed, probes vs S3 SDK, slog/HTTP dialect
**Mode:** yolo + user `sigue` — no interactive questionnaire; SPEC + groot-trigger locked the gray areas.

---

## Config fail-closed

| Option | Description | Selected |
|--------|-------------|----------|
| Default topology `vps` if unset | Friendlier local smoke | |
| Refuse if topology missing/`s3`/unknown | Same spirit as trigger empty API key | ✓ |
| Viper + config file | Extra dialect | |

**User's choice:** yolo — fail closed, env-only `GFS_*`.
**Notes:** `GFS_DATA_DIR` required (no default) so the binary does not write cwd by accident. Distroless needs a volume anyway.

---

## Readyz vs AWS SDK

| Option | Description | Selected |
|--------|-------------|----------|
| HeadBucket in Phase 2 | Matches SPEC wording now; pulls AWS SDK early | |
| Env presence + SQLite ping now; HeadBucket in Phase 5 | Keeps process phase off the S3 client | ✓ |

**User's choice:** yolo — no AWS SDK this phase.
**Notes:** SPEC HeadBucket lands with STOR in Phase 5. AUTH-04 is satisfied by refusing start without bucket/creds.

---

## HTTP / slog dialect

| Option | Description | Selected |
|--------|-------------|----------|
| Copy trigger stdlib mux + logging | Family consistency | ✓ |
| chi/gin + Viper | Second dialect | |
| slog on stderr like trigger | Matches trigger exactly | |
| slog on stdout (SPEC §8) | SPEC wins over trigger | ✓ |

**User's choice:** yolo — trigger shape, SPEC stdout.
**Notes:** Bare invoke starts HTTP (Phase 1 stub message goes away).

---

## Claude's Discretion

- `GFS_S3_*` exact key spellings
- SQLite DSN / package name `store` vs `db`

## Deferred Ideas

- HeadBucket on `/readyz` — Phase 5
- Identity / login — Phase 3
- Trusted proxies — later
