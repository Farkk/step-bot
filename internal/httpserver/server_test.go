package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServesAdminAndMiniApp(t *testing.T) {
	root := t.TempDir()
	for _, app := range []string{"app", "admin"} {
		dir := filepath.Join(root, app)
		if err := os.MkdirAll(dir, 0700); err != nil { t.Fatal(err) }
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(app+" index"), 0600); err != nil { t.Fatal(err) }
		if err := os.WriteFile(filepath.Join(dir, "asset.css"), []byte(app+" asset"), 0600); err != nil { t.Fatal(err) }
	}
	handler := New(nil, "", "", "", "local", filepath.Join(root, "app"))
	for _, tc := range []struct{ path, want string }{
		{"/app/", "app index"}, {"/app/tasks", "app index"}, {"/app/asset.css", "app asset"},
		{"/admin/", "admin index"}, {"/admin/tasks", "admin index"}, {"/admin/asset.css", "admin asset"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), tc.want) {
				t.Fatalf("GET %s: status=%d body=%q", tc.path, res.Code, res.Body.String())
			}
		})
	}
}
