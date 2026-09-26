package max

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Sender struct {
	Token, BaseURL string
	Client         *http.Client
}

func (s Sender) Send(ctx context.Context, userID int64, message string) error {
	return s.send(ctx, userID, message, 0)
}

func (s Sender) SendTask(ctx context.Context, userID, taskID int64, message string) error {
	return s.send(ctx, userID, message, taskID)
}

func (s Sender) Answer(ctx context.Context, callbackID, message string) error {
	base := s.BaseURL
	if base == "" {
		base = "https://platform-api2.max.ru"
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + "/answers")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("callback_id", callbackID)
	u.RawQuery = q.Encode()
	data, _ := json.Marshal(map[string]any{"message": map[string]any{"text": message, "attachments": []any{}}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", s.Token)
	req.Header.Set("Content-Type", "application/json")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Max callback: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s Sender) send(ctx context.Context, userID int64, message string, taskID int64) error {
	if s.Token == "" {
		return fmt.Errorf("MAX_BOT_TOKEN is empty")
	}
	base := s.BaseURL
	if base == "" {
		base = "https://platform-api2.max.ru"
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + "/messages")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("user_id", strconv.FormatInt(userID, 10))
	u.RawQuery = q.Encode()
	body := map[string]any{"text": message, "notify": true}
	if taskID > 0 {
		body["attachments"] = []any{map[string]any{"type": "inline_keyboard", "payload": map[string]any{"buttons": [][]any{{map[string]string{"type": "callback", "text": "Откликнуться", "payload": "apply:" + strconv.FormatInt(taskID, 10)}}}}}}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", s.Token)
	req.Header.Set("Content-Type", "application/json")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("Max API: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
