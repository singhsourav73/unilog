package unilog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildFromConfigBuildsPipeline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")

	cfg := Config{
		Service:     "billing-api",
		Environment: "test",
		Level:       DebugLevel,
		Sinks: []SinkConfig{
			{
				Type: "async",
				Params: map[string]any{
					"buffer_size": 16,
				},
				Next: &SinkConfig{
					Type: "retry",
					Params: map[string]any{
						"max_attempts":    3,
						"initail_backoff": "1ms",
						"max_backoff":     "2ms",
						"multiplier":      2.0,
					},
					Next: &SinkConfig{
						Type: "file",
						Params: map[string]any{
							"path":   path,
							"format": "json",
						},
						MinLevel: levelPtr(InfoLevel),
					},
				},
			},
		},
		Processors: []ProcessorConfig{
			{
				Type: "redact",
				Params: map[string]any{
					"keys": []string{"password"},
					"mask": "[MASKED]",
				},
			},
		},
	}

	log, err := BuildFromConfig(cfg, nil)
	if err != nil {
		t.Fatalf("BuildFromConfig() error = %v", err)
	}

	log.Debug(context.Background(), "skip me", String("password", "secret"))
	log.Info(context.Background(), "keep me", String("password", "secret"))

	if err := log.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if err := log.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v; body=%s", err, string(b))
	}

	if decoded["message"] != "keep me" {
		t.Fatalf("message = %v, want keep me", decoded["message"])
	}

	if decoded["password"] != "[MASKED]" {
		t.Fatalf("password = %v, want [MASKED]", decoded["password"])
	}
}

func TestBuildFromConfigUnknownSinkFails(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{Type: "does-not-exist"},
		},
	}, nil)

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestBuildFromCOnfigAsyncWithoutNextFails(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{Type: "async"},
		},
	}, nil)

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func levelPtr(l Level) *Level {
	return &l
}
