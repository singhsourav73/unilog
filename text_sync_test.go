package unilog

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTextSinkWritesReadableLine(t *testing.T) {
	var buf bytes.Buffer
	sink := NewTextSink(&buf, TextSinkOptions{})

	ev := Event{
		Time:        time.Unix(0, 0).UTC(),
		Level:       ErrorLevel,
		Message:     "payment failed",
		Service:     "billing-api",
		Environment: "test",
		LoggerName:  "payments",
		TraceID:     "trace-1",
		Err:         errors.New("gateway timeout"),
		Fields: []Field{
			String("order_id", "ord-1"),
			Int("amount", 200),
		},
	}

	if err := sink.Write(context.Background(), ev); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	out := buf.String()
	checks := []string{
		"ERROR",
		"service=billing-api",
		"env=test",
		"logger=payments",
		`msg="payment failed"`,
		"trace_id=trace-1",
		`error="gateway timeout"`,
		"order_id=ord-1",
		"amount=200",
	}

	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
}
