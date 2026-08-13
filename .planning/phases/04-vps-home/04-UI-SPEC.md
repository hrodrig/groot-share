---
phase: 04
slug: vps-home
status: approved
shadcn_initialized: false
preset: none
created: 2026-08-12
---

# Phase 4 — UI Design Contract (applied after Phase 6)

> Vanilla HTML + shared CSS. No SPA, no shadcn. Visual identity for the gfs archive door (login + home).

## Design System

| Property | Value |
|----------|-------|
| Tool | none |
| Preset | not applicable |
| Component library | none |
| Icon library | none (text + CSS marks) |
| Font | system UI + system ui-monospace (no webfonts; VPS may be air-gapped) |

**Direction:** pine-closet + packing-tape labels. Each archive row is a perforated shipping ticket, not a SaaS table. Accent tape is reserved for the wordmark tab and primary CTAs.

## Spacing Scale

| Token | Value | Usage |
|-------|-------|-------|
| xs | 4px | stamp padding |
| sm | 8px | inline gaps |
| md | 16px | field padding |
| lg | 24px | section padding |
| xl | 32px | sheet padding |
| 2xl | 48px | page gutter |
| 3xl | 64px | login vertical air |

Exceptions: none

## Typography

| Role | Size | Weight | Line Height |
|------|------|--------|-------------|
| Body | 16px | 400 | 1.45 |
| Label | 13px | 550 | 1.3 |
| Heading | 18px | 650 | 1.25 |
| Display (wordmark) | 28px | 700 | 1.0 |
| Data (keys, sizes) | 14px | 500 | 1.35, ui-monospace |

## Color

| Role | Dark | Light | Usage |
|------|------|-------|-------|
| Dominant (60%) | `#121a17` | `#efe6d4` | Page background |
| Secondary (30%) | `#1a2420` | `#f7f0e2` | Sheets / panels |
| Label surface | `#24302a` | `#fff8ec` | Archive tickets |
| Accent (10%) | `#e0943a` | `#c45e1a` | Wordmark tab + Sign in + Upload capture |
| Sage | `#8fbf9a` | `#2f6b4a` | Focus ring, source stamp |
| Destructive | `#e07070` | `#a33a32` | Errors; Delete stays ghost (not accent) |
| Ink | `#efe6d4` | `#1a2420` | Text |

Accent reserved for: wordmark tape tab, primary submit buttons (`Sign in`, `Upload capture`). Never for Delete, table links, or body text.

## Copywriting Contract

| Element | Copy |
|---------|------|
| Login eyebrow | Archive door |
| Primary CTA (login) | Sign in |
| Primary CTA (home) | Upload capture |
| Login helper | Sign in to list and download groot captures. |
| Empty archives | No captures yet. Upload a groot .tar.gz to start the list. |
| Empty audit | No activity yet. |
| Login error (unauthorized) | Username or password is wrong. |
| Destructive | Delete (ghost button; no extra confirm in v0.1) |

## UI Considerations

Applicable: empty, populated, error, long-text, overflow.

| Category | Element(s) | Status | Resolution / Reason |
|----------|------------|--------|---------------------|
| empty | archives list | ✅ covered | Empty copy from Copywriting Contract |
| empty | audit list | ✅ covered | Empty copy from Copywriting Contract |
| populated | archives list | ✅ covered | Ticket rows with name, source stamp, size, time, Download, Delete |
| error | login form | ✅ covered | Human sentence on HTML; JSON still uses machine codes |
| long-text | archive key | ✅ covered | Mono cell; overflow wrap / break-word |
| overflow | tables | ✅ covered | Horizontal scroll on narrow viewports; tickets keep actions visible |

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| none | — | not required |

## Checker Sign-Off

- [x] Dimension 1 Copywriting: PASS
- [x] Dimension 2 Visuals: PASS (ticket/perforation signature; not generic dark+blue)
- [x] Dimension 3 Color: PASS (tape reserved)
- [x] Dimension 4 Typography: PASS
- [x] Dimension 5 Spacing: PASS (4px scale)
- [x] Dimension 6 Registry Safety: PASS

**Approval:** approved 2026-08-12 (yolo lock; vanilla HTML)
