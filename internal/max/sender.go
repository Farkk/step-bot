package max

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
	return s.SendTaskWithPhoto(ctx, userID, taskID, message, false)
}

func (s Sender) SendTaskWithPhoto(ctx context.Context, userID, taskID int64, message string, hasPhoto bool) error {
	return s.sendTask(ctx, userID, message, taskID, hasPhoto)
}

func (s Sender) SendOrder(ctx context.Context, userID, taskID int64, message string) error {
	if taskID < 1 {
		return s.Send(ctx, userID, message)
	}
	button := map[string]string{"type": "open_app", "text": "Открыть заказ", "payload": "task_" + strconv.FormatInt(taskID, 10)}
	return s.postMessage(ctx, userID, map[string]any{"text": message, "notify": true, "attachments": []any{map[string]any{"type": "inline_keyboard", "payload": map[string]any{"buttons": [][]any{{button}}}}}})
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
	return s.sendTask(ctx, userID, message, taskID, false)
}

func (s Sender) sendTask(ctx context.Context, userID int64, message string, taskID int64, hasPhoto bool) error {
	body := map[string]any{"text": message, "notify": true}
	if taskID > 0 {
		buttons := [][]any{{map[string]string{"type": "callback", "text": "Откликнуться", "payload": "apply:" + strconv.FormatInt(taskID, 10)}}}
		if hasPhoto {
			buttons = append(buttons, []any{map[string]string{"type": "callback", "text": "Фото", "payload": "photo:" + strconv.FormatInt(taskID, 10)}})
		}
		buttons = append(buttons, []any{map[string]string{"type": "open_app", "text": "Открыть заказ", "payload": "task_" + strconv.FormatInt(taskID, 10)}})
		body["attachments"] = []any{map[string]any{"type": "inline_keyboard", "payload": map[string]any{"buttons": buttons}}}
	}
	return s.postMessage(ctx, userID, body)
}

func (s Sender) postMessage(ctx context.Context, userID int64, body map[string]any) error {
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

func (s Sender) SendImage(ctx context.Context, userID int64, filename string, data []byte) error {
	if s.Token == "" {
		return fmt.Errorf("MAX_BOT_TOKEN is empty")
	}
	base := s.BaseURL
	if base == "" {
		base = "https://platform-api2.max.ru"
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/uploads?type=image", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", s.Token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	var upload struct {
		URL string `json:"url"`
	}
	if resp.StatusCode == http.StatusOK {
		err = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&upload)
	} else {
		err = fmt.Errorf("Max upload: HTTP %d", resp.StatusCode)
	}
	resp.Body.Close()
	if err != nil {
		return err
	}
	u, err := url.Parse(upload.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() != "iu.oneme.ru" {
		return fmt.Errorf("unexpected Max image upload URL")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("data", filename)
	if err != nil {
		return err
	}
	if _, err = part.Write(data); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", s.Token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	var uploaded struct {
		Photos map[string]struct {
			Token string `json:"token"`
		} `json:"photos"`
	}
	if resp.StatusCode == http.StatusOK {
		err = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&uploaded)
	} else {
		err = fmt.Errorf("Max image upload: HTTP %d", resp.StatusCode)
	}
	resp.Body.Close()
	if err != nil {
		return err
	}
	var token string
	for _, photo := range uploaded.Photos {
		token = photo.Token
		break
	}
	if token == "" {
		return fmt.Errorf("Max image token is empty")
	}
	message := map[string]any{"text": filename, "attachments": []any{map[string]any{"type": "image", "payload": map[string]string{"token": token}}}}
	for attempt := 0; attempt < 3; attempt++ {
		err = s.postMessage(ctx, userID, message)
		if err == nil || !strings.Contains(err.Error(), "attachment.not.ready") {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}
	return err
}
