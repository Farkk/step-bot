package tasks

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkerNotificationsRequiresVerifiedLaunch(t *testing.T) {
	mux := http.NewServeMux()
	(Handler{}).Routes(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/worker/notifications", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
