# AGENTS.md — gfs (groot-share)

Web/API door for groot `.tar.gz` archives when a VPS exists.

| Do | Do not |
|----|--------|
| Implement against `docs/SPECIFICATIONS.md` | Invent a second packaging dialect |
| Mirror **groot-trigger** Make / GoReleaser / CI / distroless / `v`-tags | Put HTTP inside the **groot** CLI |
| English-only artifacts | `git push` unless the operator asked |
| Keep laptop `AWS_*` off machines | Build gfs for topology **S3 only** (no process) |
| First admin from `GFS_BOOTSTRAP_*` env (fail closed if empty DB) | Ship a well-known default password (`admin`/`changeme`) |

| Repo | Role |
|------|------|
| **groot** | CLI + image + `upload.s3` |
| **groot-selfhosted** | CronJob / bastion / S3-only playbooks |
| **groot-trigger** | In-cluster HTTP → collect Job (supply-chain template) |
| **gfs** (this repo) | VPS only / VPS + S3: auth, ingest, list, download, audit, retention |
| **groot-share-selfhosted** | Operator deploy for gfs: Compose, systemd, Helm (`vps` / `vps-s3`) |

Product freeze: `docs/GFS-CONSENSUS.md`.
