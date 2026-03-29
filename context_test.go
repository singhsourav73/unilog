package unilog

import (
	"context"
	"testing"
)

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-1")
	ctx = WithSpanID(ctx, "span-1")
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithCorrelationID(ctx, "corr-1")

	if got := TraceIDFromContext(ctx); got != "trace-1" {
		t.Fatalf("TraceIDFromContext() = %q, want %q", got, "trace-1")
	}

	if got := SpanIDFromContext(ctx); got != "span-1" {
		t.Fatalf("SpanIDFromContext() = %q, want %q", got, "span-1")
	}

	if got := RequestIDFromContext(ctx); got != "req-1" {
		t.Fatalf("RequestIDFromContext() = %q, want %q", got, "req-1")
	}

	if got := CorrelationIDFromContext(ctx); got != "corr-1" {
		t.Fatalf("CorrelationIDFromContext() = %q, want %q", got, "corr-1")
	}

	if got := TraceIDFromContext(context.TODO()); got != "" {
		t.Fatalf("TraceIDFromContext() = %q, want empty string", got)
	}
}
