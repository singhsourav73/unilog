package unilog

import (
	"context"
	"testing"
)

func TestFanoutSinkWritesToAllSinks(t *testing.T) {
	s1 := &memorySink{}
	s2 := &memorySink{}

	fanout := NewFanoutSink(s1, s2)
	log := New(fanout, Options{Level: DebugLevel})

	log.Info(context.Background(), "hello")

	if len(s1.events) != 1 {
		t.Fatalf("sink1 got %d events, want 1", len(s1.events))
	}

	if len(s2.events) != 1 {
		t.Fatalf("sink2 got %d events, want 1", len(s2.events))
	}
}
