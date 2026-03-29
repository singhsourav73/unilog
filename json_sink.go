package unilog

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"
)

type JSONSink struct {
	mu sync.Mutex
	w  io.Writer
}

func NewJSONSink(w io.Writer) *JSONSink {
	return &JSONSink{w: w}
}

func (s *JSONSink) Name() string {
	return "json"
}

func (s *JSONSink) Write(_ context.Context, event Event) error {
	payload := make(map[string]any, len(event.Fields)+12)

	payload["time"] = event.Time.UTC().Format(time.RFC3339Nano)
	payload["level"] = event.Level.String()
	payload["message"] = event.Message

	if event.Service != "" {
		payload["service"] = event.Service
	}
	if event.Environment != "" {
		payload["environment"] = event.Environment
	}
	if event.LoggerName != "" {
		payload["logger_name"] = event.LoggerName
	}
	if event.TraceID != "" {
		payload["trace_id"] = event.TraceID
	}
	if event.SpanID != "" {
		payload["span_id"] = event.SpanID
	}
	if event.RequestID != "" {
		payload["request_id"] = event.RequestID
	}
	if event.CorrelationID != "" {
		payload["correlation_id"] = event.CorrelationID
	}
	if event.Err != nil {
		payload["error"] = event.Err.Error()
	}
	if event.Caller != nil {
		payload["caller"] = event.Caller
	}
	if event.Stack != "" {
		payload["stack"] = event.Stack
	}

	for _, f := range event.Fields {
		payload[f.Key] = normalizeFieldValue(f.Value)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.w.Write(append(b, '\n'))
	return err
}

func (s *JSONSink) Sync(context.Context) error {
	return nil
}

func (s *JSONSink) Close(context.Context) error {
	return nil
}

func normalizeFieldValue(value any) any {
	switch v := value.(type) {
	case error:
		return v.Error()
	default:
		return v
	}
}
