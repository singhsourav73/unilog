package unilog

import (
	"context"
	"errors"
	"testing"
)

func TestRedactionProcessorRedactsTopLevelANsNestedFiels(t *testing.T) {
	p := NewRedactionProcessor(RedactionOptions{
		Keys:            []string{"password", "token", "authorization", "error"},
		Mask:            "[REDACTED]",
		CaseInsensitive: true,
		RedactError:     true,
	})

	ev := &Event{
		Err: errors.New("very secret error"),
		Fields: []Field{
			String("password", "p@ss"),
			Any("metadata", map[string]any{
				"token": "abc123",
				"profile": map[string]any{
					"authorization": "Bearer xyz",
				},
			}),
		},
	}

	if err := p.Process(context.Background(), ev); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if ev.Err == nil || ev.Err.Error() != "[REDACTED]" {
		t.Fatalf("event.Err = %v,, want [REDACTED]", ev.Err)
	}

	if got := ev.Fields[0].Value; got != "[REDACTED]" {
		t.Fatalf("password = %v, want [REDACTED]", got)
	}

	meta, ok := ev.Fields[1].Value.(map[string]any)
	if !ok {
		t.Fatalf("metadata has wrong type: %T", ev.Fields[1].Value)
	}

	if got := meta["token"]; got != "[REDACTED]" {
		t.Fatalf("token = %v, want [REDACTED]", got)
	}

	profile := meta["profile"].(map[string]any)
	if got := profile["authorization"]; got != "[REDACTED]" {
		t.Fatalf("authorization = %v, want [REDACTED]", got)
	}
}
