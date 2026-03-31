package unilog

import (
	"context"
	"testing"
)

func TestSamplingProcessorDropsDeterministically(t *testing.T) {
	p := NewSamplingProcessor(SamplingOptions{
		DebugEvery: 2,
		InfoEvery:  3,
		WarnEvery:  1,
	})

	debugEvent := &Event{Level: DebugLevel, Message: "debug-msg"}
	if err := p.Process(context.Background(), debugEvent); err != nil {
		t.Fatalf("first debug should pass, got %v", err)
	}
	if err := p.Process(context.Background(), debugEvent); !IsDropEvent(err) {
		t.Fatalf("second debug should drop, got %v", err)
	}
	if err := p.Process(context.Background(), debugEvent); err != nil {
		t.Fatalf("third debug should pass, got %v", err)
	}

	infoEvent := &Event{Level: InfoLevel, Message: "info-msg"}
	if err := p.Process(context.Background(), infoEvent); err != nil {
		t.Fatalf("first info should pass, got %v", err)
	}
	if err := p.Process(context.Background(), infoEvent); !IsDropEvent(err) {
		t.Fatalf("second info should drop, got %v", err)
	}
	if err := p.Process(context.Background(), infoEvent); !IsDropEvent(err) {
		t.Fatalf("third info should drop, got %v", err)
	}
	if err := p.Process(context.Background(), infoEvent); err != nil {
		t.Fatalf("fourth info should pass, got %v", err)
	}
}

func TestSamplingProcessorNeverDropsErrors(t *testing.T) {
	p := NewSamplingProcessor(SamplingOptions{
		DebugEvery: 100,
		InfoEvery:  100,
		WarnEvery:  100,
	})

	for range 100 {
		if err := p.Process(context.Background(), &Event{
			Level:   ErrorLevel,
			Message: "always-keep",
		}); err != nil {
			t.Fatalf("error event should not be sampled out, got %v", err)
		}
	}
}
