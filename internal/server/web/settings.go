package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/tmpl"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func (srv *Web) HandleSettingsGET(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	keys, err := srv.Store.ListAPIKeysByUser(r.Context(), ac.User.ID)
	if err != nil {
		slog.Error("settings list keys", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := srv.PageShell()
	core.MergeActorData(data, ac)
	data["Nav"] = "settings"
	data["APIKeys"] = keys
	data["CanCreateReadKey"] = ac.User.Role == auth.RoleAdmin
	data["NoticeKind"], data["NoticeText"] = settingsNotice(r.URL.Query().Get("notice"))
	_ = tmpl.Settings.Execute(w, data)
}

func (srv *Web) HandleSettingsPasswordPOST(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	hash, err := auth.HashPassword(r.Form.Get("password"))
	if err != nil {
		if errors.Is(err, auth.ErrPasswordTooShort) {
			http.Redirect(w, r, "/settings?notice=pw_short", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	if err := srv.Store.SetPassword(r.Context(), ac.User.ID, hash); err != nil {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	srv.SetSessionCookie(w, "", -1)
	http.Redirect(w, r, "/login?notice=password", http.StatusSeeOther)
}

func (srv *Web) HandleSettingsNamePOST(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	if err := srv.Store.SetName(r.Context(), ac.User.ID, r.Form.Get("name")); err != nil {
		if errors.Is(err, store.ErrNameRequired) || errors.Is(err, store.ErrNameTooLong) {
			http.Redirect(w, r, "/settings?notice=name", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings?notice=named", http.StatusSeeOther)
}

func (srv *Web) HandleSettingsAPIKeyPOST(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermAPIKeysManage) || ac.Method != auth.AuthSession {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	raw, prefix, scope, err := srv.CreateAPIKeyForUser(w, r, ac.User.ID, ac.User.Role)
	if errors.Is(err, core.ErrBadAPIKeyScope) || errors.Is(err, core.ErrForbiddenAPIKeyScope) {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	if err != nil {
		slog.Error("settings create key", "error", err)
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	keys, listErr := srv.Store.ListAPIKeysByUser(r.Context(), ac.User.ID)
	if listErr != nil {
		slog.Error("settings list keys after create", "error", listErr)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := srv.PageShell()
	core.MergeActorData(data, ac)
	data["Nav"] = "settings"
	data["APIKeys"] = keys
	data["CanCreateReadKey"] = ac.User.Role == auth.RoleAdmin
	data["NewAPIKey"] = raw
	data["NewAPIKeyPrefix"] = prefix
	data["NewAPIKeyScope"] = scope
	data["NoticeKind"] = "ok"
	data["NoticeText"] = "API key created. Copy it now — it will not be shown again."
	_ = tmpl.Settings.Execute(w, data)
}

func (srv *Web) HandleSettingsAPIKeyRevokePOST(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, err := core.ParseUserID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := srv.Store.DeleteAPIKey(r.Context(), id, ac.User.ID); err != nil {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	if err := srv.RequireUserAudit(r, "apikey.revoke", strconv.FormatInt(id, 10), ac.User.Username); err != nil {
		http.Error(w, "audit unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?notice=revoked", http.StatusSeeOther)
}

func settingsNotice(token string) (kind, text string) {
	switch token {
	case "password":
		return "ok", "Password updated."
	case "named":
		return "ok", "Name updated."
	case "name":
		return "err", "Name is required (max 80 characters)."
	case "revoked":
		return "ok", "API key deleted."
	case "pw_short":
		return "err", "Password must be at least 8 characters."
	case "error":
		return "err", "That action failed. Check the form and try again."
	default:
		return "", ""
	}
}
