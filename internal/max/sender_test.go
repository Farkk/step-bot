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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
