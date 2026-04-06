package unilog

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPSinkPostEventPayload(t *testing.T) {
	var gotMethod string
	var gotHeader string
	var gotPayload map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		gotMethod = r.Method
		gotHeader = r.Header.Get("X-Test-Header")

		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(HTTPSinkOptions{
		URL:    srv.URL,
		Method: http.MethodPost,
		Headers: map[string]string{
			"X-Test-Header": "hello",
		},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewHTTPSink() error = %v", err)
	}

	ev := Event{
		Time:      time.Unix(0, 0).UTC(),
		Level:     ErrorLevel,
		Message:   "payment failed",
		Service:   "billing-api",
		RequestID: "req-1",
		Fields: []Field{
			String("order_id", "ord-1"),
		},
	}

	if err := sink.Write(context.Background(), ev); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("header = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotHeader != "hello" {
		t.Fatalf("header = %q, want hello", gotHeader)
	}
	if gotPayload["message"] != "payment failed" {
		t.Fatalf("message = %v, want payment failed", gotPayload["message"])
	}
	if gotPayload["service"] != "billing-api" {
		t.Fatalf("service = %v, want billing-api", gotPayload["service"])
	}
	if gotPayload["request_id"] != "req-1" {
		t.Fatalf("request_id = %v, want req-1", gotPayload["request_id"])
	}
	if gotPayload["order_id"] != "ord-1" {
		t.Fatalf("order_id = %v, want ord-1", gotPayload["order_id"])
	}
}

func TestHTTPSinkReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "downstream unhappy", http.StatusBadGateway)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(HTTPSinkOptions{
		URL: srv.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPSink() error = %v", err)
	}

	err = sink.Write(context.Background(), Event{
		Time:    time.Now().UTC(),
		Level:   ErrorLevel,
		Message: "boom",
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestHTTPSinkSetsDefaultContentTypeFromEncoder(t *testing.T) {
	var gotContentType string
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)

		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(HTTPSinkOptions{
		URL:     srv.URL,
		Encoder: NewTextEncoder(TextEncoderOptions{TimeLayout: time.RFC3339}),
	})
	if err != nil {
		t.Fatalf("NewHTTPSink() error = %v", err)
	}

	err = sink.Write(context.Background(), Event{
		Time:    time.Unix(0, 0).UTC(),
		Level:   InfoLevel,
		Message: "hello world",
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if gotContentType != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", gotContentType, "text/plain; charset=utf-8")
	}
	if gotBody == "" || gotBody[0] == '{' {
		t.Fatalf("expected text body, got %q", gotBody)
	}
}

func TestHTTPSinkExplicitContentTypeOverridesEncoderDefault(t *testing.T) {
	var gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(HTTPSinkOptions{
		URL:     srv.URL,
		Encoder: NewJSONEncoder(),
		Headers: map[string]string{
			"Content-Type": "application/x-custom",
		},
	})
	if err != nil {
		t.Fatalf("NewHTTPSink() error = %v", err)
	}

	err = sink.Write(context.Background(), Event{
		Time:    time.Unix(0, 0).UTC(),
		Level:   InfoLevel,
		Message: "hello",
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if gotContentType != "application/x-custom" {
		t.Fatalf("Content-Type = %q, want %q", gotContentType, "application/x-custom")
	}
}

type failingEncoder struct{}

func (failingEncoder) Name() string        { return "failing" }
func (failingEncoder) ContentType() string { return "application/test" }
func (failingEncoder) Encode(Event) ([]byte, error) {
	return nil, errors.New("encode failed")
}

func TestHTTPSinkPropagatesEncoderError(t *testing.T) {
	hitServer := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitServer = true
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(HTTPSinkOptions{
		URL:     srv.URL,
		Encoder: failingEncoder{},
	})
	if err != nil {
		t.Fatalf("NewHTTPSink() error = %v", err)
	}

	err = sink.Write(context.Background(), Event{
		Time:    time.Unix(0, 0).UTC(),
		Level:   InfoLevel,
		Message: "hello",
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if hitServer {
		t.Fatalf("server should not have been called on encode failure")
	}
}

func TestHTTPSinkWriteBatchJSONSendsSingleRequestWithArrayPayload(t *testing.T) {
	var reqCount int
	var gotBody []byte
	var gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		gotContentType = r.Header.Get("Content-Type")
		defer r.Body.Close()

		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(HTTPSinkOptions{
		URL:     srv.URL,
		Encoder: NewJSONEncoder(),
	})
	if err != nil {
		t.Fatalf("NewHTTPSink() error = %v", err)
	}

	events := []Event{
		{
			Time:    time.Unix(0, 0).UTC(),
			Level:   InfoLevel,
			Message: "one",
		},
		{
			Time:    time.Unix(1, 0).UTC(),
			Level:   ErrorLevel,
			Message: "two",
		},
	}

	if err := sink.WriteBatch(context.Background(), events); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	if reqCount != 1 {
		t.Fatalf("request count = %d, want 1", reqCount)
	}

	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}

	var payload []map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid JSON array body: %v; body=%s", err, string(gotBody))
	}

	if len(payload) != 2 {
		t.Fatalf("payload len = %d, want 2", len(payload))
	}

	if payload[0]["message"] != "one" {
		t.Fatalf("first message = %v, want one", payload[0]["message"])
	}
	if payload[1]["message"] != "two" {
		t.Fatalf("second message = %v, want two", payload[1]["message"])
	}
}

func TestHTTPSinkWriteBatchFallsBackForNonJSONEncoder(t *testing.T) {
	var reqCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(HTTPSinkOptions{
		URL:     srv.URL,
		Encoder: NewTextEncoder(TextEncoderOptions{TimeLayout: time.RFC3339}),
	})
	if err != nil {
		t.Fatalf("NewHTTPSink() error = %v", err)
	}

	events := []Event{
		{
			Time:    time.Unix(0, 0).UTC(),
			Level:   InfoLevel,
			Message: "one",
		},
		{
			Time:    time.Unix(1, 0).UTC(),
			Level:   InfoLevel,
			Message: "two",
		},
	}

	if err := sink.WriteBatch(context.Background(), events); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	if reqCount != 2 {
		t.Fatalf("request count = %d, want 2", reqCount)
	}
}

func TestHTTPSinkWriteBatchPropagatesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(HTTPSinkOptions{
		URL:     srv.URL,
		Encoder: NewJSONEncoder(),
	})
	if err != nil {
		t.Fatalf("NewHTTPSink() error = %v", err)
	}

	events := []Event{
		{
			Time:    time.Unix(0, 0).UTC(),
			Level:   InfoLevel,
			Message: "one",
		},
		{
			Time:    time.Unix(1, 0).UTC(),
			Level:   InfoLevel,
			Message: "two",
		},
	}

	err = sink.WriteBatch(context.Background(), events)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected status code in error, got %v", err)
	}
}
