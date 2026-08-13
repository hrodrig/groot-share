# Phase 1: Supply chain - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Stand up the **gfs** Go module and the same packaging dialect as **groot-trigger** (GNU Make, Docker/distroless, GoReleaser, CI, golangci, coverage gate). Stub `cmd/gfs` so `make ci` is green. No HTTP server yet (Phase 2).

</domain>

<decisions>
## Implementation Decisions

### Copy vs invent
- **D-01:** Copy groot-trigger packaging files and rename (`groot-trigger` → `gfs`, module `github.com/hrodrig/groot-share`). Do not invent a second Make/CI dialect. — **Reversibility:** costly — every later phase assumes these targets exist.

### Names
- **D-02:** Binary / image **gfs**; GHCR `ghcr.io/hrodrig/gfs`; git folder remains `groot-share`.
- **D-03:** `COVER_MIN=60` until there is real application code; raise toward 80 like trigger after tests exist.

### Stub
- **D-04:** `cmd/gfs` implements `version` / `--version` / `-V` and ldflags. Bare invoke prints that HTTP is Phase 2 and exits 0. No listen, no SQLite.

### Ports / man
- **D-05:** Include contrib/man + BSD port stubs renamed from trigger (family consistency). `goreleaser check` skips without `origin` (local-first).

### Git
- **D-06:** Local commits only. Do not `git push`. Workflows exist for when a remote appears.

### Claude's Discretion
- Exact wording of CHANGELOG/CONTRIBUTING/SECURITY as long as they match trigger structure.
- Whether `go.sum` is empty-module (stdlib only) until Phase 2.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Contract
- `docs/SPECIFICATIONS.md` — §9 supply chain; §10 testing
- `docs/GFS-CONSENSUS.md` — product freeze
- `AGENTS.md` — do/don't table

### Template repo (read, then copy)
- `/Volumes/Data/addlink/github/groot-trigger/GNUmakefile`
- `/Volumes/Data/addlink/github/groot-trigger/.goreleaser.yaml`
- `/Volumes/Data/addlink/github/groot-trigger/Dockerfile`
- `/Volumes/Data/addlink/github/groot-trigger/Dockerfile.release`
- `/Volumes/Data/addlink/github/groot-trigger/.github/workflows/ci.yml`
- `/Volumes/Data/addlink/github/groot-trigger/.github/workflows/release.yml`
- `/Volumes/Data/addlink/github/groot-trigger/.golangci.yml`
- `/Volumes/Data/addlink/github/groot-trigger/.gitignore`

### This phase
- `.planning/REQUIREMENTS.md` — SUP-01..04, SPEC-01
- `.planning/ROADMAP.md` — Phase 1

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None in this repo yet (docs only).

### Established Patterns
- groot-trigger: deny-all `.gitignore` whitelist; `goreleaser check` skipped without origin; distroless nonroot; `v`-prefixed tags.

### Integration Points
- Phase 2 will add `internal/config` + HTTP on this stub.

</code_context>

<specifics>
## Specific Ideas

User: use groot-trigger as the supply-chain reference for everything in that chain. Local commits only.

</specifics>

<deferred>
## Deferred Ideas

- HTTP `/healthz` — Phase 2
- Helm — not until operators ask (trigger deferred it)
- `deploy/k8s` — gfs is a VPS binary, not a Job trigger

</deferred>

---

*Phase: 1-Supply chain*
*Context gathered: 2026-08-12*
