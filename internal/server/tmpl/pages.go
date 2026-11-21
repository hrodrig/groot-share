package tmpl

import (
	"html/template"
)

var pageFuncs = template.FuncMap{
	"humansize": HumanSize,
	"pagerurl":  PagerURL,
	"sortlink":  SortURL,
	"qswith":    func(b FilterURLBuilder, k, v string) template.URL { return b.With(k, v) },
	"qswithout": func(b FilterURLBuilder, k string) template.URL { return b.Without(k) },
}

var Login = template.Must(template.New("login").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.LoginTitle}}</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body class="{{.GateClass}}">
<a class="skip" href="#main">Skip to content</a>
<div class="gate-tools">{{.ThemeToggle}}</div>
<main id="main" class="gate-wrap">
<h1 class="visually-hidden">Sign in</h1>
<div class="card gate-card">
{{if .Notice}}<div class="notice notice-ok" role="status">{{.Notice}}</div>{{end}}
{{if .Error}}<div class="alert" role="alert">{{.Error}}</div>{{end}}
<form method="post" action="/login">
<label class="field"><span>Username</span><input name="username" autocomplete="username" required autofocus></label>
<label class="field field-pw"><span>Password</span><span class="input-group"><input name="password" id="login-password" type="password" autocomplete="current-password" required><button type="button" class="pw-toggle" id="pw-toggle" aria-label="Show password" aria-controls="login-password"><svg class="icon-show" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/></svg><svg class="icon-hide is-hidden" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-10-8-10-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 10 8 10 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg></button></span></label>
<button class="btn btn-block" type="submit">Sign in</button>
</form>
</div>
</main>
{{.ThemeToggleScript}}
` + PasswordToggleScript + `
</body></html>
`))

var Home = template.Must(template.New("home").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Captures</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
` + appNavTmpl + `
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
` + appWhoTmpl + `
      <form method="post" action="/logout"><button class="btn btn-quiet btn-sm" type="submit">Sign out</button></form>
    </div>
  </div>
</header>
<main id="main" class="wrap">
{{if .NoticeText}}<div class="notice notice-{{.NoticeKind}}" role="status">{{.NoticeText}}</div>{{end}}
<section class="card summary" aria-label="Inventory summary">
  <div class="summary-cell">
    <span class="summary-num tabular">{{.Summary.Count}}</span>
    <span class="summary-lbl">captures</span>
  </div>
  <div class="summary-cell">
    <span class="summary-num tabular">{{humansize .Summary.Bytes}}</span>
    <span class="summary-lbl">on disk</span>
  </div>
  <div class="summary-cell">
    <span class="summary-num tabular">{{.Summary.ClusterCount}}</span>
    <span class="summary-lbl">clusters</span>
  </div>
  <div class="summary-cell">
    <span class="summary-num tabular">{{.Summary.IncompleteCount}}</span>
    <span class="summary-lbl">in transit</span>
  </div>
  <div class="summary-cell summary-topo">
    <span class="pill{{if eq .Summary.StorageTopology "vps"}} pill-local{{else if eq .Summary.StorageTopology "vps-s3"}} pill-s3{{end}}">{{.Summary.StorageTopology}}</span>
    <span class="summary-lbl">topology</span>
  </div>
</section>
{{if .CanUpload}}
<section class="card upload-cta" aria-labelledby="up-cta-h">
  <div class="upload-cta-head">
    <h2 id="up-cta-h">Upload archive</h2>
    <p class="hint">Drop or pick a groot <code>.tar.gz</code>. Up to <span class="mono">{{humansize .MaxUpload}}</span> per file.</p>
  </div>
  <form class="upload-inline" id="upload-inline" method="post" action="/v1/archives" enctype="multipart/form-data" novalidate data-max-upload="{{.MaxUpload}}">
    <label class="dropzone" id="inline-dropzone">
      <input type="file" name="file" id="inline-file" accept=".tar.gz,.tgz,application/gzip" required>
      <span class="dz-text" id="inline-dz-text">Choose a file or drop it here</span>
    </label>
    <div class="upload-meta" id="inline-meta" hidden></div>
    <progress class="upload-progress" id="inline-progress" max="1" value="0" aria-hidden="true" hidden></progress>
    <div class="upload-status" id="inline-status" role="status" hidden></div>
    <div class="upload-actions" id="inline-actions">
      <button class="btn" type="submit" id="inline-send" disabled>Upload capture</button>
      <button class="btn btn-quiet" type="button" id="inline-cancel" hidden>Cancel</button>
    </div>
  </form>
</section>
{{end}}
{{if .Pins}}
<section class="card pin-strip" aria-label="Pinned captures">
  <div class="card-head">
    <h2>Pinned</h2>
    <span class="hint">Your quick-access captures</span>
  </div>
  <ul class="pin-list">
    {{range .Pins}}
    <li class="pin">
      <a class="pin-key mono" href="/v1/archives/{{.ArchiveID}}/file" title="Download {{.ArchiveKey}}">{{.ArchiveKey}}</a>
      <span class="pin-size muted tabular">{{humansize .Size}}</span>
      <form method="post" action="/v1/pin/archives/{{.ArchiveID}}/delete" data-confirm="Unpin {{.ArchiveKey}}?">
        <button class="btn btn-quiet btn-sm" type="submit" title="Unpin" aria-label="Unpin {{.ArchiveKey}}">Unpin</button>
      </form>
    </li>
    {{end}}
  </ul>
</section>
{{end}}
{{if gt .Summary.Count 0}}
<form class="filter-bar" method="get" action="/" id="captures-filter">
  <div class="filter-row">
    <div class="filter-chips" role="group" aria-label="Clusters">
      <a class="chip{{if not .Filter.Cluster}} is-active{{end}}" href="/?{{qswithout .FilterURL "cluster"}}">All <span class="chip-count">{{.Summary.Count}}</span></a>
      {{range .ClusterChips}}
      <a class="chip{{if eq .Slug $.Filter.Cluster}} is-active{{end}}" href="/?{{qswith $.FilterURL "cluster" .Slug}}">{{.Slug}} <span class="chip-count">{{.Count}}</span></a>
      {{end}}
    </div>
    <label class="filter-search">
      <span class="visually-hidden">Search</span>
      <input type="search" name="q" value="{{.Filter.Query}}" placeholder="filename, since, message" maxlength="80">
    </label>
    <div class="filter-window" role="group" aria-label="Time window">
      <a class="chip chip-sm{{if eq .Filter.Window ""}} is-active{{end}}" href="/?{{qswithout .FilterURL "window"}}">All time</a>
      <a class="chip chip-sm{{if eq .Filter.Window "24h"}} is-active{{end}}" href="/?{{qswith .FilterURL "window" "24h"}}">24h</a>
      <a class="chip chip-sm{{if eq .Filter.Window "7d"}} is-active{{end}}" href="/?{{qswith .FilterURL "window" "7d"}}">7d</a>
      <a class="chip chip-sm{{if eq .Filter.Window "30d"}} is-active{{end}}" href="/?{{qswith .FilterURL "window" "30d"}}">30d</a>
    </div>
    <button class="btn btn-quiet btn-sm filter-apply" type="submit">Apply</button>
  </div>
</form>
{{end}}
<div class="page-head">
  <div>
    <h1>Captures</h1>
    <p class="sub">{{.StatsLine}}</p>
  </div>
</div>
<section class="card" aria-labelledby="ar-h">
  <div class="card-head"><h2 id="ar-h">Archives</h2></div>
  {{if .Items}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col" class="sortable{{if eq $.Pager.SortField "key"}} is-active{{end}}"><a href="{{sortlink $.Pager "key"}}"{{if eq $.Pager.SortField "key"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Name<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "key"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col" class="sortable{{if eq $.Pager.SortField "source"}} is-active{{end}}"><a href="{{sortlink $.Pager "source"}}"{{if eq $.Pager.SortField "source"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Source<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "source"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col" class="sortable{{if eq $.Pager.SortField "storage"}} is-active{{end}}"><a href="{{sortlink $.Pager "storage"}}"{{if eq $.Pager.SortField "storage"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Storage<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "storage"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col" class="sortable num{{if eq $.Pager.SortField "size"}} is-active{{end}}"><a href="{{sortlink $.Pager "size"}}"{{if eq $.Pager.SortField "size"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Size<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "size"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col" class="sortable{{if eq $.Pager.SortField "uploaded"}} is-active{{end}}"><a href="{{sortlink $.Pager "uploaded"}}"{{if eq $.Pager.SortField "uploaded"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Uploaded (UTC)<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "uploaded"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col"><span class="visually-hidden">Actions</span></th>
    </tr></thead>
    <tbody>
    {{range .Items}}
    <tr>
      <td class="key">{{.Key}}{{with index $.Completeness .ID}} <span class="completeness-badge tone-{{.Tone}}">{{.Label}}</span>{{end}}</td>
      <td><span class="pill pill-{{.Source}}">{{.Source}}</span></td>
      <td>{{if .Storage}}<span class="pill pill-{{.Storage}}">{{.Storage}}</span>{{end}}</td>
      <td class="num tabular">{{humansize .Size}}</td>
      <td class="muted tabular">{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}</td>
      <td class="actions">
        <a class="btn btn-quiet btn-sm btn-icon" href="/v1/archives/{{.ID}}/file" title="Download" aria-label="Download {{.Key}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 3v10"/><path d="M8 11l4 4 4-4"/><path d="M4 20h16"/></svg></a>
        <button class="btn btn-quiet btn-sm btn-icon copy-link" type="button" data-copy-url="{{$.BaseURL}}/v1/archives/{{.ID}}/file" title="Copy download link" aria-label="Copy download link for {{.Key}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"/></svg></button>
        {{if $.CanShares}}
        <a class="btn btn-quiet btn-sm btn-icon" href="/shares/{{.ID}}" title="Share" aria-label="Share {{.Key}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg></a>
        {{end}}
        {{if $.CanDelete}}
        <form method="post" action="/v1/archives/delete/{{.ID}}" data-confirm="Delete {{.Key}}? This cannot be undone." data-confirm-require="{{.Key}}">
          <button class="btn btn-danger-quiet btn-sm btn-icon" type="submit" title="Delete" aria-label="Delete {{.Key}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M3 6h18"/><path d="M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/></svg></button>
        </form>
        {{end}}
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  <ul class="archive-cards">
    {{range .Items}}
    <li class="archive-card">
      <div class="card-title mono" title="{{.Key}}">{{.Key}}{{with index $.Completeness .ID}} <span class="completeness-badge tone-{{.Tone}}">{{.Label}}</span>{{end}}</div>
      <div class="card-meta">
        <span class="pill pill-{{.Source}}">{{.Source}}</span>
        {{if .Storage}}<span class="pill pill-{{.Storage}}">{{.Storage}}</span>{{end}}
        <span class="muted tabular">{{humansize .Size}}</span>
        <span class="muted tabular">{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}</span>
      </div>
      <div class="card-actions">
        <a class="btn" href="/v1/archives/{{.ID}}/file" title="Download {{.Key}}">Download</a>
        <button class="btn btn-quiet copy-link" type="button" data-copy-url="{{$.BaseURL}}/v1/archives/{{.ID}}/file" title="Copy download link" aria-label="Copy download link for {{.Key}}">Copy link</button>
        {{if $.CanShares}}
        <a class="btn btn-quiet" href="/shares/{{.ID}}" title="Share {{.Key}}">Share</a>
        {{end}}
        {{if $.CanDelete}}
        <form method="post" action="/v1/archives/delete/{{.ID}}" data-confirm="Delete {{.Key}}? This cannot be undone." data-confirm-require="{{.Key}}">
          <button class="btn btn-danger-quiet" type="submit" title="Delete" aria-label="Delete {{.Key}}">Delete</button>
        </form>
        {{end}}
      </div>
    </li>
    {{end}}
  </ul>
  {{if gt .Pager.Total 0}}
  <nav class="pager" aria-label="Archives pagination">
    <div class="pager-edge">
      {{if .Pager.HasPrev}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.PrevPage .Pager}}">Previous</a>{{else}}<span></span>{{end}}
    </div>
    <div class="pager-center">
      <p class="pager-meta">Page {{.Pager.Page}} of {{.Pager.TotalPages}} · {{.Pager.Total}} captures</p>
      <div class="pager-tools">
        {{if .Pager.HasPrev}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.FirstPage .Pager}}" title="First page">« First</a>{{else}}<span class="btn btn-quiet btn-sm" aria-disabled="true" title="First page">« First</span>{{end}}
        <div class="pager-pages">
          {{range .Pager.Pages}}
            {{if eq .Kind "page"}}
              {{if eq .Page $.Pager.Page}}<span class="pager-current" aria-current="page">{{.Label}}</span>{{else}}<a class="pager-num" href="{{pagerurl .Page $.Pager}}">{{.Label}}</a>{{end}}
            {{else}}<span class="pager-gap" aria-hidden="true">{{.Label}}</span>{{end}}
          {{end}}
        </div>
        {{if .Pager.HasNext}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.LastPage .Pager}}" title="Last page">Last »</a>{{else}}<span class="btn btn-quiet btn-sm" aria-disabled="true" title="Last page">Last »</span>{{end}}
      </div>
      <form class="pager-jump" method="get" action="">
        {{if .Pager.HiddenSort}}<input type="hidden" name="sort" value="{{.Pager.HiddenSort}}">{{end}}
        {{if .Pager.HiddenOrder}}<input type="hidden" name="order" value="{{.Pager.HiddenOrder}}">{{end}}
        <input type="hidden" name="per_page" value="{{.Pager.PageSize}}">
        <label for="arch-go">Go to page</label>
        <input id="arch-go" type="number" name="page" min="1" max="{{.Pager.TotalPages}}" value="{{.Pager.Page}}" inputmode="numeric">
        <button class="btn btn-quiet btn-sm" type="submit">Go</button>
      </form>
      <form class="pager-size" method="get">
        {{if .Pager.HiddenSort}}<input type="hidden" name="sort" value="{{.Pager.HiddenSort}}">{{end}}
        {{if .Pager.HiddenOrder}}<input type="hidden" name="order" value="{{.Pager.HiddenOrder}}">{{end}}
        <label for="arch-per-page">Per page</label>
        <select id="arch-per-page" name="per_page" onchange="this.form.submit()">
        {{range .PagerSizes}}<option value="{{.}}"{{if eq $.Pager.PageSize .}} selected{{end}}>{{.}}</option>{{end}}
        </select>
      </form>
    </div>
    <div class="pager-edge">
      {{if .Pager.HasNext}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.NextPage .Pager}}">Next</a>{{else}}<span></span>{{end}}
    </div>
  </nav>
  {{end}}
  {{else}}
  <div class="empty">
    {{if .Filter.IsZero}}
    <p class="empty-title">No captures yet</p>
    <p class="empty-sub">Browse will stay empty until the first archive lands. <a href="/upload">Upload a capture</a> or use the HTTP API.</p>
    {{else}}
    <p class="empty-title">No matches</p>
    <p class="empty-sub">No captures match the current filters. <a href="/" class="empty-clear">Clear filters</a> to see all captures.</p>
    {{end}}
  </div>
  {{end}}
</section>
</main>
{{.AppFoot}}
<dialog id="confirm-dialog" aria-labelledby="confirm-title">
  <form method="dialog" class="dialog-card">
    <p class="dialog-title" id="confirm-title">Delete capture</p>
    <p class="dialog-text" id="confirm-text"></p>
    <div class="dialog-typed is-hidden" id="confirm-typed">
      <label class="field"><span id="confirm-typed-hint">Type the name to confirm</span><input id="confirm-input" autocomplete="off" spellcheck="false"></label>
    </div>
    <div class="dialog-actions">
      <button class="btn btn-quiet" value="cancel">Cancel</button>
      <button class="btn btn-danger" value="ok" id="confirm-ok">Delete</button>
    </div>
  </form>
</dialog>
<script>
(function () {
  var dlg = document.getElementById('confirm-dialog');
  var txt = document.getElementById('confirm-text');
  var typed = document.getElementById('confirm-typed');
  var input = document.getElementById('confirm-input');
  var hint = document.getElementById('confirm-typed-hint');
  var ok = document.getElementById('confirm-ok');
  var pending = null;
  if (dlg && dlg.showModal) {
    document.querySelectorAll('form[data-confirm]').forEach(function (f) {
      f.addEventListener('submit', function (e) {
        e.preventDefault();
        var requireVal = f.getAttribute('data-confirm-require');
        pending = f;
        txt.textContent = f.getAttribute('data-confirm');
        if (requireVal !== null && requireVal !== undefined) {
          typed.classList.remove('is-hidden');
          hint.textContent = 'Type ' + requireVal + ' to confirm';
          input.value = '';
          input.dataset.require = requireVal;
          ok.disabled = true;
        } else {
          typed.classList.add('is-hidden');
          input.dataset.require = '';
          ok.disabled = false;
        }
        dlg.showModal();
      });
    });
    input.addEventListener('input', function () {
      ok.disabled = input.value !== (input.dataset.require || '');
    });
    dlg.addEventListener('close', function () {
      if (dlg.returnValue === 'ok' && pending) { pending.submit(); }
      pending = null;
    });
  }
  document.querySelectorAll('.copy-link').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var url = btn.getAttribute('data-copy-url');
      if (!url || !navigator.clipboard) return;
      navigator.clipboard.writeText(url).then(function () {
        btn.setAttribute('title', 'Copied');
        setTimeout(function () { btn.setAttribute('title', 'Copy download link'); }, 2000);
      });
    });
  });
  // Inline dropzone upload (XHR — fetch has no upload progress).
  var form = document.getElementById('upload-inline');
  var dz = document.getElementById('inline-dropzone');
  var fileInput = document.getElementById('inline-file');
  var dzText = document.getElementById('inline-dz-text');
  var meta = document.getElementById('inline-meta');
  var bar = document.getElementById('inline-progress');
  var status = document.getElementById('inline-status');
  var sendBtn = document.getElementById('inline-send');
  var cancelBtn = document.getElementById('inline-cancel');
  var xhr = null;
  if (form && dz && fileInput && dzText) {
    var maxUpload = parseInt(form.getAttribute('data-max-upload'), 10) || 0;
    function humanSize(n) {
      if (!n) return '0 B';
      var units = ['B', 'KB', 'MB', 'GB', 'TB'];
      var i = 0; while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
      return (i === 0 ? n : n.toFixed(1)) + ' ' + units[i];
    }
    function setStatus(kind, html) {
      status.className = 'upload-status' + (kind ? ' ' + kind : '');
      status.innerHTML = html;
      status.hidden = !html;
    }
    function resetUI() {
      bar.hidden = true; bar.value = 0;
      cancelBtn.hidden = true;
    }
    function refresh(selected) {
      // name + size before send
      var f = selected[0];
      if (!f) { dzText.textContent = 'Choose a file or drop it here'; dzText.classList.remove('has-file'); meta.hidden = true; sendBtn.disabled = true; return; }
      dzText.textContent = f.name;
      dzText.classList.add('has-file');
      meta.textContent = f.name + ' · ' + humanSize(f.size);
      meta.hidden = false;
      if (maxUpload > 0 && f.size > maxUpload) {
        meta.textContent += ' — exceeds the ' + humanSize(maxUpload) + ' limit';
        sendBtn.disabled = true;
        setStatus('err', 'File exceeds the ' + humanSize(maxUpload) + ' size limit.');
        return;
      }
      setStatus('', '');
      sendBtn.disabled = false;
    }
    fileInput.addEventListener('change', function () { if (fileInput.files) refresh(fileInput.files); });
    ['dragenter', 'dragover'].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.add('drag'); });
    });
    ['dragleave', 'drop'].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.remove('drag'); });
    });
    dz.addEventListener('drop', function (e) {
      if (e.dataTransfer && e.dataTransfer.files.length) {
        fileInput.files = e.dataTransfer.files;
        refresh(e.dataTransfer.files);
      }
    });
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var f = fileInput.files && fileInput.files.length ? fileInput.files[0] : null;
      if (!f) return;
      var fd = new FormData();
      fd.append('file', f);
      xhr = new XMLHttpRequest();
      xhr.open('POST', '/v1/archives');
      xhr.setRequestHeader('Accept', 'application/json');
      xhr.upload.onprogress = function (ev) {
        if (ev.lengthComputable) {
          bar.hidden = false;
          bar.value = ev.loaded / ev.total;
        }
      };
      xhr.onload = function () {
        var body = {};
        try { body = JSON.parse(xhr.responseText || '{}'); } catch (err) { body = {}; }
        resetUI();
        if (xhr.status === 201) {
          if (body.storage === 'transit') {
            setStatus('transit', 'Uploaded — in transit, will appear in Captures once the bucket copy completes.');
          } else {
            setStatus('ok', 'Capture uploaded.');
          }
          setTimeout(function () { window.location.assign(window.location.pathname + window.location.search); }, 800);
        } else if (xhr.status === 409) {
          var existing = body.existing && body.existing.key ? body.existing.key : f.name;
          setStatus('err', 'Already uploaded (same content): <span class="mono">' + existing + '</span>. <a href="/">View in Captures</a>.');
        } else if (xhr.status === 413) {
          setStatus('err', 'File exceeds the size limit.');
        } else {
          setStatus('err', 'Upload failed. Check the file and try again.');
        }
        sendBtn.disabled = false;
      };
      xhr.onerror = function () {
        resetUI();
        setStatus('err', 'Upload failed (network). Check the file and try again.');
        sendBtn.disabled = false;
      };
      xhr.onabort = function () {
        resetUI();
        setStatus('err', 'Upload canceled.');
        sendBtn.disabled = false;
        dzText.textContent = 'Choose a file or drop it here';
        dzText.classList.remove('has-file');
        meta.hidden = true;
      };
      setStatus('', '');
      sendBtn.disabled = true;
      cancelBtn.hidden = false;
      xhr.send(fd);
    });
    cancelBtn.addEventListener('click', function () {
      if (xhr) { xhr.abort(); xhr = null; }
    });
  }
})();
</script>
{{.ThemeToggleScript}}
</body></html>
`))

var Upload = template.Must(template.New("upload").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Upload</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
` + appNavTmpl + `
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
` + appWhoTmpl + `
      <form method="post" action="/logout"><button class="btn btn-quiet btn-sm" type="submit">Sign out</button></form>
    </div>
  </div>
</header>
<main id="main" class="wrap">
{{if .NoticeText}}<div class="notice notice-{{.NoticeKind}}" role="status">{{.NoticeText}}</div>{{end}}
<div class="page-head">
  <div>
    <h1>Upload capture</h1>
    <p class="sub">Send a groot .tar.gz from your browser. For automation, use <span class="mono">POST /v1/archives</span> with an API key.</p>
  </div>
  <a class="btn btn-quiet" href="/">Back to captures</a>
</div>
<section class="card" aria-labelledby="up-h">
  <div class="card-head">
    <h2 id="up-h">Choose file</h2>
    <p class="hint">groot .tar.gz, up to {{humansize .MaxUpload}} per file</p>
  </div>
  <form method="post" action="/v1/archives" enctype="multipart/form-data" class="upload" id="upload-form">
    <label class="dropzone" id="dropzone">
      <input type="file" name="file" id="file" accept=".tar.gz,.tgz,application/gzip" required>
      <span class="dz-text" id="dz-text">Choose a file or drop it here</span>
    </label>
    <label class="field-check"><input type="checkbox" id="pin-after" value="1"> Pin after upload (survives retention)</label>
    <button class="btn" type="submit">Upload capture</button>
  </form>
</section>
</main>
{{.AppFoot}}
<script>
(function () {
  var dz = document.getElementById('dropzone');
  var input = document.getElementById('file');
  var label = document.getElementById('dz-text');
  var form = document.getElementById('upload-form');
  var pin = document.getElementById('pin-after');
  if (form && pin) {
    form.addEventListener('submit', function () {
      form.action = pin.checked ? '/v1/archives?pin=1' : '/v1/archives';
    });
  }
  if (dz && input) {
    var show = function () {
      if (input.files && input.files.length) {
        label.textContent = input.files[0].name;
        label.classList.add('has-file');
      }
    };
    input.addEventListener('change', show);
    ['dragenter', 'dragover'].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.add('drag'); });
    });
    ['dragleave', 'drop'].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.remove('drag'); });
    });
    dz.addEventListener('drop', function (e) {
      if (e.dataTransfer && e.dataTransfer.files.length) {
        input.files = e.dataTransfer.files;
        show();
      }
    });
  }
})();
</script>
{{.ThemeToggleScript}}
</body></html>
`))

var Activity = template.Must(template.New("activity").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Activity</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
` + appNavTmpl + `
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
` + appWhoTmpl + `
      <form method="post" action="/logout"><button class="btn btn-quiet btn-sm" type="submit">Sign out</button></form>
    </div>
  </div>
</header>
<main id="main" class="wrap">
<div class="page-head">
  <div>
    <h1>Activity</h1>
    <p class="sub">Audit log for uploads, downloads, and deletions. JSON at <span class="mono">GET /v1/audit</span>.</p>
  </div>
  <div class="page-actions">
    <a class="btn btn-quiet" href="/">Back to captures</a>
    {{if .CanExport}}
    <span class="export-group">
      <a class="btn btn-sm" href="/v1/activity/export?format=csv{{if .FilterActor}}&amp;actor={{.FilterActor}}{{end}}{{if .FilterAction}}&amp;action={{.FilterAction}}{{end}}{{if .FilterWindow}}&amp;window={{.FilterWindow}}{{end}}">Export CSV</a>
      <a class="btn btn-quiet btn-sm" href="/v1/activity/export?format=json{{if .FilterActor}}&amp;actor={{.FilterActor}}{{end}}{{if .FilterAction}}&amp;action={{.FilterAction}}{{end}}{{if .FilterWindow}}&amp;window={{.FilterWindow}}{{end}}">Export JSON</a>
    </span>
    {{end}}
  </div>
</div>
<section class="card" aria-labelledby="ac-h">
  <div class="card-head">
    <h2 id="ac-h">Recent events</h2>
    <p class="hint">{{.Pager.Total}} total</p>
  </div>
  <form class="filter-row" method="get" action="/activity">
    <label class="field-inline">
      <span class="visually-hidden">Action</span>
      <select name="action">
        <option value="">All actions</option>
        {{range .AuditActions}}<option value="{{.}}"{{if eq $.FilterAction .}} selected{{end}}>{{.}}</option>{{end}}
      </select>
    </label>
    <label class="field-inline">
      <span class="visually-hidden">Actor</span>
      <input type="search" name="actor" value="{{.FilterActor}}" placeholder="Actor" maxlength="80">
    </label>
    <label class="field-inline">
      <span class="visually-hidden">Window</span>
      <select name="window">
        <option value="">All time</option>
        <option value="24h"{{if eq .FilterWindow "24h"}} selected{{end}}>24h</option>
        <option value="7d"{{if eq .FilterWindow "7d"}} selected{{end}}>7d</option>
        <option value="30d"{{if eq .FilterWindow "30d"}} selected{{end}}>30d</option>
      </select>
    </label>
    <button class="btn btn-quiet btn-sm filter-apply" type="submit">Filter</button>
  </form>
  {{if .Audit}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col">When (UTC)</th>
      <th scope="col">Who</th>
      <th scope="col">Action</th>
      <th scope="col">Object</th>
      <th scope="col">IP</th>
    </tr></thead>
    <tbody>
    {{range .Audit}}
    <tr>
      <td class="muted tabular">{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}</td>
      <td>{{.Actor}}</td>
      <td>{{.Action}}</td>
      <td class="muted mono" style="word-break: break-all;">{{.ObjectKey}}</td>
      <td class="muted tabular">{{.RemoteIP}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{if gt .Pager.Total 0}}
  <nav class="pager" aria-label="Activity pagination">
    <div class="pager-edge">
      {{if .Pager.HasPrev}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.PrevPage .Pager}}">Previous</a>{{else}}<span></span>{{end}}
    </div>
    <div class="pager-center">
      <p class="pager-meta">Page {{.Pager.Page}} of {{.Pager.TotalPages}} · {{.Pager.Total}} events</p>
      <div class="pager-tools">
        {{if .Pager.HasPrev}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.FirstPage .Pager}}" title="First page">« First</a>{{else}}<span class="btn btn-quiet btn-sm" aria-disabled="true" title="First page">« First</span>{{end}}
        <div class="pager-pages">
          {{range .Pager.Pages}}
            {{if eq .Kind "page"}}
              {{if eq .Page $.Pager.Page}}<span class="pager-current" aria-current="page">{{.Label}}</span>{{else}}<a class="pager-num" href="{{pagerurl .Page $.Pager}}">{{.Label}}</a>{{end}}
            {{else}}<span class="pager-gap" aria-hidden="true">{{.Label}}</span>{{end}}
          {{end}}
        </div>
        {{if .Pager.HasNext}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.LastPage .Pager}}" title="Last page">Last »</a>{{else}}<span class="btn btn-quiet btn-sm" aria-disabled="true" title="Last page">Last »</span>{{end}}
      </div>
      <form class="pager-jump" method="get" action="">
        {{if .Pager.HiddenSort}}<input type="hidden" name="sort" value="{{.Pager.HiddenSort}}">{{end}}
        {{if .Pager.HiddenOrder}}<input type="hidden" name="order" value="{{.Pager.HiddenOrder}}">{{end}}
        <input type="hidden" name="per_page" value="{{.Pager.PageSize}}">
        <label for="act-go">Go to page</label>
        <input id="act-go" type="number" name="page" min="1" max="{{.Pager.TotalPages}}" value="{{.Pager.Page}}" inputmode="numeric">
        <button class="btn btn-quiet btn-sm" type="submit">Go</button>
      </form>
      <form class="pager-size" method="get">
        <label for="act-per-page">Per page</label>
        <select id="act-per-page" name="per_page" onchange="this.form.submit()">
        {{range .PagerSizes}}<option value="{{.}}"{{if eq $.Pager.PageSize .}} selected{{end}}>{{.}}</option>{{end}}
        </select>
      </form>
    </div>
    <div class="pager-edge">
      {{if .Pager.HasNext}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.NextPage .Pager}}">Next</a>{{else}}<span></span>{{end}}
    </div>
  </nav>
  {{end}}
  {{else}}
  <div class="empty">
    <p class="empty-title">No activity yet</p>
    <p class="empty-sub">Uploads, downloads, and deletions will appear here.</p>
  </div>
  {{end}}
</section>
</main>
{{.AppFoot}}
{{.ThemeToggleScript}}
</body></html>
`))

var Settings = template.Must(template.New("settings").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Settings</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
` + appNavTmpl + `
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
` + appWhoTmpl + `
      <form method="post" action="/logout"><button class="btn btn-quiet btn-sm" type="submit">Sign out</button></form>
    </div>
  </div>
</header>
<main id="main" class="wrap">
{{if .NoticeText}}<div class="notice notice-{{.NoticeKind}}" role="status">{{.NoticeText}}</div>{{end}}
<div class="page-head">
  <div><h1>Settings</h1><p class="sub">Name, password, and API keys.</p></div>
</div>
<section class="card" aria-labelledby="name-h">
  <div class="card-head"><h2 id="name-h">Name</h2></div>
  <div class="card-body">
  <form method="post" action="/settings/name" class="stack-form">
    <label class="field"><span>Username</span><input value="{{.Username}}" readonly autocomplete="username"><small class="field-hint">Login id. Only an admin can change it.</small></label>
    <label class="field"><span>Name</span><input name="name" required autocomplete="name" maxlength="80" value="{{.Name}}"><small class="field-hint">Shown in the header. Long names shorten (Juan ...egro).</small></label>
    <button class="btn" type="submit">Update name</button>
  </form>
  </div>
</section>
<section class="card" aria-labelledby="pw-h">
  <div class="card-head"><h2 id="pw-h">Password</h2></div>
  <div class="card-body">
  <form method="post" action="/settings/password" class="stack-form">
    <label class="field"><span>New password</span><input name="password" type="password" required autocomplete="new-password" minlength="8"><small class="field-hint">At least 8 characters.</small></label>
    <button class="btn" type="submit">Update password</button>
  </form>
  </div>
</section>
{{if .CanManageKeys}}
<section class="card" aria-labelledby="keys-h">
  <div class="card-head"><h2 id="keys-h">API keys</h2></div>
  <div class="card-body card-stack">
  <div class="notice notice-warn" role="note">The secret is shown once. Copy it now — it cannot be recovered.</div>
  {{if .NewAPIKey}}
  <div class="key-reveal">
    <p class="field-hint">Prefix <span class="mono">{{.NewAPIKeyPrefix}}</span> · scope {{.NewAPIKeyScope}}</p>
    <div class="key-once" id="new-key">{{.NewAPIKey}}</div>
    <button class="btn" type="button" id="copy-new-key">Copy key</button>
  </div>
  {{end}}
  {{if .APIKeys}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col">Prefix</th>
      <th scope="col">Scope</th>
      <th scope="col">Created (UTC)</th>
      <th scope="col">Last used (UTC)</th>
      <th scope="col"><span class="visually-hidden">Actions</span></th>
    </tr></thead>
    <tbody>
    {{range .APIKeys}}
    <tr>
      <td class="mono">{{.Prefix}}</td>
      <td>{{.Scope}}</td>
      <td class="muted tabular">{{if .CreatedAt.IsZero}}—{{else}}{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}{{end}}</td>
      <td class="muted tabular">{{if .LastUsedAt.IsZero}}never{{else}}{{.LastUsedAt.UTC.Format "2006-01-02 15:04"}}{{end}}</td>
      <td class="actions">
        <form method="post" action="/settings/api-keys/{{.ID}}/revoke" data-confirm="Delete API key {{.Prefix}}? Anything using it will fail immediately." data-confirm-require="{{.Prefix}}">
          <button class="btn btn-danger btn-sm" type="submit">Delete</button>
        </form>
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{else if not .NewAPIKey}}
  <div class="empty"><p class="empty-title">No API keys</p></div>
  {{end}}
  <form method="post" action="/settings/api-keys" class="stack-form">
    <label class="field"><span>Scope</span>
      <select name="scope">
        <option value="upload">upload — POST /v1/archives only</option>
        {{if .CanCreateReadKey}}<option value="read">read — list, download, audit</option>{{end}}
      </select>
    </label>
    <button class="btn" type="submit">Create API key</button>
  </form>
  </div>
</section>
{{end}}
</main>
{{.AppFoot}}
<dialog id="confirm-dialog" aria-labelledby="confirm-title">
  <form method="dialog" class="dialog-card">
    <p class="dialog-title" id="confirm-title">Delete API key</p>
    <p class="dialog-text" id="confirm-text"></p>
    <div class="dialog-typed is-hidden" id="confirm-typed">
      <label class="field"><span id="confirm-typed-hint">Type the name to confirm</span><input id="confirm-input" autocomplete="off" spellcheck="false"></label>
    </div>
    <div class="dialog-actions">
      <button class="btn btn-quiet" value="cancel">Cancel</button>
      <button class="btn btn-danger" value="ok" id="confirm-ok">Delete</button>
    </div>
  </form>
</dialog>
<script>
(function () {
  var btn = document.getElementById('copy-new-key');
  var el = document.getElementById('new-key');
  if (btn && el && navigator.clipboard) {
    btn.addEventListener('click', function () {
      navigator.clipboard.writeText(el.textContent.trim()).then(function () {
        btn.textContent = 'Copied';
        setTimeout(function () { btn.textContent = 'Copy key'; }, 2000);
      });
    });
  }
  var dlg = document.getElementById('confirm-dialog');
  var txt = document.getElementById('confirm-text');
  var typed = document.getElementById('confirm-typed');
  var input = document.getElementById('confirm-input');
  var hint = document.getElementById('confirm-typed-hint');
  var ok = document.getElementById('confirm-ok');
  var pending = null;
  if (dlg && dlg.showModal) {
    document.querySelectorAll('form[data-confirm]').forEach(function (f) {
      f.addEventListener('submit', function (e) {
        e.preventDefault();
        var requireVal = f.getAttribute('data-confirm-require');
        pending = f;
        txt.textContent = f.getAttribute('data-confirm');
        if (requireVal !== null && requireVal !== undefined) {
          typed.classList.remove('is-hidden');
          hint.textContent = 'Type ' + requireVal + ' to confirm';
          input.value = '';
          input.dataset.require = requireVal;
          ok.disabled = true;
        } else {
          typed.classList.add('is-hidden');
          input.dataset.require = '';
          ok.disabled = false;
        }
        dlg.showModal();
      });
    });
    input.addEventListener('input', function () {
      ok.disabled = input.value !== (input.dataset.require || '');
    });
    dlg.addEventListener('close', function () {
      if (dlg.returnValue === 'ok' && pending) { pending.submit(); }
      pending = null;
    });
  }
})();
</script>
{{.ThemeToggleScript}}
</body></html>
`))

var AdminUsers = template.Must(template.New("admin").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Users</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
` + appNavTmpl + `
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
` + appWhoTmpl + `
      <form method="post" action="/logout"><button class="btn btn-quiet btn-sm" type="submit">Sign out</button></form>
    </div>
  </div>
</header>
<main id="main" class="wrap">
{{if .NoticeText}}<div class="notice notice-{{.NoticeKind}}" role="status">{{.NoticeText}}</div>{{end}}
<div class="page-head">
  <div><h1>Users</h1><p class="sub">Manage accounts, logins, and roles.</p></div>
</div>
<section class="card" aria-labelledby="create-h">
  <div class="card-head"><h2 id="create-h">Create user</h2></div>
  <div class="card-body">
  <form method="post" action="/admin/users/create" class="stack-form">
    <label class="field"><span>Name</span><input name="name" required autocomplete="name" maxlength="80"><small class="field-hint">Required. Shown in the header. Long names shorten (Juan ...egro).</small></label>
    <label class="field"><span>Username</span><input name="username" required autocomplete="off" maxlength="64"><small class="field-hint">Required. Login id. Must be unique.</small></label>
    <label class="field"><span>Password</span><input name="password" type="password" required autocomplete="new-password" minlength="8"><small class="field-hint">At least 8 characters.</small></label>
    <label class="field"><span>Role</span>
      <select name="role">
        <option value="viewer">viewer</option>
        <option value="uploader" selected>uploader</option>
        <option value="admin">admin</option>
      </select>
      <small class="field-hint">viewer reads; uploader also writes; admin manages users.</small>
    </label>
    <button class="btn" type="submit">Create user</button>
  </form>
  </div>
</section>
<section class="card" aria-labelledby="users-h">
  <div class="card-head"><h2 id="users-h">Accounts</h2></div>
  {{if .Users}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col">Name</th>
      <th scope="col">Username</th>
      <th scope="col">Role</th>
      <th scope="col">Active</th>
      <th scope="col">Created (UTC)</th>
      <th scope="col"><span class="visually-hidden">Actions</span></th>
    </tr></thead>
    <tbody>
    {{range .Users}}
    <tr>
      <td title="{{.Name}}">{{.DisplayName}}</td>
      <td>
        <form method="post" action="/admin/users/{{.ID}}/username" class="inline-form">
          <input name="username" value="{{.Username}}" required maxlength="64" autocomplete="off" aria-label="Login for {{.Name}}">
          <button class="btn btn-quiet btn-sm" type="submit">Set login</button>
        </form>
      </td>
      <td>{{.Role}}</td>
      <td>{{if .Active}}yes{{else}}no{{end}}</td>
      <td class="muted tabular">{{if .CreatedAt.IsZero}}—{{else}}{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}{{end}}</td>
      <td class="actions">
        {{if .Active}}
        <form method="post" action="/admin/users/{{.ID}}/role" class="inline-form">
          <select name="role" aria-label="Role for {{.Username}}">
            <option value="viewer"{{if eq .Role "viewer"}} selected{{end}}>viewer</option>
            <option value="uploader"{{if eq .Role "uploader"}} selected{{end}}>uploader</option>
            <option value="admin"{{if eq .Role "admin"}} selected{{end}}>admin</option>
          </select>
          <button class="btn btn-quiet btn-sm" type="submit">Set role</button>
        </form>
        <form method="post" action="/admin/users/{{.ID}}/deactivate">
          <button class="btn btn-danger-quiet btn-sm" type="submit">Deactivate</button>
        </form>
        {{else}}
        <form method="post" action="/admin/users/{{.ID}}/activate">
          <button class="btn btn-quiet btn-sm" type="submit">Activate</button>
        </form>
        {{if ne .ID $.ActorID}}
        <form method="post" action="/admin/users/{{.ID}}/remove" data-confirm="This deletes {{.Username}} and their sessions and API keys. It cannot be undone." data-confirm-require="{{.Username}}">
          <button class="btn btn-danger-quiet btn-sm" type="submit">Remove</button>
        </form>
        {{end}}
        {{end}}
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{else}}
  <div class="empty"><p class="empty-title">No users</p></div>
  {{end}}
</section>
</main>
{{.AppFoot}}
<dialog id="confirm-dialog" aria-labelledby="confirm-title">
  <form method="dialog" class="dialog-card">
    <p class="dialog-title" id="confirm-title">Remove user</p>
    <p class="dialog-text" id="confirm-text"></p>
    <div class="dialog-actions">
      <button class="btn btn-quiet" value="cancel">Cancel</button>
      <button class="btn btn-danger" value="ok">Remove</button>
    </div>
  </form>
</dialog>
<script>
(function () {
  var dlg = document.getElementById('confirm-dialog');
  var txt = document.getElementById('confirm-text');
  var pending = null;
  if (dlg && dlg.showModal) {
    document.querySelectorAll('form[data-confirm]').forEach(function (f) {
      f.addEventListener('submit', function (e) {
        e.preventDefault();
        pending = f;
        txt.textContent = f.getAttribute('data-confirm');
        dlg.showModal();
      });
    });
    dlg.addEventListener('close', function () {
      if (dlg.returnValue === 'ok' && pending) { pending.submit(); }
      pending = null;
    });
  }
})();
</script>
{{.ThemeToggleScript}}
</body></html>
`))

var Shares = template.Must(template.New("shares").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Shares</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
` + appNavTmpl + `
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
` + appWhoTmpl + `
      <form method="post" action="/logout"><button class="btn btn-quiet btn-sm" type="submit">Sign out</button></form>
    </div>
  </div>
</header>
<main id="main" class="wrap">
{{if .Shares.NoticeText}}<div class="notice notice-{{.Shares.NoticeKind}}" role="status">{{.Shares.NoticeText}}</div>{{end}}
<div class="page-actions">
  <a class="btn btn-quiet" href="/">Back to captures</a>
</div>
<div class="page-head">
  <div>
    <h1>Share links</h1>
    <p class="sub mon">{{.Shares.Key}}</p>
  </div>
</div>

{{if .Shares.CreatedURL}}
<section class="card" aria-labelledby="created-h">
  <div class="card-head"><h2 id="created-h">Copy this link now</h2></div>
  <div class="card-body">
    <p class="share-warn">This URL is shown only once. Copy it before you leave this page.</p>
    <div class="share-url">
      <input class="mono" type="text" readonly value="{{.Shares.CreatedURL}}" aria-label="Share URL">
      <button class="btn copy-link" type="button" data-copy-url="{{.Shares.CreatedURL}}">Copy</button>
    </div>
  </div>
</section>
{{end}}

<section class="card" aria-labelledby="create-h">
  <div class="card-head"><h2 id="create-h">Create share link</h2></div>
  <div class="card-body">
  <form method="post" action="/shares/{{.Shares.ArchiveID}}" class="stack-form">
    <fieldset class="ttl-fieldset">
      <legend>Expiry</legend>
      <div class="ttl-presets" role="group" aria-label="Preset TTL">
        <button class="btn btn-quiet btn-sm" type="button" data-ttl="24h">24 hours</button>
        <button class="btn btn-quiet btn-sm" type="button" data-ttl="168h">7 days</button>
      </div>
      <label class="field"><span>Custom until (your local time)</span><input type="datetime-local" name="expires_at_local" value="{{.Shares.FormUntil}}"><small class="field-hint">Sets an absolute expiry. Leave empty when using a preset.</small></label>
      <input type="hidden" name="expires_in" id="expires-in" value="">
    </fieldset>
    <label class="field"><span>Label (optional)</span><input name="label" value="{{.Shares.FormLabel}}" maxlength="120" autocomplete="off"><small class="field-hint">A short note for whoever this link is for.</small></label>
    <label class="field"><span>Max uses (optional)</span><input name="max_uses" type="number" min="0" value="{{.Shares.FormMaxUses}}" autocomplete="off"><small class="field-hint">0 (or blank) = unlimited. After N downloads the link stops.</small></label>
    <button class="btn" type="submit">Create share link</button>
  </form>
  </div>
</section>

<section class="card" aria-labelledby="links-h">
  <div class="card-head"><h2 id="links-h">Links</h2></div>
  {{if .Shares.Links}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col">Label</th>
      <th scope="col">Created (UTC)</th>
      <th scope="col">Expires (UTC)</th>
      <th scope="col">Uses</th>
      <th scope="col">Status</th>
      <th scope="col"><span class="visually-hidden">Actions</span></th>
    </tr></thead>
    <tbody>
    {{range .Shares.Links}}
    <tr>
      <td class="mono">{{if .Label}}{{.Label}}{{else}}—{{end}}</td>
      <td class="muted tabular">{{if .CreatedAt.IsZero}}—{{else}}{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}{{end}}</td>
      <td class="tabular">{{if .ExpiresAt.IsZero}}never{{else}}{{.ExpiresAt.UTC.Format "2006-01-02 15:04"}}{{end}}</td>
      <td class="tabular">{{.UseCount}} / {{if .MaxUses}}{{.MaxUses}}{{else}}∞{{end}}</td>
      <td><span class="pill pill-{{.Status}}">{{.Status}}</span></td>
      <td class="actions">
        {{if .Active}}
        <form method="post" action="/shares/{{.ID}}/revoke" data-confirm="Revoke this share link? It will stop working immediately.">
          <button class="btn btn-danger-quiet btn-sm" type="submit">Revoke</button>
        </form>
        {{end}}
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{else}}
  <div class="empty"><p class="empty-title">No share links</p><p class="empty-sub">Create one above to share this capture externally.</p></div>
  {{end}}
</section>
</main>
{{.AppFoot}}
<dialog id="confirm-dialog" aria-labelledby="confirm-title">
  <form method="dialog" class="dialog-card">
    <p class="dialog-title" id="confirm-title">Revoke share link</p>
    <p class="dialog-text" id="confirm-text"></p>
    <div class="dialog-actions">
      <button class="btn btn-quiet" value="cancel">Cancel</button>
      <button class="btn btn-danger" value="ok">Revoke</button>
    </div>
  </form>
</dialog>
<script>
(function () {
  var presets = document.querySelectorAll('button[data-ttl]');
  var expiresIn = document.getElementById('expires-in');
  var untilInput = document.querySelector('input[name="expires_at_local"]');
  function clearActive() {
    presets.forEach(function (b) { b.classList.remove('is-active'); });
  }
  presets.forEach(function (b) {
    b.addEventListener('click', function () {
      expiresIn.value = b.getAttribute('data-ttl');
      untilInput.value = '';
      clearActive();
      b.classList.add('is-active');
    });
  });
  untilInput.addEventListener('input', function () {
    if (untilInput.value !== '') {
      expiresIn.value = '';
      clearActive();
    }
  });
  var copyTargets = document.querySelectorAll('button.copy-link[data-copy-url]');
  copyTargets.forEach(function (btn) {
    btn.addEventListener('click', function () {
      var url = btn.getAttribute('data-copy-url');
      var done = function () {
        var original = btn.textContent;
        btn.textContent = 'Copied!';
        btn.classList.add('is-active');
        setTimeout(function () {
          btn.textContent = original;
          btn.classList.remove('is-active');
        }, 1600);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(url).then(done, function () { fallbackCopy(url, done); });
      } else {
        fallbackCopy(url, done);
      }
    });
  });
  function fallbackCopy(text, done) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand('copy');
      done();
    } catch (e) {}
    document.body.removeChild(ta);
  }
  var dlg = document.getElementById('confirm-dialog');
  var txt = document.getElementById('confirm-text');
  var pending = null;
  if (dlg && dlg.showModal) {
    document.querySelectorAll('form[data-confirm]').forEach(function (f) {
      f.addEventListener('submit', function (e) {
        e.preventDefault();
        pending = f;
        txt.textContent = f.getAttribute('data-confirm');
        dlg.showModal();
      });
    });
    dlg.addEventListener('close', function () {
      if (dlg.returnValue === 'ok' && pending) { pending.submit(); }
      pending = null;
    });
  }
})();
</script>
{{.ThemeToggleScript}}
</body></html>
`))

var ShareGone = template.Must(template.New("share-gone").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Share link</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
    </div>
  </div>
</header>
<main id="main" class="wrap">
  <section class="card" style="max-width:460px;margin:48px auto;">
    <div class="card-head">
      <h2>410 — {{.GoneTitle}}</h2>
      <p class="hint">{{.GoneBody}}</p>
    </div>
    <div style="padding:16px 24px 20px;">
      <a class="btn" href="/">Go to Home</a>
    </div>
  </section>
</main>
{{.ThemeToggleScript}}
</body></html>
`))

var NotFound = template.Must(template.New("notfound").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Not found</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
    </div>
  </div>
</header>
<main id="main" class="wrap">
<div class="page-head">
  <div>
    <h1>Not found</h1>
    <p class="sub"><code class="mono">{{.Path}}</code> doesn't exist or has been removed.</p>
  </div>
</div>
<section class="card" aria-labelledby="nf-h">
  <div class="card-head">
    <h2 id="nf-h">Page not found</h2>
    <p class="hint">HTTP 404 — check the URL or head back to Captures.</p>
  </div>
  <div style="padding: 16px 24px 20px;">
    <a class="btn" href="/">Back to Captures</a>
  </div>
</section>
</main>
{{.ThemeToggleScript}}
</body></html>
`))
