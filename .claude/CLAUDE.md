<!-- GSD:project-start source:PROJECT.md -->

## Project

**gfs (groot files share)**

**gfs** is the authenticated web/API door for a team (~20 people on a shared develop cluster) to ingest, list, download, and later analyze groot `.tar.gz` archives **without** putting object-storage keys on every laptop.

It exists only when there is a **VPS**. Three operator topologies: **VPS only** (disk is home), **S3 only** (no gfs — groot `upload.s3` + an S3 client), **VPS + S3** (VPS disk is transit; the bucket is home; gfs lists from the bucket).

**Core Value:** Laptops never hold long-lived bucket credentials; cluster collect can still land multi-GB archives in object storage without hairpinning them through the VPS.

### Constraints

- **Language:** Go (align with groot / groot-trigger; Go 1.26.x when pinning).
- **Metadata:** SQLite for users, sessions, api_keys, audit. File inventory in VPS + S3 **is the bucket listing**, not a second source of truth.
- **Object storage:** S3-compatible (custom endpoint, path-style when needed). Optional; preferred when the operator has a bucket.
- **Auth:** Web = username + password (session/cookie). Upload API = username + api_key (hashed, rotatable, full secret shown once). Trigger’s `GROOT_TRIGGER_API_KEY` is a different secret.
- **Artifacts:** English only (same family rule as groot / trigger).
- **Git:** Local commits. Do not `git push` unless explicitly asked.
- **Logging:** slog JSON (gghstats / groot-trigger style), not groot logx.
- **CGO:** `CGO_ENABLED=0` for release binaries (trigger pattern). SQLite: use a pure-Go driver (e.g. modernc.org/sqlite) so distroless/static builds stay CGO-free.
- **UI (MVP):** server-rendered / vanilla HTML like trigger — no SPA framework unless a later phase says so.

<!-- GSD:project-end -->

<!-- GSD:stack-start source:STACK.md -->

## Technology Stack

Technology stack not yet documented. Will populate after codebase mapping or first phase.
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
