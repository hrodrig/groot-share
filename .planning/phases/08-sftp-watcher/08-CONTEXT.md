# Phase 8: SFTP inbox watcher — Context

**Gathered:** 2026-08-13
**Status:** Ready for planning
**Target release:** v0.3.0 (after Phase 7 RBAC)

<domain>
## Phase Boundary

**groot** already uploads `.tar.gz` via `upload.sftp` into an SFTP inbox on the VPS (e.g. `/home/groot-inbox/inbox/` — see groot-selfhosted `run/examples/sftp-vps/`). **gfs does not run an SFTP server.**

This phase adds an **in-process inbox watcher** that picks up completed files from a configured directory, ingests them like HTTP uploads, records `source=sftp`, and shows an **SFTP** pill in Captures.

Works on topology **`vps`** (local home) and **`vps-s3`** (staging → bucket transit, same as HTTP).

</domain>

<spec_lock>
## Requirements

New **ING-04** (v1.2). Amend `docs/SPECIFICATIONS.md` §4 list JSON: `source` becomes `"http"|"s3"|"sftp"`.

**In scope:**
- Env `GFS_SFTP_INBOX` — absolute path to watch; empty/unset → watcher off (no fail)
- Poll loop (same spirit as retention/transit retry); default interval 30s (`GFS_SFTP_POLL`)
- Only `*.tar.gz`; skip dotfiles and subdirs
- **Stable-file gate:** size unchanged across two consecutive polls before ingest (groot may still be writing)
- Sanitize filename (`SanitizeArchiveKey`); SHA256 dedupe (same as HTTP — drop inbox file on duplicate, log)
- `source=sftp` in sqlite; audit row `action=upload`, `actor=sftp` (no user id)
- UI: `.pill-sftp` in Source column; sort/filter unchanged
- **vps:** `CommitLocal` with `source=sftp`
- **vps-s3:** S3 key `{prefix}sftp/{yyyy}/{mm}/{dd}/{32hex}.tar.gz` (distinct from HTTP dated keys); extend `SourceForKey` → `sftp`; transit retry unchanged
- On success: **delete** inbox file (inbox is a drop zone; Captures is the catalog)
- On hard ingest error: leave file, log warn; optional quarantine subdir deferred

**Out of scope:**
- SFTP server inside gfs (OpenSSH/sshd stays operator-managed)
- Per-SFTP-user attribution (groot machine key only today)
- Watching multiple inboxes
- fsnotify / inotify (poll-first; revisit if latency matters)
- Changing groot `upload.sftp` behavior
- RBAC changes beyond treating SFTP archives like HTTP archives for delete/list

</spec_lock>

<decisions>
## Implementation Decisions

- **D-01:** Watcher disabled unless `GFS_SFTP_INBOX` is set and directory exists at start (warn + skip if missing; do not crash the process).
- **D-02:** Poll ticker in `cmd/gfs/main.go` (`WatchLoop`), callable `WatchOnce` for tests — mirror `SweepLoop` / `RetryLoop`.
- **D-03:** Refactor store commit paths to accept `source` string instead of hard-coded `'http'` in `CommitLocal` and transit `copyOrTransit`.
- **D-04:** `uploaded_by=0` for SFTP ingests; audit `actor=sftp`.
- **D-05:** Inbox permissions are operator concern: gfs user needs read+unlink on inbox (document: shared group with `groot-inbox`, or run gfs as a user with ACL). Default doc path aligns with groot-selfhosted: `/home/groot-inbox/inbox`.
- **D-06:** Stable-file: record `(path → size)` from last poll; ingest only when size > 0 and equals previous poll.
- **D-07:** List on `vps-s3` still bucket-driven; SFTP visibility via S3 key prefix `sftp/` in `SourceForKey` (same pattern as HTTP dated keys).

</decisions>

<canonical_refs>
- groot `internal/uploader/sftp.go` — remote path = `remote_dir` + basename (+ optional run_id suffix)
- groot-selfhosted `run/examples/sftp-vps/README.md` — inbox layout, operator checklist
- `docs/SPECIFICATIONS.md` §4 — ingest + list `source`
- `docs/GFS-CONSENSUS.md` — SFTP-VPS playbook ancestor of topology VPS only
- `.planning/REQUIREMENTS.md` — ING-04
- Phase 5 `SourceForKey` / HTTP key scheme — extend, do not collide

</canonical_refs>

<code_context>
- `internal/store` — `Stage`, `Ingest`, `CommitLocal`, `InsertArchiveMeta`; hard-coded `http` today
- `internal/server/catalog.go` — `ingestTransit`, `copyOrTransit`, `listItems`
- `internal/blob/key.go` — add `SFTPKey()` + `SourceForKey` branch
- `internal/server/html.go` — `.pill-http`, `.pill-s3`; add `.pill-sftp`
- `cmd/gfs/main.go` — start watcher goroutine when inbox configured

</code_context>

<operator_notes>
## Deploy sketch (same VPS as SFTP inbox)

```bash
# groot (bastion) → SFTP → /home/groot-inbox/inbox/*.tar.gz
export GFS_SFTP_INBOX=/home/groot-inbox/inbox
export GFS_SFTP_POLL=30s
# Ensure gfs service user can read+delete files in inbox
```

Captures will show **SFTP** for cluster/laptop uploads that landed via groot `upload.sftp`; **HTTP** for gfs web/API; **S3** for in-cluster `upload.s3` objects (vps-s3 only).

</operator_notes>

<deferred>
- Quarantine dir for corrupt `.tar.gz`
- Merge sqlite source overlay when bucket object lacks sftp prefix (only if key scheme changes later)
- Metrics: `gfs_sftp_ingest_total`, `gfs_sftp_inbox_pending`
</deferred>

---

*Phase: 8-SFTP inbox watcher*
*Context gathered: 2026-08-13*
