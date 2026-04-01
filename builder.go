package unilog

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"time"
)

type SinkFactory func(params map[string]any) (Sink, error)
type ProcessorFactory func(params map[string]any) (Processor, error)

type Registry struct {
	mu         sync.RWMutex
	sinks      map[string]SinkFactory
	processors map[string]ProcessorFactory
}

func NewRegistry() *Registry {
	r := &Registry{
		sinks:      make(map[string]SinkFactory),
		processors: make(map[string]ProcessorFactory),
	}
	r.registerBuiltins()
	return r
}

func (r *Registry) RegisterSink(name string, factory SinkFactory) error {
	if factory == nil {
		return fmt.Errorf("unilog: sink factory for %q is nil", name)
	}

	name = normalizeName(name)
	if name == "" {
		return fmt.Errorf("unilog: sink name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sinks[name]; exists {
		return fmt.Errorf("unilog: sink %q already registered", name)
	}
	r.sinks[name] = factory
	return nil
}

func (r *Registry) RegisterProcessor(name string, factory ProcessorFactory) error {
	if factory == nil {
		return fmt.Errorf("unilog: processor factory for %q is nil", name)
	}

	name = normalizeName(name)
	if name == "" {
		return fmt.Errorf("unilog: processor name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.processors[name]; exists {
		return fmt.Errorf("unilog: processor %q already registered", name)
	}
	r.processors[name] = factory
	return nil
}

func (r *Registry) sinkFactory(name string) (SinkFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.sinks[normalizeName(name)]
	return f, ok
}

func (r *Registry) processorFactory(name string) (ProcessorFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.processors[normalizeName(name)]
	return f, ok
}

func (r *Registry) registerBuiltins() {
	_ = r.RegisterSink("nop", func(_ map[string]any) (Sink, error) {
		return NopSink{}, nil
	})

	_ = r.RegisterSink("json", func(params map[string]any) (Sink, error) {
		w, err := getWriterParam(params, "writer")
		if err != nil {
			return nil, err
		}
		return NewJSONSink(w), nil
	})

	_ = r.RegisterSink("text", func(params map[string]any) (Sink, error) {
		w, err := getWriterParam(params, "writer")
		if err != nil {
			return nil, err
		}
		return NewTextSink(w, TextSinkOptions{
			TimeLayout: getStringParam(params, "time_layout", ""),
		}), nil
	})

	_ = r.RegisterSink("file", func(params map[string]any) (Sink, error) {
		path := getStringParam(params, "path", "")
		if path == "" {
			return nil, fmt.Errorf("unilog: missing %q", "path")
		}

		return NewFileSink(FileSinkOptions{
			Path:   path,
			Format: getStringParam(params, "format", "json"),
			Perm:   getFileModeParam(params, "perm", 0o644),
			TextOptions: TextSinkOptions{
				TimeLayout: getStringParam(params, "time_layout", ""),
			},
		})
	})

	_ = r.RegisterProcessor("redact", func(params map[string]any) (Processor, error) {
		keys, err := getStringSliceParam(params, "keys")
		if err != nil {
			return nil, err
		}
		return NewRedactionProcessor(RedactionOptions{
			Keys:            keys,
			Mask:            getStringParam(params, "mask", "***REDACTED***"),
			CaseInsensitive: getBoolParam(params, "case_insensitive", true),
			RedactError:     getBoolParam(params, "redact_error", false),
		}), nil
	})

	_ = r.RegisterProcessor("sampling", func(params map[string]any) (Processor, error) {
		return NewSamplingProcessor(SamplingOptions{
			DebugEvery: getUint64Param(params, "debug_every", 1),
			InfoEvery:  getUint64Param(params, "info_every", 1),
			WarnEvery:  getUint64Param(params, "warn_every", 1),
		}), nil
	})
}

type Config struct {
	Service     string
	Environment string
	Level       Level
	AddCaller   bool
	AddStack    bool

	Sinks      []SinkConfig
	Processors []ProcessorConfig

	OnInternalError InternalErrorHandler
}

type SinkConfig struct {
	Type     string
	Params   map[string]any
	MinLevel *Level

	Next  *SinkConfig
	Sinks []SinkConfig
}

type ProcessorConfig struct {
	Type   string
	Params map[string]any
}

func BuildFromConfig(cfg Config, registry *Registry) (Logger, error) {
	if registry == nil {
		registry = NewRegistry()
	}

	processors := make([]Processor, 0, len(cfg.Processors))
	for _, pc := range cfg.Processors {
		factory, ok := registry.processorFactory(pc.Type)
		if !ok {
			return nil, fmt.Errorf("unilog: unknown processor type %q", pc.Type)
		}
		p, err := factory(copyMap(pc.Params))
		if err != nil {
			return nil, fmt.Errorf("unilog: build processor %q: %w", pc.Type, err)
		}
		processors = append(processors, p)
	}

	sink, err := buildSinks(cfg.Sinks, registry)
	if err != nil {
		return nil, err
	}

	return New(sink, Options{
		Service:         cfg.Service,
		Environment:     cfg.Environment,
		Level:           cfg.Level,
		AddCaller:       cfg.AddCaller,
		AddStack:        cfg.AddStack,
		OnInternalError: cfg.OnInternalError,
	}, processors...), nil
}

func buildSinks(configs []SinkConfig, registry *Registry) (Sink, error) {
	switch len(configs) {
	case 0:
		return NopSink{}, nil
	case 1:
		return buildSink(configs[0], registry)
	default:
		children := make([]Sink, 0, len(configs))
		for _, sc := range configs {
			child, err := buildSink(sc, registry)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		return NewFanoutSink(children...), nil
	}
}

func buildSink(cfg SinkConfig, registry *Registry) (Sink, error) {
	t := normalizeName(cfg.Type)

	var sink Sink
	switch t {
	case "fanout":
		if len(cfg.Sinks) == 0 {
			return nil, fmt.Errorf("unilog: fanout sink requires child sinks")
		}
		children := make([]Sink, 0, len(cfg.Sinks))
		for _, childCfg := range cfg.Sinks {
			child, err := buildSink(childCfg, registry)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		sink = NewFanoutSink(children...)

	case "async":
		if cfg.Next == nil {
			return nil, fmt.Errorf("unilog: async sink requires next sink")
		}
		next, err := buildSink(*cfg.Next, registry)
		if err != nil {
			return nil, err
		}
		sink = NewAsyncSink(next, AsyncSinkOptions{
			BufferSize:  getIntParam(cfg.Params, "buffer_size", 256),
			BlockOnFull: getBoolParam(cfg.Params, "block_on_full", false),
		})

	case "retry":
		if cfg.Next == nil {
			return nil, fmt.Errorf("unilog: retry sink requires next sink")
		}
		next, err := buildSink(*cfg.Next, registry)
		if err != nil {
			return nil, err
		}
		sink = NewRetrySink(next, RetrySinkOptions{
			MaxAttempts:    getIntParam(cfg.Params, "max_attempts", 3),
			InitialBackoff: getDurationParam(cfg.Params, "initial_backoff", 50*time.Millisecond),
			MaxBackoff:     getDurationParam(cfg.Params, "max_backoff", time.Second),
			Multiplier:     getFloat64Param(cfg.Params, "multiplier", 2),
		})

	default:
		factory, ok := registry.sinkFactory(cfg.Type)
		if !ok {
			return nil, fmt.Errorf("unilog: unknown sink type %q", cfg.Type)
		}
		var err error
		sink, err = factory(copyMap(cfg.Params))
		if err != nil {
			return nil, fmt.Errorf("unilog: build sink %q: %w", cfg.Type, err)
		}
	}

	if cfg.MinLevel != nil {
		sink = NewLevelFilterSink(*cfg.MinLevel, sink)
	}

	return sink, nil
}

func copyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func normalizeName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func getWriterParam(params map[string]any, key string) (io.Writer, error) {
	v, ok := params[key]
	if !ok {
		return nil, fmt.Errorf("unilog: missing %q", key)
	}
	w, ok := v.(io.Writer)
	if !ok {
		return nil, fmt.Errorf("unilog: %q must implement io.Writer", key)
	}
	return w, nil
}

func getStringParam(params map[string]any, key, def string) string {
	v, ok := params[key]
	if !ok {
		return def
	}
	s, ok := v.(string)
	if !ok {
		return def
	}
	return s
}

func getBoolParam(params map[string]any, key string, def bool) bool {
	v, ok := params[key]
	if !ok {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

func getUint64Param(params map[string]any, key string, def uint64) uint64 {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case uint64:
		return x
	case uint32:
		return uint64(x)
	case uint:
		return uint64(x)
	case int:
		if x >= 0 {
			return uint64(x)
		}
	case int64:
		if x >= 0 {
			return uint64(x)
		}
	}
	return def
}

func getIntParam(params map[string]any, key string, def int) int {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case int32:
		return int(x)
	case uint:
		return int(x)
	case uint64:
		return int(x)
	case float64:
		return int(x)
	default:
		return def
	}
}

func getFloat64Param(params map[string]any, key string, def float64) float64 {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return def
	}
}

func getDurationParam(params map[string]any, key string, def time.Duration) time.Duration {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case time.Duration:
		return x
	case string:
		d, err := time.ParseDuration(x)
		if err != nil {
			return def
		}
		return d
	case int:
		return time.Duration(x)
	case int64:
		return time.Duration(x)
	case float64:
		return time.Duration(x)
	default:
		return def
	}
}

func getFileModeParam(params map[string]any, key string, def fs.FileMode) fs.FileMode {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case fs.FileMode:
		return x
	case uint32:
		return fs.FileMode(x)
	case uint64:
		return fs.FileMode(x)
	case int:
		return fs.FileMode(x)
	default:
		return def
	}
}

func getStringSliceParam(params map[string]any, key string) ([]string, error) {
	v, ok := params[key]
	if !ok {
		return nil, fmt.Errorf("unilog: missing %q", key)
	}

	switch x := v.(type) {
	case []string:
		out := make([]string, len(x))
		copy(out, x)
		return out, nil
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("unilog: %q contains non-string value", key)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unilog: %q must be []string", key)
	}
}

func joinErrors(errs ...error) error {
	var nonNil []error
	for _, err := range errs {
		if err != nil {
			nonNil = append(nonNil, err)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	return fmt.Errorf("%w", nonNil[0])
}
