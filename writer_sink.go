package unilog

import (
	"context"
	"fmt"
	"io"
	"sync"
)

type WriterSink struct {
	mu      sync.Mutex
	w       io.Writer
	encoder Encoder
}

func NewWriterSink(w io.Writer, encoder Encoder) (*WriterSink, error) {
	if w == nil {
		return nil, fmt.Errorf("unilog: writer sink writer is nil")
	}
	if encoder == nil {
		return nil, fmt.Errorf("unilog: writer sink encoder is nil")
	}
	return &WriterSink{w: w, encoder: encoder}, nil
}

func (s *WriterSink) Name() string {
	return "writer(" + s.encoder.Name() + ")"
}

func (s *WriterSink) Write(_ context.Context, event Event) error {
	b, err := s.encoder.Encode(event)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.w.Write(b)
	return err
}

func (s *WriterSink) Sync(context.Context) error { return nil }

func (s *WriterSink) Close(context.Context) error { return nil }
