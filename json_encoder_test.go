package unilog

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestJSONEncoderEncodeProducesValidJSON(t *testing.T) {
	enc := NewJSONEncoder()

	ev := Event{
		Time:      time.Unix(0, 0).UTC(),
		Level:     ErrorLevel,
		Message:   "payment failed",
		Service:   "billing-api",
		RequestID: "req-1",
		Err:       errors.New("boom"),
		Fields: []Field{
			String("order_id", "ord-1"),
		},
	}

	b, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("invalid JSON: %v; body=%s", err, string(b))
	}

	if got := payload["message"]; got != "payment failed" {
		t.Fatalf("message = %v, want payment failed", got)
	}
	if got := payload["service"]; got != "billing-api" {
		t.Fatalf("service = %v, want billing-api", got)
	}
	if got := payload["request_id"]; got != "req-1" {
		t.Fatalf("request_id = %v, want req-1", got)
	}
	if got := payload["order_id"]; got != "ord-1" {
		t.Fatalf("order_id = %v, want ord-1", got)
	}
	if got := payload["error"]; got != "boom" {
		t.Fatalf("error = %v, want boom", got)
	}
}

func TestJSONEncoderReservedFieldCollision(t *testing.T) {
	enc := NewJSONEncoder()

	ev := Event{
		Time:    time.Unix(0, 0).UTC(),
		Level:   InfoLevel,
		Message: "canonical",
		Fields: []Field{
			String("message", "user-message"),
			String("trace_id", "user-trace"),
		},
	}

	b, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if got := payload["message"]; got != "canonical" {
		t.Fatalf("message = %v, want canonical", got)
	}
	if got := payload["field.message"]; got != "user-message" {
		t.Fatalf("field.message = %v, want user-message", got)
	}
	if got := payload["field.trace_id"]; got != "user-trace" {
		t.Fatalf("field.trace_id = %v, want user-trace", got)
	}
}
