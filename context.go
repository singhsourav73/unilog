package unilog

import "context"

type contextKey string

const (
	traceIdKey       contextKey = "unilog_trace_id"
	spanIdKey        contextKey = "unilog_span_id"
	requestIdKey     contextKey = "unilog_request_id"
	correlationIdKey contextKey = "unilog_correlation_id"
)

func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIdKey, id)
}

func WithSpanID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, spanIdKey, id)
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIdKey, id)
}

func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIdKey, id)
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(traceIdKey).(string)
	return v
}

func SpanIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(spanIdKey).(string)
	return v
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(requestIdKey).(string)
	return v
}

func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(correlationIdKey).(string)
	return v
}
