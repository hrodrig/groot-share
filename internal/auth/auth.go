// Package auth hashes passwords, issues api keys, and extracts keys from headers.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// MinPasswordLen is the shortest accepted password (bootstrap and create user).
	MinPasswordLen = 8
	apiKeyBytes    = 32
	sessionBytes   = 32
	bcryptCost     = bcrypt.DefaultCost
)

// dummyHash is used when the user is unknown so Compare still burns bcrypt time.
var dummyHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("dummy-password-not-used"), bcryptCost)
	if err != nil {
		panic(err)
	}
	dummyHash = h
}

// HashPassword returns a bcrypt hash. Rejects short passwords.
func HashPassword(password string) (string, error) {
	if len(password) < MinPasswordLen {
		return "", fmt.Errorf("password must be at least %d characters", MinPasswordLen)
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(b), nil
}

// CheckPassword reports whether password matches hash. Unknown user still hashes.
func CheckPassword(hash, password string) bool {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// NewAPIKey returns a raw key (show once) and its SHA-256 hex.
func NewAPIKey() (raw, hash, prefix string, err error) {
	var b [apiKeyBytes]byte
	if _, err = rand.Read(b[:]); err != nil {
		return "", "", "", fmt.Errorf("rand api key: %w", err)
	}
	raw = "gfs_" + hex.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	prefix = raw
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return raw, hash, prefix, nil
}

// HashSecret SHA-256 hex-encodes s (sessions and presented api keys).
func HashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// EqualHash is constant-time compare for equal-length hex hashes.
func EqualHash(a, b string) bool {
	x, y := []byte(a), []byte(b)
	if len(x) != len(y) {
		return false
	}
	return subtle.ConstantTimeCompare(x, y) == 1
}

// NewSessionToken returns a raw cookie value and its SHA-256 hex.
func NewSessionToken() (raw, hash string, err error) {
	var b [sessionBytes]byte
	if _, err = rand.Read(b[:]); err != nil {
		return "", "", fmt.Errorf("rand session: %w", err)
	}
	raw = hex.EncodeToString(b[:])
	return raw, HashSecret(raw), nil
}

// ExtractKey returns the presented API key from X-API-Key or Bearer only.
func ExtractKey(r *http.Request) string {
	if h := strings.TrimSpace(r.Header.Get("X-API-Key")); h != "" {
		return h
	}
	ah := strings.TrimSpace(r.Header.Get("Authorization"))
	const p = "Bearer "
	if len(ah) > len(p) && strings.EqualFold(ah[:len(p)], p) {
		return strings.TrimSpace(ah[len(p):])
	}
	return ""
}
