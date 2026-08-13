# Contributing to gfs

## Ground rules

- English only for all project artifacts.
- Security issues: [SECURITY.md](SECURITY.md) — no public issues for undisclosed vulns.
- Day-to-day work on **`develop`**. `main` is release-only (git flow).
- Local commits until the operator asks to push.

## Planning triad

| Document | Role |
|----------|------|
| [docs/SPECIFICATIONS.md](docs/SPECIFICATIONS.md) | Behavior contract |
| [.planning/ROADMAP.md](.planning/ROADMAP.md) | Planned work |
| [CHANGELOG.md](CHANGELOG.md) | What shipped |

## Before a PR

```bash
make lint-fix
make ci
```

Maintainers before tag: `make release-check` (`COVER_MIN=80`).

Release (when a GitHub remote exists): PR `develop` → `main`, annotated tag `vX.Y.Z` on `main` → `.github/workflows/release.yml` publishes `ghcr.io/hrodrig/gfs:vX.Y.Z`.

## Scope

- **In:** VPS / VPS+S3 door for groot archives (see SPEC).
- **Out:** groot CLI collect; CronJob packaging → groot-selfhosted; in-cluster Job trigger → groot-trigger.

## Branches

Day-to-day work on **`develop`**. Do not commit features on **`main`**.

### GitHub branch protection (TODO)

`main` exists as of v0.2.0 PR [#1](https://github.com/hrodrig/groot-share/pull/1) but is **not protected yet**. After merge and before the next release, enable on **Settings → Branches → `main`** (mirror [groot `protect-main`](https://github.com/hrodrig/groot)):

- Block force-push and deletion
- Require PR before merge (no direct pushes)
- Require status check: **CI** (`ci.yml` on the PR head)
- Optional: require linear history; restrict who can push (maintainers only)

Until then, dismiss the “Your main branch isn't protected” banner is expected — **protect before tagging v0.2.1+**.
