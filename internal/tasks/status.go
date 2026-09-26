package tasks

import (
	"context"
	"database/sql"
	"fmt"
)

func nextStatus(current, action string) string {
	switch current + ":" + action {
	case "assigned:start", "paused:resume":
		return "in_progress"
	case "in_progress:pause":
		return "paused"
	case "in_progress:complete":
		return "awaiting_confirmation"
	case "awaiting_confirmation:confirm":
		return "completed"
	case "open:cancel", "assigned:cancel", "in_progress:cancel", "paused:cancel", "awaiting_confirmation:cancel":
		return "cancelled"
	}
	return ""
}

func enqueue(ctx context.Context, tx *sql.Tx, taskID int64, userID int64, status, title string) error {
	message := fmt.Sprintf("Заявка №%d «%s»: %s. Откройте ШАГ в MAX, чтобы увидеть детали.", taskID, title, statusLabel(status))
	_, err := tx.ExecContext(ctx, `INSERT INTO notification_outbox(task_id,user_id,text) SELECT $1,m.user_id,$3 FROM max_identities m WHERE m.user_id=$2 AND m.max_id>0`, taskID, userID, message)
	return err
}

func statusLabel(status string) string {
	switch status {
	case "open":
		return "Открыта"
	case "assigned":
		return "Назначен исполнитель"
	case "in_progress":
		return "На исполнении"
	case "paused":
		return "Приостановлена"
	case "awaiting_confirmation":
		return "Ожидает подтверждения"
	case "completed":
		return "Завершена"
	case "cancelled":
		return "Отменена"
	case "rejected":
		return "Отклик отклонён"
	}
	return status
}
