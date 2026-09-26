package max

import (
	"testing"
	"time"
)

func TestFormatStartTimeUsesCreatorOffset(t *testing.T) {
	instant := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	if got := formatStartTime(instant, 180); got != "27.09.2026 13:00" {
		t.Fatalf("got %q, want 13:00", got)
	}
	if got := formatStartTime(instant, 360); got != "27.09.2026 16:00" {
		t.Fatalf("got %q, want 16:00", got)
	}
}
