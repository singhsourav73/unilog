package unilog

import "time"

type Caller struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

type Event struct {
	Time          time.Time `json:"time"`
	Level         Level     `json:"level"`
	Message       string    `json:"message"`
	Err           error     `json:"error,omitempty"`
	Service       string    `json:"service,omitempty"`
	Environment   string    `json:"environment,omitempty"`
	LoggerName    string    `json:"logger_name,omitempty"`
	TraceID       string    `json:"trace_id,omitempty"`
	SpanID        string    `json:"span_id,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	Fields        []Field   `json:"fields,omitempty"`
	Caller        *Caller   `json:"caller,omitempty"`
	Stack         string    `json:"stack,omitempty"`
}
