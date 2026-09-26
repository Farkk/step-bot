package max

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type Webhook struct {
	DB     *sql.DB
	Secret string
}

func (w Webhook) ServeHTTP(out http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(out, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	supplied := r.Header.Get("X-Max-Bot-Api-Secret")
	if subtle.ConstantTimeCompare([]byte(supplied), []byte(w.Secret)) != 1 {
		http.Error(out, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(out, r.Body, 1<<20)
	var payload json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || !json.Valid(payload) {
		http.Error(out, "invalid JSON", http.StatusBadRequest)
		return
	}
	hash := sha256.Sum256(payload)
	_, err := w.DB.ExecContext(r.Context(),
		`INSERT INTO max_webhook_inbox(event_hash, payload) VALUES($1, $2) ON CONFLICT (event_hash) DO NOTHING`,
		hex.EncodeToString(hash[:]), string(payload))
	if err != nil {
		http.Error(out, "storage unavailable", http.StatusServiceUnavailable)
		return
	}
	out.WriteHeader(http.StatusOK)
}
