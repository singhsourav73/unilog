package unilog

import (
	"context"
	"testing"
)

func TestContextEnricherAddsFields(t *testing.T) {
	p := NewContextEnricher(ContextEnricherOptions{})

	ctx := context.Background()
	ctx = WithUserID(ctx, "u-123")
	ctx = WithTenantID(ctx, "t-456")
	ctx = WithSessionID(ctx, "s-789")

	ev := &Event{
		Message: "hello",
	}

	if err := p.Process(ctx, ev); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	got := map[string]any{}
	for _, f := range ev.Fields {
		got[f.Key] = f.Value
	}

	if got["user_id"] != "u-123" {
		t.Fatalf("user_id = %v, want u-123", got["user_id"])
	}
	if got["tenant_id"] != "t-456" {
		t.Fatalf("tenant_id = %v, want t-456", got["tenant_id"])
	}
	if got["session_id"] != "s-789" {
		t.Fatalf("session_id = %v, want s-789", got["session_id"])
	}
}

func TestContextEnricherDoesNotOverwriteExistingFields(t *testing.T) {
	p := NewContextEnricher(ContextEnricherOptions{})

	ctx := WithUserID(context.Background(), "u-123")
	ev := &Event{
		Message: "hello",
		Fields: []Field{
			String("user_id", "existing"),
		},
	}

	if err := p.Process(ctx, ev); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	var count int
	var value string
	for _, f := range ev.Fields {
		if f.Key == "user_id" {
			count++
			if s, ok := f.Value.(string); ok {
				value = s
			}
		}
	}

	if count != 1 {
		t.Fatalf("user_id count = %d, want 1", count)
	}
	if value != "existing" {
		t.Fatalf("user_id value = %q, want existing", value)
	}
}
