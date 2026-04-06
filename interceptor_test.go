package unilog

import (
	"context"
	"testing"
)

func TestChainInterceptorsRunsInOrder(t *testing.T) {
	var order []string

	first := func(next Handler) Handler {
		return func(ctx context.Context, event Event) error {
			order = append(order, "first:before")
			err := next(ctx, event)
			order = append(order, "first:after")
			return err
		}
	}

	second := func(next Handler) Handler {
		return func(ctx context.Context, event Event) error {
			order = append(order, "second:before")
			err := next(ctx, event)
			order = append(order, "second:after")
			return err
		}
	}

	handler := ChainInterceptors(func(context.Context, Event) error {
		order = append(order, "final")
		return nil
	}, first, second)

	if err := handler(context.Background(), Event{Message: "hello"}); err != nil {
		t.Fatalf("handler error = %v", err)
	}

	want := []string{"first:before", "second:before", "final", "second:after", "first:after"}
	if len(order) != len(want) {
		t.Fatalf("order len = %d, want %d (%v)", len(order), len(want), order)
	}

	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestLoggerInterceptorIntegration(t *testing.T) {
	mem := &memorySink{}
	seen := false

	interceptor := func(next Handler) Handler {
		return func(ctx context.Context, event Event) error {
			seen = true
			return next(ctx, event)
		}
	}

	log := NewWithInterceptors(mem, Options{Level: DebugLevel}, []Interceptor{interceptor})
	log.Info(context.Background(), "hello")

	if !seen {
		t.Fatal("expected interceptor to run")
	}

	if len(mem.events) != 1 {
		t.Fatalf("got %d events, want 1", len(mem.events))
	}
}
