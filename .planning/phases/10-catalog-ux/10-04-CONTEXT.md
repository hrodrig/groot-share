# Plan 10-04 — Responsive table → card layout — Context

**Locked against:** `10-CONTEXT.md` decisions D-03, D-04, D-14.
**Prepared:** 2026-08-21
**Status:** in progress

## Goal

On narrow viewports, the Captures archive list stops forcing a horizontal
scroll on a 6-column table and renders as **cards** instead. Download stays a
primary action on each card (not buried in a kebab), and copy-download-link is
preserved.

## Non-goals (deferred)

- Share-link admin UI on cards → **10-07** (D-14 reserves the top-row slot but
  the button itself lands later).
- Activity filters/export, settings safety, destructive confirm → **10-05**.
- Manifest peek → **10-06**.

## Locked decisions (from 10-CONTEXT.md)

- **D-03:** Captures layout ends in **table** (desktop) / **cards** (mobile).
  Source/storage pills stay secondary (below the name on a card).
- **D-04:** **Download** is a primary action on every row **and** every card.
  Copy download URL (`/v1/archives/{id}/file`) preserved.
- **D-14:** On a card, the (future) share-link action becomes a top-row button
  between Download and pin. For 10-04 we lay the card action row out so that
  slot exists, but render nothing extra yet.

## Implementation approach

Two render passes in `homeTmpl`, gated by CSS media queries:

1. **Table** (unchanged structure) — visible `≥ 720px`, hidden below.
2. **Cards** — a `<ul class="archive-cards">` rendered right after the table,
   hidden `≥ 720px`, visible below.

Cards are the source of truth for the mobile layout, not a CSS
`data-label` remap of the table. Reasons:

- D-04 wants Download **primary** (large button), while the row keeps it as an
  icon — a `display:block` remap cannot restructure prominence cleanly.
- The row uses `<form>` + button for delete; a `<li>` card can carry the same
  controls but laid out vertically.
- Explicit parallel markup is readable, testable (`strings.Contains`), and
  does not fight the existing `.grid` CSS.

Shared helpers keep it DRY: the SVGs and actions are duplicated literally
(markup-level, acceptable for a server-rendered template) rather than
factored into template partials, keeping the diff mechanical.

## Breakpoint

Match the project's existing narrow-viewport breakpoint: `@media (max-width:
719px)` for cards-on/table-off (the existing rules use `max-width: 720px`,
so `719px` avoids a 1px overlap edge).

## Card structure (per archive)

```
<li class="archive-card">
  <div class="card-title" title="{key}">{key}</div>   <!-- mono, break-all -->
  <div class="card-meta">
    <span class="pill pill-{source}">{source}</span>
    <span class="pill pill-{storage}">{storage}</span>  <!-- if present -->
    <span class="muted tabular">{humansize size}</span>
    <span class="muted tabular">{uploaded UTC}</span>
  </div>
  <div class="card-actions">
    <a class="btn" href="/v1/archives/{id}/file">Download</a>   <!-- PRIMARY -->
    <button copy-link data-copy-url=...>Copy link</button>
    <form delete data-confirm=...> <button>Delete</button> </form>  <!-- if CanDelete -->
  </div>
</li>
```

`Download` is a full-width primary `.btn` (accent) — satisfied D-04.

## Verification (binary)

- `make ci` exits 0 (fmt-check, lint, gocyclo ≤ 14, `go test ./... -race`).
- New test asserts the card markup is present for an uploader/admin and
  **absent** for a viewer, and that the table is still present for wide CSS.

## Commit order (short, per-thing-functional)

1. `feat(ui): archive cards markup for narrow viewports`
2. `style(ui): card layout CSS + breakpoint (table off / cards on)`
3. `test(ui): archive cards visible to uploader, hidden from viewer`
4. `docs: Phase 10 10-04 — responsive cards (SPEC §4, CHANGELOG, README)`
5. `docs(plan): 10-04 SUMMARY + STATE/ROADMAP sync`
