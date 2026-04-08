package unilog

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
)

type FileSinkOptions struct {
	Path    string
	Encoder Encoder
	Perm    fs.FileMode
}

type FileSink struct {
	mu       sync.Mutex
	file     *os.File
	delegate Sink
	path     string
	closed   bool
}

func NewFileSink(opts FileSinkOptions) (*FileSink, error) {
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		return nil, fmt.Errorf("unilog: file sink path is empty")
	}

	perm := opts.Perm
	if perm == 0 {
		perm = 0o644
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, perm)
	if err != nil {
		return nil, fmt.Errorf("unilog: open file sink: %w", err)
	}

	encoder := opts.Encoder
	if encoder == nil {
		encoder = NewJSONEncoder()
	}

	delegate, err := NewWriterSink(f, encoder)
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	return &FileSink{
		file:     f,
		delegate: delegate,
		path:     path,
	}, nil
}

func (s *FileSink) Name() string {
	return "file(" + s.delegate.Name() + ")"
}

func (s *FileSink) Write(ctx context.Context, event Event) error {
	s.mu.Lock()
	closed := s.closed
	delegate := s.delegate
	s.mu.Unlock()

	if closed {
		return fmt.Errorf("unilog: write to closed file sink")
	}
	return delegate.Write(ctx, event)
}

func (s *FileSink) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	var errs []error
	if s.delegate != nil {
		if err := s.delegate.Sync(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if s.file != nil {
		if err := s.file.Sync(); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors(errs...)
}

func (s *FileSink) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	var errs []error
	if s.delegate != nil {
		if err := s.delegate.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if s.file != nil {
		if err := s.file.Close(); err != nil {
			errs = append(errs, err)
		}
		s.file = nil
	}
	return joinErrors(errs...)
}
