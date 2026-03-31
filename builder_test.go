package unilog

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestBuildFromCOnfigBuildsPipeline(t *testing.T) {
	var buf bytes.Buffer

	cfg := Config{
		Service:     "billing-api",
		Environment: "test",
		Level:       DebugLevel,
		Sinks: []SinkConfig{
			{
				Type: "json",
				Params: map[string]any{
					"writer": &buf,
				},
				MinLevel: levelPtr(InfoLevel),
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

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v; body=%s", err, buf.String())
	}

	if decoded["message"] != "keep me" {
		t.Fatalf("message = %v, want keep me", decoded["message"])
	}

	if decoded["password"] != "[MASKED]" {
		t.Fatalf("password = %v, want [MASKED]", decoded["password"])
	}
}

func TestBuildFromCOnfigUnknownSinkFails(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{Type: "does-not-exist"},
		},
	}, nil)

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func levelPtr(l Level) *Level {
	return &l
}
