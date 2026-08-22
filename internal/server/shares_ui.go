package server

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

// shareView is the template-facing shape of a share link for the admin UI.
type shareView struct {
	ID        int64
	Label     string
	MaxUses   int // 0 = unlimited
	UseCount  int
	ExpiresAt time.Time // zero = never
	RevokedAt time.Time // zero = active
	CreatedAt time.Time
	Active    bool
	Status    string // "active" | "expired" | "revoked" | "exhausted"
}

func shareViews(links []store.ShareLink, now time.Time) []shareView {
	out := make([]shareView, 0, len(links))
	for _, l := range links {
		v := shareView{
			ID:        l.ID,
			Label:     l.Label,
			MaxUses:   l.MaxUses,
			UseCount:  l.UseCount,
			Active:    l.Active(now),
			ExpiresAt: l.ExpiresAt.UTC(),
			RevokedAt: l.RevokedAt.UTC(),
			CreatedAt: l.CreatedAt.UTC(),
		}
		v.Status = shareStatus(l, now)
		out = append(out, v)
	}
	return out
}

func shareStatus(l store.ShareLink, now time.Time) string {
	switch {
	case !l.RevokedAt.IsZero():
		return "revoked"
	case !l.ExpiresAt.IsZero() && !now.Before(l.ExpiresAt):
		return "expired"
	case l.MaxUses > 0 && l.UseCount >= l.MaxUses:
		return "exhausted"
	default:
		return "active"
	}
}

// sharesData carries everything the shares page template needs.
type sharesData struct {
	ArchiveID string
	Key       string
	Links     []shareView
	// CreatedURL is the one-shot share URL (with raw token) to show after a
	// successful create. Empty except in the create response that renders it.
	CreatedURL string
	// Echoed form values on validation failure, so the operator keeps input.
	FormLabel   string
	FormMaxUses string
	FormUntil   string // echoed datetime-local value
	NoticeKind  string
	NoticeText  string
}

// handleSharesPage renders the per-archive share-link admin UI (GET).
func (s *Server) handleSharesPage(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermSharesManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id := strings.Trim(r.PathValue("id"), "/")
	a, err := s.Store.ArchiveByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	links, err := s.Store.ListShareLinks(r.Context(), id)
	if err != nil {
		slog.Error("list share links", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	kind, text := shareNotice(r.URL.Query().Get("notice"))
	renderSharesPage(w, s, ac, sharesData{
		ArchiveID:  id,
		Key:        a.Key,
		Links:      shareViews(links, time.Now().UTC()),
		NoticeKind: kind,
		NoticeText: text,
	})
}

func renderSharesPage(w http.ResponseWriter, s *Server, ac *Actor, d sharesData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := s.pageShell()
	mergeActorData(data, ac)
	data["Nav"] = "captures"
	data["BaseURL"] = "" // set by callers that need it (create handler sets on the created URL)
	data["Shares"] = d
	_ = sharesTmpl.Execute(w, data)
}

// handleSharesCreate handles POST /archives/{id}/shares (form-encoded).
func (s *Server) handleSharesCreate(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermSharesManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id := strings.Trim(r.PathValue("id"), "/")
	archive, err := s.Store.ArchiveByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderSharesNotice(w, r, ac, id, "error")
		return
	}
	label := strings.TrimSpace(r.Form.Get("label"))
	maxUses, err := parseMaxUses(r.Form.Get("max_uses"))
	if err != nil {
		s.renderSharesField(w, r, ac, id, "bad_max", label, r.Form.Get("max_uses"), r.Form.Get("expires_at_local"))
		return
	}
	expires, err := parseShareExpiry(r)
	if err != nil {
		s.renderSharesField(w, r, ac, id, "bad_expiry", label, r.Form.Get("max_uses"), r.Form.Get("expires_at_local"))
		return
	}
	raw, hash, err := auth.NewShareToken()
	if err != nil {
		slog.Error("share token", "error", err)
		s.renderSharesNotice(w, r, ac, id, "error")
		return
	}
	link, err := s.Store.CreateShareLink(r.Context(), id, hash, ac.User.ID, label, maxUses, expires)
	if err != nil {
		slog.Error("create share link", "error", err)
		s.renderSharesNotice(w, r, ac, id, "error")
		return
	}
	s.recordUserAudit(r, "share_create", strconv.FormatInt(link.ID, 10), id)
	links, err := s.Store.ListShareLinks(r.Context(), id)
	if err != nil {
		slog.Error("list share links", "error", err)
		links = []store.ShareLink{link}
	}
	url := requestBaseURL(r) + "/s/" + raw
	renderSharesPage(w, s, ac, sharesData{
		ArchiveID:  id,
		Key:        archive.Key,
		Links:      shareViews(links, time.Now().UTC()),
		CreatedURL: url,
		NoticeKind: "ok",
		NoticeText: "Share link created. Copy it now — it is shown only once.",
	})
}

func parseMaxUses(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, errors.New("invalid max uses")
	}
	return n, nil
}

func parseShareExpiry(r *http.Request) (time.Time, error) {
	in := strings.TrimSpace(r.Form.Get("expires_in"))
	local := strings.TrimSpace(r.Form.Get("expires_at_local"))
	now := time.Now().UTC()
	hasIn, hasAt := in != "", local != ""
	if hasIn == hasAt {
		return time.Time{}, errors.New("exactly one expiry")
	}
	var t time.Time
	if hasIn {
		d, err := time.ParseDuration(in)
		if err != nil || d <= 0 {
			return time.Time{}, errors.New("invalid expires_in")
		}
		t = now.Add(d)
	} else {
		parsed, err := time.ParseInLocation("2006-01-02T15:04", local, time.Local)
		if err != nil {
			return time.Time{}, errors.New("invalid expires_at_local")
		}
		t = parsed.UTC()
	}
	if !t.After(now) {
		return time.Time{}, errors.New("expiry already passed")
	}
	return t, nil
}

func (s *Server) renderSharesNotice(w http.ResponseWriter, r *http.Request, ac *Actor, id, notice string) {
	renderSharesPage(w, s, ac, sharesData{
		ArchiveID:  id,
		Key:        archiveKeyOrID(s, r, id),
		Links:      mustListShares(s, r, id),
		NoticeKind: shareNoticeKind(notice),
		NoticeText: shareNoticeText(notice),
	})
}

func (s *Server) renderSharesField(w http.ResponseWriter, r *http.Request, ac *Actor, id, notice, label, maxUses, until string) {
	renderSharesPage(w, s, ac, sharesData{
		ArchiveID:   id,
		Key:         archiveKeyOrID(s, r, id),
		Links:       mustListShares(s, r, id),
		FormLabel:   label,
		FormMaxUses: maxUses,
		FormUntil:   until,
		NoticeKind:  shareNoticeKind(notice),
		NoticeText:  shareNoticeText(notice),
	})
}

func archiveKeyOrID(s *Server, r *http.Request, id string) string {
	if a, err := s.Store.ArchiveByID(r.Context(), id); err == nil {
		return a.Key
	}
	return id
}

func mustListShares(s *Server, r *http.Request, id string) []shareView {
	links, err := s.Store.ListShareLinks(r.Context(), id)
	if err != nil {
		slog.Warn("list share links", "error", err)
		return nil
	}
	return shareViews(links, time.Now().UTC())
}

// handleSharesRevoke handles the HTML revoke form alias.
func (s *Server) handleSharesRevoke(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermSharesManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id := strings.Trim(r.PathValue("id"), "/")
	raw := strings.Trim(r.PathValue("share_id"), "/")
	shareID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || shareID <= 0 {
		http.NotFound(w, r)
		return
	}
	if err := s.Store.RevokeShareLink(r.Context(), shareID, time.Now().UTC()); err != nil {
		http.Redirect(w, r, "/archives/"+url.PathEscape(id)+"/shares?notice=missing", http.StatusSeeOther)
		return
	}
	s.recordUserAudit(r, "share_revoke", raw, id)
	http.Redirect(w, r, "/archives/"+url.PathEscape(id)+"/shares?notice=revoked", http.StatusSeeOther)
}

func shareNotice(token string) (kind, text string) {
	if token == "" {
		return "", ""
	}
	return shareNoticeKind(token), shareNoticeText(token)
}

func shareNoticeKind(token string) string {
	switch token {
	case "revoked":
		return "ok"
	case "created":
		return "ok"
	case "missing":
		return "err"
	case "bad_expiry", "bad_max", "error":
		return "err"
	default:
		return ""
	}
}

func shareNoticeText(token string) string {
	switch token {
	case "revoked":
		return "Share link revoked."
	case "bad_expiry":
		return "Choose a TTL or a future date — exactly one, in the future."
	case "bad_max":
		return "Max uses must be 0 (unlimited) or a positive integer."
	case "missing":
		return "That share link no longer exists."
	case "error":
		return "That action failed. Check the form and try again."
	default:
		return ""
	}
}

var sharesTmpl = template.Must(template.New("shares").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
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
  <form method="post" action="/archives/{{.Shares.ArchiveID}}/shares" class="stack-form">
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
        <form method="post" action="/archives/{{$.Shares.ArchiveID}}/shares/{{.ID}}/revoke" data-confirm="Revoke this share link? It will stop working immediately.">
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
  presets.forEach(function (b) {
    b.addEventListener('click', function () {
      expiresIn.value = b.getAttribute('data-ttl');
      untilInput.value = '';
    });
  });
  untilInput.addEventListener('input', function () {
    if (untilInput.value !== '') { expiresIn.value = ''; }
  });
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
