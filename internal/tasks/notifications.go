package tasks

import (
	"net/http"
	"time"
)

type workerNotification struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"taskId"`
	Title     string    `json:"title"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"createdAt"`
}

// Notifications are derived from committed task events so they also work for
// local preview users who do not have a MAX identity or an outbox entry.
func (h Handler) workerNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.workerID(w, r)
	if !ok {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT e.id,t.id,t.title,e.kind,e.created_at
		FROM task_events e JOIN tasks t ON t.id=e.task_id
		WHERE e.kind='published'
		   OR (e.subject_user_id=$1 AND e.kind IN ('application_accepted','application_rejected','application_rejected_auto'))
		   OR (t.assigned_user_id=$1 AND e.kind IN ('in_progress','paused','awaiting_confirmation','completed','cancelled','rating_created','rating_updated'))
		ORDER BY e.id DESC LIMIT 50`, userID)
	if err != nil {
		http.Error(w, "Не удалось загрузить уведомления", 500)
		return
	}
	defer rows.Close()
	items := []workerNotification{}
	for rows.Next() {
		var item workerNotification
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Title, &item.Kind, &item.CreatedAt); err != nil {
			http.Error(w, "Не удалось загрузить уведомления", 500)
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		http.Error(w, "Не удалось загрузить уведомления", 500)
		return
	}
	respond(w, items)
}
