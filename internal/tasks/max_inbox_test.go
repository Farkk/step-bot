package tasks

import (
	"encoding/json"
	"testing"
)

func TestMaxCallbackPayload(t *testing.T) {
	var e maxEvent
	raw := []byte(`{"update_type":"message_callback","callback":{"payload":"apply:123","callback_id":"c1","user":{"user_id":44}}}`)
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e.UpdateType != "message_callback" || e.Callback.Payload != "apply:123" || e.Callback.User.UserID != 44 {
		t.Fatalf("bad callback: %+v", e)
	}
}
