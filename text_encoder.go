package unilog

import (
	"fmt"
	"sort"
	"strings"
)

type TextEncoderOptions struct {
	TimeLayout string
}

type TextEncoder struct {
	timeLayout string
}

func NewTextEncoder(opts TextEncoderOptions) *TextEncoder {
	if opts.TimeLayout == "" {
		opts.TimeLayout = "2006-01-02T15:04:05.000Z07:00"
	}
	return &TextEncoder{timeLayout: opts.TimeLayout}
}

func (e *TextEncoder) Name() string { return "text" }

func (e *TextEncoder) ContentType() string { return "text/plain; charset=utf-8" }

func (e *TextEncoder) Encode(event Event) ([]byte, error) {
	var b strings.Builder

	b.WriteString(event.Time.UTC().Format(e.timeLayout))
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
	return []byte(b.String()), nil
}
