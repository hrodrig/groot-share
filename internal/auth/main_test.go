package auth

import (
	"os"
	"testing"
)

// TestMain lowers the bcrypt work factor for the whole package so the hash/
// compare tests stay fast. Production code paths keep bcrypt.DefaultCost.
func TestMain(m *testing.M) {
	UseTestCost()
	os.Exit(m.Run())
}
