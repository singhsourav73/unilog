package unilog

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type blockingSink struct {
	started chan struct{}
	release chan struct{}

	mu     sync.Mutex
	events []Event
}

func newBlockingSink() *blockingSink {
	return &blockingSink{
		started: make(chan struct{}, 10),
		release: make(chan struct{}),
	}
}

func (s *blockingSink) Name() string { return "blocking" }

func (s *blockingSink) Write(_ context.Context, e Event) error {
	s.started <- struct{}{}
	<-s.release

	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *blockingSink) Sync(context.Context) error {
	return nil
}

func (s *blockingSink) Close(context.Context) error {
	return nil
}

type failingAsyncSink struct {
	mu       sync.Mutex
	failures int
	events   []Event
}

func (s *failingAsyncSink) Name() string { return "failing-async" }

func (s *failingAsyncSink) Write(_ context.Context, e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failures > 0 {
		s.failures--
		return errors.New("async worker write failed")
	}

	s.events = append(s.events, e)
	return nil
}

func (s *failingAsyncSink) Sync(context.Context) error  { return nil }
func (s *failingAsyncSink) Close(context.Context) error { return nil }

func TestAsyncSinkWritesEventually(t *testing.T) {
	mem := &memorySink{}
	async := NewAsyncSink(mem, AsyncSinkOptions{
		BufferSize: 8,
	})

	for i := 0; i < 3; i++ {
		if err := async.Write(context.Background(), Event{
			Level:   InfoLevel,
			Message: "hello",
		}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if err := async.Sync(context.Background()); err != nil {
		t.Fatalf("SYnc() error = %v", err)
	}

	if err := async.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if len(mem.events) != 3 {
		t.Fatalf("got %d events, want 3", len(mem.events))
	}
}

func TestAsyncSinkDropsWhenBufferFull(t *testing.T) {
	slow := newBlockingSink()

	var seenErrs []error
	var mu sync.Mutex

	async := NewAsyncSink(slow, AsyncSinkOptions{
		BufferSize:  1,
		BlockOnFull: false,
		OnError: func(err error) {
			mu.Lock()
			defer mu.Unlock()
			seenErrs = append(seenErrs, err)
		},
	})

	if err := async.Write(context.Background(), Event{Level: InfoLevel, Message: "e1"}); err != nil {
		t.Fatalf("Write(e1) error = %v", err)
	}

	<-slow.started

	if err := async.Write(context.Background(), Event{Level: InfoLevel, Message: "e2"}); err != nil {
		t.Fatalf("Write(e2) error = %v", err)
	}

	if err := async.Write(context.Background(), Event{Level: InfoLevel, Message: "e3"}); err != nil {
		t.Fatalf("Write(e3) error = %v", err)
	}

	close(slow.release)

	if err := async.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	stats := async.Stats()
	if stats.Dropped != 1 {
		t.Fatalf("Dropped = %d, want 1", stats.Dropped)
	}

	slow.mu.Lock()
	defer slow.mu.Unlock()

	if len(slow.events) != 2 {
		t.Fatalf("git %d event, want 2", len(slow.events))
	}
	if slow.events[0].Message != "e1" || slow.events[1].Message != "e2" {
		t.Fatalf("unexpected event orders: %+v", slow.events)
	}
}

func TestAsyncSinkSyncReturnsWorkerErrors(t *testing.T) {
	fail := &failingAsyncSink{failures: 1}
	async := NewAsyncSink(fail, AsyncSinkOptions{
		BufferSize: 8,
	})

	if err := async.Write(context.Background(), Event{Level: InfoLevel, Message: "e1"}); err != nil {
		t.Fatalf("Write(e1) error = %v", err)
	}
	if err := async.Write(context.Background(), Event{Level: InfoLevel, Message: "e2"}); err != nil {
		t.Fatalf("Write(e2) error = %v", err)
	}

	err := async.Sync(context.Background())
	if err == nil {
		t.Fatalf("expected Sync() error, got nil")
	}

	if err := async.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
