# Plan 10-05 — Activity filters + admin export + typed-name confirm — Summary

**Closed:** 2026-08-21

## What shipped

### UX-05 — Activity filters + admin export
- **Store filters** (`internal/store/audit.go`): `AuditFilter{Actor, Action,
  Since}` + `CountAuditFiltered` / `ListAuditFiltered`. `Actor` is a
  case-insensitive substring (`LIKE ? ESCAPE '\'`), `Action` exact match,
  `Since` a `created_at >= ?` bound. `auditWhere` builds a parameterized
  clause with stable placeholder positions. `limit < 0` = "no limit" (export
  path); `limit <= 0` = 50/page.
- **Indexes** (`internal/store/schema.go`): `audit_actor_idx`,
  `audit_action_idx`, `audit_created_at_idx` (idempotent `CREATE INDEX IF
  NOT EXISTS`, applied on every migrate).
- **Handlers** (`internal/server/audit.go`): `handleActivityGET` and
  `handleListAudit` read `actor`/`action`/`window` query params via
  `auditFilterFrom`; unknown windows dropped. `handleActivityExport` streams
  the full log as CSV or JSON, admin-only (`ac.User.Role == auth.RoleAdmin`).
  CSV escapes commas/quotes/newlines (`csvField`).
- **UI** (`identity.go` + `html.go`): filter bar on the Activity page
  (action dropdown, actor search, window select, Filter button) with
  `auditActions` catalog; admin-only Export CSV/JSON buttons carrying the
  current filters.

### UX-06 — Typed-name confirm + tokens
- **Typed-name confirm** (`identity.go`, `settings.go`, `html.go`): the
  generic `confirm-dialog` now supports `data-confirm-require="<text>"`.
  When present, the modal shows a text input and disables the confirm button
  until the typed value exactly matches. Applied to destructive actions:
  delete archive (`.Key`), remove user (`.Username`), revoke API key
  (`.Prefix`). Reversible unpin stays a plain confirm.
- **Four-color tokens** already existed (`--accent`/`--ok`/`--warn`/`--err`).
- **API key shown once + copy** (D-13) already existed; unchanged.

## Commit order

1. `docs(plan)`: 10-05 CONTEXT + PLAN
2. `feat(store)`: audit filters + indexes
3. `feat(server)`: activity + audit filters + admin export
4. `feat(ui)`: activity filter bar + admin export button
5. `feat(ui)`: typed-name confirm on destructive actions
6. `test(ui)`: filters + export + typed-confirm markup
7. `docs`: SPEC §8, CHANGELOG, README

## Verification

- `go test ./internal/server/ -run 'TestAuditFilters|TestActivityExport|TestTypedConfirm'` green
  (4 new tests: store filter matrix, admin CSV/JSON export + CSV escaping,
  viewer export → 403, typed-confirm markup on delete).
- `make ci` green (fmt-check, lint 0 issues, gocyclo ≤ 14, `go test ./... -race`).

## Decisions

- **`limit < 0` = "no limit"** for the export path instead of a separate
  `ListAuditAll`, keeping one filtered query.
- **Reuse `windowSince`** from `filter.go` rather than a second window parser.
- **`CanExport` derived from role** (`RoleAdmin`) set explicitly in the
  handler, not inferred from `CanManageUsers`.
- **Reversible unpin left as plain confirm** — only irreversible,
  data-destroying actions got typed-name confirm.

## Notes / deferred

- Nav grouping ("Audit" subsection) was optional per ROADMAP; left unchanged.
- `remote_ip` relies on `RemoteAddr` (trusted-proxy handling remains SPEC §8 open).

## Next

- **10-06** — optional cheap `extras/manifest.json` peek → partial-capture badge.
- **10-07** — share-link admin UI (UX-09). Bump + push only after 10-07 SUMMARY.
