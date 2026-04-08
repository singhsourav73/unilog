# unilog

`unilog` is a lightweight, structured logging package for Go with a composable delivery pipeline.

It is designed around a simple rule:

> your application should describe log events once, and the logging pipeline should decide how those events are processed and delivered.

`unilog` provides:

- a small logger API for application code
- structured fields and normalized events
- pluggable processors and sinks
- async and batch delivery wrappers
- separate encoders and transports
- config-driven assembly for production pipelines

It is suitable for local development, services writing to stdout or files, and systems that need buffered or remote delivery over HTTP.

## Features

- leveled logging
- structured fields
- child loggers with `With`, `WithName`, and `WithContext`
- context-based correlation IDs
- JSON and text encoders
- writer, file, and HTTP sinks
- fanout, level filters, async, batching, retry, and circuit breaker wrappers
- redaction, sampling, and context enrichment processors
- JSON and YAML config loading
- environment variable expansion in config
- race-tested core behavior

## Installation

```bash
go get github.com/singhsourav73/unilog
```

## Quick start

```go
package main

import (
	"context"
	"errors"
	"os"

	"unilog"
)

func main() {
	sink := unilog.NewJSONSink(os.Stdout)

	log := unilog.New(sink, unilog.Options{
		Service:     "billing-api",
		Environment: "dev",
		Level:       unilog.InfoLevel,
		AddCaller:   true,
	})

	ctx := context.Background()
	ctx = unilog.WithTraceID(ctx, "trace-123")
	ctx = unilog.WithRequestID(ctx, "req-456")

	log.Info(ctx, "payment started",
		unilog.String("order_id", "ord-1"),
		unilog.Int("amount", 100),
	)

	log.Error(ctx, errors.New("gateway timeout"), "payment failed",
		unilog.String("order_id", "ord-1"),
		unilog.String("provider", "stripe"),
	)

	if err := log.Sync(ctx); err != nil {
		panic(err)
	}
}
```

## Package model

`unilog` is built from a few small concepts.

### Logger

Application code writes logs through a `Logger`.

A logger:

- creates normalized events
- adds configured metadata
- applies processors
- forwards events to sinks

Derived loggers are immutable.

### Event

Every log entry is normalized into an `Event` with fields such as:

- time
- level
- message
- service
- environment
- trace and request identifiers
- structured fields
- error
- caller
- stack

### Encoder

Encoders turn events into bytes for transport sinks.

Built-in encoders:

- `JSONEncoder`
- `TextEncoder`

### Sink

A sink is a destination or delivery stage for events.

Built-in sinks and wrappers:

- `NopSink`
- `WriterSink`
- `JSONSink`
- `TextSink`
- `FileSink`
- `HTTPSink`
- `FanoutSink`
- `LevelFilterSink`
- `AsyncSink`
- `BatchSink`
- `RetrySink`
- `CircuitBreakerSink`

### Processor

Processors can inspect, mutate, or drop events before they are delivered.

Built-in processors:

- `RedactionProcessor`
- `SamplingProcessor`
- `ContextEnricher`

## Basic usage

### Structured fields

```go
log.Info(ctx, "user login",
	unilog.String("user_id", "u-123"),
	unilog.Bool("success", true),
	unilog.Int("attempt", 1),
)
```

### Child loggers

```go
base := log.With(unilog.String("component", "auth"))
child := base.WithName("token")

base.Info(ctx, "base logger")
child.Info(ctx, "child logger")
```

### Context correlation

```go
ctx := context.Background()
ctx = unilog.WithTraceID(ctx, "trace-123")
ctx = unilog.WithSpanID(ctx, "span-456")
ctx = unilog.WithRequestID(ctx, "req-789")
ctx = unilog.WithCorrelationID(ctx, "corr-999")

log.Info(ctx, "request received")
```

## Encoders and sinks

### JSON to stdout

```go
sink := unilog.NewJSONSink(os.Stdout)
log := unilog.New(sink, unilog.Options{
	Service: "billing-api",
	Level:   unilog.DebugLevel,
})
```

### Text output

```go
sink := unilog.NewTextSink(os.Stdout, unilog.TextSinkOptions{})
log := unilog.New(sink, unilog.Options{
	Service: "billing-api",
})
```

### File sink

```go
sink, err := unilog.NewFileSink(unilog.FileSinkOptions{
	Path:    "app.log",
	Encoder: unilog.NewJSONEncoder(),
})
if err != nil {
	panic(err)
}
defer sink.Close(context.Background())
```

### HTTP sink

```go
sink, err := unilog.NewHTTPSink(unilog.HTTPSinkOptions{
	URL:     "https://logs.example.com/ingest",
	Method:  "POST",
	Encoder: unilog.NewJSONEncoder(),
})
if err != nil {
	panic(err)
}
```

## Composition

### Fanout

```go
sink := unilog.NewFanoutSink(
	unilog.NewTextSink(os.Stdout, unilog.TextSinkOptions{}),
	unilog.NewJSONSink(os.Stdout),
)
```

### Level filtering

```go
textSink := unilog.NewLevelFilterSink(
	unilog.DebugLevel,
	unilog.NewTextSink(os.Stdout, unilog.TextSinkOptions{}),
)

jsonSink := unilog.NewLevelFilterSink(
	unilog.InfoLevel,
	unilog.NewJSONSink(os.Stdout),
)

sink := unilog.NewFanoutSink(textSink, jsonSink)
```

## Delivery wrappers

### Async delivery

```go
sink := unilog.NewAsyncSink(
	unilog.NewJSONSink(os.Stdout),
	unilog.AsyncSinkOptions{
		BufferSize:     256,
		OverflowPolicy: unilog.OverflowDropNewest,
	},
)
```

Supported overflow policies:

- `OverflowDropNewest`
- `OverflowBlock`

`AsyncSink.Sync()` and `AsyncSink.Close()` surface accumulated worker errors.

### Batch delivery

```go
httpSink, err := unilog.NewHTTPSink(unilog.HTTPSinkOptions{
	URL:     "https://logs.example.com/ingest",
	Encoder: unilog.NewJSONEncoder(),
})
if err != nil {
	panic(err)
}

sink := unilog.NewBatchSink(httpSink, unilog.BatchSinkOptions{
	MaxBatchSize:   100,
	FlushInterval:  time.Second,
	MaxQueueSize:   400,
	OverflowPolicy: unilog.OverflowDropNewest,
})
```

`BatchSink` flushes when:

- the batch reaches `MaxBatchSize`
- the flush interval expires
- `Sync()` is called
- `Close()` is called

If the downstream sink implements batch writing, `BatchSink` uses it. Otherwise it falls back to single-event writes.

### Retry wrapper

```go
sink := unilog.NewRetrySink(
	httpSink,
	unilog.RetrySinkOptions{
		MaxAttempts:    3,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     time.Second,
		Multiplier:     2,
	},
)
```

### Circuit breaker wrapper

```go
sink := unilog.NewCircuitBreakerSink(
	httpSink,
	unilog.CircuitBreakerOptions{
		FailureThreshold: 3,
		OpenTimeout:      5 * time.Second,
	},
)
```

## Processors

### Redaction

```go
processor := unilog.NewRedactionProcessor(unilog.RedactionOptions{
	Keys:            []string{"password", "token", "authorization"},
	Mask:            "***REDACTED***",
	CaseInsensitive: true,
})
```

### Sampling

```go
processor := unilog.NewSamplingProcessor(unilog.SamplingOptions{
	DebugEvery: 10,
	InfoEvery:  5,
	WarnEvery:  1,
})
```

### Context enrichment

```go
processor := unilog.NewContextEnricher(unilog.ContextEnricherOptions{
	IncludeUserID:    true,
	IncludeTenantID:  true,
	IncludeSessionID: true,
	UserFieldName:    "user_id",
	TenantFieldName:  "tenant_id",
	SessionFieldName: "session_id",
})
```

## Error handling

Logging methods do not return delivery errors directly.

Internal pipeline errors are routed through `OnInternalError`:

```go
log := unilog.New(
	unilog.NewJSONSink(os.Stdout),
	unilog.Options{
		Service: "billing-api",
		OnInternalError: func(ctx context.Context, err error) {
			fmt.Fprintf(os.Stderr, "unilog internal error: %v\n", err)
		},
	},
)
```

Wrapper sinks such as `AsyncSink` and `BatchSink` expose delivery errors through `Sync()` and `Close()`.

## Fatal behavior

`Fatal()` logs at `FatalLevel`.

It does **not** call `os.Exit`. Process termination remains the application’s responsibility.

## Configuration

`unilog` can build a logger pipeline from config.

### Build from config in code

```go
cfg := unilog.Config{
	Service:     "billing-api",
	Environment: "prod",
	Level:       unilog.DebugLevel,
	Sinks: []unilog.SinkConfig{
		{
			Type: "batch",
			Params: map[string]any{
				"max_batch_size":  100,
				"flush_interval":  "1s",
				"max_queue_size":  400,
				"overflow_policy": "drop_newest",
			},
			Next: &unilog.SinkConfig{
				Type: "async",
				Params: map[string]any{
					"buffer_size":     512,
					"overflow_policy": "drop_newest",
				},
				Next: &unilog.SinkConfig{
					Type: "http",
					Params: map[string]any{
						"url":    "https://logs.example.com/ingest",
						"method": "POST",
						"format": "json",
					},
				},
			},
		},
	},
	Processors: []unilog.ProcessorConfig{
		{
			Type: "redact",
			Params: map[string]any{
				"keys": []string{"password", "token"},
			},
		},
	},
}

log, err := unilog.BuildFromConfig(cfg, nil)
if err != nil {
	panic(err)
}
```

### Load JSON or YAML

```go
cfg, err := unilog.LoadConfigFile("unilog.yaml")
if err != nil {
	panic(err)
}

log, err := unilog.BuildFromConfig(cfg, nil)
if err != nil {
	panic(err)
}
```

### Supported built-in sink types

- `nop`
- `json`
- `text`
- `file`
- `http`

### Supported sink wrappers

- `async`
- `batch`
- `retry`
- `circuit_breaker`

### Supported processors

- `redact`
- `sampling`
- `context_enricher`

### Environment expansion

```yaml
service: ${SERVICE_NAME}
environment: ${ENVIRONMENT}
sinks:
  - type: http
    params:
      url: ${LOG_INGEST_URL}
      method: POST
      format: json
```

## Examples

### Local development

Use text logs to stdout:

```go
sink := unilog.NewTextSink(os.Stdout, unilog.TextSinkOptions{})
log := unilog.New(sink, unilog.Options{
	Service: "billing-api",
	Level:   unilog.DebugLevel,
})
```

### Production-style HTTP delivery

```go
httpSink, err := unilog.NewHTTPSink(unilog.HTTPSinkOptions{
	URL:     "https://logs.example.com/ingest",
	Encoder: unilog.NewJSONEncoder(),
})
if err != nil {
	panic(err)
}

batchSink := unilog.NewBatchSink(httpSink, unilog.BatchSinkOptions{
	MaxBatchSize:   100,
	FlushInterval:  time.Second,
	MaxQueueSize:   400,
	OverflowPolicy: unilog.OverflowDropNewest,
})

asyncSink := unilog.NewAsyncSink(batchSink, unilog.AsyncSinkOptions{
	BufferSize:     512,
	OverflowPolicy: unilog.OverflowDropNewest,
})

log := unilog.New(asyncSink, unilog.Options{
	Service:     "billing-api",
	Environment: "prod",
	Level:       unilog.InfoLevel,
})
```

## Performance notes

- `AddCaller` and `AddStack` increase overhead
- JSON encoding is typically better for machine ingestion
- text encoding is easier for local debugging
- `AsyncSink` reduces caller-path latency
- `BatchSink` improves remote delivery efficiency
- `OverflowDropNewest` avoids blocking under pressure
- `OverflowBlock` preserves events at the cost of latency

## API stability

The package is still evolving and some APIs may continue to change while the architecture settles.

At this stage, the core design is already in place:

- logger API
- event normalization
- processors
- encoders
- sinks and wrappers
- config-driven assembly

## Roadmap

Near-term directions include:

- repository/package structure cleanup
- stronger backend-specific adapters
- improved observability around delivery stats
- more examples and docs
- additional encoder and sink capabilities

## Contributing

Contributions and design feedback are welcome.

A few guiding principles for changes:

- keep the application-facing API small
- keep the core vendor-neutral
- prefer explicit delivery semantics
- avoid hidden global state
- preserve composability
