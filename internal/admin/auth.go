package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func validEmail(s string) bool { return len(s) <= 254 && emailPattern.MatchString(s) }
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func tokenHash(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

type Member struct {
	ID        int64  `json:"id"`
	CompanyID int64  `json:"companyId"`
	Company   string `json:"company"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
}
type Handler struct {
	DB      *sql.DB
	Secure  bool
	Limiter *LoginLimiter
}

type loginAttempts struct {
	count int
	until time.Time
}
type LoginLimiter struct {
	mu      sync.Mutex
	entries map[string]loginAttempts
}

func NewLoginLimiter() *LoginLimiter { return &LoginLimiter{entries: make(map[string]loginAttempts)} }
func (l *LoginLimiter) allowed(key string) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.entries[key]
	if time.Now().After(a.until) {
		delete(l.entries, key)
		return true
	}
	return a.count < 10
}
func (l *LoginLimiter) failed(key string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) > 10000 {
		for k, a := range l.entries {
			if time.Now().After(a.until) {
				delete(l.entries, k)
			}
		}
	}
	a := l.entries[key]
	if time.Now().After(a.until) {
		a = loginAttempts{until: time.Now().Add(10 * time.Minute)}
	}
	a.count++
	l.entries[key] = a
}
func (l *LoginLimiter) clear(key string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

func (h Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("GET /api/v1/auth/session", h.session)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.HandleFunc("POST /api/v1/auth/password", h.changePassword)
	mux.HandleFunc("GET /api/v1/admin/members", h.members)
	mux.HandleFunc("GET /api/v1/admin/executors", h.executors)
	mux.HandleFunc("POST /api/v1/admin/members", h.createMember)
	mux.HandleFunc("PUT /api/v1/admin/members/{id}/role", h.changeRole)
}
func (h Handler) Current(r *http.Request) (Member, string, error) {
	c, err := r.Cookie("step_admin")
	if err != nil {
		return Member{}, "", err
	}
	var m Member
	var csrf string
	err = h.DB.QueryRowContext(r.Context(), `SELECT m.id,m.company_id,c.name,m.full_name,m.email,m.role,s.csrf_token FROM admin_sessions s JOIN company_members m ON m.id=s.member_id JOIN companies c ON c.id=m.company_id WHERE s.token_hash=$1 AND s.expires_at>now()`, tokenHash(c.Value)).Scan(&m.ID, &m.CompanyID, &m.Company, &m.Name, &m.Email, &m.Role, &csrf)
	return m, csrf, err
}
func (h Handler) Authorize(w http.ResponseWriter, r *http.Request, write bool) (Member, bool) {
	m, csrf, err := h.Current(r)
	if err != nil {
		http.Error(w, "Требуется вход", http.StatusUnauthorized)
		return Member{}, false
	}
	if write && (m.Role == "viewer" || r.Header.Get("X-CSRF-Token") != csrf || r.Header.Get("Origin") != "" && !sameOrigin(r)) {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return Member{}, false
	}
	return m, true
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}
func (h Handler) login(w http.ResponseWriter, r *http.Request) {
	key, _, errAddr := net.SplitHostPort(r.RemoteAddr)
	if errAddr != nil {
		key = r.RemoteAddr
	}
	if !h.Limiter.allowed(key) {
		http.Error(w, "Слишком много попыток. Попробуйте позже", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		http.Error(w, "Некорректный запрос", 400)
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	var m Member
	var hash string
	err := h.DB.QueryRowContext(r.Context(), `SELECT m.id,m.company_id,c.name,m.full_name,m.email,m.role,m.password_hash FROM company_members m JOIN companies c ON c.id=m.company_id WHERE m.email=$1`, input.Email).Scan(&m.ID, &m.CompanyID, &m.Company, &m.Name, &m.Email, &m.Role, &hash)
	if err != nil {
		hash = "$2a$12$kfb.aNoZb1gBEqZ7PtDb6eBvgLmPAobPfKYZBxGaIkCQfvSEpXXkW"
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil || err != nil {
		h.Limiter.failed(key)
		http.Error(w, "Неверная почта или пароль", http.StatusUnauthorized)
		return
	}
	h.Limiter.clear(key)
	token, e := randomToken()
	if e != nil {
		http.Error(w, "Ошибка входа", 500)
		return
	}
	csrf, e := randomToken()
	if e != nil {
		http.Error(w, "Ошибка входа", 500)
		return
	}
	_, e = h.DB.ExecContext(r.Context(), `INSERT INTO admin_sessions(token_hash,member_id,csrf_token,expires_at) VALUES($1,$2,$3,now()+interval '7 days')`, tokenHash(token), m.ID, csrf)
	if e != nil {
		http.Error(w, "Ошибка входа", 500)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "step_admin", Value: token, Path: "/api/v1", HttpOnly: true, Secure: h.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 7 * 24 * 3600})
	respond(w, map[string]any{"member": m, "csrfToken": csrf})
}
func (h Handler) session(w http.ResponseWriter, r *http.Request) {
	m, csrf, err := h.Current(r)
	if err != nil {
		http.Error(w, "Требуется вход", 401)
		return
	}
	respond(w, map[string]any{"member": m, "csrfToken": csrf})
}
func (h Handler) logout(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" && !sameOrigin(r) {
		http.Error(w, "Доступ запрещён", 403)
		return
	}
	_, csrf, err := h.Current(r)
	if err != nil {
		http.Error(w, "Требуется вход", 401)
		return
	}
	if r.Header.Get("X-CSRF-Token") != csrf {
		http.Error(w, "Доступ запрещён", 403)
		return
	}
	c, _ := r.Cookie("step_admin")
	_, _ = h.DB.ExecContext(r.Context(), `DELETE FROM admin_sessions WHERE token_hash=$1`, tokenHash(c.Value))
	http.SetCookie(w, &http.Cookie{Name: "step_admin", Path: "/api/v1", HttpOnly: true, Secure: h.Secure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(204)
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func CreateOwner(db *sql.DB, company, email, name, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	company = strings.TrimSpace(company)
	name = strings.TrimSpace(name)
	if !validEmail(email) || company == "" || name == "" || len(password) < 12 {
		return errors.New("нужны компания, имя, корректная почта и пароль от 12 символов")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var companyID int64
	if err = tx.QueryRow(`INSERT INTO companies(name) VALUES($1) RETURNING id`, company).Scan(&companyID); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO company_members(company_id,email,password_hash,full_name,role) VALUES($1,$2,$3,$4,'owner')`, companyID, email, string(hash), name); err != nil {
		return err
	}
	return tx.Commit()
}
