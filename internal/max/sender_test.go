package max

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSenderPush(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/messages" || r.URL.Query().Get("user_id") != "42" || r.Header.Get("Authorization") != "test-token" {
			t.Errorf("bad request: %s %s", r.URL, r.Header.Get("Authorization"))
		}
		var body struct {
			Text   string `json:"text"`
			Notify bool   `json:"notify"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text != "Заявка обновлена" || !body.Notify {
			t.Errorf("bad body: %+v, %v", body, err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}
	if err := (Sender{Token: "test-token", BaseURL: "https://example.test", Client: client}).Send(context.Background(), 42, "Заявка обновлена"); err != nil {
		t.Fatal(err)
	}
}

func TestSenderTaskCallback(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Attachments []struct {
				Type    string `json:"type"`
				Payload struct {
					Buttons [][]struct {
						Type    string `json:"type"`
						Payload string `json:"payload"`
					} `json:"buttons"`
				} `json:"payload"`
			} `json:"attachments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Attachments) != 1 || body.Attachments[0].Type != "inline_keyboard" || body.Attachments[0].Payload.Buttons[0][0].Payload != "apply:42" {
			t.Fatalf("bad callback body: %+v", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}
	if err := (Sender{Token: "test-token", BaseURL: "https://example.test", Client: client}).SendTask(context.Background(), 7, 42, "Заявка"); err != nil {
		t.Fatal(err)
	}
}

func TestSenderTaskPhotoButtons(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Attachments []struct {
				Payload struct {
					Buttons [][]struct {
						Type    string `json:"type"`
						Text    string `json:"text"`
						Payload string `json:"payload"`
					} `json:"buttons"`
				} `json:"payload"`
			} `json:"attachments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		buttons := body.Attachments[0].Payload.Buttons
		if len(buttons) != 3 || buttons[1][0].Text != "Фото" || buttons[1][0].Payload != "photo:42" || buttons[2][0].Type != "open_app" || buttons[2][0].Payload != "task_42" {
			t.Fatalf("wrong buttons: %+v", buttons)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}
	if err := (Sender{Token: "test-token", BaseURL: "https://example.test", Client: client}).SendTaskWithPhoto(context.Background(), 7, 42, "Заявка", true); err != nil {
		t.Fatal(err)
	}
}

func TestSenderOrderOpenButton(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Attachments []struct {
				Payload struct {
					Buttons [][]struct {
						Type    string `json:"type"`
						Payload string `json:"payload"`
					} `json:"buttons"`
				} `json:"payload"`
			} `json:"attachments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		buttons := body.Attachments[0].Payload.Buttons
		if len(buttons) != 1 || buttons[0][0].Type != "open_app" || buttons[0][0].Payload != "task_42" {
			t.Fatalf("wrong order button: %+v", buttons)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}
	if err := (Sender{Token: "test-token", BaseURL: "https://example.test", Client: client}).SendOrder(context.Background(), 7, 42, "Статус изменился"); err != nil {
		t.Fatal(err)
	}
}

func TestSenderImageUploadAndSend(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var response string
		switch calls {
		case 1:
			if r.URL.Path != "/uploads" || r.URL.Query().Get("type") != "image" {
				t.Errorf("bad upload request: %s", r.URL)
			}
			response = `{"url":"https://iu.oneme.ru/upload-image"}`
		case 2:
			if r.URL.Path != "/upload-image" || r.Header.Get("Content-Type") == "" {
				t.Errorf("bad file upload")
			}
			response = `{"photos":{"1":{"token":"photo-token"}}}`
		case 3:
			var body struct {
				Attachments []struct {
					Type    string `json:"type"`
					Payload struct {
						Token string `json:"token"`
					} `json:"payload"`
				} `json:"attachments"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body.Attachments) != 1 || body.Attachments[0].Type != "image" || body.Attachments[0].Payload.Token != "photo-token" {
				t.Errorf("bad image message: %+v", body)
			}
		default:
			t.Errorf("unexpected request: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	})}
	if err := (Sender{Token: "test-token", BaseURL: "https://example.test", Client: client}).SendImage(context.Background(), 7, "photo.jpg", []byte("image")); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("got %d requests", calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
