# Alternatives and when to use what

**Purpose:** Honest comparison between **gfs**, the rest of the **groot** family, and common ways teams share large diagnostic archives. No marketing spin — pick the path that matches your constraints.

**Related:** [GFS-CONSENSUS.md](GFS-CONSENSUS.md) (product freeze) · [SPECIFICATIONS.md](SPECIFICATIONS.md) (behavior contract) · [groot-selfhosted examples](https://github.com/hrodrig/groot-selfhosted/tree/main/run/examples)

---

## The problem

A platform or SRE team running a shared Kubernetes cluster (for example a **develop** environment with ~20 engineers) needs **one place** for [groot](https://github.com/hrodrig/groot) `.tar.gz` captures: incident evidence, RCA attachments, compliance retention.

Typical pain:

| Pain | Why it hurts |
|------|----------------|
| **Bucket keys on every laptop** | Many S3-compatible providers (Contabo, Hetzner Object Storage, Wasabi, MinIO, RGW, …) issue **one key pair per bucket**. Revoke one leaked laptop → rotate for **everyone**. A leak often means **full bucket** access. |
| **No team-facing door** | `aws s3 cp` and Cyberduck work for power users, but there is no per-user login, audit trail, or retention policy unless you build it. |
| **Multi-GB archives** | A collect can be several gigabytes. Hairpinning cluster → VPS → bucket when the cluster could write **directly to S3** wastes bandwidth and disk. |
| **Three producer classes** | Archives come from **laptops** (adhoc collect), **bastion hosts** (groot on a jump box — Docker, cron, or systemd via [groot-selfhosted](https://github.com/hrodrig/groot-selfhosted)), and **in-cluster** jobs ([groot-trigger](https://github.com/hrodrig/groot-trigger), CronJob). They should land in the **same catalog** without duplicating secrets. |
| **Trigger is not storage** | groot-trigger starts a collect Job and returns `202` — by design it does **not** list, download, or retain archives ([trigger SPEC](https://github.com/hrodrig/groot-trigger/blob/main/docs/SPECIFICATIONS.md)). |

**gfs** targets teams that accept a **VPS** as an authenticated HTTP door while keeping **long-lived object-store credentials off laptops** (and optionally off the cluster except one in-cluster Secret for `upload.s3`).

---

## How gfs solves it

```
  laptop / bastion                 cluster (trigger / CronJob)
    │  HTTP + session / api_key          │  upload.s3 (preferred when bucket exists)
    ▼                                    ▼
┌─────────────────────────────────────────────────────────┐
│  gfs (VPS)                                               │
│  · per-user auth (viewer / uploader / admin)             │
│  · HTTP ingest → staging (vps-s3) → bucket               │
│  · list + download from home (disk or bucket prefix)     │
│  · audit (no secrets in rows) + retention job            │
└─────────────────────────────────────────────────────────┘
         ▲
         │  same S3 prefix — cluster objects appear as source=s3
         │
    S3-compatible bucket (home on vps-s3)
```

**What gfs adds that raw S3 does not:**

- Username/password web UI and rotatable **scoped** API keys (`upload` vs `read`)
- Unified **Captures** list for HTTP uploads and cluster `upload.s3` keys (topology **vps-s3**)
- **Audit** (who uploaded, downloaded, deleted; timestamps)
- **Retention** (`keep_last` **or** `max_age_days`, defaults 20 / 90)
- **RBAC** (admin-only delete and user management)

**What gfs deliberately does not do:**

- Run `groot collect` (stay in groot + trigger + selfhosted)
- Replace an S3 client when you have **no VPS** and only need bucket storage (**S3 only** topology — no gfs binary)
- Proxy or poll Job status inside groot-trigger (trigger non-goal)
- Analyze archives in the browser (phase 2 / BYOK LLM — not MVP)

---

## Quick decision guide

Use this first. Details below.

| Your situation | Best choice | Why |
|----------------|-------------|-----|
| Shared team bucket; **no VPS**; engineers OK with S3 tools or IAM | **[S3 only](#s3-only-groot-uploads3--s3-client-no-gfs)** — groot `upload.s3` + Cyberduck / `aws` / rclone | No second service; cluster already has a Secret; gfs would add cost without a door you need |
| Small team; **one VPS**; archives stay on disk; no object store | **[VPS only](#vps-only-gfs-on-disk)** — gfs `GFS_TOPOLOGY=vps` | Simple home on VPS; HTTP auth without bucket ops |
| Team bucket **and** laptops must upload **without** `AWS_*`; multi-GB from cluster | **[VPS + S3](#vps--s3-gfs--bucket-home)** — gfs + cluster `upload.s3` to same prefix | Laptops → gfs HTTP; cluster → S3 direct; gfs lists bucket |
| On-demand collect **button in cluster** only; archives go elsewhere | **[groot-trigger](https://github.com/hrodrig/groot-trigger)** + your storage path | Trigger fires Job; pair with S3 only, SFTP, or gfs — trigger alone is not a catalog |
| **Bastion / jump host** with kubeconfig (Docker, cron, systemd) | **[groot-selfhosted](https://github.com/hrodrig/groot-selfhosted) standalone** → gfs HTTP or `upload.s3` / `upload.sftp` | Common ops pattern; same catalog as laptops and cluster |
| Scheduled collects only; no gfs UI yet | **[groot-selfhosted](https://github.com/hrodrig/groot-selfhosted)** CronJob / Helm | Operator packaging; add gfs later if you need a web door |
| Legacy SFTP drop box; no gfs | **[SFTP inbox](#sftp-drop-groot-uploadsftp--sshd)** — groot `upload.sftp` + OpenSSH | Today: selfhosted playbook; gfs Phase 8 will **watch** inbox, not run SFTP |
| Need full DLP, SSO, document workflow | **Enterprise file platform** (SharePoint, Box, …) or **self-hosted Nextcloud** | gfs is a thin archive door, not a collaboration suite |
| Want presigned PUT from laptop or bastion straight to bucket | **Not gfs today** — spike per provider; until then HTTP to gfs | [SPEC non-goal](SPECIFICATIONS.md); vendor SigV4 quirks vary |
| Single maintainer; archives on laptop disk | **`groot collect` locally** | No shared service required |

---

## Groot family (internal choices)

### S3 only: groot `upload.s3` + S3 client (no gfs)

**Pattern:** [groot-selfhosted `run/examples/s3-contabo/`](https://github.com/hrodrig/groot-selfhosted/tree/main/run/examples/s3-contabo) (same idea for AWS, MinIO, R2, Wasabi, …).

| | |
|--|--|
| **Pros** | No VPS process; multipart upload in groot; validated in lab; one in-cluster Secret |
| **Cons** | No per-user web login; audit/retention = bucket lifecycle + cloud trail (if you configure it); laptops need another path (manual upload via CLI or desktop S3 client with shared key — **avoid**) |
| **gfs role** | **Do not deploy gfs.** AGENTS.md and SPEC forbid shipping gfs for this topology. |

**Best when:** Object store is the system of record; operators are comfortable with S3 tooling; you do not need a team web UI on a VPS.

---

### VPS only: gfs on disk

**Pattern:** `GFS_TOPOLOGY=vps`, archives under `GFS_DATA_DIR/home/`.

| | |
|--|--|
| **Pros** | Simplest gfs deploy; no AWS SDK on host; good for labs and small teams |
| **Cons** | VPS disk is durability boundary; multi-GB hairpin if cluster could have used S3; backup/DR is your problem |
| **Cluster path** | trigger/CronJob can HTTP POST to gfs, **`upload.s3`**, or SFTP; bastion can HTTP POST or use `upload.s3` / `upload.sftp` per playbook |

**Best when:** You have a VPS, no bucket (or bucket not worth the complexity), archive volume fits disk budget.

---

### VPS + S3: gfs + bucket home

**Pattern:** `GFS_TOPOLOGY=vps-s3`, `GFS_S3_*` + host `AWS_*`; cluster uses `upload.s3` to **same prefix** (e.g. `captures/`).

| | |
|--|--|
| **Pros** | Durable home in bucket; laptops never get bucket keys; cluster multi-GB skips VPS transit; one Captures list |
| **Cons** | More moving parts (staging, transit retry, endpoint/path-style quirks); VPS still required for auth UI |
| **Ingest preference** | Cluster → **S3 direct**; laptop or bastion → **HTTP → gfs → staging → bucket** (or `upload.s3` / `upload.sftp` from bastion when configured) |

**Best when:** Production-like setup — team door on VPS, bytes at rest in object storage.

See [GFS-CONSENSUS § VPS + S3 ingest](GFS-CONSENSUS.md#vps--s3-ingest) for transit/retry semantics.

---

### groot-trigger (companion, not a catalog)

| | |
|--|--|
| **Does** | `GET/POST /v1/collect` → Kubernetes Job running `groot collect`; optional Job env for `upload.s3` |
| **Does not** | Store archives, list captures, download tarballs, user accounts |
| **Pairs with** | S3 only (Job uploads), VPS+gfs (Job POSTs to gfs — possible but usually worse than S3), SFTP |

**Best when:** Operators want a **Generate GROOT files** button inside the cluster. Always pair with a **storage** choice above.

---

### SFTP drop: groot `upload.sftp` + sshd

**Pattern:** [groot-selfhosted `run/examples/sftp-vps/`](https://github.com/hrodrig/groot-selfhosted/tree/main/run/examples/sftp-vps/) — inbox directory, groot pushes `.tar.gz`.

| | |
|--|--|
| **Pros** | Familiar SFTP; no gfs required for VPS-only file drop |
| **Cons** | SSH user management; no built-in audit/RBAC like gfs; separate from HTTP UI |
| **gfs roadmap** | Phase 8: **watcher** on `GFS_SFTP_INBOX` ingests into Captures (`source=sftp`) — gfs still **does not** run SFTP server |

**Best when:** Producers already use SFTP; you may add gfs later for unified list/audit.

---

## Generic and third-party alternatives

### Shared bucket credentials on laptops

| | |
|--|--|
| **Reality** | Fastest day-one hack; worst blast radius |
| **vs gfs** | gfs exists largely to **avoid** this |
| **Verdict** | **Never best** for a multi-person team if you can deploy gfs or IAM properly |

---

### Desktop / CLI S3 clients (Cyberduck, aws cli, rclone, MinIO Client)

| | |
|--|--|
| **Pros** | Mature; work with any S3-compatible endpoint; no custom app |
| **Cons** | Shared keys or per-user IAM outside gfs; no groot-specific retention/audit unless you build it |
| **vs gfs** | **Better** for S3-only topology with skilled operators. **Worse** when you need per-user auth without IAM federation. |

---

### MinIO Console / AWS S3 Console / vendor object-store UI

| | |
|--|--|
| **Pros** | Native bucket browser; IAM integration (AWS) |
| **Cons** | Not groot-aware; no `keep_last` semantics for capture names; mixed vendor UX |
| **vs gfs** | **Better** if you already live in AWS IAM and S3 is the only interface. **Worse** for a flat team + mixed laptop/bastion/cluster producers without IdP. |

---

### Nextcloud / ownCloud / Seafile

| | |
|--|--|
| **Pros** | Full sync/share, versioning, desktop clients, plugins |
| **Cons** | Heavy stack; not tuned for multi-GB one-shot `.tar.gz` from groot; ops burden |
| **vs gfs** | **Better** for general document collaboration. **Worse** as a dedicated groot capture door — gfs is intentionally minimal. |

---

### nginx / Caddy + basic auth + upload module

| | |
|--|--|
| **Pros** | Minimal dependencies |
| **Cons** | You rebuild auth, audit, retention, S3 listing, cluster ingest dedupe |
| **vs gfs** | **Better** only for a throwaway lab. **Worse** once you need RBAC, API keys, and bucket-backed catalog. |

---

### Git LFS / artifact repository (Nexus, Artifactory, GitHub Releases)

| | |
|--|--|
| **Pros** | Versioning, CI integration |
| **Cons** | Wrong abstraction for cluster diagnostic tarballs; size/cost; not optimized for groot layout |
| **vs gfs** | **Better** for build artifacts tied to repos. **Worse** for operational K8s captures shared across a team. |

---

### WebDAV (including groot [#97](https://github.com/hrodrig/groot/issues/97) sink idea)

| | |
|--|--|
| **Consensu** | WebDAV as a **groot upload sink** is a different product direction — **not** a substitute for gfs ([GFS-CONSENSUS out of scope](GFS-CONSENSUS.md#out-of-scope-for-gfs-stay-elsewhere)) |
| **vs gfs** | **Better** if your org standardizes on WebDAV everywhere. **Worse** for audit/retention/RBAC spec gfs already implements. |

---

## Side-by-side summary

| Approach | Per-user auth | Audit | Retention | Cluster multi-GB → S3 | Laptop upload w/o `AWS_*` | Ops weight |
|----------|---------------|-------|-----------|------------------------|---------------------------|------------|
| **gfs vps** | yes | yes | yes | via HTTP (hairpin) | yes | VPS + sqlite |
| **gfs vps-s3** | yes | yes | yes | **direct** `upload.s3` | yes | VPS + bucket + staging |
| **S3 only** | IAM/vendor | optional (CloudTrail…) | lifecycle rules | **direct** | no (unless presigned/IAM) | bucket + groot |
| **SFTP inbox** | SSH users | no (gfs Phase 8 partial) | manual | via SFTP | no | sshd + disk |
| **groot-trigger alone** | API key (collect) | no | no | optional in Job | N/A | in-cluster |
| **Nextcloud / etc.** | yes | varies | varies | no native | yes | high |
| **Shared `AWS_*` on laptops** | no | no | no | yes | “yes” (bad) | lowest day-1 |

---

## Transparency: where gfs is weak or unfinished

Be explicit so you do not over-deploy:

| Limitation | Detail |
|------------|--------|
| **Requires VPS** (for gfs topologies) | No gfs process in pure S3-only — by design |
| **Not IdP/OIDC yet** | Local users + bootstrap env; SSO is v2 ([REQUIREMENTS](../.planning/REQUIREMENTS.md)) |
| **No analyze/compare UI** | MVP is ingest/list/download/audit; LLM phase 2 |
| **Presigned PUT from laptop / bastion** | Not promised; HTTP through gfs until provider spikes land |
| **Share archive with external third party** | **Phase 9** — admin-only time-limited `/s/{token}` + download audit; not in v0.1–v0.3 |
| **SFTP server** | Never in gfs; Phase 8 watcher only |
| **Visibility modes** | MVP: authenticated users see captures; fine-grained visibility TBD |
| **Single binary trust** | BYOK LLM (future) trusts gfs host for envelope encryption |
| **Coverage / maturity** | Young repo; run `make ci` and your own soak before production |

When any row above is a hard requirement and gfs does not meet it, **choose the alternative** — do not wait for gfs to become something else.

---

## References

- [groot SPEC — post-collect upload](https://github.com/hrodrig/groot/blob/main/SPECIFICATIONS.md#10-post-collect-upload)
- [groot-trigger SPEC](https://github.com/hrodrig/groot-trigger/blob/main/docs/SPECIFICATIONS.md)
- [groot-selfhosted run/examples](https://github.com/hrodrig/groot-selfhosted/tree/main/run/examples)
- [gfs SPECIFICATIONS.md](SPECIFICATIONS.md)

*Last updated: 2026-08-13*
