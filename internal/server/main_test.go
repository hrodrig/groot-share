package server

import (
	"os"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
)

// TestMain lowers the bcrypt work factor for the whole package. Login/login-
// rejection tests hash and compare on every request; DefaultCost (10) burns
// ~2 minutes under -race, MinCost keeps the suite fast without weakening prod.
func TestMain(m *testing.M) {
	auth.UseTestCost()
	os.Exit(m.Run())
}
