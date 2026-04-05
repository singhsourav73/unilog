package unilog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigYAMLWithEnvExpansion(t *testing.T) {
	t.Setenv("UNILOG_SERVICE", "billing-api")
	t.Setenv("UNILOG_ENV", "prod")
	t.Setenv("UNILOG_URL", "https://logs.example.com/ingest")

	yamlConfig := `
service: ${UNILOG_SERVICE}
environment: ${UNILOG_ENV}
level: 1
sinks:
  - type: circuit_breaker
    params:
      failure_threshold: 3
      open_timeout: 2s
    next:
      type: http
      params:
        url: ${UNILOG_URL}
        method: POST
        headers:
          X-Env: ${UNILOG_ENV}
processors:
  - type: redact
    params:
      keys:
        - password
`

	cfg, err := LoadConfigYAMLBytes([]byte(yamlConfig))
	if err != nil {
		t.Fatalf("LoadConfigYAMLBytes() error = %v", err)
	}

	if cfg.Service != "billing-api" {
		t.Fatalf("service = %q, want billing-api", cfg.Service)
	}
	if cfg.Environment != "prod" {
		t.Fatalf("environment = %q, want prod", cfg.Environment)
	}
	if cfg.Sinks[0].Next == nil {
		t.Fatalf("expected nested next sink")
	}

	url, _ := cfg.Sinks[0].Next.Params["url"].(string)
	if url != "https://logs.example.com/ingest" {
		t.Fatalf("url = %q, want expanded url", url)
	}

	headers, ok := cfg.Sinks[0].Next.Params["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers type = %T, want map[string]any", cfg.Sinks[0].Next.Params["headers"])
	}
	if headers["X-Env"] != "prod" {
		t.Fatalf("header X-Env = %v, want prod", headers["X-Env"])
	}
}

func TestLoadConfigFileJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	content := `{
		"service": "billing-api",
		"environment": "test",
		"level": 1,
		"sinks": [
			{
				"type": "http",
				"params": {
					"url": "https://example.com/logs"
				}
			}
		]
	}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}

	if cfg.Service != "billing-api" {
		t.Fatalf("service = %q, want billing-api", cfg.Service)
	}
	if len(cfg.Sinks) != 1 {
		t.Fatalf("sinks len = %d, want 1", len(cfg.Sinks))
	}
	if cfg.Sinks[0].Type != "http" {
		t.Fatalf("sink type = %q, want http", cfg.Sinks[0].Type)
	}
}
