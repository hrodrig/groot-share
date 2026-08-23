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

Release (when a GitHub remote exists): PR `develop` → `main`, annotated tag `vX.Y.Z` on `main` → `.github/workflows/release.yml` publishes `ghcr.io/hrodrig/gfs:vX.Y.Z`. After the tag is pushed, merge `main` back into `develop` (`git checkout develop && git merge origin/main --no-edit && git push origin develop`) so the next `develop → main` PR is not `BEHIND`.

## Scope

- **In:** VPS / VPS+S3 door for groot archives (see SPEC).
- **Out:** groot CLI collect; CronJob packaging → groot-selfhosted; in-cluster Job trigger → groot-trigger.

## Branches

Day-to-day work on **`develop`**. Do not commit features on **`main`**.

### GitHub branch protection

`main` is protected (enabled **2026-08-13** after v0.2.0 release). Settings → Branches → `main`:

- Block force-push and deletion
- Require pull request before merge (0 approvals — solo maintainer; PR still required)
- Require status checks: **`gofmt + golangci-lint + gocyclo`**, **`test`** (strict: branch must be up to date)

`develop` stays unprotected for day-to-day pushes; release flow remains PR `develop` → `main`. Because `main` protection is "strict" (branch must be up to date), a `--merge` PR creates a merge commit on `main` that is absent from `develop`, so the next PR would be `BEHIND` — merge `main` back into `develop` after each tag (see Release above).
