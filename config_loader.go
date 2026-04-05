package unilog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadConfigFile(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("unilog: read config file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return LoadConfigJSONBytes(b)
	case ".yaml", ".yml":
		return LoadConfigYAMLBytes(b)
	default:
		return Config{}, fmt.Errorf("unilog: unsupported config file extension %q", ext)
	}
}

func LoadConfigJSON(r io.Reader) (Config, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return Config{}, fmt.Errorf("unilog: read json config: %w", err)
	}
	return LoadConfigJSONBytes(b)
}

func LoadConfigJSONBytes(b []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("unilog: unmarshal json config: %w", err)
	}
	return ExpandEnvConfig(cfg), nil
}

func LoadConfigYAML(r io.Reader) (Config, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return Config{}, fmt.Errorf("unilog: read yaml config: %w", err)
	}
	return LoadConfigYAMLBytes(b)
}

func LoadConfigYAMLBytes(b []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("unilog: unmarshal yaml config: %w", err)
	}
	return ExpandEnvConfig(cfg), nil
}

func ExpandEnvConfig(cfg Config) Config {
	cfg.Service = os.ExpandEnv(cfg.Service)
	cfg.Environment = os.ExpandEnv(cfg.Environment)

	for i := range cfg.Sinks {
		cfg.Sinks[i] = expandEnvSinkConfig(cfg.Sinks[i])
	}

	for i := range cfg.Processors {
		cfg.Processors[i] = expandEnvProcessorConfig(cfg.Processors[i])
	}

	return cfg
}

func expandEnvSinkConfig(cfg SinkConfig) SinkConfig {
	cfg.Type = os.ExpandEnv(cfg.Type)
	cfg.Params = expandEnvMap(cfg.Params)

	if cfg.Next != nil {
		next := expandEnvSinkConfig((*cfg.Next))
		cfg.Next = &next
	}

	for i := range cfg.Sinks {
		cfg.Sinks[i] = expandEnvSinkConfig(cfg.Sinks[i])
	}

	return cfg
}

func expandEnvProcessorConfig(cfg ProcessorConfig) ProcessorConfig {
	cfg.Type = os.ExpandEnv(cfg.Type)
	cfg.Params = expandEnvMap(cfg.Params)
	return cfg
}

func expandEnvMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return in
	}

	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = expandEnvValue(v)
	}
	return out
}

func expandEnvValue(v any) any {
	switch x := v.(type) {
	case string:
		return os.ExpandEnv(x)

	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = expandEnvValue(item)
		}
		return out

	case map[string]any:
		return expandEnvMap(x)

	case []string:
		out := make([]string, len(x))
		for i, s := range x {
			out[i] = os.ExpandEnv(s)
		}
		return out

	default:
		return v
	}
}
