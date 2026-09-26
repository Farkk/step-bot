package admin

import "testing"

func TestEmailValidation(t *testing.T) {
	if !validEmail("owner@example.ru") {
		t.Fatal("valid email rejected")
	}
	for _, email := range []string{"", "owner", "a@", "a@b c"} {
		if validEmail(email) {
			t.Fatalf("invalid email accepted: %q", email)
		}
	}
}

func TestLoginLimiter(t *testing.T) {
	l := NewLoginLimiter()
	for i := 0; i < 10; i++ {
		if !l.allowed("127.0.0.1") {
			t.Fatal("blocked too early")
		}
		l.failed("127.0.0.1")
	}
	if l.allowed("127.0.0.1") {
		t.Fatal("too many failures allowed")
	}
	if !l.allowed("127.0.0.2") {
		t.Fatal("other address blocked")
	}
	l.clear("127.0.0.1")
	if !l.allowed("127.0.0.1") {
		t.Fatal("clear failed")
	}
}
