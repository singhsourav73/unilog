package unilog

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildFromConfigContextEnricherUsesUserFieldName(t *testing.T) {
	var out strings.Builder
	cfg := Config{
		Level: InfoLevel,
		Sinks: []SinkConfig{
			{
				Type: "json",
				Params: map[string]any{
					"writer": &out,
				},
			},
		},
		Processors: []ProcessorConfig{
			{
				Type: "context_enricher",
				Params: map[string]any{
					"user_field_name":    "actor_id",
					"tenant_field_name":  "org_id",
					"session_field_name": "session",
				},
			},
		},
	}

	log, err := BuildFromConfig(cfg, nil)
	if err != nil {
		t.Fatalf("BuildFromConfig() error = %v", err)
	}

	ctx := context.Background()
	ctx = WithUserID(ctx, "u-1")
	ctx = WithTenantID(ctx, "t-1")
	ctx = WithSessionID(ctx, "s-1")
	log.Info(ctx, "request done")

	var decoded map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if got := decoded["actor_id"]; got != "u-1" {
		t.Fatalf("actor_id = %v, want u-1", got)
	}
	if got := decoded["org_id"]; got != "t-1" {
		t.Fatalf("org_id = %v, want t-1", got)
	}
	if got := decoded["session"]; got != "s-1" {
		t.Fatalf("session = %v, want s-1", got)
	}
	if _, exists := decoded["user_id"]; exists {
		t.Fatalf("unexpected default user_id key in output")
	}
}

func levelPtr(l Level) *Level {
	return &l
}

func TestBuildFromConfigRejectsInvalidAsyncBufferSizeType(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{
				Type: "async",
				Params: map[string]any{
					"buffer_size": "large",
				},
				Next: &SinkConfig{
					Type: "json",
					Params: map[string]any{
						"writer": &bytes.Buffer{},
					},
				},
			},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), `"buffer_size"`) {
		t.Fatalf("expected buffer_size type error, got %v", err)
	}
}

func TestBuildFromConfigRejectsInvalidHTTPHeadersShape(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{
				Type: "http",
				Params: map[string]any{
					"url":     "http://example.com",
					"headers": []string{"bad"},
				},
			},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), `"headers"`) {
		t.Fatalf("expected headers type error, got %v", err)
	}
}

func TestBuildFromConfigRejectsInvalidHTTPTimeout(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{
				Type: "http",
				Params: map[string]any{
					"url":     "http://example.com",
					"timeout": "definitely-not-a-duration",
				},
			},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), `"timeout"`) {
		t.Fatalf("expected timeout parse error, got %v", err)
	}
}

func TestBuildFromConfigRejectsInvalidRedactionFlagType(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Processors: []ProcessorConfig{
			{
				Type: "redact",
				Params: map[string]any{
					"keys":             []string{"password"},
					"case_insensitive": "yes",
				},
			},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), `"case_insensitive"`) {
		t.Fatalf("expected case_insensitive type error, got %v", err)
	}
}

func TestBuildFromConfigRejectsInvalidSamplingValue(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Processors: []ProcessorConfig{
			{
				Type: "sampling",
				Params: map[string]any{
					"debug_every": -1,
				},
			},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), `"debug_every"`) {
		t.Fatalf("expected debug_every validation error, got %v", err)
	}
}

func TestBuildFromConfigRejectsInvalidOverflowPolicy(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{
				Type: "async",
				Params: map[string]any{
					"overflow_policy": "explode",
				},
				Next: &SinkConfig{
					Type: "json",
					Params: map[string]any{
						"writer": &bytes.Buffer{},
					},
				},
			},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), `"explode"`) {
		t.Fatalf("expected overflow_policy parse error, got %v", err)
	}
}

func TestBuildFromConfigRejectsInvalidOverflowPolicyType(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{
				Type: "async",
				Params: map[string]any{
					"overflow_policy": 123,
				},
				Next: &SinkConfig{
					Type: "json",
					Params: map[string]any{
						"writer": &bytes.Buffer{},
					},
				},
			},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), `"overflow_policy"`) {
		t.Fatalf("expected overflow_policy type error, got %v", err)
	}
}

func TestBuildFromConfigRejectsInvalidBatchOverflowPolicy(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{
				Type: "batch",
				Params: map[string]any{
					"overflow_policy": "explode",
				},
				Next: &SinkConfig{
					Type: "http",
					Params: map[string]any{
						"url": "http://example.com",
					},
				},
			},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), `"explode"`) {
		t.Fatalf("expected overflow_policy parse error, got %v", err)
	}
}

func TestBuildFromConfigRejectsInvalidBatchFlushInterval(t *testing.T) {
	_, err := BuildFromConfig(Config{
		Sinks: []SinkConfig{
			{
				Type: "batch",
				Params: map[string]any{
					"flush_interval": "not-a-duration",
				},
				Next: &SinkConfig{
					Type: "http",
					Params: map[string]any{
						"url": "http://example.com",
					},
				},
			},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), `"flush_interval"`) {
		t.Fatalf("expected flush_interval parse error, got %v", err)
	}
}
