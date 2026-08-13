# groot-share — **gfs** (groot files share)

Working name: **gfs** = **groot files share**.

Web layer so a team can upload, list, download, and later analyze groot `.tar.gz` archives **without** distributing S3/SFTP credentials to every laptop.

Three operator topologies (see consensus): **VPS only** (gfs is home), **S3 only** (no gfs — Contabo, MinIO, AWS, R2, …), **VPS + S3** (gfs is the door; VPS disk is transit; bucket is home).

| Piece | Role |
|-------|------|
| **[groot](https://github.com/hrodrig/groot)** | CLI: collect / validate / inspect / analyze → produces `.tar.gz` |
| **[groot-trigger](https://github.com/hrodrig/groot-trigger)** | In-cluster HTTP → on-demand collect Job; optional `upload.s3` |
| **gfs** (this repo) | Web + API when a VPS exists: auth, HTTP ingest, list, download, audit, retention |
| **[groot-selfhosted](https://github.com/hrodrig/groot-selfhosted)** | CronJob / bastion; S3 playbooks = S3-only topology |

## Status

**Design only.** No application code yet. Product shape is being clarified over a few days before a GSD kickoff.

See [docs/GFS-CONSENSUS.md](docs/GFS-CONSENSUS.md) for decisions captured so far (updated 2026-08-12).

## Language

English for all repo artifacts.
