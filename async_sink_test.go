package unilog

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type blockingSink struct {
	writeStarted chan struct{}
	releaseWrite chan struct{}
	closeCalled  chan struct{}
}

func newBlockingSink() *blockingSink {
	return &blockingSink{
		writeStarted: make(chan struct{}, 1),
		releaseWrite: make(chan struct{}),
		closeCalled:  make(chan struct{}, 1),
	}
}

func (s *blockingSink) Name() string { return "blocking" }

func (s *blockingSink) Write(ctx context.Context, event Event) error {
	select {
	case s.writeStarted <- struct{}{}:
	default:
	}
	select {
	case <-s.releaseWrite:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (s *blockingSink) Sync(ctx context.Context) error { return nil }

func (s *blockingSink) Close(ctx context.Context) error {
	select {
	case s.closeCalled <- struct{}{}:
	default:
	}
	return nil
}

type countingSink struct {
	mu     sync.Mutex
	writes int
}

func (s *countingSink) Name() string { return "counting" }

func (s *countingSink) Write(ctx context.Context, event Event) error {
	s.mu.Lock()
	s.writes++
	s.mu.Unlock()
	return nil
}

func (s *countingSink) Sync(ctx context.Context) error  { return nil }
func (s *countingSink) Close(ctx context.Context) error { return nil }

func (s *countingSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writes
}

func TestAsyncSinkWriteRejectedAfterCloseStarts(t *testing.T) {
	next := newBlockingSink()
	sink := NewAsyncSink(next, AsyncSinkOptions{
		BufferSize:     1,
		OverflowPolicy: OverflowBlock,
	})

	ctx := context.Background()

	err := sink.Write(ctx, Event{Message: "first"})
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	select {
	case <-next.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first write to start")
	}

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- sink.Close(context.Background())
	}()

	time.Sleep(50 * time.Millisecond)

	err = sink.Write(ctx, Event{Message: "late"})
	if !errors.Is(err, ErrAsyncSinkClosed) {
		t.Fatalf("expected ErrAsyncSinkClosed, got %v", err)
	}

	close(next.releaseWrite)

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("close failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for close")
	}
}

func TestAsyncSinkBlockOnFullRespectsContext(t *testing.T) {
	next := newBlockingSink()
	sink := NewAsyncSink(next, AsyncSinkOptions{
		BufferSize:     1,
		OverflowPolicy: OverflowBlock,
	})

	err := sink.Write(context.Background(), Event{Message: "first"})
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	select {
	case <-next.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first write to start")
	}

	err = sink.Write(context.Background(), Event{Message: "second"})
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = sink.Write(ctx, Event{Message: "third"})
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}

	close(next.releaseWrite)

	if err := sink.Close(context.Background()); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestAsyncSinkDropsWhenBufferFull(t *testing.T) {
	next := newBlockingSink()

	var dropped int
	var mu sync.Mutex

	sink := NewAsyncSink(next, AsyncSinkOptions{
		BufferSize:     1,
		OverflowPolicy: OverflowDropNewest,
		OnError: func(err error) {
			if errors.Is(err, ErrAsyncBufferFull) {
				mu.Lock()
				dropped++
				mu.Unlock()
			}
		},
	})

	if err := sink.Write(context.Background(), Event{Message: "first"}); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	select {
	case <-next.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first write to start")
	}

	if err := sink.Write(context.Background(), Event{Message: "second"}); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	if err := sink.Write(context.Background(), Event{Message: "third"}); err != nil {
		t.Fatalf("third write should be dropped silently, got %v", err)
	}

	stats := sink.Stats()
	if stats.Dropped == 0 {
		t.Fatalf("expected dropped > 0, got %+v", stats)
	}

	mu.Lock()
	gotDropped := dropped
	mu.Unlock()

	if gotDropped == 0 {
		t.Fatal("expected OnError to be called with ErrAsyncBufferFull")
	}

	close(next.releaseWrite)

	if err := sink.Close(context.Background()); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestAsyncSinkSyncAfterCloseReturnsClosed(t *testing.T) {
	next := &countingSink{}
	sink := NewAsyncSink(next, AsyncSinkOptions{BufferSize: 8})

	if err := sink.Close(context.Background()); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	err := sink.Sync(context.Background())
	if !errors.Is(err, ErrAsyncSinkClosed) {
		t.Fatalf("expected ErrAsyncSinkClosed, got %v", err)
	}
}

type failingSink struct{}

func (f *failingSink) Name() string { return "failing" }
func (f *failingSink) Write(ctx context.Context, event Event) error {
	return errors.New("write failed")
}
func (f *failingSink) Sync(ctx context.Context) error  { return nil }
func (f *failingSink) Close(ctx context.Context) error { return nil }

func TestAsyncSinkStatsTrackWriteErrorsAndFlushes(t *testing.T) {
	sink := NewAsyncSink(&failingSink{}, AsyncSinkOptions{
		BufferSize:     8,
		OverflowPolicy: OverflowBlock,
	})

	err := sink.Write(context.Background(), Event{
		Time:    time.Unix(0, 0).UTC(),
		Level:   InfoLevel,
		Message: "hello",
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	err = sink.Sync(context.Background())
	if err == nil {
		t.Fatalf("expected Sync() to return accumulated worker error, got nil")
	}
	if !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected Sync() error to contain write failure, got %v", err)
	}

	if err := sink.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	stats := sink.Stats()
	if stats.WriteErrors == 0 {
		t.Fatalf("expected WriteErrors > 0, got %+v", stats)
	}
	if stats.Flushes == 0 {
		t.Fatalf("expected Flushes > 0, got %+v", stats)
	}
	if stats.Closes == 0 {
		t.Fatalf("expected Closes > 0, got %+v", stats)
	}
}
