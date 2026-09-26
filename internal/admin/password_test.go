package admin

import "testing"

func TestPasswordPolicy(t *testing.T) {
	if validNewPassword("short") || !validNewPassword("abcdefghijkl") {
		t.Fatal("password policy mismatch")
	}
}
