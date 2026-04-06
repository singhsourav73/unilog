package unilog

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTextEncoderEncodeIncludesCoreFields(t *testing.T) {
	enc := NewTextEncoder(TextEncoderOptions{
		TimeLayout: time.RFC3339,
	})

	ev := Event{
		Time:          time.Unix(0, 0).UTC(),
		Level:         WarnLevel,
		Message:       "payment delayed",
		Service:       "billing-api",
		Environment:   "test",
		LoggerName:    "payments",
		TraceID:       "trace-1",
		SpanID:        "span-1",
		RequestID:     "req-1",
		CorrelationID: "corr-1",
		Err:           errors.New("temporary"),
		Fields: []Field{
			String("z_key", "last"),
			String("a_key", "first"),
		},
	}

	b, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	out := string(b)

	checks := []string{
		"1970-01-01T00:00:00Z",
		"WARN",
		`service=billing-api`,
		`env=test`,
		`logger=payments`,
		`msg="payment delayed"`,
		`trace_id=trace-1`,
		`span_id=span-1`,
		`request_id=req-1`,
		`correlation_id=corr-1`,
		`error=temporary`,
	}

	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}

	aIdx := strings.Index(out, "a_key=first")
	zIdx := strings.Index(out, "z_key=last")
	if aIdx == -1 || zIdx == -1 {
		t.Fatalf("sorted fields not found in output: %s", out)
	}
	if aIdx > zIdx {
		t.Fatalf("fields not sorted: %s", out)
	}

	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected trailing newline, got %q", out)
	}
}

func TestTextEncoderEncodeQuotesWhenNeeded(t *testing.T) {
	enc := NewTextEncoder(TextEncoderOptions{
		TimeLayout: time.RFC3339,
	})

	ev := Event{
		Time:    time.Unix(0, 0).UTC(),
		Level:   InfoLevel,
		Message: `hello world`,
		Fields: []Field{
			String("plain", "ok"),
			String("spaced", "hello world"),
			String("quoted", `a"b`),
		},
	}

	b, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	out := string(b)

	if !strings.Contains(out, `msg="hello world"`) {
		t.Fatalf("message not quoted as expected: %s", out)
	}
	if !strings.Contains(out, `plain=ok`) {
		t.Fatalf("plain field not rendered as expected: %s", out)
	}
	if !strings.Contains(out, `spaced="hello world"`) {
		t.Fatalf("spaced field not quoted as expected: %s", out)
	}
	if !strings.Contains(out, `quoted="a\"b"`) {
		t.Fatalf("quoted field not escaped as expected: %s", out)
	}
}
