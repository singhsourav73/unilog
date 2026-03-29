package unilog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestJSONSinkProducesValidJSON(t *testing.T) {
	var buf bytes.Buffer
	sink := NewJSONSink(&buf)

	ev := Event{
		Level:       ErrorLevel,
		Message:     "payment failed",
		Service:     "billing-api",
		Environment: "test",
		LoggerName:  "payments",
		TraceID:     "trace-1",
		Err:         errors.New("gateway timeout"),
		Fields: []Field{
			String("order_id", "ord-1"),
			Int("amount", 100),
		},
	}

	if err := sink.Write(context.Background(), ev); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, body: %s", err, buf.String())
	}

	if decoded["level"] != "error" {
		t.Errorf("Expected level to be 'error', got %v", decoded["level"])
	}

	if decoded["message"] != "payment failed" {
		t.Errorf("Expected message to be 'payment failed', got %v", decoded["message"])
	}

	if decoded["service"] != "billing-api" {
		t.Errorf("Expected service to be 'billing-api', got %v", decoded["service"])
	}

	if decoded["environment"] != "test" {
		t.Errorf("Expected environment to be 'test', got %v", decoded["environment"])
	}

	if decoded["logger_name"] != "payments" {
		t.Errorf("Expected logger_name to be 'payments', got %v", decoded["logger_name"])
	}

	if decoded["trace_id"] != "trace-1" {
		t.Errorf("Expected trace_id to be 'trace-1', got %v", decoded["trace_id"])
	}

	if decoded["error"] != "gateway timeout" {
		t.Errorf("Expected error to be 'gateway timeout', got %v", decoded["error"])
	}

	if decoded["order_id"] != "ord-1" {
		t.Errorf("Expected order_id to be 'ord-1', got %v", decoded["order_id"])
	}

	if decoded["amount"] != float64(100) { // JSON numbers are decoded as float64
		t.Errorf("Expected amount to be 100, got %v", decoded["amount"])
	}
}
