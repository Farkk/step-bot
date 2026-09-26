package tasks

import (
	"testing"
	"time"
)

func TestValidateCreate(t *testing.T) {
	good := CreateInput{Title: "Доставка документов", Category: "Доставка", Description: "Отвезти документы в офис", Budget: 3000, Deadline: time.Now().Add(time.Hour), Location: "Омск"}
	if err := good.Validate(time.Now()); err != nil {
		t.Fatalf("valid task: %v", err)
	}
	bad := good
	bad.Budget = 0
	if bad.Validate(time.Now()) == nil {
		t.Fatal("zero budget accepted")
	}
	bad = good
	bad.Deadline = time.Now().Add(-time.Hour)
	if bad.Validate(time.Now()) == nil {
		t.Fatal("past deadline accepted")
	}
	invalidOffset := 900
	bad = good
	bad.StartOffsetMinutes = &invalidOffset
	if bad.Validate(time.Now()) == nil {
		t.Fatal("invalid timezone offset accepted")
	}
}
