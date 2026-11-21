package web

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/tmpl"

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
func (srv *Web) HandleSharesPage(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermSharesManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id := strings.Trim(r.PathValue("id"), "/")
	a, err := srv.ResolveArchive(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	links, err := srv.Store.ListShareLinks(r.Context(), id)
	if err != nil {
		slog.Error("list share links", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	kind, text := shareNotice(r.URL.Query().Get("notice"))
	renderSharesPage(w, srv, ac, sharesData{
		ArchiveID:  id,
		Key:        a.Key,
		Links:      shareViews(links, time.Now().UTC()),
		NoticeKind: kind,
		NoticeText: text,
	})
}

func renderSharesPage(w http.ResponseWriter, srv *Web, ac *core.Actor, d sharesData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := srv.PageShell()
	core.MergeActorData(data, ac)
	data["Nav"] = "captures"
	data["BaseURL"] = "" // set by callers that need it (create handler sets on the created URL)
	data["Shares"] = d
	_ = tmpl.Shares.Execute(w, data)
}

// handleSharesCreate handles POST /shares/{id...} (form-encoded).
func (srv *Web) HandleSharesCreate(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermSharesManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id := strings.Trim(r.PathValue("id"), "/")
	archive, err := srv.ResolveArchive(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		srv.renderSharesNotice(w, r, ac, id, "error")
		return
	}
	label := strings.TrimSpace(r.Form.Get("label"))
	maxUses, err := parseMaxUses(r.Form.Get("max_uses"))
	if err != nil {
		srv.renderSharesField(w, r, ac, id, "bad_max", label, r.Form.Get("max_uses"), r.Form.Get("expires_at_local"))
		return
	}
	expires, err := parseShareExpiry(r)
	if err != nil {
		srv.renderSharesField(w, r, ac, id, "bad_expiry", label, r.Form.Get("max_uses"), r.Form.Get("expires_at_local"))
		return
	}
	raw, hash, err := auth.NewShareToken()
	if err != nil {
		slog.Error("share token", "error", err)
		srv.renderSharesNotice(w, r, ac, id, "error")
		return
	}
	link, err := srv.Store.CreateShareLink(r.Context(), id, hash, ac.User.ID, label, maxUses, expires)
	if err != nil {
		slog.Error("create share link", "error", err)
		srv.renderSharesNotice(w, r, ac, id, "error")
		return
	}
	srv.RecordUserAudit(r, "share_create", strconv.FormatInt(link.ID, 10), id)
	links, err := srv.Store.ListShareLinks(r.Context(), id)
	if err != nil {
		slog.Error("list share links", "error", err)
		links = []store.ShareLink{link}
	}
	url := srv.RequestBaseURL(r) + "/s/" + raw
	renderSharesPage(w, srv, ac, sharesData{
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

func (srv *Web) renderSharesNotice(w http.ResponseWriter, r *http.Request, ac *core.Actor, id, notice string) {
	renderSharesPage(w, srv, ac, sharesData{
		ArchiveID:  id,
		Key:        archiveKeyOrID(srv, r, id),
		Links:      listSharesOrNil(srv, r, id),
		NoticeKind: shareNoticeKind(notice),
		NoticeText: shareNoticeText(notice),
	})
}

func (srv *Web) renderSharesField(w http.ResponseWriter, r *http.Request, ac *core.Actor, id, notice, label, maxUses, until string) {
	renderSharesPage(w, srv, ac, sharesData{
		ArchiveID:   id,
		Key:         archiveKeyOrID(srv, r, id),
		Links:       listSharesOrNil(srv, r, id),
		FormLabel:   label,
		FormMaxUses: maxUses,
		FormUntil:   until,
		NoticeKind:  shareNoticeKind(notice),
		NoticeText:  shareNoticeText(notice),
	})
}

func archiveKeyOrID(srv *Web, r *http.Request, id string) string {
	if a, err := srv.ResolveArchive(r.Context(), id); err == nil {
		return a.Key
	}
	return id
}

func listSharesOrNil(srv *Web, r *http.Request, id string) []shareView {
	links, err := srv.Store.ListShareLinks(r.Context(), id)
	if err != nil {
		slog.Warn("list share links", "error", err)
		return nil
	}
	return shareViews(links, time.Now().UTC())
}

// handleSharesRevoke handles the HTML revoke form alias.
func (srv *Web) HandleSharesRevoke(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermSharesManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	raw := strings.Trim(r.PathValue("share_id"), "/")
	shareID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || shareID <= 0 {
		http.NotFound(w, r)
		return
	}
	// Look up the archive id so we can redirect to its shares page.
	link, err := srv.Store.ShareByID(r.Context(), shareID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := srv.Store.RevokeShareLink(r.Context(), shareID, time.Now().UTC()); err != nil {
		http.Redirect(w, r, "/shares/"+url.PathEscape(link.ArchiveID)+"?notice=missing", http.StatusSeeOther)
		return
	}
	srv.RecordUserAudit(r, "share_revoke", raw, link.ArchiveID)
	http.Redirect(w, r, "/shares/"+url.PathEscape(link.ArchiveID)+"?notice=revoked", http.StatusSeeOther)
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
