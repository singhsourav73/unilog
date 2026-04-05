package unilog

import (
	"context"
	"encoding/json"
	"io"
	"sync"
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
	payload := EventPayload(event)

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
