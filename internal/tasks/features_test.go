package tasks

import (
	"testing"
	"time"
)

func TestRatingWindowAndComment(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	if !ratingAllowed(now.Add(-14*24*time.Hour), now) {
		t.Fatal("boundary must be editable")
	}
	if ratingAllowed(now.Add(-14*24*time.Hour-time.Nanosecond), now) {
		t.Fatal("expired rating accepted")
	}
	if validRating(0, "ok") || validRating(6, "ok") || validRating(5, "  ") || !validRating(5, "Отличная работа") {
		t.Fatal("invalid rating validation")
	}
}

func TestDynamicFields(t *testing.T) {
	defs := []FieldDefinition{{Key: "volume", Type: "number", Required: true}, {Key: "service", Type: "select", Options: []string{"Сборка", "Разборка"}}}
	if err := validateFields(defs, map[string]any{"volume": float64(2), "service": "Сборка"}); err != nil {
		t.Fatal(err)
	}
	if validateFields(defs, map[string]any{"volume": "много"}) == nil {
		t.Fatal("wrong number accepted")
	}
	if validateFields(defs, map[string]any{"volume": float64(1), "service": "Другое"}) == nil {
		t.Fatal("unknown option accepted")
	}
	if validateFields(defs, map[string]any{"volume": float64(1), "extra": true}) == nil {
		t.Fatal("unknown field accepted")
	}
}
