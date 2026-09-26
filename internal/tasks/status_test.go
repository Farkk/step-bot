package tasks

import "testing"

func TestNextStatus(t *testing.T) {
	cases := []struct{ current, action, want string }{
		{"assigned", "start", "in_progress"},
		{"in_progress", "complete", "awaiting_confirmation"},
		{"awaiting_confirmation", "confirm", "completed"},
		{"in_progress", "pause", "paused"},
		{"paused", "resume", "in_progress"},
		{"open", "cancel", "cancelled"},
		{"assigned", "cancel", "cancelled"},
		{"completed", "pause", ""},
		{"open", "complete", ""},
	}
	for _, tc := range cases {
		if got := nextStatus(tc.current, tc.action); got != tc.want {
			t.Errorf("%s + %s: got %q, want %q", tc.current, tc.action, got, tc.want)
		}
	}
}
