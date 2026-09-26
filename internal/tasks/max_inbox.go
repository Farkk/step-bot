package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"step-bot/internal/max"
	"step-bot/internal/storage"
)

type maxEvent struct {
	UpdateType string `json:"update_type"`
	Callback   struct {
		Payload    string `json:"payload"`
		CallbackID string `json:"callback_id"`
		User       struct {
			UserID int64 `json:"user_id"`
		} `json:"user"`
	} `json:"callback"`
	User struct {
		UserID int64 `json:"user_id"`
	} `json:"user"`
	Message struct {
		Sender struct {
			UserID int64 `json:"user_id"`
		} `json:"sender"`
		Body struct {
			Text string `json:"text"`
		} `json:"body"`
	} `json:"message"`
}

func RunMaxInbox(ctx context.Context, db *sql.DB, store *storage.ObjectStore, sender max.Sender, logger *slog.Logger) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if err := processMaxEvent(ctx, db, store, sender); err != nil && ctx.Err() == nil {
			logger.Error("Max inbox", "error", err)
		}
	}
}
func processMaxEvent(ctx context.Context, db *sql.DB, store *storage.ObjectStore, sender max.Sender) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	var payload []byte
	err = tx.QueryRowContext(ctx, `SELECT id,payload FROM max_webhook_inbox WHERE processed_at IS NULL ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &payload)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var event maxEvent
	if err = json.Unmarshal(payload, &event); err != nil {
		if _, updateErr := tx.ExecContext(ctx, `UPDATE max_webhook_inbox SET processed_at=now(),last_error=$2 WHERE id=$1`, id, err.Error()); updateErr != nil {
			return updateErr
		}
		return tx.Commit()
	}
	result := ""
	callbackID := ""
	var photoKey, photoFilename string
	var photoRecipient int64
	replyUserID := event.Message.Sender.UserID
	if event.UpdateType == "message_callback" && strings.HasPrefix(event.Callback.Payload, "apply:") {
		taskID, parseErr := strconv.ParseInt(strings.TrimPrefix(event.Callback.Payload, "apply:"), 10, 64)
		if parseErr != nil || taskID < 1 {
			result = "Некорректная заявка"
		} else {
			maxID := event.Callback.User.UserID
			if maxID == 0 {
				maxID = event.User.UserID
			}
			result, err = applyFromMax(ctx, tx, taskID, maxID)
			if err != nil {
				return err
			}
		}
		callbackID = event.Callback.CallbackID
	}
	if event.UpdateType == "message_callback" && strings.HasPrefix(event.Callback.Payload, "photo:") {
		callbackID = event.Callback.CallbackID
		photoRecipient = event.Callback.User.UserID
		if photoRecipient == 0 {
			photoRecipient = event.User.UserID
		}
		taskID, parseErr := strconv.ParseInt(strings.TrimPrefix(event.Callback.Payload, "photo:"), 10, 64)
		if parseErr != nil || taskID < 1 || photoRecipient < 1 {
			result = "Некорректная заявка"
		} else {
			err = tx.QueryRowContext(ctx, `SELECT a.object_key,a.filename FROM task_attachments a JOIN tasks t ON t.id=a.task_id JOIN max_identities m ON m.max_id=$2 WHERE a.task_id=$1 AND a.content_type LIKE 'image/%' AND (t.status='open' OR t.assigned_user_id=m.user_id) ORDER BY a.id LIMIT 1`, taskID, photoRecipient).Scan(&photoKey, &photoFilename)
			if errors.Is(err, sql.ErrNoRows) {
				result = "Фото недоступно или заявка закрыта"
				err = nil
			} else if err != nil {
				return err
			}
		}
	}
	if event.UpdateType == "message_created" && strings.TrimSpace(event.Message.Body.Text) == "/tasks" {
		result, err = listTasksFromMax(ctx, tx, event.Message.Sender.UserID)
		if err != nil {
			return err
		}
	}
	if event.UpdateType == "bot_started" {
		result = "Откройте Mini App ШАГ, заполните профиль и отправьте /tasks, чтобы получить доступные заявки."
		replyUserID = event.User.UserID
	}
	if _, err = tx.ExecContext(ctx, `UPDATE max_webhook_inbox SET processed_at=now(),last_error=NULL WHERE id=$1`, id); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if photoKey != "" && sender.Token != "" {
		data, getErr := store.Get(ctx, photoKey)
		if getErr == nil {
			getErr = sender.SendImage(ctx, photoRecipient, photoFilename, data)
		}
		if getErr != nil {
			result = "Не удалось отправить фото. Попробуйте ещё раз."
		} else {
			result = "Фото отправлено"
		}
	}
	if callbackID != "" && sender.Token != "" {
		if err = sender.Answer(ctx, callbackID, result); err != nil {
			return err
		}
	}
	if result != "" && callbackID == "" && replyUserID > 0 && sender.Token != "" {
		return sender.Send(ctx, replyUserID, result)
	}
	return nil
}
func listTasksFromMax(ctx context.Context, tx *sql.Tx, maxID int64) (string, error) {
	var userID int64
	err := tx.QueryRowContext(ctx, `SELECT user_id FROM max_identities WHERE max_id=$1`, maxID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "Сначала заполните профиль в Mini App ШАГ", nil
	}
	if err != nil {
		return "", err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,title FROM tasks WHERE status='open' ORDER BY created_at DESC LIMIT 5`)
	if err != nil {
		return "", err
	}
	type summary struct {
		id    int64
		title string
	}
	tasks := []summary{}
	for rows.Next() {
		var item summary
		if err = rows.Scan(&item.id, &item.title); err != nil {
			rows.Close()
			return "", err
		}
		tasks = append(tasks, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	for _, task := range tasks {
		_, err = tx.ExecContext(ctx, `INSERT INTO notification_outbox(task_id,user_id,text) VALUES($1,$2,$3)`, task.id, userID, fmt.Sprintf("Новая заявка «%s»", task.title))
		if err != nil {
			return "", err
		}
	}
	if len(tasks) == 0 {
		return "Открытых заявок пока нет", nil
	}
	return fmt.Sprintf("Отправляю %d последних заявок", len(tasks)), nil
}
func applyFromMax(ctx context.Context, tx *sql.Tx, taskID, maxID int64) (string, error) {
	if maxID <= 0 {
		return "Сначала заполните профиль в Mini App", nil
	}
	var userID int64
	err := tx.QueryRowContext(ctx, `SELECT user_id FROM max_identities WHERE max_id=$1`, maxID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "Сначала заполните профиль в Mini App", nil
	}
	if err != nil {
		return "", err
	}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id=$1 FOR UPDATE`, taskID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "Заявка не найдена", nil
	}
	if err != nil {
		return "", err
	}
	if status != "open" {
		return "Заявка больше не открыта", nil
	}
	var applicationID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO applications(task_id,user_id) VALUES($1,$2) ON CONFLICT(task_id,user_id) DO NOTHING RETURNING id`, taskID, userID).Scan(&applicationID)
	if errors.Is(err, sql.ErrNoRows) {
		return "Вы уже откликнулись", nil
	}
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO task_events(task_id,kind,actor_user_id) VALUES($1,'application_created',$2)`, taskID, userID)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_outbox(task_id,user_id,text) VALUES($1,$2,$3)`, taskID, userID, "Отклик на заявку отправлен")
	if err != nil {
		return "", err
	}
	return "Отклик на заявку отправлен", nil
}
