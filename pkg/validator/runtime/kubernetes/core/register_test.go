package validation

import "testing"

func TestRegister(t *testing.T) {
	// Verify that Register() can be called without panicking
	// (this tests that all checks have unique IDs and valid metadata)
	Register()
}
