package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/auth"
)

var (
	ErrBadAPIKeyScope       = errors.New("bad scope")
	ErrForbiddenAPIKeyScope = errors.New("forbidden scope")
)

func (c *Core) CreateAPIKeyForUser(w http.ResponseWriter, r *http.Request, userID int64, role auth.Role) (raw, prefix string, scope auth.KeyScope, err error) {
	scope = auth.KeyScopeUpload
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Scope string `json:"scope"`
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if decErr := dec.Decode(&body); decErr == nil && body.Scope != "" {
			scope = auth.KeyScope(body.Scope)
		}
	} else if err = r.ParseForm(); err == nil {
		if v := strings.TrimSpace(r.Form.Get("scope")); v != "" {
			scope = auth.KeyScope(v)
		}
	}
	if scope != auth.KeyScopeUpload && scope != auth.KeyScopeRead {
		return "", "", "", ErrBadAPIKeyScope
	}
	if role == auth.RoleViewer {
		return "", "", "", ErrForbiddenAPIKeyScope
	}
	if role == auth.RoleUploader && scope == auth.KeyScopeRead {
		return "", "", "", ErrForbiddenAPIKeyScope
	}
	raw, hash, prefix, err := auth.NewAPIKey()
	if err != nil {
		return "", "", "", err
	}
	if err := c.Store.CreateAPIKey(r.Context(), userID, hash, prefix, scope); err != nil {
		return "", "", "", err
	}
	return raw, prefix, scope, nil
}
