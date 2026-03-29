package unilog

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type memorySink struct {
	mu       sync.Mutex
	events   []Event
	synced   bool
	closed   bool
	writeErr error
}

func (m *memorySink) Name() string {
	return "memory"
}

func (m *memorySink) Write(_ context.Context, event Event) error {
	if m.writeErr != nil {
		return m.writeErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return m.writeErr
}

func (m *memorySink) Sync(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synced = true
	return nil
}

func (m *memorySink) Close(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

type addFieldProcessor struct {
	key   string
	value any
}

func (p addFieldProcessor) Name() string {
	return "add_field"
}

func (p addFieldProcessor) Process(_ context.Context, e *Event) error {
	e.Fields = append(e.Fields, Any(p.key, p.value))
	return nil
}

type failingProcessor struct{}

func (f failingProcessor) Name() string { return "failing" }
func (f failingProcessor) Process(context.Context, *Event) error {
	return errors.New("processor failed")
}

func TestLoggerWritesStructuredEvents(t *testing.T) {
	sink := &memorySink{}
	log := New(sink, Options{
		Service:     "billing-api",
		Environment: "test",
		Level:       DebugLevel,
	}, addFieldProcessor{key: "processor", value: "ok"})

	ctx := context.Background()
	ctx = WithTraceID(ctx, "tarce-123")
	ctx = WithSpanID(ctx, "span-456")
	ctx = WithRequestID(ctx, "req=789")
	ctx = WithCorrelationID(ctx, "corr-999")

	log = log.WithName("payments").With(
		String("component", "checkout"),
		Int("amount", 100),
	)

	log.Error(ctx, errors.New("gateway timeout"), "payment failed", String("order_id", "ord-1"))

	if len(sink.events) != 1 {
		t.Fatalf("got %d events, want 1", len(sink.events))
	}

	ev := sink.events[0]
	if ev.Level != ErrorLevel {
		t.Fatalf("even.Level = %v, want %q", ev.Level, ErrorLevel)
	}

	if ev.Message != "payment failed" {
		t.Fatalf("event.Message = %q, want %q", ev.Message, "payment failed")
	}

	if ev.Service != "billing-api" {
		t.Fatalf("event.Service = %q, want %q", ev.Service, "billing-api")
	}

	if ev.Environment != "test" {
		t.Fatalf("event.Environment = %q, want %q", ev.Environment, "test")
	}

	if ev.LoggerName != "payments" {
		t.Fatalf("event.LoggerName = %q, want %q", ev.LoggerName, "payments")
	}

	if ev.TraceID != "tarce-123" {
		t.Fatalf("event.TraceID = %q, want %q", ev.TraceID, "tarce-123")
	}

	if ev.SpanID != "span-456" {
		t.Fatalf("event.SpanID = %q, want %q", ev.SpanID, "span-456")
	}

	if ev.RequestID != "req=789" {
		t.Fatalf("event.RequestID = %q, want %q", ev.RequestID, "req=789")
	}

	if ev.CorrelationID != "corr-999" {
		t.Fatalf("event.CorrelationID = %q, want %q", ev.CorrelationID, "corr-999")
	}

	if ev.Err == nil || ev.Err.Error() != "gateway timeout" {
		t.Fatalf("event.Err = %v, want gateway timeout", ev.Err)
	}

	got := map[string]any{}
	for _, g := range ev.Fields {
		got[g.Key] = g.Value
	}

	if got["component"] != "checkout" {
		t.Fatalf("field component = %q, want %q", got["component"], "checkout")
	}

	if got["amount"] != 100 {
		t.Fatalf("field amount = %v, want %v", got["amount"], 100)
	}

	if got["processor"] != "ok" {
		t.Fatalf("field processor = %q, want %q", got["processor"], "ok")
	}

	if got["order_id"] != "ord-1" {
		t.Fatalf("field order_id = %q, want %q", got["order_id"], "ord-1")
	}
}

func TestLoggerWithIsImmutable(t *testing.T) {
	// TODO: Implement this test to verify that Logger.With() returns a new Logger instance and does not modify the original Logger's state.
}

func TestLoggerWithContextUsesDefaultContext(t *testing.T) {
	// TODO: Implement this test to verify that Logger.WithContext() uses the default context when no context is provided.
}

func TestLoggerLevelFiltering(t *testing.T) {
	// TODO: Implement this test to verify that the Logger correctly filters events based on the configured log level.
}

func TestLoggerCallerAndStack(t *testing.T) {
	// TODO: Implement this test to verify that the Logger correctly captures caller information and stack traces when configured to do so.
}

func TestLoggerSyncAndCLose(t *testing.T) {
	// TODO: Implement this test to verify that the Logger correctly calls Sync() and Close() on its sinks when the Logger is closed.
}

func TestLoggerInternalErrorHandler(t *testing.T) {
	sink := &memorySink{}
	var captured []string
	var mu sync.Mutex

	log := New(sink, Options{
		Level: DebugLevel,
		OnInternalError: func(_ context.Context, err error) {
			mu.Lock()
			defer mu.Unlock()
			captured = append(captured, err.Error())
		},
	}, failingProcessor{})

	log.Info(context.Background(), "hello")

	mu.Lock()
	defer mu.Unlock()

	if len(captured) != 1 {
		t.Fatalf("got %d internal errors, want 1", len(captured))
	}

	if captured[0] != "processor failed" {
		t.Fatalf("got internal error %q, want %q", captured[0], "processor failed")
	}
}
