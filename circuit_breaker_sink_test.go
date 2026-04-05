package unilog

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type breakerTestSink struct {
	mu       sync.Mutex
	failures int
	calls    int
}

func (s *breakerTestSink) Name() string { return "breaker-test" }

func (s *breakerTestSink) Write(_ context.Context, _ Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls++
	if s.failures > 0 {
		s.failures--
		return errors.New("temporary failure")
	}
	return nil
}

func (s *breakerTestSink) Sync(context.Context) error  { return nil }
func (s *breakerTestSink) Close(context.Context) error { return nil }

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	now := time.Unix(100, 0)

	next := &breakerTestSink{failures: 10}
	breaker := NewCircuitBreakerSink(next, CircuitBreakerOptions{
		FailureThreshold: 2,
		OpenTimeout:      10 * time.Second,
		Now:              func() time.Time { return now },
	})

	err := breaker.Write(context.Background(), Event{Level: ErrorLevel, Message: "e1"})
	if err == nil {
		t.Fatalf("expected first write to fail")
	}

	err = breaker.Write(context.Background(), Event{Level: ErrorLevel, Message: "e2"})
	if err == nil {
		t.Fatalf("expected second write to fail")
	}

	if breaker.State() != CircuitOpen {
		t.Fatalf("state = %q, want %q", breaker.State(), CircuitOpen)
	}

	err = breaker.Write(context.Background(), Event{Level: ErrorLevel, Message: "e3"})
	if !IsCircuitOpen(err) {
		t.Fatalf("expected circuit open error, got %v", err)
	}

	next.mu.Lock()
	calls := next.calls
	next.mu.Unlock()

	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestCircuitBreakerHalfOpenThenClosesOnSuccess(t *testing.T) {
	now := time.Unix(100, 0)

	next := &breakerTestSink{failures: 2}
	breaker := NewCircuitBreakerSink(next, CircuitBreakerOptions{
		FailureThreshold: 2,
		OpenTimeout:      10 * time.Second,
		Now:              func() time.Time { return now },
	})

	_ = breaker.Write(context.Background(), Event{Level: ErrorLevel, Message: "e1"})
	_ = breaker.Write(context.Background(), Event{Level: ErrorLevel, Message: "e2"})

	if breaker.State() != CircuitOpen {
		t.Fatalf("state = %q, want %q", breaker.State(), CircuitOpen)
	}

	now = now.Add(11 * time.Second)

	err := breaker.Write(context.Background(), Event{Level: ErrorLevel, Message: "e3"})
	if err != nil {
		t.Fatalf("expected half-open test write to succeed, got %v", err)
	}

	if breaker.State() != CircuitClosed {
		t.Fatalf("state = %q, want %q", breaker.State(), CircuitClosed)
	}
}
