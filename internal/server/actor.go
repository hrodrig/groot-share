package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

// Actor is an authenticated request principal.
type Actor struct {
	User     store.User
	Method   auth.AuthMethod
	KeyScope auth.KeyScope
}

func (a *Actor) Can(perm auth.Permission) bool {
	if a == nil {
		return false
	}
	return auth.Can(a.User.Role, perm, a.Method, a.KeyScope)
}

func actorFrom(ctx context.Context) *Actor {
	ac, _ := ctx.Value(actorKey).(*Actor)
	return ac
}

func (s *Server) requirePermission(perm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := s.actorFromRequest(r)
		if ac == nil {
			s.authFail(w, r)
			return
		}
		if !ac.Can(perm) {
			s.forbidden(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey, ac)))
	}
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := s.actorFromRequest(r)
		if ac == nil {
			s.authFail(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey, ac)))
	}
}

func (s *Server) authFail(w http.ResponseWriter, r *http.Request) {
	if wantsJSON(r) || strings.HasPrefix(r.URL.Path, "/v1/") {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) forbidden(w http.ResponseWriter, r *http.Request) {
	if wantsJSON(r) || strings.HasPrefix(r.URL.Path, "/v1/") {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	http.Error(w, "Forbidden", http.StatusForbidden)
}

func shellUserData(ac *Actor) map[string]any {
	if ac == nil {
		return map[string]any{}
	}
	return map[string]any{
		"ActorID":        ac.User.ID,
		"Username":       ac.User.Username,
		"Name":           ac.User.Name,
		"DisplayName":    ac.User.DisplayName(),
		"Role":           string(ac.User.Role),
		"CanDelete":      ac.Can(auth.PermArchivesDelete),
		"CanUpload":      ac.Can(auth.PermArchivesWrite),
		"CanManageUsers": ac.Can(auth.PermUsersManage),
		"CanManageKeys":  ac.Can(auth.PermAPIKeysManage) && ac.Method == auth.AuthSession,
		"CanShares":      ac.Can(auth.PermSharesManage),
	}
}

func mergeActorData(data map[string]any, ac *Actor) {
	for k, v := range shellUserData(ac) {
		data[k] = v
	}
}
