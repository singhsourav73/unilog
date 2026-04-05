package unilog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
