# gfs (groot files share)

## What This Is

**gfs** is the authenticated web/API door for a team (~20 people on a shared develop cluster) to ingest, list, download, and later analyze groot `.tar.gz` archives **without** putting object-storage keys on every laptop.

It exists only when there is a **VPS**. Three operator topologies: **VPS only** (disk is home), **S3 only** (no gfs — groot `upload.s3` + an S3 client), **VPS + S3** (VPS disk is transit; the bucket is home; gfs lists from the bucket).

## Core Value

Laptops never hold long-lived bucket credentials; cluster collect can still land multi-GB archives in object storage without hairpinning them through the VPS.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Operators can run gfs on a VPS with local disk as home (topology VPS only)
- [ ] Operators can run gfs on a VPS in front of an S3-compatible bucket (topology VPS + S3): HTTP ingest transits local staging → bucket, then staging is deleted
- [ ] In-cluster groot/trigger can `upload.s3` directly to the same prefix (preferred for multi-GB); gfs listing still sees those objects
- [ ] Laptops upload via HTTP (user + api_key), never `AWS_*`
- [ ] Web users log in (password) to list and download
- [ ] Audit log of upload/download/delete (no secrets)
- [ ] Retention: keep last N **and** max age D days (defaults 20 / 90)
- [ ] Supply chain mirrors **groot-trigger**: GNU Make, GoReleaser, distroless image, `v`-prefixed tags, CI, coverage gate, English-only artifacts
- [ ] Behavior contract lives in `docs/SPECIFICATIONS.md` (this file implements it)

### Out of Scope

- Topology **S3 only** as a gfs feature — that is groot `upload.s3` + Cyberduck/aws cli/rclone; do not invent gfs there
- Mass-distributing bucket keys to 20 laptops
- Putting HTTP inside the **groot** CLI (one-shot philosophy); optional `groot upload --gfs` is phase 2
- Analyze / compare / BYOK LLM — phase 2
- OIDC, rich RBAC, quotas, WebDAV-as-gfs-substitute (groot #97 is a different sink)
- Status poll / download proxy inside **groot-trigger** (trigger SPEC non-goal)
- Replacing CronJob scheduled collect (**groot-selfhosted**)

## Context

- Product freeze: [docs/GFS-CONSENSUS.md](../docs/GFS-CONSENSUS.md) (2026-08-12).
- Sibling **groot-trigger** v0.1.x is the supply-chain and companion-repo template: `GNUmakefile`, `.goreleaser.yaml`, `Dockerfile` / `Dockerfile.release`, `.github/workflows/ci.yml` + `release.yml`, `.golangci.yml`, `COVER_MIN`, gocyclo, govulncheck, grype, distroless numeric UID, `v`-prefixed GHCR tags, slog JSON, English artifacts, `docs/SPECIFICATIONS.md` as the implementation contract. **Copy that chain; do not invent a second packaging dialect.**
- **groot** produces archives and already multipart-uploads via `manager.Uploader` (`internal/uploader/s3.go`). Preferred cluster path in VPS + S3 is that uploader, not HTTP through gfs.
- **groot-selfhosted** `run/examples/s3-contabo/` is one validated S3-only playbook (Contabo, path-style). Same topology for MinIO, AWS, R2, Wasabi, Hetzner, DigitalOcean Spaces, …
- First gfs host is a VPS, not an in-cluster replacement for trigger.
- Repo git folder `groot-share`; product/binary name **gfs**. Module: `github.com/hrodrig/groot-share`.
- No GitHub push from this kickoff — **local commits only** until the operator says otherwise.

## Constraints

- **Language:** Go (align with groot / groot-trigger; Go 1.26.x when pinning).
- **Metadata:** SQLite for users, sessions, api_keys, audit. File inventory in VPS + S3 **is the bucket listing**, not a second source of truth.
- **Object storage:** S3-compatible (custom endpoint, path-style when needed). Optional; preferred when the operator has a bucket.
- **Auth:** Web = username + password (session/cookie). Upload API = username + api_key (hashed, rotatable, full secret shown once). Trigger’s `GROOT_TRIGGER_API_KEY` is a different secret.
- **Artifacts:** English only (same family rule as groot / trigger).
- **Git:** Local commits. Do not `git push` unless explicitly asked.
- **Logging:** slog JSON (gghstats / groot-trigger style), not groot logx.
- **CGO:** `CGO_ENABLED=0` for release binaries (trigger pattern). SQLite: use a pure-Go driver (e.g. modernc.org/sqlite) so distroless/static builds stay CGO-free.
- **UI (MVP):** server-rendered / vanilla HTML like trigger — no SPA framework unless a later phase says so.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Three topologies, no per-upload S3 flag | Operator deploy-time choice; mixing sinks desyncs catalog vs bytes | — Pending |
| VPS is transit when S3 is configured | S3 is durable at rest; VPS disk dies with the VM; staging sized for in-flight only | — Pending |
| Cluster `upload.s3` preferred; HTTP also possible | Multi-GB must not hairpin the VPS; laptops cannot hold `AWS_*` | — Pending |
| Listing from the bucket in VPS + S3 | In-flight staging is not a groot file yet; trigger S3-direct still appears | — Pending |
| Supply chain = groot-trigger | One family packaging dialect; already validated | — Pending |
| SPEC in `docs/SPECIFICATIONS.md` | Trigger model: implement against SPEC, not against chat | — Pending |
| Local git only for kickoff | Operator asked: no push | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-08-12 after initialization*
