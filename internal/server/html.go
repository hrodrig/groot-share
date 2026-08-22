package server

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

const appWhoTmpl = `      <span class="who"{{if .Name}} title="{{.Name}}"{{end}}>{{.DisplayName}} <span class="role">{{.Role}}</span></span>`

const appNavTmpl = `
      <nav class="appnav" aria-label="Primary">
        <a href="/" {{if eq .Nav "captures"}}aria-current="page"{{end}}>Captures</a>
        {{if .CanUpload}}<a href="/upload" {{if eq .Nav "upload"}}aria-current="page"{{end}}>Upload</a>{{end}}
        <a href="/activity" {{if eq .Nav "activity"}}aria-current="page"{{end}}>Activity</a>
        <a href="/settings" {{if eq .Nav "settings"}}aria-current="page"{{end}}>Settings</a>
        {{if .CanManageUsers}}<a href="/admin/users" {{if eq .Nav "admin"}}aria-current="page"{{end}}>Users</a>{{end}}
      </nav>`

// Design system: neutral slate surfaces, one institutional accent, hairline
// borders, monospace data. The amber "crate" mark is the only brand flourish.
const layoutCSS = `
:root {
  color-scheme: light;
  --bg: #f4f6f8;
  --surface: #ffffff;
  --surface-2: #f9fafb;
  --ink: #101828;
  --muted: #475467;
  --faint: #98a2b3;
  --line: #e4e7ec;
  --line-strong: #cfd4dc;
  --accent: #175cd3;
  --accent-hover: #1849a9;
  --accent-ink: #ffffff;
  --accent-soft: #eff4ff;
  --brand: #b54708;
  --ok: #067647;
  --ok-soft: #ecfdf3;
  --warn: #b54708;
  --warn-soft: #fffaeb;
  --err: #b42318;
  --err-soft: #fef3f2;
  --radius: 8px;
  --radius-sm: 6px;
  --shadow: 0 1px 2px rgb(16 24 40 / 0.06), 0 1px 3px rgb(16 24 40 / 0.08);
  --mono: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  --sans: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", sans-serif;
}
html[data-theme="dark"] {
  color-scheme: dark;
  --bg: #0c1116;
  --surface: #131a21;
  --surface-2: #0f151c;
  --ink: #e6e9ed;
  --muted: #8b94a1;
  --faint: #5d6673;
  --line: #26303a;
  --line-strong: #36414d;
  --accent: #6ea8fe;
  --accent-hover: #93c0ff;
  --accent-ink: #0b1526;
  --accent-soft: #16233a;
  --brand: #f79009;
  --ok: #47cd89;
  --ok-soft: #0e2b1e;
  --warn: #fdb022;
  --warn-soft: #2b1f05;
  --err: #f97066;
  --err-soft: #331512;
  --shadow: none;
}
@media (prefers-color-scheme: dark) {
  html:not([data-theme="light"]) {
    color-scheme: dark;
    --bg: #0c1116;
    --surface: #131a21;
    --surface-2: #0f151c;
    --ink: #e6e9ed;
    --muted: #8b94a1;
    --faint: #5d6673;
    --line: #26303a;
    --line-strong: #36414d;
    --accent: #6ea8fe;
    --accent-hover: #93c0ff;
    --accent-ink: #0b1526;
    --accent-soft: #16233a;
    --brand: #f79009;
    --ok: #47cd89;
    --ok-soft: #0e2b1e;
    --warn: #fdb022;
    --warn-soft: #2b1f05;
    --err: #f97066;
    --err-soft: #331512;
    --shadow: none;
  }
}

* { box-sizing: border-box; }
html { background: var(--bg); min-height: 100%; }
body {
  margin: 0;
  font: 400 15px/1.55 var(--sans);
  -webkit-font-smoothing: antialiased;
  background: var(--bg);
  color: var(--ink);
}
body:not(.gate) {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
}
body:not(.gate) > main { flex: 1 0 auto; }
::selection { background: var(--accent-soft); }

.skip {
  position: absolute; left: -999px; top: 0;
}
.skip:focus {
  left: 8px; top: 8px; z-index: 10;
  padding: 8px 14px; background: var(--surface);
  border: 1px solid var(--line-strong); border-radius: var(--radius-sm);
}
.visually-hidden {
  position: absolute; width: 1px; height: 1px;
  clip-path: inset(50%); overflow: hidden; white-space: nowrap;
}
.mono { font-family: var(--mono); }
.muted { color: var(--muted); }
.tabular { font-variant-numeric: tabular-nums; }

a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
a:focus-visible, button:focus-visible, input:focus-visible, .dropzone:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

/* ---- brand ---- */
.crate {
  display: inline-block;
  width: 20px; height: 16px;
  border: 2px solid var(--brand);
  border-radius: 3px;
  position: relative;
  flex: none;
}
.crate::before {
  content: "";
  position: absolute; left: -2px; right: -2px; top: 3px;
  border-top: 2px solid var(--brand);
}
.crate::after {
  content: "";
  position: absolute; top: 5px; bottom: -2px; left: 50%;
  border-left: 2px solid var(--brand);
  transform: translateX(-50%);
}
/* ---- app bar ---- */
.appbar {
  position: sticky;
  top: 0;
  z-index: 20;
  background: var(--surface);
  border-bottom: 1px solid var(--line);
}
.appbar-in {
  max-width: 72rem; margin: 0 auto; padding: 0 24px;
  min-height: 56px;
  display: flex; align-items: center; justify-content: space-between; gap: 16px;
}
.appbar-start { display: flex; align-items: center; gap: 20px; min-width: 0; }
.appnav { display: flex; align-items: center; gap: 4px; }
.appnav a {
  padding: 6px 12px;
  font: 600 13px/1 var(--sans);
  color: var(--muted);
  border-radius: var(--radius-sm);
  text-decoration: none;
}
.appnav a:hover { background: var(--surface-2); color: var(--ink); text-decoration: none; }
.appnav a[aria-current="page"] { color: var(--ink); background: var(--surface-2); }
.brand {
  display: flex; align-items: center; gap: 10px;
  color: inherit; text-decoration: none;
}
.brand:hover { text-decoration: none; opacity: 0.92; }
.wordmark {
  font: 650 17px/1 var(--sans);
  letter-spacing: 0.01em;
}
.brand-sub {
  font: 500 12px/1 var(--mono);
  letter-spacing: 0.08em; text-transform: uppercase;
  color: var(--faint);
  padding-left: 10px;
  border-left: 1px solid var(--line-strong);
}
.appbar-side { display: flex; align-items: center; gap: 12px; }
.who { color: var(--muted); font-size: 14px; white-space: nowrap; }
.role {
  display: inline-block;
  font: 600 11px/1 var(--mono);
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--muted);
  border: 1px solid var(--line-strong);
  border-radius: 999px;
  padding: 3px 8px;
  margin-left: 4px;
  vertical-align: 1px;
}
.appbar form { margin: 0; }

/* ---- layout ---- */
.wrap { max-width: 72rem; margin: 0 auto; padding: 28px 24px 32px; width: 100%; }
.page-head {
  display: flex; align-items: flex-end; justify-content: space-between;
  gap: 16px; margin-bottom: 24px;
}
.page-head h1 {
  margin: 0;
  font-size: 22px; font-weight: 650; letter-spacing: -0.01em;
}
.page-head .sub { margin: 4px 0 0; color: var(--muted); font-size: 14px; }
.page-actions { display: inline-flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.export-group { display: inline-flex; align-items: center; gap: 6px; }
.field-inline { display: inline-flex; align-items: center; }
.field-inline input,
.field-inline select {
  padding: 6px 10px;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--ink);
  font: inherit;
}
.field-inline input:focus,
.field-inline select:focus { border-color: var(--accent); outline: 2px solid var(--accent-soft); }
.field-inline input { width: 11rem; max-width: 100%; }

.card {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  margin-bottom: 20px;
}
.summary {
  display: flex;
  flex-wrap: wrap;
  align-items: stretch;
  padding: 0;
  overflow: hidden;
}
.summary-cell {
  flex: 1 1 8rem;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
  padding: 16px 20px;
  border-right: 1px solid var(--line);
  min-width: 0;
}
.summary-cell:last-child { border-right: 0; }
.summary-num {
  display: block;
  font: 650 22px/1.2 var(--sans);
  color: var(--ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.summary-lbl {
  display: block;
  font: 600 11px/1 var(--mono);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--muted);
}
.summary-topo {
  align-items: center;
  flex-direction: row;
  gap: 10px;
}
.summary-topo .pill { margin: 0; }
@media (max-width: 720px) {
  .summary-cell { flex-basis: 50%; border-bottom: 1px solid var(--line); }
  .summary-cell:nth-child(2n) { border-right: 0; }
  .summary-cell:nth-last-child(-n+2) { border-bottom: 0; }
}
.upload-cta { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.upload-cta h2 { margin: 0; font: 600 16px/1.3 var(--sans); }
.upload-cta .hint { margin: 4px 0 0; color: var(--muted); font-size: 14px; }
.upload-cta code { font-family: var(--mono); background: var(--surface-2); padding: 1px 6px; border-radius: 4px; }
@media (max-width: 720px) {
  .upload-cta { flex-direction: column; align-items: stretch; }
  .upload-cta .btn { width: 100%; }
}
.pin-list { list-style: none; margin: 0; padding: 0; }
.pin { display: flex; align-items: center; gap: 12px; padding: 10px 24px; border-top: 1px solid var(--line); }
.pin:first-child { border-top: 0; }
.pin-key { flex: 1 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; color: var(--ink); }
.pin-size { font-size: 12px; white-space: nowrap; }
.pin form { margin: 0; }
.filter-bar { background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px 16px; margin-bottom: 16px; }
.filter-row { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 14px; }
.filter-chips { display: flex; flex-wrap: wrap; gap: 6px; flex: 1 1 auto; min-width: 0; }
.chip { display: inline-flex; align-items: center; gap: 6px; padding: 5px 11px; border: 1px solid var(--line-strong); border-radius: 999px; font: 500 13px/1 var(--sans); color: var(--ink); text-decoration: none; background: var(--surface); }
.chip:hover { background: var(--surface-2); text-decoration: none; }
.chip.is-active { background: var(--accent); border-color: var(--accent); color: var(--accent-ink); }
.chip-count { font: 600 11px/1 var(--mono); color: var(--muted); }
.chip.is-active .chip-count { color: var(--accent-ink); opacity: 0.85; }
.chip-sm { padding: 3px 9px; font-size: 12px; }
.filter-search { flex: 0 1 240px; min-width: 0; }
.filter-search input { width: 100%; padding: 6px 10px; border: 1px solid var(--line-strong); border-radius: var(--radius-sm); background: var(--surface); color: var(--ink); font: inherit; }
.filter-window { display: inline-flex; gap: 4px; flex-wrap: wrap; }
.filter-apply { margin-left: auto; }
@media (max-width: 720px) {
  .filter-row { flex-direction: column; align-items: stretch; }
  .filter-search, .filter-apply { width: 100%; margin-left: 0; }
}
.card-head {
  display: flex; align-items: baseline; justify-content: space-between; gap: 12px;
  padding: 18px 24px 14px;
  border-bottom: 1px solid var(--line);
}
.card-head h2 {
  margin: 0;
  font: 600 13px/1.4 var(--sans);
  letter-spacing: 0.04em; text-transform: uppercase;
  color: var(--muted);
}
.card-head .hint { margin: 0; color: var(--faint); font-size: 13px; }
.card-body { padding: 20px 24px 24px; }
.card-stack { display: flex; flex-direction: column; gap: 20px; }
.card-stack .stack-form { margin: 0; }
.key-reveal { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; }
.key-reveal .key-once { margin: 0; width: 100%; }

/* ---- buttons ---- */
.btn {
  display: inline-flex; align-items: center; justify-content: center; gap: 8px;
  padding: 9px 16px;
  min-height: 38px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: var(--accent);
  color: var(--accent-ink);
  font: 600 14px/1 var(--sans);
  cursor: pointer;
  text-decoration: none;
  white-space: nowrap;
}
.btn:hover { background: var(--accent-hover); text-decoration: none; }
.btn:active { transform: translateY(0.5px); }
.btn-quiet {
  background: var(--surface);
  color: var(--ink);
  border-color: var(--line-strong);
}
.btn-quiet:hover { background: var(--surface-2); }
.btn-danger-quiet {
  background: transparent;
  color: var(--err);
  border-color: var(--line-strong);
}
.btn-danger-quiet:hover { background: var(--err-soft); border-color: var(--err); }
.btn-danger { background: var(--err); color: #fff; }
.btn-danger:hover { background: var(--err); filter: brightness(0.92); }
.btn-sm { min-height: 30px; padding: 6px 12px; font-size: 13px; }
.btn-icon {
  display: inline-flex; align-items: center; justify-content: center;
  width: 34px; min-width: 34px; padding: 0;
}
.btn-icon svg { width: 16px; height: 16px; display: block; }
.btn-block { width: 100%; }
.theme-toggle {
  display: inline-flex; align-items: center; justify-content: center;
  width: 38px; min-width: 38px; padding: 0;
}
.theme-toggle svg { width: 18px; height: 18px; display: block; }
.theme-toggle .is-hidden { display: none; }

/* ---- forms ---- */
.field { display: block; margin: 0 0 16px; }
.field > span:first-child {
  display: block; margin-bottom: 6px;
  font: 550 13px/1.3 var(--sans);
  color: var(--muted);
}
.field-hint {
  display: block;
  margin: 6px 0 0;
  font: 400 12px/1.45 var(--sans);
  color: var(--faint);
}
.field input {
  display: block; width: 100%;
  padding: 9px 12px;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--ink);
  font: inherit;
}
.field input:focus { border-color: var(--accent); outline: 2px solid var(--accent-soft); }
.field input:read-only {
  background: var(--surface-2);
  color: var(--muted);
  cursor: default;
}
.field select {
  display: block; width: 100%;
  padding: 9px 12px;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--ink);
  font: inherit;
}
.stack-form { max-width: 28rem; }
.stack-form .btn { margin-top: 4px; }
.inline-form { display: inline-flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.inline-form input {
  width: 11rem;
  max-width: 100%;
  padding: 6px 8px;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--ink);
  font: inherit;
}
.key-once {
  margin: 0 0 16px; padding: 12px 14px;
  border: 1px solid var(--line-strong); border-radius: var(--radius-sm);
  background: var(--surface-2); font-family: var(--mono); font-size: 13px;
  word-break: break-all;
}
.input-group {
  display: flex; align-items: stretch;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius-sm);
  background: var(--surface);
  overflow: hidden;
}
.input-group:focus-within {
  border-color: var(--accent);
  outline: 2px solid var(--accent-soft);
}
.input-group input {
  flex: 1 1 auto; min-width: 0;
  border: 0; border-radius: 0;
  background: transparent;
  padding: 9px 12px;
  font: inherit; color: var(--ink);
}
.input-group input:focus { outline: none; box-shadow: none; border-color: transparent; }
.pw-toggle {
  flex: none;
  display: inline-flex; align-items: center; justify-content: center;
  width: 42px; padding: 0; margin: 0;
  border: 0; border-left: 1px solid var(--line-strong);
  background: var(--surface-2);
  color: var(--muted);
  cursor: pointer;
}
.pw-toggle:hover { background: var(--surface); color: var(--ink); }
.pw-toggle:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
.pw-toggle svg { width: 18px; height: 18px; display: block; }
.pw-toggle .is-hidden { display: none; }

/* ---- theme + footer ---- */
.gate-tools {
  position: fixed; top: 16px; right: 16px; z-index: 5;
}
.app-foot {
  flex-shrink: 0;
  width: 100%;
  margin-top: auto;
  padding: 14px 24px 18px;
  border-top: 1px solid var(--line);
  background: var(--bg);
  text-align: center;
}
.app-foot p {
  max-width: 72rem;
  margin: 0 auto;
  font: 500 12px/1 var(--mono);
  letter-spacing: 0.06em;
  color: var(--faint);
}
.app-foot a {
  color: var(--muted);
  text-decoration: none;
}
.app-foot a:hover { color: var(--accent); text-decoration: underline; }

/* ---- upload ---- */
.upload { display: flex; gap: 12px; align-items: stretch; flex-wrap: wrap; padding: 16px 20px 20px; }
.upload-inline { display: flex; flex-direction: column; gap: 12px; padding: 16px 20px 20px; }
.upload-cta-head { display: flex; flex-direction: column; gap: 4px; }
.upload-meta { font: 500 12.5px/1.3 var(--mono); color: var(--muted); }
.upload-progress { width: 100%; height: 8px; appearance: none; border: none; border-radius: 999px; background: var(--surface-2); overflow: hidden; }
.upload-progress::-webkit-progress-bar { background: var(--surface-2); border-radius: 999px; }
.upload-progress::-webkit-progress-value { background: var(--accent); transition: width 120ms linear; }
.upload-progress::-moz-progress-bar { background: var(--accent); }
.upload-status { font-size: 13.5px; padding: 8px 12px; border-radius: var(--radius-sm); }
.upload-status.ok { color: var(--ok); background: var(--ok-soft); }
.upload-status.transit { color: var(--warn); background: var(--warn-soft); }
.upload-status.err { color: var(--err); background: var(--err-soft); }
.upload-actions { display: flex; gap: 10px; }
.dropzone {
  flex: 1 1 320px;
  display: flex; align-items: center; justify-content: center;
  min-height: 44px;
  padding: 10px 16px;
  border: 1.5px dashed var(--line-strong);
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  color: var(--muted);
  font-size: 14px;
  cursor: pointer;
  transition: border-color 120ms ease, background 120ms ease;
}
.dropzone:hover { border-color: var(--accent); }
.dropzone.drag {
  border-color: var(--accent);
  background: var(--accent-soft);
  color: var(--accent);
}
.dropzone input[type=file] {
  position: absolute; width: 1px; height: 1px;
  clip-path: inset(50%); overflow: hidden; white-space: nowrap;
}
.dz-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 100%; }
.dz-text.has-file { color: var(--ink); font-family: var(--mono); font-size: 13px; }

/* ---- data grid ---- */
.table-wrap { overflow-x: auto; }
table.grid {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}
.grid th {
  padding: 10px 16px;
  text-align: left;
  font: 600 11.5px/1.3 var(--mono);
  letter-spacing: 0.07em; text-transform: uppercase;
  color: var(--faint);
  border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.grid th.sortable a {
  display: inline-flex; align-items: center; gap: 4px;
  color: inherit; text-decoration: none;
}
.grid th.sortable a:hover { color: var(--ink); }
.grid th.sortable.is-active a { color: var(--ink); }
.sort-ind { font-size: 10px; opacity: 0.45; line-height: 1; }
.grid th.sortable.is-active .sort-ind { opacity: 1; color: var(--accent); }
.grid td {
  padding: 11px 16px;
  border-bottom: 1px solid var(--line);
  vertical-align: middle;
}
.grid tbody tr:last-child td { border-bottom: 0; }
.grid tbody tr { transition: background 80ms ease; }
.grid tbody tr:hover { background: var(--surface-2); }
.grid .num { text-align: right; white-space: nowrap; }
.grid td.key { font-family: var(--mono); font-size: 13px; word-break: break-all; max-width: 26rem; }
.grid td.actions {
  display: flex; justify-content: flex-end; align-items: center; gap: 6px;
  text-align: right; white-space: nowrap;
}
.grid td.actions form { display: inline; margin: 0; }
.grid td.actions .btn { margin-left: 0; }

.pager {
  display: flex; align-items: center; justify-content: space-between; gap: 12px;
  padding: 12px 20px 16px;
  border-top: 1px solid var(--line);
}
.pager > span { min-width: 5.5rem; }
.pager-center { flex: 1; display: flex; flex-direction: column; align-items: center; gap: 6px; min-width: 0; }
.pager-meta { font-size: 13px; margin: 0; text-align: center; }
.pager-size {
  display: flex; align-items: center; gap: 6px; margin: 0;
  font-size: 13px; color: var(--muted);
}
.pager-size select {
  font: 600 13px/1 var(--sans);
  padding: 4px 8px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--ink);
  cursor: pointer;
}

.pill {
  display: inline-block;
  font: 600 11px/1 var(--mono);
  letter-spacing: 0.05em; text-transform: uppercase;
  padding: 4px 8px;
  border-radius: 999px;
  border: 1px solid var(--line-strong);
  color: var(--muted);
  background: var(--surface-2);
}
.pill-http { color: var(--accent); border-color: var(--accent); background: var(--accent-soft); }
.pill-sftp { color: #0e7490; border-color: #0e7490; background: #cffafe; }
.pill-s3 { color: var(--muted); }
.pill-local { color: var(--ok); border-color: var(--ok); background: var(--ok-soft); }
.pill-transit { color: var(--warn); border-color: var(--warn); background: var(--warn-soft); }

/* ---- archive cards (narrow viewports) ---- */
.archive-cards { display: none; }
.archive-card .card-title { font: 13px/1.4 var(--mono); word-break: break-all; }
.archive-card .card-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; font-size: 13px; }
.archive-card .card-actions { display: flex; gap: 8px; margin-top: 2px; }
.archive-card .card-actions form { margin: 0; }
.archive-card .card-actions .btn { flex: 0 0 auto; }
.archive-card .card-actions a.btn { flex: 1 1 auto; text-align: center; }
@media (max-width: 719px) {
  .table-wrap { display: none; }
  .archive-cards {
    display: flex;
    flex-direction: column;
    gap: 12px;
    list-style: none;
    margin: 0;
    padding: 16px;
  }
  .archive-card {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 14px 16px;
    border: 1px solid var(--line);
    border-radius: var(--radius);
    background: var(--surface);
  }
}

/* ---- notices ---- */
.notice {
  display: flex; align-items: center; gap: 10px;
  margin-bottom: 20px;
  padding: 11px 16px;
  border-radius: var(--radius-sm);
  border: 1px solid;
  font-size: 14px;
}
.notice::before {
  content: "";
  width: 8px; height: 8px; border-radius: 50%;
  flex: none;
}
.notice-ok { background: var(--ok-soft); border-color: var(--ok); color: var(--ok); }
.notice-ok::before { background: var(--ok); }
.notice-warn { background: var(--warn-soft); border-color: var(--warn); color: var(--warn); font-weight: 600; font-size: 15px; }
.notice-warn::before { background: var(--warn); }
.notice-err { background: var(--err-soft); border-color: var(--err); color: var(--err); }
.notice-err::before { background: var(--err); }
.card-stack .notice { margin-bottom: 0; }

/* ---- empty states ---- */
.empty { padding: 36px 20px; text-align: center; }
.empty-title { margin: 0 0 4px; font-weight: 600; }
.empty-sub { margin: 0; color: var(--muted); font-size: 14px; }
.empty-clear { font-weight: 600; }

/* ---- login gate ---- */
html:has(body.gate) { background: #07090c; }
body.gate {
  --bg: transparent;
  --surface: rgb(19 26 33 / 0.78);
  --surface-2: rgb(15 21 28 / 0.9);
  --ink: #e6e9ed;
  --muted: #c5ccd4;
  --faint: #9aa3ad;
  --line: rgb(255 255 255 / 0.12);
  --line-strong: rgb(255 255 255 / 0.18);
  --shadow: 0 16px 48px rgb(0 0 0 / 0.45);
  min-height: 100vh;
  min-height: 100dvh;
  display: grid;
  place-items: center;
  color: var(--ink);
  background-color: #07090c;
  background-image: url("/static/login-hero.jpg");
  background-size: cover;
  background-position: center 42%;
  background-repeat: no-repeat;
}
body.gate::before {
  content: "";
  position: fixed;
  inset: 0;
  background: linear-gradient(180deg, rgb(7 9 12 / 0.42), rgb(7 9 12 / 0.58));
  pointer-events: none;
  z-index: 0;
}
.gate-wrap {
  position: relative;
  z-index: 1;
  width: min(24rem, calc(100% - 32px));
  padding: 32px 0;
}
.gate-tools { z-index: 5; }
body.gate .theme-toggle {
  color: #e6e9ed;
  background: rgb(19 26 33 / 0.55);
  border-color: rgb(255 255 255 / 0.16);
}
.gate-card {
  padding: 24px;
  margin-bottom: 0;
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}
.gate-card form { margin: 0; }
html:has(body.gate.gate-simple) { background: #fff; }
body.gate.gate-simple {
  --bg: #fff;
  --surface: #fff;
  --surface-2: #f4f6f8;
  --ink: #1a1f24;
  --muted: #5c6670;
  --faint: #8b949e;
  --line: #e4e7eb;
  --line-strong: #cfd4da;
  --shadow: 0 8px 24px rgb(16 24 40 / 0.08);
  background: #fff;
  background-image: none;
}
body.gate.gate-simple::before { display: none; }
body.gate.gate-simple .theme-toggle {
  color: var(--ink);
  background: var(--surface);
  border-color: var(--line-strong);
}
body.gate.gate-simple .gate-card {
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}
.alert {
  margin: 0 0 16px;
  padding: 10px 14px;
  border: 1px solid var(--err);
  border-left-width: 3px;
  border-radius: var(--radius-sm);
  background: var(--err-soft);
  color: var(--err);
  font-size: 14px;
}

/* ---- confirm dialog ---- */
dialog#confirm-dialog {
  border: 1px solid var(--line);
  border-radius: var(--radius);
  background: var(--surface);
  color: var(--ink);
  padding: 0;
  width: min(26rem, calc(100vw - 48px));
  box-shadow: 0 12px 32px rgb(16 24 40 / 0.18);
}
dialog#confirm-dialog::backdrop { background: rgb(16 24 40 / 0.45); }
.dialog-card { padding: 20px; margin: 0; }
.dialog-title { margin: 0 0 6px; font-size: 16px; font-weight: 650; }
.dialog-text { margin: 0 0 18px; color: var(--muted); font-size: 14px; overflow-wrap: anywhere; }
.dialog-actions { display: flex; justify-content: flex-end; gap: 10px; }
.dialog-actions .btn { margin: 0; }

@media (max-width: 720px) {
  .wrap { padding: 20px 16px 24px; }
  .app-foot { padding: 12px 16px 16px; }
  .appbar-in { padding: 0 16px; }
  .brand-sub { display: none; }
  .appnav { display: none; }
  .page-head { flex-direction: column; align-items: flex-start; }
  .grid td.key { max-width: 12rem; }
}
@media (prefers-reduced-motion: reduce) {
  * { transition: none !important; }
}
`

func humanSize(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%d B", n)
	case n < k*k:
		return fmt.Sprintf("%.1f KiB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1f MiB", float64(n)/(k*k))
	default:
		return fmt.Sprintf("%.1f GiB", float64(n)/(k*k*k))
	}
}

// statsLine summarizes the archive inventory for the page header.
func statsLine(count int, total int64) string {
	noun := "archives"
	if count == 1 {
		noun = "archive"
	}
	return fmt.Sprintf("%d %s · %s total", count, noun, humanSize(total))
}

// noticeFromQuery maps flash-notice query params to (kind, text). Only fixed
// notice tokens are accepted; optional name= is basename-validated for uploads.
func noticeFromQuery(q url.Values) (kind, text string) {
	switch q.Get("notice") {
	case "uploaded":
		if raw := strings.TrimSpace(q.Get("name")); raw != "" {
			return "ok", fmt.Sprintf("Capture %s uploaded. You can send another.", store.SanitizeArchiveKey(raw))
		}
		return "ok", "Capture uploaded. You can send another."
	case "duplicate":
		if raw := strings.TrimSpace(q.Get("name")); raw != "" {
			return "err", fmt.Sprintf("Capture %s is already uploaded (same content). Check Captures or pick another file.", store.SanitizeArchiveKey(raw))
		}
		return "err", "This file is already uploaded (same content). Check Captures or pick another file."
	default:
		return noticeCopy(q.Get("notice"))
	}
}

// noticeCopy maps a flash-notice query token to (kind, text). Unknown tokens
// render nothing, so no user input is ever reflected into the page.
func noticeCopy(token string) (kind, text string) {
	switch token {
	case "uploaded":
		return "ok", "Capture uploaded. You can send another."
	case "deleted":
		return "ok", "Capture deleted."
	case "upload_error":
		return "err", "Upload failed. Check the file and try again."
	case "too_large":
		return "err", "Upload failed: the file exceeds the size limit."
	case "duplicate":
		return "err", "This file is already uploaded (same content). Check Captures or pick another file."
	default:
		return "", ""
	}
}

func loginErrorCopy(code string) string {
	switch code {
	case "unauthorized":
		return "Incorrect username or password."
	case "bad_request":
		return "Enter your username and password."
	case "not_ready":
		return "gfs is not ready. Try again in a moment."
	case "rate_limited":
		return "Too many sign-in attempts. Try again later."
	case "":
		return ""
	default:
		return "Sign-in failed. Try again."
	}
}

func displayVersion(v string) string {
	if strings.TrimSpace(v) == "" {
		return "dev"
	}
	return strings.TrimSpace(v)
}

// themeHeadScript applies saved or system theme before first paint.
const themeHeadScript = `<script>(function(){try{var k='gfs-theme',t=localStorage.getItem(k);if(t==='dark'||t==='light'){document.documentElement.setAttribute('data-theme',t);return;}}catch(e){}if(window.matchMedia&&window.matchMedia('(prefers-color-scheme: dark)').matches){document.documentElement.setAttribute('data-theme','dark');}})();</script>`

const themeToggleScript = `<script>(function(){var b=document.getElementById('theme-toggle');if(!b)return;var sun=b.querySelector('.icon-sun');var moon=b.querySelector('.icon-moon');function cur(){var t=document.documentElement.getAttribute('data-theme');if(t==='dark'||t==='light')return t;return window.matchMedia&&window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}function sync(){var dark=cur()==='dark';if(sun)sun.classList.toggle('is-hidden',!dark);if(moon)moon.classList.toggle('is-hidden',dark);b.setAttribute('aria-label',dark?'Switch to light mode':'Switch to dark mode');}sync();b.addEventListener('click',function(){var n=cur()==='dark'?'light':'dark';document.documentElement.setAttribute('data-theme',n);try{localStorage.setItem('gfs-theme',n);}catch(e){}sync();});})();</script>`

const themeToggleHTML = `<button type="button" class="theme-toggle btn btn-quiet btn-sm" id="theme-toggle" aria-label="Switch to dark mode"><svg class="icon-moon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M21 12.79A9 9 0 1111.21 3 7 7 0 0021 12.79z"/></svg><svg class="icon-sun is-hidden" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"/></svg></button>`

const passwordToggleScript = `<script>(function(){var i=document.getElementById('login-password');var b=document.getElementById('pw-toggle');if(!i||!b)return;var showIcon=b.querySelector('.icon-show');var hideIcon=b.querySelector('.icon-hide');b.addEventListener('click',function(){var vis=i.type==='password';i.type=vis?'text':'password';b.setAttribute('aria-label',vis?'Hide password':'Show password');if(showIcon)showIcon.classList.toggle('is-hidden',vis);if(hideIcon)hideIcon.classList.toggle('is-hidden',!vis);});})();</script>`

func faviconHeadHTML(version string) template.HTML {
	v := template.HTMLEscapeString(displayVersion(version))
	return template.HTML(
		`<meta name="theme-color" content="#f79009">` +
			`<link rel="icon" type="image/x-icon" href="/static/favicon.ico?v=` + v + `">` +
			`<link rel="icon" type="image/png" sizes="16x16" href="/static/favicon-16x16.png?v=` + v + `">` +
			`<link rel="icon" type="image/png" sizes="32x32" href="/static/favicon-32x32.png?v=` + v + `">` +
			`<link rel="icon" href="/static/favicon.svg?v=` + v + `" type="image/svg+xml">` +
			`<link rel="apple-touch-icon" sizes="180x180" href="/static/apple-touch-icon.png?v=` + v + `">` +
			`<link rel="manifest" href="/static/manifest.json?v=` + v + `">`,
	)
}

func appFootHTML(version string) template.HTML {
	v := displayVersion(version)
	return template.HTML(
		`<footer class="app-foot"><p>gfs v` + template.HTMLEscapeString(v) +
			` · <a href="https://github.com/hrodrig/groot" rel="noopener noreferrer">groot</a>` +
			` · <a href="https://github.com/hrodrig/groot-share" rel="noopener noreferrer">groot-share</a></p></footer>`,
	)
}

func pageShellData(version string) map[string]any {
	return map[string]any{
		"CSS":               template.CSS(layoutCSS),
		"FaviconHead":       faviconHeadHTML(version),
		"ThemeHead":         template.HTML(themeHeadScript),
		"ThemeToggle":       template.HTML(themeToggleHTML),
		"ThemeToggleScript": template.HTML(themeToggleScript),
		"Version":           displayVersion(version),
		"PagerSizes":        HTMLPageSizes,
		"AppFoot":           appFootHTML(version),
		"BrandSub":          config.DefaultBrandSub,
		"LoginTitle":        "gfs — Sign in",
		"GateClass":         "gate",
	}
}

func (s *Server) pageShell() map[string]any {
	data := pageShellData(s.Version)
	data["BrandSub"] = config.DisplayBrandSub(s.Cfg.BrandSub)
	data["AppFoot"] = s.footerHTML()
	if s.Cfg.LoginSimple {
		data["LoginTitle"] = "Sign in"
		data["GateClass"] = "gate gate-simple"
		data["FaviconHead"] = template.HTML("")
	}
	return data
}

func (s *Server) footerHTML() template.HTML {
	if strings.TrimSpace(s.Cfg.Footer) == "-" {
		return ""
	}
	if text := config.DisplayFooter(s.Cfg.Footer); text != "" {
		return template.HTML(`<footer class="app-foot"><p>` + template.HTMLEscapeString(text) + `</p></footer>`)
	}
	return appFootHTML(s.Version)
}
