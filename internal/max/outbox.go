package max

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

func RunOutbox(ctx context.Context, db *sql.DB, sender Sender, logger *slog.Logger) {
	if sender.Token == "" {
		logger.Info("Max push disabled: no bot token")
		return
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if err := deliverOne(ctx, db, sender); err != nil && ctx.Err() == nil {
			logger.Error("Max push delivery", "error", err)
		}
	}
}

func deliverOne(ctx context.Context, db *sql.DB, sender Sender) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, maxID, taskID int64
	var body string
	var attempts int
	err = tx.QueryRowContext(ctx, `SELECT o.id,m.max_id,o.task_id,o.text,o.attempts FROM notification_outbox o JOIN max_identities m ON m.user_id=o.user_id WHERE o.sent_at IS NULL AND o.next_attempt_at<=now() AND m.max_id>0 ORDER BY o.next_attempt_at,o.id LIMIT 1 FOR UPDATE OF o SKIP LOCKED`).Scan(&id, &maxID, &taskID, &body, &attempts)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	taskOpen := true
	if strings.HasPrefix(body, "Новая заявка №") {
		var title, description, location, status string
		var budget int64
		var deadline time.Time
		var valuesRaw, schemaRaw []byte
		if e := tx.QueryRowContext(ctx, `SELECT title,description,budget,deadline,location,status,field_values,field_schema FROM tasks WHERE id=$1`, taskID).Scan(&title, &description, &budget, &deadline, &location, &status, &valuesRaw, &schemaRaw); e == nil {
			runes := []rune(description)
			if len(runes) > 2500 {
				description = string(runes[:2500]) + "…"
			}
			body = fmt.Sprintf("Новая заявка №%d: %s\n%s\nБюджет: %d ₽\nСрок: %s\nМесто: %s", taskID, title, description, budget, deadline.Format("02.01.2006 15:04"), location)
			var values map[string]any
			var schema []struct {
				Key   string `json:"key"`
				Label string `json:"label"`
			}
			_ = json.Unmarshal(valuesRaw, &values)
			_ = json.Unmarshal(schemaRaw, &schema)
			for _, field := range schema {
				if value, ok := values[field.Key]; ok {
					line := fmt.Sprintf("\n%s: %v", field.Label, value)
					if len([]rune(body))+len([]rune(line)) < 3700 {
						body += line
					}
				}
			}
			var files int
			if tx.QueryRowContext(ctx, `SELECT count(*) FROM task_attachments WHERE task_id=$1`, taskID).Scan(&files) == nil && files > 0 {
				body += fmt.Sprintf("\nВложений: %d (откройте Mini App)", files)
			}
			if status != "open" {
				taskOpen = false
				body += "\nЗаявка уже закрыта для откликов."
			}
		} else {
			return e
		}
	}
	var sendErr error
	if strings.HasPrefix(body, "Новая заявка №") && taskOpen {
		sendErr = sender.SendTask(sendCtx, maxID, taskID, body)
	} else {
		sendErr = sender.Send(sendCtx, maxID, body)
	}
	if err = sendErr; err != nil {
		// Retry with capped exponential delay; keep failed messages visible in the outbox.
		delay := time.Duration(1<<min(attempts, 8)) * time.Second
		_, updateErr := tx.ExecContext(ctx, `UPDATE notification_outbox SET attempts=attempts+1,next_attempt_at=now()+$2::interval,last_error=$3 WHERE id=$1`, id, delay.String(), err.Error())
		if updateErr != nil {
			return updateErr
		}
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE notification_outbox SET attempts=attempts+1,sent_at=now(),last_error=NULL WHERE id=$1`, id)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
