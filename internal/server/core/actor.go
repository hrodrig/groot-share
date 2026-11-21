package core

import (
	"context"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

type ctxKey int

const actorCtxKey ctxKey = 1

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

// ActorFrom returns the actor stored on ctx by auth middleware.
func ActorFrom(ctx context.Context) *Actor {
	ac, _ := ctx.Value(actorCtxKey).(*Actor)
	return ac
}

// WithActor returns ctx carrying ac for downstream handlers.
func WithActor(ctx context.Context, ac *Actor) context.Context {
	return context.WithValue(ctx, actorCtxKey, ac)
}

// ShellUserData builds template fields for the signed-in user.
func ShellUserData(ac *Actor) map[string]any {
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

// MergeActorData copies ShellUserData into data.
func MergeActorData(data map[string]any, ac *Actor) {
	for k, v := range ShellUserData(ac) {
		data[k] = v
	}
}
