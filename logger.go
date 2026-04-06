package unilog

import (
	"context"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"
)

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

type logger struct {
	sink         Sink
	processors   []Processor
	interceptors []Interceptor
	opts         Options

	name       string
	fields     []Field
	defaultCtx context.Context
}

func New(sink Sink, opts Options, processors ...Processor) Logger {
	return NewWithInterceptors(sink, opts, nil, processors...)
}

func NewWithInterceptors(
	sink Sink,
	opts Options,
	interceptors []Interceptor,
	processors ...Processor) Logger {
	if sink == nil {
		sink = NopSink{}
	}

	cp := make([]Processor, 0, len(processors))
	for _, p := range processors {
		if p != nil {
			cp = append(cp, p)
		}
	}

	icp := make([]Interceptor, 0, len(interceptors))
	for _, in := range interceptors {
		if in != nil {
			icp = append(icp, in)
		}
	}

	return &logger{
		sink:         sink,
		processors:   cp,
		interceptors: icp,
		opts:         opts.withDefaults(),
	}
}

func (l *logger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.Log(ctx, DebugLevel, msg, fields...)
}

func (l *logger) Info(ctx context.Context, msg string, fields ...Field) {
	l.Log(ctx, InfoLevel, msg, fields...)
}

func (l *logger) Warn(ctx context.Context, msg string, fields ...Field) {
	l.Log(ctx, WarnLevel, msg, fields...)
}

func (l *logger) Error(ctx context.Context, err error, msg string, fields ...Field) {
	l.LogErr(ctx, ErrorLevel, err, msg, fields...)
}

func (l *logger) Fatal(ctx context.Context, err error, msg string, fields ...Field) {
	l.LogErr(ctx, FatalLevel, err, msg, fields...)
}

func (l *logger) Log(ctx context.Context, level Level, msg string, fields ...Field) {
	l.LogErr(ctx, level, nil, msg, fields...)
}

func (l *logger) LogErr(ctx context.Context, level Level, err error, msg string, fields ...Field) {
	if !l.Enabled(level) {
		return
	}

	ctx = l.resolveContext(ctx)

	ev := Event{
		Time:          time.Now().UTC(),
		Level:         level,
		Message:       msg,
		Err:           err,
		Service:       l.opts.Service,
		Environment:   l.opts.Environment,
		LoggerName:    l.name,
		TraceID:       TraceIDFromContext(ctx),
		SpanID:        SpanIDFromContext(ctx),
		RequestID:     RequestIDFromContext(ctx),
		CorrelationID: CorrelationIDFromContext(ctx),
		Fields:        append([]Field(nil), l.fields...),
	}

	if len(fields) > 0 {
		ev.Fields = append(ev.Fields, fields...)
	}

	if l.opts.AddCaller {
		ev.Caller = captureCaller(3)
	}

	if l.opts.AddStack && level >= ErrorLevel {
		ev.Stack = string(debug.Stack())
	}

	final := func(ctx context.Context, event Event) error {
		for _, p := range l.processors {
			if p == nil {
				continue
			}
			if err := p.Process(ctx, &event); err != nil {
				return err
			}
		}

		if l.sink == nil {
			return nil
		}
		return l.sink.Write(ctx, event)
	}

	handler := ChainInterceptors(final, l.interceptors...)
	if err := handler(ctx, ev); err != nil {
		if IsDropEvent(err) {
			return
		}
		l.opts.OnInternalError(ctx, err)
	}
}

func (l *logger) With(fields ...Field) Logger {
	cp := l.clone()
	if len(fields) > 0 {
		cp.fields = append(cp.fields, fields...)
	}
	return cp
}

func (l *logger) WithName(name string) Logger {
	cp := l.clone()
	if name == "" {
		return cp
	}
	if cp.name == "" {
		cp.name = name
	} else {
		cp.name = cp.name + "." + name
	}
	return cp
}

func (l *logger) WithContext(ctx context.Context) Logger {
	cp := l.clone()
	cp.defaultCtx = ctx
	return cp
}

func (l *logger) Enabled(level Level) bool {
	return level.Valid() && level >= l.opts.Level
}

func (l *logger) Sync(ctx context.Context) error {
	if l.sink == nil {
		return nil
	}
	return l.sink.Sync(l.resolveContext(ctx))
}

func (l *logger) Close(ctx context.Context) error {
	if l.sink == nil {
		return nil
	}
	return l.sink.Close(l.resolveContext(ctx))
}

func (l *logger) clone() *logger {
	return &logger{
		sink:         l.sink,
		processors:   append([]Processor(nil), l.processors...),
		interceptors: append([]Interceptor(nil), l.interceptors...),
		opts:         l.opts,
		name:         l.name,
		fields:       cloneFields(l.fields),
		defaultCtx:   l.defaultCtx,
	}
}

func (l *logger) resolveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}

	if l.defaultCtx != nil {
		return l.defaultCtx
	}

	return context.Background()
}

func captureCaller(skip int) *Caller {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}

	fn := runtime.FuncForPC(pc)
	name := ""
	if fn != nil {
		name = fn.Name()
	}

	return &Caller{
		File:     filepath.Base(file),
		Line:     line,
		Function: name,
	}
}
