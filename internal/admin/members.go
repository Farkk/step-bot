package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type teamMember struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (h Handler) owner(w http.ResponseWriter, r *http.Request) (Member, bool) {
	m, ok := h.Authorize(w, r, true)
	if !ok {
		return m, false
	}
	if m.Role != "owner" {
		http.Error(w, "Только владелец управляет командой", 403)
		return m, false
	}
	return m, true
}
func (h Handler) members(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Authorize(w, r, false)
	if !ok {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,full_name,email,role FROM company_members WHERE company_id=$1 ORDER BY id`, m.CompanyID)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer rows.Close()
	items := []teamMember{}
	for rows.Next() {
		var item teamMember
		if rows.Scan(&item.ID, &item.Name, &item.Email, &item.Role) != nil {
			http.Error(w, "Ошибка базы", 500)
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	respond(w, items)
}
func (h Handler) createMember(w http.ResponseWriter, r *http.Request) {
	m, ok := h.owner(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		http.Error(w, "Некорректные данные", 400)
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if len([]rune(input.Name)) < 3 || len([]rune(input.Name)) > 150 || !validEmail(input.Email) || (input.Role != "manager" && input.Role != "viewer") || len(input.Password) < 12 {
		http.Error(w, "Укажите имя, email, роль и пароль от 12 символов", 400)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Ошибка сервера", 500)
		return
	}
	var id int64
	err = h.DB.QueryRowContext(r.Context(), `INSERT INTO company_members(company_id,email,password_hash,full_name,role) VALUES($1,$2,$3,$4,$5) RETURNING id`, m.CompanyID, input.Email, string(hash), input.Name, input.Role).Scan(&id)
	if err != nil {
		http.Error(w, "Не удалось создать сотрудника; проверьте email", 409)
		return
	}
	w.WriteHeader(201)
	respond(w, teamMember{ID: id, Name: input.Name, Email: input.Email, Role: input.Role})
}
func (h Handler) changeRole(w http.ResponseWriter, r *http.Request) {
	m, ok := h.owner(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "Некорректный номер", 400)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var input struct {
		Role string `json:"role"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || (input.Role != "manager" && input.Role != "viewer") {
		http.Error(w, "Некорректная роль", 400)
		return
	}
	res, err := h.DB.ExecContext(r.Context(), `UPDATE company_members SET role=$1 WHERE id=$2 AND company_id=$3 AND role<>'owner'`, input.Role, id, m.CompanyID)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "Сотрудник не найден", 404)
		return
	}
	respond(w, map[string]string{"role": input.Role})
}
