package store

import (
	"errors"
	"strings"
)

// ErrNameRequired is an empty display name after trim.
var ErrNameRequired = errors.New("name is required")

// ErrNameTooLong is a display name over MaxNameRunes.
var ErrNameTooLong = errors.New("name too long")

// ErrUsernameRequired is an empty login id after trim.
var ErrUsernameRequired = errors.New("username is required")

// ErrUsernameTooLong is a login id over MaxUsernameRunes.
var ErrUsernameTooLong = errors.New("username too long")

const (
	// DefaultName is the first-admin display name when GFS_BOOTSTRAP_ADMIN_NAME is unset.
	DefaultName = "Administrator"
	// MaxNameRunes is the stored name cap.
	MaxNameRunes = 80
	// MaxUsernameRunes is the login id cap.
	MaxUsernameRunes = 64
	displayNameMax   = 30
	displayNameTail  = 4
	displayNameMid   = " ..."
)

// NormalizeUsername trims and enforces the login id length.
func NormalizeUsername(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrUsernameRequired
	}
	if len([]rune(s)) > MaxUsernameRunes {
		return "", ErrUsernameTooLong
	}
	return s, nil
}

// NormalizeName trims, collapses spaces, and enforces length.
func NormalizeName(s string) (string, error) {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "", ErrNameRequired
	}
	if len([]rune(s)) > MaxNameRunes {
		return "", ErrNameTooLong
	}
	return s, nil
}

// TruncateName shortens a long name: "Juan Carlos …egro" (30 runes, last 4 kept).
func TruncateName(s string) string {
	r := []rune(s)
	if len(r) <= displayNameMax {
		return s
	}
	mid := []rune(displayNameMid)
	prefix := displayNameMax - displayNameTail - len(mid)
	if prefix < 1 {
		return string(r[:displayNameMax])
	}
	return string(r[:prefix]) + displayNameMid + string(r[len(r)-displayNameTail:])
}

// DisplayName is the header label: name (or username), truncated.
func (u User) DisplayName() string {
	n := strings.TrimSpace(u.Name)
	if n == "" {
		n = u.Username
	}
	return TruncateName(n)
}
