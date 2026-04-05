# unilog

`unilog` is a lightweight, vendor-agnostic structured logging package for Go.

It is designed around a simple idea:

> application code should describe events, not logging vendors

`unilog` provides a stable logger API, a normalized event model, pluggable sinks, event processors, per-sink level routing, and config-based pipeline assembly. It is built to make observability portable, composable, and easier to evolve over time.

## Why `unilog`?

In many Go services, logging starts simple and gradually turns into infrastructure:

- logs go to stdout
- errors go to another platform
- trace and request IDs get added later
- sensitive fields need redaction
- noisy logs need sampling
- switching providers becomes painful

`unilog` addresses that by separating:

- **event creation** from **event delivery**
- **application logging API** from **vendor-specific integrations**
- **event transformation** from **sink implementations**

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

	"github.com/yourorg/unilog"
)

func main() {
	sink := unilog.NewJSONSink(os.Stdout)

	log := unilog.New(sink, unilog.Options{
		Service:     "billing-api",
		Environment: "dev",
		Level:       unilog.DebugLevel,
		AddCaller:   true,
		AddStack:    true,
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
}
```

### Example output:

```json
{"time":"2026-04-01T10:00:00Z","level":"info","message":"payment started","service":"billing-api","environment":"dev","trace_id":"trace-123","request_id":"req-456","order_id":"ord-1","amount":100}
{"time":"2026-04-01T10:00:01Z","level":"error","message":"payment failed","service":"billing-api","environment":"dev","trace_id":"trace-123","request_id":"req-456","error":"gateway timeout","order_id":"ord-1","provider":"stripe"}
```

## Core concepts

### Logger

The logger is the API your appication uses

```go
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, err error, msg string, fields ...Field)
	Fatal(ctx context.Context, err error, msg string, fields ...Field)

	Log(ctx context.Context, level Level, msg string, fields ...Field)
	LogErr(ctx context.Context, level Level, err error, msg string, fields ...Field)

	With(fields ...Field) Logger
	WithName(name string) Logger
	WithContext(ctx context.Context) Logger

	Enabled(level Level) bool
	Sync(ctx context.Context) error
	Close(ctx context.Context) error
}
```

### Event

Every log entry is converted into a normalized event before it is processed or written to a backened

```go
type Event struct {
	Time          time.Time
	Level         Level
	Message       string
	Err           error
	Service       string
	Environment   string
	LoggerName    string
	TraceID       string
	SpanID        string
	RequestID     string
	CorrelationID string
	Fields        []Field
	Caller        *Caller
	Stack         string
}
```

### Sink

A sink is a destination for events.
Built-in sinks in Milestone 2:

- JSONSink
- TextSink
- FanoutSink
- LevelFilterSink
- NopSink

### Processor

A processor can inspect, modify, or drop an event before it reaches sinks.

Built-in processors in Milestone 2:

- RedactionProcessor
- SamplingProcessor

## Feature

### Structured fields

```go
log.Info(ctx, "user login",
	unilog.String("user_id", "u-123"),
	unilog.Bool("success", true),
	unilog.Int("attempt", 1),
)
```

### Immutable child loggers

`With`, `WithName`, and `WithContext` return derived loggers. They do not mutate the original logger.

```go
base := log.With(unilog.String("component", "auth"))
child := base.With(unilog.String("subsystem", "token"))

base.Info(ctx, "base logger")
child.Info(ctx, "child logger")
```

### Context propagation

`unilog` can carry request-scoped metadata through `context.Context`.

```go
ctx := context.Background()
ctx = unilog.WithTraceID(ctx, "trace-123")
ctx = unilog.WithSpanID(ctx, "span-456")
ctx = unilog.WithRequestID(ctx, "req-789")
ctx = unilog.WithCorrelationID(ctx, "corr-999")
```

These values are automatically attached to emitted events.

### JSON output

Use `JSONSink` when logs are meant for machines and log ingestion pipelines.

```go
sink := unilog.NewJSONSink(os.Stdout)
log := unilog.New(sink, unilog.Options{Service: "billing-api"})
```

### Human-readable text output

Use `TextSink` for local development and debugging.

```go
sink := unilog.NewTextSink(os.Stdout, unilog.TextSinkOptions{})
log := unilog.New(sink, unilog.Options{Service: "billing-api"})
```

### Fanout to multiple destinations

A single event can be written to multiple sinks.

```go
sink := unilog.NewFanoutSink(
	unilog.NewTextSink(os.Stdout, unilog.TextSinkOptions{}),
	unilog.NewJSONSink(os.Stdout),
)
log := unilog.New(sink, unilog.Options{Service: "billing-api"})
```

### Per-sink level routing

Wrap a sink with `LevelFilterSink` to only forward events at or above a given level.

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
log := unilog.New(sink, unilog.Options{Service: "billing-api"})
```

### Redaction

Protect sensitive fields before they are written anywhere.

```go
processor := unilog.NewRedactionProcessor(unilog.RedactionOptions{
	Keys:            []string{"password", "token", "authorization"},
	Mask:            "***REDACTED***",
	CaseInsensitive: true,
})

log := unilog.New(
	unilog.NewJSONSink(os.Stdout),
	unilog.Options{Service: "billing-api"},
	processor,
)

log.Info(ctx, "user login",
	unilog.String("user_id", "u-1"),
	unilog.String("password", "super-secret"),
)
```

### Sampling

Reduce noisy logs deterministically.

```go
processor := unilog.NewSamplingProcessor(unilog.SamplingOptions{
	DebugEvery: 10,
	InfoEvery:  5,
	WarnEvery:  1,
})
```

This keeps:

- every 10th debug event
- every 5th info event
- every warning
- all errors and fatals

### Config-driven pipeline assembly

Milestone 2 includes a registry-based builder so sinks and processors can be wired from configuration.

```go
package main

import (
	"context"
	"os"

	"github.com/yourorg/unilog"
)

func main() {
	cfg := unilog.Config{
		Service:     "billing-api",
		Environment: "prod",
		Level:       unilog.DebugLevel,
		Sinks: []unilog.SinkConfig{
			{
				Type: "text",
				Params: map[string]any{
					"writer": os.Stdout,
				},
				MinLevel: levelPtr(unilog.DebugLevel),
			},
			{
				Type: "json",
				Params: map[string]any{
					"writer": os.Stdout,
				},
				MinLevel: levelPtr(unilog.InfoLevel),
			},
		},
		Processors: []unilog.ProcessorConfig{
			{
				Type: "redact",
				Params: map[string]any{
					"keys": []string{"password", "token", "authorization"},
				},
			},
			{
				Type: "sampling",
				Params: map[string]any{
					"debug_every": uint64(10),
					"info_every":  uint64(5),
				},
			},
		},
	}

	log, err := unilog.BuildFromConfig(cfg, nil)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	ctx = unilog.WithTraceID(ctx, "trace-123")
	ctx = unilog.WithRequestID(ctx, "req-456")

	log.Info(ctx, "user login",
		unilog.String("user_id", "u-1"),
		unilog.String("password", "super-secret"),
	)
}

func levelPtr(l unilog.Level) *unilog.Level { return &l }
```

### Built-in registry entries

#### Built-in sinks

- nop
- json
- text

#### Built-in processors

- redact
- sampling

You can also register your own sinks and processors:

```go
reg := unilog.NewRegistry()

err := reg.RegisterSink("custom", func(params map[string]any) (unilog.Sink, error) {
	return newCustomSink(params)
})
if err != nil {
	panic(err)
}
```

### Error handling

Logging calls do not return errors.

Internal pipeline failures are routed to `OnInternalError`:

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

Processors may also intentionally drop events using the internal sentinel `ErrDropEvent`. This is used by the sampling processor.

## Philosophy

`unilog` is built around a few design rules:

- loggers are immutable
- events are normalized before reaching sinks
- processors transform events before delivery
- sinks handle transport, not business meaning
- application code should not be tightly coupled to a logging vendor

## Development status

This project is under active development. The API is still evolving, though the core abstractions introduced till now are intended to remain stable.

## Testing

Run the test suite with:

```bash
go test ./...
```

For race detection:

```bash
go test -race ./...
```

### Contributing

Issues, design feedback, and implementation suggestions are welcome.
If you plan to contribute a new sink or processor, try to preserve the core design principles:

- keep the core vendor-neutral
- avoid mutating shared logger state
- prefer composition over backend-specific shortcuts
