package unilog

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

type TextSinkOptions struct {
	TimeLayout string
}

type TextSink struct {
	mu         sync.Mutex
	w          io.Writer
	timeLayout string
}

func NewTextSink(w io.Writer, opts TextSinkOptions) *TextSink {
	if opts.TimeLayout == "" {
		opts.TimeLayout = "2006-01-02T15:04:05.000Z07:00"
	}
	return &TextSink{w: w, timeLayout: opts.TimeLayout}
}

func (s *TextSink) Name() string {
	return "text"
}

func (s *TextSink) Write(_ context.Context, event Event) error {
	var b strings.Builder

	b.WriteString(event.Time.UTC().Format(s.timeLayout))
	b.WriteString(" ")
	b.WriteString(strings.ToUpper(event.Level.String()))

	if event.Service != "" {
		b.WriteString(" service=")
		b.WriteString(quoteIfNeeded(event.Service))
	}

	if event.Environment != "" {
		b.WriteString(" env=")
		b.WriteString(quoteIfNeeded(event.Environment))
	}

	if event.LoggerName != "" {
		b.WriteString(" logger=")
		b.WriteString(quoteIfNeeded(event.LoggerName))
	}

	b.WriteString(" msg=")
	b.WriteString(quoteIfNeeded(event.Message))

	if event.TraceID != "" {
		b.WriteString(" trace_id=")
		b.WriteString(quoteIfNeeded(event.TraceID))
	}

	if event.SpanID != "" {
		b.WriteString(" span_id=")
		b.WriteString(quoteIfNeeded(event.SpanID))
	}

	if event.RequestID != "" {
		b.WriteString(" request_id=")
		b.WriteString(quoteIfNeeded(event.RequestID))
	}

	if event.CorrelationID != "" {
		b.WriteString(" correlation_id=")
		b.WriteString(quoteIfNeeded(event.CorrelationID))
	}

	if event.Err != nil {
		b.WriteString(" error=")
		b.WriteString(quoteIfNeeded(event.Err.Error()))
	}

	if event.Caller != nil {
		b.WriteString(" caller=")
		b.WriteString(quoteIfNeeded(fmt.Sprintf("%s:%d %s", event.Caller.File, event.Caller.Line, event.Caller.Function)))
	}

	fields := cloneFields(event.Fields)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})

	for _, f := range fields {
		b.WriteString(" ")
		b.WriteString(f.Key)
		b.WriteString("=")
		b.WriteString(quoteIfNeeded(fmt.Sprint(normalizeFieldValue(f.Value))))
	}

	if event.Stack != "" {
		b.WriteString(" stack=")
		b.WriteString(quoteIfNeeded(event.Stack))
	}

	b.WriteByte('\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := io.WriteString(s.w, b.String())
	return err
}

func (s *TextSink) Sync(context.Context) error {
	return nil
}

func (s *TextSink) Close(context.Context) error {
	return nil
}

func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\r\n=\"") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
