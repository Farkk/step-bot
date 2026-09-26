package profile

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"step-bot/internal/max"
)

type Handler struct {
	DB        *sql.DB
	BotToken  string
	LocalMode bool
}

type ContactProof struct {
	AuthDate string `json:"authDate"`
	Hash     string `json:"hash"`
}
type Profile struct {
	FullName      string        `json:"fullName"`
	Phone         string        `json:"phone"`
	Gender        string        `json:"gender"`
	Age           int           `json:"age"`
	PhoneVerified bool          `json:"phoneVerified"`
	ContactProof  *ContactProof `json:"contactProof,omitempty"`
}

var phonePattern = regexp.MustCompile(`^\+?[0-9]{10,15}$`)
var namePartPattern = regexp.MustCompile(`^[\p{L}][\p{L}'’-]+$`)

func validFullName(name string) bool {
	parts := strings.Fields(name)
	if len([]rune(name)) > 150 || len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if !namePartPattern.MatchString(part) {
			return false
		}
	}
	return true
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, err := h.authenticate(r)
	if err != nil {
		http.Error(w, "Недействительные данные запуска MAX", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.get(w, r, user)
	case http.MethodPut:
		h.put(w, r, user)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h Handler) authenticate(r *http.Request) (max.User, error) {
	data := r.Header.Get("X-Max-Init-Data")
	if data == "local-preview" {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if h.LocalMode && (host == "localhost" || host == "127.0.0.1" || host == "::1") {
			return max.User{ID: -1}, nil
		}
		return max.User{}, max.ErrInvalidLaunch
	}
	return max.VerifyLaunch(data, h.BotToken, time.Now())
}

func (h Handler) Authenticate(r *http.Request) (max.User, error) { return h.authenticate(r) }

func (h Handler) get(w http.ResponseWriter, r *http.Request, user max.User) {
	var p Profile
	err := h.DB.QueryRowContext(r.Context(), `SELECT u.full_name, u.phone, u.gender, u.age, u.phone_verified FROM users u JOIN max_identities m ON m.user_id=u.id WHERE m.max_id=$1`, user.ID).Scan(&p.FullName, &p.Phone, &p.Gender, &p.Age, &p.PhoneVerified)
	if errors.Is(err, sql.ErrNoRows) {
		respond(w, http.StatusOK, map[string]any{"registered": false})
		return
	}
	if err != nil {
		http.Error(w, "Не удалось загрузить профиль", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, map[string]any{"registered": true, "profile": p})
}

func (h Handler) put(w http.ResponseWriter, r *http.Request, user max.User) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var p Profile
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		http.Error(w, "Некорректные данные профиля", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "Некорректные данные профиля", http.StatusBadRequest)
		return
	}
	p.FullName = strings.Join(strings.Fields(p.FullName), " ")
	p.Phone = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(p.Phone, " ", ""), "-", ""), "(", ""), ")", "")
	if !validFullName(p.FullName) || !phonePattern.MatchString(p.Phone) || p.Age < 1 || p.Age > 120 || (p.Gender != "male" && p.Gender != "female") {
		http.Error(w, "Проверьте поля профиля", http.StatusBadRequest)
		return
	}
	if p.ContactProof != nil {
		if err := max.VerifyContact(p.Phone, p.ContactProof.AuthDate, p.ContactProof.Hash, user.ID, h.BotToken, time.Now()); err != nil {
			http.Error(w, "Номер из MAX не подтверждён", http.StatusBadRequest)
			return
		}
		p.PhoneVerified = true
	} else {
		p.PhoneVerified = false
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Не удалось сохранить профиль", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock($1)`, user.ID); err != nil {
		http.Error(w, "Не удалось сохранить профиль", http.StatusInternalServerError)
		return
	}
	var id int64
	err = tx.QueryRowContext(r.Context(), `SELECT user_id FROM max_identities WHERE max_id=$1`, user.ID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.QueryRowContext(r.Context(), `INSERT INTO users(full_name, phone, phone_verified, gender, age) VALUES($1,$2,$3,$4,$5) RETURNING id`, p.FullName, p.Phone, p.PhoneVerified, p.Gender, p.Age).Scan(&id); err == nil {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO max_identities(max_id,user_id,display_name) VALUES($1,$2,$3)`, user.ID, id, strings.TrimSpace(user.FirstName+" "+user.LastName))
		}
	} else if err == nil {
		err = tx.QueryRowContext(r.Context(), `UPDATE users SET full_name=$1,phone=$2,phone_verified=CASE WHEN $3 THEN true WHEN phone=$2 THEN phone_verified ELSE false END,gender=$4,age=$5,updated_at=now() WHERE id=$6 RETURNING phone_verified`, p.FullName, p.Phone, p.PhoneVerified, p.Gender, p.Age, id).Scan(&p.PhoneVerified)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE max_identities SET display_name=$1,updated_at=now() WHERE max_id=$2`, strings.TrimSpace(user.FirstName+" "+user.LastName), user.ID)
		}
	}
	if err != nil || tx.Commit() != nil {
		http.Error(w, "Не удалось сохранить профиль", http.StatusInternalServerError)
		return
	}
	p.ContactProof = nil
	respond(w, http.StatusOK, map[string]any{"registered": true, "profile": p, "userId": strconv.FormatInt(id, 10)})
}

func respond(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
