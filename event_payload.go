package unilog

import "time"

func EventPayload(event Event) map[string]any {
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
	if event.Caller != nil {
		payload["caller"] = event.Caller
	}
	if event.Stack != "" {
		payload["stack"] = event.Stack
	}
	if event.Err != nil {
		payload["error"] = event.Err.Error()
	}

	for _, f := range event.Fields {
		payload[f.Key] = normalizeFieldValue(f.Value)
	}

	return payload
}

func normalizeFieldValue(value any) any {
	switch v := value.(type) {
	case error:
		return v.Error()
	default:
		return v
	}
}
