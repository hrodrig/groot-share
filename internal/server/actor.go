package server

import (
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/server/core"
)

func (s *Server) requirePermission(perm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := s.ActorFromRequest(r)
		if ac == nil {
			s.authFail(w, r)
			return
		}
		if !ac.Can(perm) {
			s.forbidden(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(core.WithActor(r.Context(), ac)))
	}
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := s.ActorFromRequest(r)
		if ac == nil {
			s.authFail(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(core.WithActor(r.Context(), ac)))
	}
}

func (s *Server) authFail(w http.ResponseWriter, r *http.Request) {
	if core.WantsJSON(r) || strings.HasPrefix(r.URL.Path, "/v1/") {
		core.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) forbidden(w http.ResponseWriter, r *http.Request) {
	if core.WantsJSON(r) || strings.HasPrefix(r.URL.Path, "/v1/") {
		core.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	http.Error(w, "Forbidden", http.StatusForbidden)
}
