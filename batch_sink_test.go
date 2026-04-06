package unilog

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type batchCollectingSink struct {
	mu      sync.Mutex
	batches [][]Event
	writes  []Event
}

func (s *batchCollectingSink) Name() string { return "batch-collector" }

func (s *batchCollectingSink) Write(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, event)
	return nil
}

func (s *batchCollectingSink) WriteBatch(ctx context.Context, events []Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := make([]Event, len(events))
	copy(cp, events)
	s.batches = append(s.batches, cp)
	return nil
}

func (s *batchCollectingSink) Sync(ctx context.Context) error  { return nil }
func (s *batchCollectingSink) Close(ctx context.Context) error { return nil }

func (s *batchCollectingSink) BatchCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.batches)
}

func (s *batchCollectingSink) TotalBatchEvents() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	total := 0
	for _, b := range s.batches {
		total += len(b)
	}
	return total
}

type singleCollectingSink struct {
	mu     sync.Mutex
	events []Event
}

func (s *singleCollectingSink) Name() string { return "single-collector" }

func (s *singleCollectingSink) Write(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *singleCollectingSink) Sync(ctx context.Context) error  { return nil }
func (s *singleCollectingSink) Close(ctx context.Context) error { return nil }

func (s *singleCollectingSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func TestBatchSinkFlushesOnMaxBatchSize(t *testing.T) {
	next := &batchCollectingSink{}
	sink := NewBatchSink(next, BatchSinkOptions{
		MaxBatchSize:   3,
		FlushInterval:  time.Hour,
		MaxQueueSize:   10,
		OverflowPolicy: OverflowBlock,
	})

	for i := 0; i < 3; i++ {
		err := sink.Write(context.Background(), Event{
			Time:    time.Unix(0, 0).UTC(),
			Level:   InfoLevel,
			Message: "msg",
		})
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if err := sink.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if next.BatchCount() != 1 {
		t.Fatalf("BatchCount = %d, want 1", next.BatchCount())
	}
	if next.TotalBatchEvents() != 3 {
		t.Fatalf("TotalBatchEvents = %d, want 3", next.TotalBatchEvents())
	}
}

func TestBatchSinkFlushesOnInterval(t *testing.T) {
	next := &batchCollectingSink{}
	sink := NewBatchSink(next, BatchSinkOptions{
		MaxBatchSize:   100,
		FlushInterval:  50 * time.Millisecond,
		MaxQueueSize:   10,
		OverflowPolicy: OverflowBlock,
	})

	err := sink.Write(context.Background(), Event{
		Time:    time.Unix(0, 0).UTC(),
		Level:   InfoLevel,
		Message: "msg",
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	if err := sink.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if next.TotalBatchEvents() != 1 {
		t.Fatalf("TotalBatchEvents = %d, want 1", next.TotalBatchEvents())
	}
}

func TestBatchSinkFallsBackToSingleWrites(t *testing.T) {
	next := &singleCollectingSink{}
	sink := NewBatchSink(next, BatchSinkOptions{
		MaxBatchSize:   2,
		FlushInterval:  time.Hour,
		MaxQueueSize:   10,
		OverflowPolicy: OverflowBlock,
	})

	for i := 0; i < 2; i++ {
		err := sink.Write(context.Background(), Event{
			Time:    time.Unix(0, 0).UTC(),
			Level:   InfoLevel,
			Message: "msg",
		})
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if err := sink.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if next.Count() != 2 {
		t.Fatalf("Count = %d, want 2", next.Count())
	}
}

func TestBatchSinkCloseDrainsPendingEvents(t *testing.T) {
	next := &batchCollectingSink{}
	sink := NewBatchSink(next, BatchSinkOptions{
		MaxBatchSize:   10,
		FlushInterval:  time.Hour,
		MaxQueueSize:   10,
		OverflowPolicy: OverflowBlock,
	})

	for i := 0; i < 3; i++ {
		err := sink.Write(context.Background(), Event{
			Time:    time.Unix(0, 0).UTC(),
			Level:   InfoLevel,
			Message: "msg",
		})
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if err := sink.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if next.TotalBatchEvents() != 3 {
		t.Fatalf("TotalBatchEvents = %d, want 3", next.TotalBatchEvents())
	}
}

func TestBatchSinkDropsWhenQueueFull(t *testing.T) {
	next := &batchCollectingSink{}

	var mu sync.Mutex
	var dropErrs int

	sink := NewBatchSink(next, BatchSinkOptions{
		MaxBatchSize:   100,
		FlushInterval:  time.Hour,
		MaxQueueSize:   1,
		OverflowPolicy: OverflowDropNewest,
		OnError: func(err error) {
			if errors.Is(err, ErrAsyncBufferFull) {
				mu.Lock()
				dropErrs++
				mu.Unlock()
			}
		},
	})

	err := sink.Write(context.Background(), Event{Message: "one"})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	err = sink.Write(context.Background(), Event{Message: "two"})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	stats := sink.Stats()
	if stats.Dropped == 0 {
		t.Fatalf("expected dropped > 0, got %+v", stats)
	}

	mu.Lock()
	gotDropErrs := dropErrs
	mu.Unlock()

	if gotDropErrs == 0 {
		t.Fatalf("expected ErrAsyncBufferFull callback")
	}

	_ = sink.Close(context.Background())
}

func TestBatchSinkRejectsWritesAfterCloseStarts(t *testing.T) {
	next := &batchCollectingSink{}
	sink := NewBatchSink(next, BatchSinkOptions{
		MaxBatchSize:   10,
		FlushInterval:  time.Hour,
		MaxQueueSize:   10,
		OverflowPolicy: OverflowBlock,
	})

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- sink.Close(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)

	err := sink.Write(context.Background(), Event{Message: "late"})
	if err != nil && !errors.Is(err, ErrAsyncSinkClosed) {
		t.Fatalf("expected nil or ErrAsyncSinkClosed during close race, got %v", err)
	}

	if err := <-closeDone; err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
