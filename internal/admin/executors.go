package admin

import (
	"net/http"
	"time"
)

type executor struct {
	ID            int64     `json:"id"`
	FullName      string    `json:"fullName"`
	Phone         string    `json:"phone"`
	PhoneVerified bool      `json:"phoneVerified"`
	Gender        string    `json:"gender"`
	Age           int       `json:"age"`
	RegisteredAt  time.Time `json:"registeredAt"`
}

func (h Handler) executors(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.Authorize(w, r, false); !ok {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT u.id,u.full_name,u.phone,u.phone_verified,u.gender,u.age,u.created_at FROM users u JOIN max_identities m ON m.user_id=u.id WHERE m.max_id>0 ORDER BY u.created_at DESC,u.id DESC`)
	if err != nil {
		http.Error(w, "Не удалось загрузить исполнителей", 500)
		return
	}
	defer rows.Close()
	items := []executor{}
	for rows.Next() {
		var item executor
		if err := rows.Scan(&item.ID, &item.FullName, &item.Phone, &item.PhoneVerified, &item.Gender, &item.Age, &item.RegisteredAt); err != nil {
			http.Error(w, "Не удалось загрузить исполнителей", 500)
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		http.Error(w, "Не удалось загрузить исполнителей", 500)
		return
	}
	respond(w, items)
}
