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
	Rating        float64   `json:"rating"`
	Ratings       int       `json:"ratings"`
	Completed     int       `json:"completed"`
	RegisteredAt  time.Time `json:"registeredAt"`
}

func (h Handler) executors(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.Authorize(w, r, false); !ok {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT u.id,u.full_name,u.phone,u.phone_verified,u.gender,u.age,u.created_at,COALESCE(r.rating,0),COALESCE(r.ratings,0),COALESCE(done.completed,0)
FROM users u JOIN max_identities m ON m.user_id=u.id
LEFT JOIN (SELECT t.assigned_user_id,AVG(tr.score) rating,COUNT(*) ratings FROM task_ratings tr JOIN tasks t ON t.id=tr.task_id GROUP BY t.assigned_user_id) r ON r.assigned_user_id=u.id
LEFT JOIN (SELECT assigned_user_id,COUNT(*) completed FROM tasks WHERE status='completed' GROUP BY assigned_user_id) done ON done.assigned_user_id=u.id
WHERE m.max_id>0
ORDER BY (COALESCE(r.ratings,0)>0) DESC,COALESCE(r.rating,0) DESC,COALESCE(r.ratings,0) DESC,COALESCE(done.completed,0) DESC,u.created_at DESC,u.id DESC`)
	if err != nil {
		http.Error(w, "Не удалось загрузить исполнителей", 500)
		return
	}
	defer rows.Close()
	items := []executor{}
	for rows.Next() {
		var item executor
		if err := rows.Scan(&item.ID, &item.FullName, &item.Phone, &item.PhoneVerified, &item.Gender, &item.Age, &item.RegisteredAt, &item.Rating, &item.Ratings, &item.Completed); err != nil {
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
