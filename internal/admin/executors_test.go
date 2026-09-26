package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecutorsRequireAdminSession(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/executors", nil)
	w := httptest.NewRecorder()
	(Handler{}).executors(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", w.Code, http.StatusUnauthorized)
	}
}
