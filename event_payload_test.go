package unilog

import (
	"errors"
	"testing"
	"time"
)

func TestEventPayloadReservedFieldCollision(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	event := Event{
		Time:    now,
		Level:   InfoLevel,
		Message: "canonical-message",
		Err:     errors.New("canonical-error"),
		Fields: []Field{
			{Key: "message", Value: "user-message"},
			{Key: "error", Value: "user-error"},
			{Key: "trace_id", Value: "user-trace"},
			{Key: "custom", Value: "ok"},
		},
	}

	payload := EventPayload(event)

	if got := payload["message"]; got != "canonical-message" {
		t.Fatalf("expected canonical message, got %#v", got)
	}

	if got := payload["error"]; got != "canonical-error" {
		t.Fatalf("expected canonical error, got %#v", got)
	}

	if got := payload["field.message"]; got != "user-message" {
		t.Fatalf("expected user message under field.message, got %#v", got)
	}

	if got := payload["field.error"]; got != "user-error" {
		t.Fatalf("expected user error under field.error, got %#v", got)
	}

	if got := payload["field.trace_id"]; got != "user-trace" {
		t.Fatalf("expected user trace_id under field.trace_id, got %#v", got)
	}

	if got := payload["custom"]; got != "ok" {
		t.Fatalf("expected custom field to remain unchanged, got %#v", got)
	}
}
