# gfs — consensus so far

**Captured:** 2026-08-11, updated **2026-08-12** (bootstrap admin locked)  
**Repo:** `groot-share` (product name **gfs** = groot files share)  
**Next:** more open questions remain; **no application code** until those are clearer; then kick off **GSD**.

This document freezes what we already agree. Open questions stay open.

---

## Problem

- Shared **develop** (or similar) cluster; ~20 users from local machines.
- Want **one place** for all groot `.tar.gz` archives.
- **S3/SFTP credentials on every laptop** is a bad fit (many S3-compatible providers — Contabo, Hetzner Object Storage, Wasabi, cheap Ceph/RGW — expose one key pair for the whole bucket; revoke = rotate for everyone; leak = full bucket).
- Need **web** upload/access, not “everyone gets `AWS_*`”.
- Archives can be **several GB**. Bytes of that size should not be a VPS hairpin when a direct S3 path exists.

## Product one-liner

**gfs** = web/API door (auth, list, download, audit, retention) in front of groot archives.  
**groot** produces the tarball. **groot-trigger** starts an in-cluster collect on demand.  
gfs exists only when there is a VPS. S3-only is groot `upload.s3` to any compatible bucket, with no gfs.

Laptop `AWS_*` never. Cluster may keep one Secret for `upload.s3` (preferred when S3 exists).

---

## Family

| Repo | Role |
|------|------|
| **[groot](https://github.com/hrodrig/groot)** | CLI: collect / validate / inspect / analyze → `.tar.gz`; `upload.s3` / `upload.gcs` / `upload.sftp` |
| **[groot-trigger](https://github.com/hrodrig/groot-trigger)** | In-cluster HTTP → Job `groot collect` (validated v0.1.0). Does not store or serve archives. |
| **[groot-selfhosted](https://github.com/hrodrig/groot-selfhosted)** | CronJob / bastion; S3 playbooks = topology **S3 only**; SFTP-VPS playbook ≈ topology **VPS only** without gfs UI |
| **gfs** (this repo) | Topologies **VPS only** and **VPS + S3**: users, HTTP ingest, listing, download, audit, retention |

---

## Three operator topologies (locked 2026-08-12)

Deploy-time choice. **No per-upload “also S3” flag.**

| Topology | gfs? | Who receives the `.tar` | Where it lives | Who lists |
|----------|------|-------------------------|----------------|-----------|
| **VPS only** | yes | gfs HTTP (trigger / groot / laptops / bastion) | VPS disk (**home**) | gfs (local) |
| **S3 only** | **no** | groot / trigger `upload.s3` | the bucket (Contabo, AWS, MinIO, R2, …) | S3 client (Cyberduck, aws cli, rclone, …) |
| **VPS + S3** | yes | see ingest below | VPS disk = **transit**; bucket = **home** | gfs, **from the bucket** |

**S3 only** is today’s validated path: groot `upload.s3` + custom `endpoint` (lab: Contabo; same topology for MinIO, AWS, R2). Do not invent gfs there.

### VPS + S3 ingest

Both doors exist. **S3-direct is preferred** when the producer already has cluster `AWS_*`.

| Producer | Path | Notes |
|----------|------|--------|
| trigger / CronJob / groot with in-cluster Secret | **`upload.s3` → bucket** (preferred) | Multi-GB never touches the VPS. Listing still sees it (list from S3). |
| same producer, optional | HTTP → gfs → staging → S3 | Possible; worse for large archives. |
| laptop or bastion (no long-lived keys) | HTTP → gfs → staging → S3 | Laptop/bastion path without bucket creds on the operator machine |

HTTP ingest on VPS + S3:

1. Land on local staging (transit).
2. Copy to the bucket (same logical key / prefix, e.g. `captures/`).
3. Delete staging when the object is in the bucket.
4. Listing is S3: in-flight staging does **not** count as a groot file yet.

S3 is the **durable disk at rest**, not an infallible PUT. If the copy to the bucket fails, the tar stays in transit and **retries**. That is not the happy state; it is “not home yet”. Do not fail the whole HTTP upload (laptop would re-POST gigabytes). Do not treat local-only as success-for-ever when S3 is configured.

VPS staging disk is sized for **in-flight** objects, not for `keep_last` × 90 days.

### Durability vs availability

- **VPS disk:** available for this HTTP request; dies with the VM.
- **Bucket:** home once the object is stored; a given PUT can still timeout / quota / network-fail.

### Presigned URLs

Vendor panels (and most S3-compatible control planes) do **not** issue presigned URLs. gfs (or Cyberduck / AWS SDK) **mints** SigV4 URLs with the host credentials. Path-style vs virtual-hosted depends on the endpoint (Contabo and MinIO need path-style; AWS usually virtual-hosted; Cloudflare R2 has its own host). GET-for-download is the usual shape; PUT/multipart for multi-GB is a **lab spike per provider**, not assumed. Laptop and bastion upload in VPS + S3 is HTTP to gfs, not a presigned PUT, until a spike says otherwise.

---

## Decisions (locked for now)

| # | Topic | Decision |
|---|--------|----------|
| 1 | Retention | **Both** rules apply: keep last **N** *and* max age **D** days. Delete when **either** fires. Defaults: `keep_last=20`, `max_age_days=90` (parametrizable). In VPS + S3, delete **home** (bucket); staging is not the retention set. |
| 2 | Visibility | **Configurable** (e.g. private / team / hybrid). Admin sees all. Exact modes TBD. |
| 3 | LLM / analyze | **Phase 2** — not MVP. |
| 4 | Implementation language | **Go**. |
| 5 | Metadata DB | **SQLite** for users, sessions, api_keys, audit. File **inventory** in VPS + S3 is the bucket listing, not a parallel source of truth. |
| 6 | Topologies | **VPS only** / **S3 only (no gfs)** / **VPS + S3**. S3-compatible object storage is preferred when the operator has it (Contabo, AWS, MinIO, R2, Wasabi, Hetzner, DigitalOcean Spaces, …). |
| 7 | Auth model | Per user: **login + password** (web) and **login + api_key** (upload API). Trigger’s shared `GROOT_TRIGGER_API_KEY` is a different secret (can start a cluster collect). |
| 8 | Audit | Required (who uploaded/downloaded/deleted/analyzed; timestamps; useful request metadata). Never log secrets. |
| 9 | LLM credentials (phase 2) | Each user may store **their own** provider credentials (BYOK) to interact with archives; encrypted at rest; not a single global team key for user chats. |
| 10 | Origins | **Three classes** from day one: in-cluster (trigger / CronJob), **bastion** (groot-selfhosted Docker / cron / systemd), and laptops. |
| 11 | gfs host | First gfs is a **VPS** (topologies that include gfs). Not a substitute for trigger inside the cluster. |

---

## Auth detail

- **Web UI:** username + password → session/cookie.
- **Upload API:** username + api_key (opaque, rotatable; show full secret only at creation).
- Password hash in SQLite; api_key stored hashed (or equivalent); raw secrets never in audit rows.
- **First admin (locked 2026-08-12):** empty user table → create one admin from `GFS_BOOTSTRAP_ADMIN` + `GFS_BOOTSTRAP_PASSWORD` (once). Missing env → refuse start. Users already present → ignore the env. No well-known default password.
- Phase 2: BYOK LLM keys encrypted with a server envelope key; used only in-memory when calling the provider; document that the gfs host is trusted.

---

## MVP (phase 1) — in scope

- Users: password + api_key
- Upload `.tar.gz` via HTTP API (VPS only and VPS + S3)
- List + download via web (respect visibility config); VPS + S3 list from bucket
- Audit log
- Transit + retry when S3 is configured
- Retention job: `keep_last` **and** `max_age_days`
- Coexist with trigger `upload.s3` on the same prefix (preferred cluster path)

## Explicitly later (phase 2+)

- `groot analyze` / compare from gfs UI
- BYOK LLM per user
- Optional CLI: `groot upload --gfs` with api_key (token), never AWS keys
- Richer roles/scopes, quotas, OIDC, etc. (ideas — not committed)
- Presigned PUT for laptop / bastion → bucket (only if a spike against that provider succeeds)

---

## Ideas parked (not decided)

- Roles: `uploader` / `viewer` / `admin`
- api_key scopes: `upload` / `read` / `analyze`
- Idempotent upload (content hash / Idempotency-Key)
- Peek `extras/manifest.json` on upload → store `run_id`, cluster, groot version
- Soft quotas (GB/user, uploads/day)
- Presigned **GET** for download when listing from S3 (spike: SigV4 against the operator’s endpoint)
- Object key scheme so gfs transit PutObject and groot `upload.s3` do not collide on `captures/`
- Repo naming: product **gfs**, git folder **groot-share** — OK unless we rename later

---

## Out of scope for gfs (stay elsewhere)

- Changing groot collect pipeline as the main delivery for 20-laptop S3 keys
- Mass-distributing bucket `AWS_*` to operators “until gfs exists”
- Building WebDAV into groot CLI as a substitute for gfs (**groot #97** is a different sink)
- Status poll / download proxy inside **groot-trigger** (trigger SPEC non-goal)

Related ops: groot-selfhosted S3 examples (e.g. `run/examples/s3-contabo/` as one EU vendor) **are** topology S3 only — same topology for MinIO-in-cluster, AWS, or R2. SFTP-VPS example is the ancestor of topology VPS only. gfs is the authenticated door for the VPS-involving topologies.

---

## Open questions

1. Visibility modes — exact enum and defaults for a 20-person develop team.
2. Retention — global only vs per-user overrides; edge cases (keep_last vs age when count is low).
3. ~~Who provisions first admin~~ — **locked:** env once (`GFS_BOOTSTRAP_*`); fail closed if the user table is empty. Later users: admin creates them in-app (Phase 3).
4. First internal gfs **hostname** / which VPS (host role is locked: VPS).
5. Whether MVP peeks manifest on upload or only stores filename/size/sha256.
6. Envelope encryption approach for BYOK (phase 2) — KDF from server secret vs external KMS.
7. Monorepo vs this repo only; module layout when GSD starts.
8. How tightly gfs should vendor/call groot (`exec` binary vs Go module import) for analyze.
9. Download in VPS + S3: presigned GET vs proxy through gfs (depends on a GET spike against the operator’s S3 endpoint).
10. Staging retry policy (interval, give-up, operator alert when stuck in transit).

---

## Working agreement

- **No application code** until the remaining open questions are clearer (order of days, not hours).
- Keep refining in this repo (`docs/`).
- Then start **GSD** for real planning/execution.

---

## Session breadcrumbs

- **2026-08-11:** after shipping groot **v1.1.1**, groot-selfhosted **v0.2.10**, landing/get pin bumps, and `protect-main` on github.com/hrodrig/groot — focus shifted from “shared S3 on laptops” to **gfs** as the correct product for team archive sharing.
- **2026-08-12:** groot-trigger **v0.1.0** validated (in-cluster HTTP → collect Job, optional `upload.s3` to Contabo in lab). Locked three topologies; VPS as transit when S3 exists; cluster S3-direct preferred; laptops HTTP to gfs; no per-upload S3 flag. Object storage contract is S3-compatible, not one vendor.
