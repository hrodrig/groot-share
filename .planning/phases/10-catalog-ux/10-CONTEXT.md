# Phase 10: Operational catalog UX — Context

**Gathered:** 2026-08-21 (transcribed from lock 2026-08-19)
**Status:** Ready for planning
**Target release:** v0.5.0 (after Phase 9 share links v0.4.0)

<domain>
## Phase Boundary

gfs Captures is an **incident evidence locker**, not a generic Dropbox. Operators
must find a `.tar.gz` under pressure (cluster, capture window, completeness),
download it, paste the link into a ticket, and know who fetched it.

The product chrome is **gfs** + `VERSION` (no fictional mock version). UI stays
server-rendered vanilla HTML — **no SPA**, **no component library** (LIST-03).

The work covers polish on shipped list/upload HTML and the admin share-link UI
that backs the Phase 9 API. Does **not** replace Phase 8 (SFTP watcher is a
separate door; here it just shows the SFTP pill consistently).

</domain>

<spec_lock>
## Requirements

Operator UX (Phase 10). Amend `docs/SPECIFICATIONS.md` §4 / §5 / §6 when list
HTML, JSON facets, or manifest peek land. Filename parse conforms to groot
archive basename (SPEC §5).

**In scope (UX-01..08 from REQUIREMENTS.md + UX-09 share-link admin):**

- **UX-01** — Captures dashboard: totals (count, bytes), cluster count, incomplete
  count when known, storage topology, primary **Upload archive**; default sort
  newest first; optional per-user pin strip
- **UX-02** — Search/filters via query params: cluster chips with counts,
  name/`since`/message search, capture window 24h/7d/30d/all. Empty states:
  **no archives yet** vs **no match** + clear. Secondary: source/storage pills.
- **UX-03** — HTTP upload UX: `.tar.gz` + size limit visible (32 GiB copy unless
  SPEC differs), name+size before send, progress, transit copy, cancel; duplicate
  called out
- **UX-04** — Narrow viewports: table row → compact card; **Download** primary
  on the card; desktop table also exposes Download (not kebab-only)
- **UX-05** — Activity is compliance-grade: filter by user/action/date;
  **download** events first-class; admin CSV/JSON export **required**
- **UX-06** — Typed-name confirm on destructive actions; api_key shown once
  with copy feedback; four-color state tokens (blue=primary, green=ready,
  amber=transit/partial, red=error/failed); `mono` for IDs/hashes/filenames/cluster
- **UX-07** — Filename facets from groot basename
  (`<prefix>-<ts>[-since-<slug>]-<cluster>[-message].tar.gz`): cluster, capture
  time, since, optional message; per-user pin optional
- **UX-08** — Partial-capture badge from `extras/manifest.json` job failed
  counts: only via **cheap gzip-member peek** (SPEC §11 still Open); never full
  unpack; unmarked if peek missing; labels Complete / `N of M jobs failed` / Failed
- **UX-09** — Share-link admin UI (plan 10-07): admin can create TTL/until-date
  (presets 24h/7d + custom), copy URL once, list/revoke active links per
  archive. Backs onto Phase 9 API (`POST/GET/DELETE /v1/archives/{id}/shares`).
  No raw token shown again after create.

**Out of scope (explicit, lock 2026-08-19):**

- Next/React/SPA / component libraries
- `groot analyze` in-process or `exec` (GFS-CONSENSUS Q8 — locked 2026-08-19)
- Origin pills Trigger / Scheduled / Manual until a **producer** field exists
  (not inferred from `source` http/s3/sftp)
- Secrets-redacted lock icon (groot profile, not catalog metadata)
- Environment enum (prod/staging/dev) as a first-class facet
- Replacing Phase 8 SFTP watcher
- New topology or new storage backend

</spec_lock>

<decisions>
## Implementation Decisions

- **D-01:** Phase target **v0.5.0** (post v0.4.0 share links; no version bump
  during the phase — bump only on `release:` commit).
- **D-02:** Server-rendered Go `html/template` only. No JS framework, no
  bundler. Vanilla CSS with shared tokens; `mono` font face for IDs / hashes /
  filenames / cluster column.
- **D-03:** Captures layout (top → bottom): **summary strip** (count, bytes,
  cluster count, incomplete count when known, storage topology) → optional
  **per-user pin strip** → **Upload archive** primary CTA → **cluster chips
  with counts** → **search + capture-time window** controls → **table** (desktop)
  / **cards** (mobile). Source/storage pills stay **secondary** below cluster.
- **D-04:** **Download** is a primary action on every row AND every card
  (not kebab-only). Copy download URL (`/v1/archives/{id}/file`) preserved
  from the v0 mock.
- **D-05:** Cluster chips from filename parse only (groot basename per SPEC §5).
  No server-side cluster taxonomy until a producer field exists.
- **D-06:** Query params persist for **cluster**, **search**, **time window**,
  **source**, **storage**; shareable URL. Empty state distinction:
  *no archives yet* (first visit, no data) vs *no match* (filters exclude all).
  Clear-filters link only in the *no match* state.
- **D-07:** Color tokens locked and used everywhere:
  - **blue** = primary / available action
  - **green** = ready / success
  - **amber** = transit / partial capture
  - **red** = error / failed
  Mono for: archive id, sha256, filename, cluster slug.
- **D-08:** Product chrome shows `gfs v{VERSION}` (read `VERSION` at template
  execute). No fictional mock version like `v0.0.0-mock`.
- **D-09:** Manifest peek (UX-08) implementation: open the `.tar.gz` as a
  streaming gzip reader, advance to **one** member, peek `extras/manifest.json`
  by **name** without buffering other members. **Fail closed** if the archive
  is not a valid groot archive (gzip magic + tar header + member not present
  → no badge, no error, just unmarked). Never run a full unpack; cap peek at
  one member or ~64 KiB, whichever comes first.
- **D-10:** HTTP upload UX (UX-03) progress uses `XMLHttpRequest` upload
  `progress` events (no `fetch` — fetch has no upload progress). Server-Sent
  Events are not used; the upload form posts and the listing page re-renders.
  Cancel = `AbortController` on the XHR.
- **D-11:** Activity export (UX-05): admin-only `GET /v1/activity/export?format=csv|json`
  with the same filters as the HTML activity page. JSON is the wire format
  gfs already returns; CSV is a flat row per event. No scheduled reports.
- **D-12:** Destructive actions (delete archive, revoke share link, delete user)
  require **typed-name confirm**: user types the archive id (or username) into
  an input that is disabled until the value matches. No single-click delete.
- **D-13:** api_key is shown once with a copy-to-clipboard control on the
  `/settings` page. After navigation, the key is gone from the rendered HTML;
  recovery is rotate.
- **D-14:** Share-link admin UI (UX-09, plan 10-07) lives in the Captures row
  as a kebab/row action **only because the row is wide enough**; on a card,
  it becomes a top-row button between Download and pin. Modal/panel pattern
  is server-rendered, not JS.
- **D-15:** Operator repo **groot-share-selfhosted** owns theme tokens (CSS
  variables) and the empty-state illustrations. gfs references the same tokens.
  No design drift between gfs and the operator portal.
- **D-16:** Phase 10 ships **7 plans** (10-01..10-07). Execution order:
  **10-01** (dashboard) → **10-02** (chips + search + time window) →
  **10-03** (upload UX) → **10-04** (responsive) → **10-05** (activity + safety
  + tokens) → **10-06** (manifest peek) → **10-07** (share-link admin UI).
  Commits are short and per-thing-functional (per Hermes: "commits cortos, cada
  vez que se tenga algo funcional").

</decisions>

<canonical_refs>
- `docs/GFS-CONSENSUS.md` — Q8 (analyze boundary) + topology + share-link scope
- `docs/SPECIFICATIONS.md` §4 (list JSON) / §5 (filename) / §6 (RBAC) / §11 (manifest peek, Open)
- `docs/SPECIFICATIONS.md` §12 — share-link HTTP contract (Phase 9)
- `.planning/REQUIREMENTS.md` — UX-01..08 + SHARE-01..03 (UX-09 backs onto Phase 9)
- `.planning/ROADMAP.md` — Phase 10 + Backlog UX-3 / UX-4
- `.planning/STATE.md` — Accumulated decisions (Phase 10 lock entry)
- `docs/SPECIFICATIONS.md` §11 — manifest peek (cheap gzip-member read)
- Commit `4c263fa` — Phase 10 evidence-locker UX lock + analyze boundary
- Phase 9 `09-CONTEXT.md` — share-link contract that UX-09 backs onto

</canonical_refs>

<code_context>
- `internal/server/html.go` — list page templates (replace with dashboard)
- `internal/server/identity.go` — login + bootstrap admin chrome
- `internal/server/archives.go` — upload + download + delete (XHR upload progress in 10-03)
- `internal/server/admin.go` — user CRUD; share-link admin UI lives here in 10-07
- `internal/server/audit.go` — Activity list; export endpoint in 10-05
- `internal/server/catalog.go` — listItems / filter pipeline; facets in 10-02
- `internal/store/schema.go` — share_links table (Phase 9); no new schema
  unless filename parse cache lands (deferred)
- `internal/blob/key.go` — `SourceForKey`; filename parse from basename
- `cmd/gfs/main.go` — no new goroutines; templates render VERSION
- `assets/` — empty-state illustrations + css tokens (kept in gfs; operator
  repo imports via documented path)

</code_context>

<operator_notes>
## Use sketch (evidence locker, three roles)

1. **Uploader** opens Captures → sees summary + cluster chips + newest-first
   table → drops `.tar.gz` in upload zone → progress bar → archive appears at top.
2. **Admin** opens Captures → cluster chip `prod-eks-1` → clicks Download on a
   row → chooses **Share externally** → 24h TTL → copies URL once → pastes into
   Jira ticket → revoke from the same row when ticket closes.
3. **Viewer** opens Captures → search `since:2h` → table filtered → click row →
   Download. No kebab for delete or share-link.

## Visual lock

- Primary action color: blue
- Ready / available: green
- Transit / partial: amber
- Error / failed: red
- ID / hash / filename / cluster: monospace

</operator_notes>

<deferred>
- **UX-3** Role walkthrough (viewer / uploader / admin) — after Phase 10
- **UX-4** Producer origin (Trigger / Scheduled / Manual) — only when groot
  supplies the field; never infer from `source` http/s3/sftp
- **I-5** openpgp false positive (govulncheck) — documented, no fix
- Filename parse cache (SQLite) — only if list latency justifies
- Per-user quota UI — separate phase if needed
- Quarantine dir for corrupt uploads (Phase 8 deferred this too)
- gfs CLI: `gfs list` mirroring HTML facets — backlog
- gfs metrics endpoint: `gfs_ux_*` — backlog

</deferred>

---

*Phase: 10-Operational catalog UX*
*Context gathered: 2026-08-21 (transcribed from ROADMAP lock 2026-08-19)*
