# Plan 10-01 — Captures dashboard — Summary

**Completed:** 2026-08-21
**Branch:** develop
**Commits:** 6 functional + 1 lint + 1 docs = 8 commits

## What landed

### 1. Storage — `internal/store/pins.go` + `schema.go`
- `archive_pins` table (user_id, archive_id, archive_key, archive_size,
  created_at) with `PRIMARY KEY (user_id, archive_id)` and FK CASCADE on
  user delete; index `(user_id, created_at DESC)` for the per-user list.
- `Store.AddPin(ctx, userID, Archive)` — `INSERT OR IGNORE`; snapshots
  `archive_key` and size at pin time so the row stays useful when the
  archive is in transit (vps-s3) or deleted.
- `Store.RemovePin(ctx, userID, archiveID) (bool, error)` — returns
  `removed=true` if a row was deleted; idempotent (second call → false, nil).
- `Store.ListPins(ctx, userID, limit)` — newest-first; limit ≤ 0 = no cap.

### 2. Filename parser — `internal/store/cluster.go`
- `ParseClusterSlug(key) (slug string, ok bool)` extracts the cluster slug
  from a groot basename `<prefix>-<cluster>-<YYYYMMDD>[<sep>?<HHMMSS>]
  [-since-<slug>].tar.gz`. Conservative: returns `""` rather than guessing
  when the name does not match.
- Used by the dashboard to count distinct cluster slugs in the inventory.

### 3. Pin endpoints — `internal/server/pins.go`
- `POST /v1/pin/archives/{id...}` — pin (idempotent). `201` JSON for API
  clients; `303 → /` for browser forms.
- `DELETE /v1/pin/archives/{id...}` — unpin. `204` JSON; idempotent.
- `POST /v1/pin/archives/{id...}/delete` — same path with trailing
  `/delete` (HTML form-alias for the unpin button) → `303 → /`.
- RBAC: any authenticated user (viewer allowed; pinning is a personal
  preference).
- Path is `/v1/pin/archives/{id...}` rather than `/v1/archives/{id}/pin`
  because the Go 1.22 mux requires the wildcard terminal — and the
  share-link route already owns the `/v1/archives/{id}/...` namespace.

### 4. Captures UI — `homeTmpl` + `handleHome`
- **Inventory summary strip** above the page-head: count, bytes on disk
  (humanized), distinct cluster slugs (from `ParseClusterSlug`), in-transit
  count (items where `storage == "transit"`), storage topology pill
  (`vps` → `pill-local` green; `vps-s3` → `pill-s3` muted).
- **Upload archive CTA card** for uploader and admin (hidden for viewer).
  Links to the existing `/upload` page; the inline dropzone with XHR
  progress lands in plan 10-03.
- **Per-user pin strip** between the CTA and the table; renders only when
  the user has at least one pin. Each row: download link (key, mono,
  truncated), humanized size, unpin form.
- Default sort newest-first (was already the case via `parseSort`).

### 5. CSS — `internal/server/html.go`
- `.summary` flex row with 5 cells, 1px hairline separators, mono labels
  in `var(--muted)`. Mobile: 2-column grid.
- `.upload-cta` flex row with copy + primary button. Mobile: stacks.
- `.pin-list` rows with key + size + unpin button.

## Verification

- `gofmt -l` clean
- `go vet ./...` clean
- `golangci-lint run ./...` 0 issues
- `gocyclo` ≤ 14 across the whole tree
- `go test ./... -race -count=1` green (server 9.4s, store 33.0s)
- `make ci` exits 0
- New tests:
  - `internal/store/cluster_test.go` — 10 cases for the parser
  - `internal/store/pins_test.go` — 6 cases for Add/Remove/List/idempotency/cascade
  - `internal/server/pins_test.go` — 7 cases for the endpoints (POST/DELETE/form/JSON/404/401/viewer-allowed)
  - `internal/server/dashboard_test.go` — 9 cases for the summary strip + CTA + pin strip

## Commits

| SHA | Message |
|-----|---------|
| `ba763ee` | docs(plan): seed Phase 10 CONTEXT.md from 2026-08-19 lock |
| `b6f3569` | docs(plan): 10-01 Captures dashboard — summary strip, upload CTA, pin strip |
| `cc00b61` | feat(store): archive_pins schema + Add/Remove/List (idempotent, per-user, cascade) |
| `8d9343b` | feat(server): pin/unpin endpoints (POST/DELETE /v1/pin/archives/{id...}) + form alias |
| `c1cd23d` | feat(ui): Captures inventory summary strip — count, bytes, clusters, in-transit, topology |
| `0a3782c` | feat(ui): Upload archive primary CTA on Captures page (uploader+ only) |
| `e4e1eb0` | feat(ui): per-user pin strip on Captures (download + unpin form) |
| `a420700` | test(store): lint fixes — misspell + split gocyclo over-14 test |

Docs sync (SPEC §4, CHANGELOG, README, ROADMAP, STATE) follows in the next
commit; this SUMMARY is written first so the diff that ships is mechanical.

## Not in this plan (deferred, per the PLAN's "Out of scope")

- Cluster chip rendering with counts → **10-02**
- Search + time-window filters → **10-02**
- Inline dropzone + XHR upload progress on Captures → **10-03**
- Card layout for narrow viewports → **10-04**
- Color-token audit pass (D-07 sweep) → **10-05**
- Manifest peek (incomplete count is the *transit* proxy for 10-01 only) → **10-06**
- Share-link admin UI on each row → **10-07**

## Notes for the next plan

- `useBucket()` is the source of truth for topology; the summary strip reads
  from it via the same helper, so plan 10-05 (visual tokens) only has to
  swap the `.pill-local`/`.pill-s3` classes if the design wants different
  colors.
- The form-alias `POST /v1/pin/archives/{id}/delete` is necessary because
  HTML forms only support GET/POST; the DELETE method has no form path.
- The `archive_pins` row snapshots `archive_key` and `archive_size` so
  the strip remains useful when the underlying archive is in transit or
  deleted. Plan 10-02 can decide whether to mark orphan pins visually.

---

*Plan: 10-01 — Captures dashboard*
*Summary: 2026-08-21*
