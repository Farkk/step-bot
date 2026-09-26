package profile

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileRejectsOtherGender(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "http://localhost:8080/api/v1/me/profile", strings.NewReader(`{"fullName":"Иван Петров","phone":"+79991234567","gender":"other","age":25}`))
	r.Header.Set("X-Max-Init-Data", "local-preview")
	w := httptest.NewRecorder()
	(Handler{LocalMode: true}).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", w.Code)
	}
}

func TestLocalPreviewAccess(t *testing.T) {
	for _, test := range []struct {
		name    string
		local   bool
		host    string
		allowed bool
	}{
		{"local host", true, "localhost:8080", true},
		{"loopback", true, "127.0.0.1:8080", true},
		{"production", false, "localhost:8080", false},
		{"remote host", true, "example.com", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://"+test.host+"/api/v1/me/profile", nil)
			r.Header.Set("X-Max-Init-Data", "local-preview")
			user, err := (Handler{LocalMode: test.local}).authenticate(r)
			if test.allowed && (err != nil || user.ID != -1) {
				t.Fatalf("user=%+v err=%v", user, err)
			}
			if !test.allowed && err == nil {
				t.Fatal("accepted local preview outside local mode")
			}
		})
	}
}

func TestValidFullName(t *testing.T) {
	for _, value := range []string{"Иван", "@ivan", "Иван 123", "А Б"} {
		if validFullName(value) {
			t.Errorf("accepted %q", value)
		}
	}
	for _, value := range []string{"Иван Петров", "Анна-Мария Иванова"} {
		if !validFullName(value) {
			t.Errorf("rejected %q", value)
		}
	}
}
