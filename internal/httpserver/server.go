package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"step-bot/internal/admin"
	"step-bot/internal/max"
	"step-bot/internal/profile"
	"step-bot/internal/storage"
	"step-bot/internal/tasks"
)

func New(db *sql.DB, s3Endpoint, webhookSecret, botToken, appEnv, staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		endpoint, err := url.Parse(s3Endpoint)
		if err != nil {
			http.Error(w, "storage endpoint invalid", http.StatusServiceUnavailable)
			return
		}
		endpoint.Path = "/minio/health/live"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			http.Error(w, "storage endpoint invalid", http.StatusServiceUnavailable)
			return
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})
	mux.Handle("POST /integrations/max/webhook", max.Webhook{DB: db, Secret: webhookSecret})
	mux.Handle("/api/v1/me/profile", profile.Handler{DB: db, BotToken: botToken, LocalMode: appEnv == "local"})
	adminHandler := admin.Handler{DB: db, Secure: appEnv != "local", Limiter: admin.NewLoginLimiter()}
	adminHandler.Routes(mux)
	(tasks.Handler{DB: db, Admin: adminHandler, Profile: profile.Handler{DB: db, BotToken: botToken, LocalMode: appEnv == "local"}, Store: storage.ObjectStoreFromEnv(s3Endpoint)}).Routes(mux)
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})
	mux.Handle("/app/", spa(staticDir, "/app/"))
	mux.Handle("/admin/", spa(filepath.Join(filepath.Dir(staticDir), "admin"), "/admin/"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/app/", http.StatusTemporaryRedirect)
	})
	return mux
}

func spa(dir, prefix string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, prefix)
		if path == "" {
			path = "index.html"
		}
		full := filepath.Join(dir, filepath.Clean("/"+path))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			http.StripPrefix(prefix, files).ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
