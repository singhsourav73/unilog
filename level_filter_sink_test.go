package unilog

import (
	"context"
	"testing"
)

func TestLevelFilterSinkBlocksLowerLevels(t *testing.T) {
	mem := &memorySink{}
	sink := NewLevelFilterSink(ErrorLevel, mem)

	err := sink.Write(context.Background(), Event{
		Level:   InfoLevel,
		Message: "ignore me",
	})
	if err != nil {
		t.Fatalf("Wriet() error = %v", err)
	}

	err = sink.Write(context.Background(), Event{
		Level:   ErrorLevel,
		Message: "keep me",
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if len(mem.events) != 1 {
		t.Fatalf("got %d events, want 1", len(mem.events))
	}
	if mem.events[0].Message != "keep me" {
		t.Fatalf("message = %q, want %q", mem.events[0].Message, "keep me")
	}
}
