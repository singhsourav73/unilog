package unilog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileSinkJSONWritesToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.json.log")

	sink, err := NewFileSink(FileSinkOptions{
		Path:    path,
		Encoder: NewJSONEncoder(),
	})
	if err != nil {
		t.Fatalf("NewFileSInk() error = %v", err)
	}

	ev := Event{
		Time:      time.Unix(0, 0).UTC(),
		Level:     InfoLevel,
		Message:   "hello",
		Service:   "billing-api",
		RequestID: "req-1",
		Fields: []Field{
			String("order_id", "ord-1"),
		},
	}

	if err := sink.Write(context.Background(), ev); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := sink.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if err := sink.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	out := string(b)
	checks := []string{
		`"message":"hello"`,
		`"service":"billing-api"`,
		`"request_id":"req-1"`,
		`"order_id":"ord-1"`,
	}

	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
}
