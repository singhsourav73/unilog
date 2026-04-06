package unilog

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

type stubEncoder struct {
	name        string
	contentType string
	out         []byte
	err         error
}

func (e stubEncoder) Name() string        { return e.name }
func (e stubEncoder) ContentType() string { return e.contentType }
func (e stubEncoder) Encode(Event) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}
	return e.out, nil
}

type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

func TestNewWriterSinkRejectsNilDeps(t *testing.T) {
	if _, err := NewWriterSink(nil, NewJSONEncoder()); err == nil {
		t.Fatalf("expected error for nil writer")
	}
	if _, err := NewWriterSink(io.Discard, nil); err == nil {
		t.Fatalf("expected error for nil encoder")
	}
}

func TestWriterSinkWriteWritesEncodedBytes(t *testing.T) {
	var buf bytes.Buffer

	sink, err := NewWriterSink(&buf, stubEncoder{
		name:        "stub",
		contentType: "application/test",
		out:         []byte("encoded-output"),
	})
	if err != nil {
		t.Fatalf("NewWriterSink() error = %v", err)
	}

	err = sink.Write(context.Background(), Event{
		Time:    time.Unix(0, 0).UTC(),
		Level:   InfoLevel,
		Message: "hello",
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := buf.String(); got != "encoded-output" {
		t.Fatalf("output = %q, want %q", got, "encoded-output")
	}
}

func TestWriterSinkWritePropagatesEncoderError(t *testing.T) {
	wantErr := errors.New("encode failed")

	sink, err := NewWriterSink(io.Discard, stubEncoder{
		name:        "stub",
		contentType: "application/test",
		err:         wantErr,
	})
	if err != nil {
		t.Fatalf("NewWriterSink() error = %v", err)
	}

	err = sink.Write(context.Background(), Event{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestWriterSinkWritePropagatesWriterError(t *testing.T) {
	wantErr := errors.New("write failed")

	sink, err := NewWriterSink(errWriter{err: wantErr}, stubEncoder{
		name:        "stub",
		contentType: "application/test",
		out:         []byte("encoded-output"),
	})
	if err != nil {
		t.Fatalf("NewWriterSink() error = %v", err)
	}

	err = sink.Write(context.Background(), Event{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
