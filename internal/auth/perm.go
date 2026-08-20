package auth

// Role is a user's permission tier (session auth).
type Role string

const (
	RoleViewer   Role = "viewer"
	RoleUploader Role = "uploader"
	RoleAdmin    Role = "admin"
)

// Permission is a guarded capability on a route.
type Permission string

const (
	PermArchivesRead   Permission = "archives:read"
	PermArchivesWrite  Permission = "archives:write"
	PermArchivesDelete Permission = "archives:delete"
	PermAuditRead      Permission = "audit:read"
	PermUsersManage    Permission = "users:manage"
	PermAPIKeysManage  Permission = "apikeys:manage"
	PermSharesManage   Permission = "shares:manage"
)

// AuthMethod is how the request authenticated.
type AuthMethod string

const (
	AuthSession AuthMethod = "session"
	AuthAPIKey  AuthMethod = "api_key"
)

// KeyScope is the capability of an api_key (empty for session auth).
type KeyScope string

const (
	KeyScopeUpload KeyScope = "upload"
	KeyScopeRead   KeyScope = "read"
)

// ValidRole reports whether r is a known role string.
func ValidRole(r Role) bool {
	switch r {
	case RoleViewer, RoleUploader, RoleAdmin:
		return true
	default:
		return false
	}
}

// ValidKeyScope reports whether s is a known api_key scope.
func ValidKeyScope(s KeyScope) bool {
	return s == KeyScopeUpload || s == KeyScopeRead
}

// Can reports whether role/method/scope may perform perm.
func Can(role Role, perm Permission, method AuthMethod, scope KeyScope) bool {
	if method == AuthAPIKey {
		return canAPIKey(role, perm, scope)
	}
	return canSession(role, perm)
}

func canAPIKey(role Role, perm Permission, scope KeyScope) bool {
	switch perm {
	case PermArchivesWrite:
		return scope == KeyScopeUpload && (role == RoleUploader || role == RoleAdmin)
	case PermArchivesRead, PermAuditRead:
		return scope == KeyScopeRead && (role == RoleViewer || role == RoleUploader || role == RoleAdmin)
	default:
		return false
	}
}

func canSession(role Role, perm Permission) bool {
	switch perm {
	case PermArchivesRead, PermAuditRead:
		return role == RoleViewer || role == RoleUploader || role == RoleAdmin
	case PermArchivesWrite:
		return role == RoleUploader || role == RoleAdmin
	case PermArchivesDelete, PermUsersManage:
		return role == RoleAdmin
	case PermSharesManage:
		return role == RoleAdmin
	case PermAPIKeysManage:
		return role == RoleUploader || role == RoleAdmin
	default:
		return false
	}
}
