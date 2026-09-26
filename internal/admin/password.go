package admin

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func validNewPassword(s string) bool { return len(s) >= 12 && len(s) <= 128 }
func (h Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	m, csrf, err := h.Current(r)
	if err != nil {
		http.Error(w, "Требуется вход", 401)
		return
	}
	if r.Header.Get("X-CSRF-Token") != csrf || r.Header.Get("Origin") != "" && !sameOrigin(r) {
		http.Error(w, "Доступ запрещён", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Current string `json:"current"`
		Next    string `json:"next"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || !validNewPassword(input.Next) {
		http.Error(w, "Новый пароль должен содержать 12–128 символов", 400)
		return
	}
	var hash string
	err = h.DB.QueryRowContext(r.Context(), `SELECT password_hash FROM company_members WHERE id=$1`, m.ID).Scan(&hash)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Current)) != nil {
		http.Error(w, "Текущий пароль неверен", 403)
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(input.Next), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Ошибка сервера", 500)
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `UPDATE company_members SET password_hash=$1 WHERE id=$2`, string(newHash), m.ID)
	if err != nil {
		http.Error(w, "Не удалось изменить пароль", 500)
		return
	}
	w.WriteHeader(204)
}
