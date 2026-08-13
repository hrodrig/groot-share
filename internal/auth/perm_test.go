package auth

import "testing"

func TestCanSessionMatrix(t *testing.T) {
	cases := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RoleViewer, PermArchivesRead, true},
		{RoleViewer, PermArchivesWrite, false},
		{RoleViewer, PermArchivesDelete, false},
		{RoleViewer, PermAPIKeysManage, false},
		{RoleUploader, PermArchivesWrite, true},
		{RoleUploader, PermArchivesDelete, false},
		{RoleUploader, PermAPIKeysManage, true},
		{RoleAdmin, PermArchivesDelete, true},
		{RoleAdmin, PermUsersManage, true},
	}
	for _, c := range cases {
		if got := Can(c.role, c.perm, AuthSession, ""); got != c.want {
			t.Fatalf("session %s %s = %v want %v", c.role, c.perm, got, c.want)
		}
	}
}

func TestValidRoleAndScope(t *testing.T) {
	if ValidRole(Role("root")) {
		t.Fatal("invalid role")
	}
	if ValidKeyScope(KeyScope("admin")) {
		t.Fatal("invalid scope")
	}
}

func TestCanAPIKeyMatrix(t *testing.T) {
	if Can(RoleAdmin, PermArchivesRead, AuthAPIKey, KeyScopeUpload) {
		t.Fatal("upload key cannot read")
	}
	if !Can(RoleUploader, PermArchivesWrite, AuthAPIKey, KeyScopeUpload) {
		t.Fatal("upload key can write")
	}
	if Can(RoleUploader, PermArchivesRead, AuthAPIKey, KeyScopeUpload) {
		t.Fatal("upload key cannot read")
	}
	if !Can(RoleViewer, PermArchivesRead, AuthAPIKey, KeyScopeRead) {
		t.Fatal("read key can list")
	}
	if Can(RoleViewer, PermArchivesWrite, AuthAPIKey, KeyScopeRead) {
		t.Fatal("read key cannot write")
	}
	if Can(RoleAdmin, PermArchivesDelete, AuthAPIKey, KeyScopeRead) {
		t.Fatal("api key never deletes")
	}
	if Can(RoleAdmin, PermUsersManage, AuthAPIKey, KeyScopeRead) {
		t.Fatal("api key never manages users")
	}
}
