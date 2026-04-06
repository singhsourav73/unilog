package unilog

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type FanoutSink struct {
	sinks []Sink
}

func NewFanoutSink(sinks ...Sink) *FanoutSink {
	out := make([]Sink, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			out = append(out, s)
		}
	}
	return &FanoutSink{sinks: out}
}

func (f *FanoutSink) Name() string {
	names := make([]string, 0, len(f.sinks))
	for _, s := range f.sinks {
		names = append(names, s.Name())
	}
	return "fanout(" + strings.Join(names, ", ") + ")"
}

func (f *FanoutSink) Write(ctx context.Context, event Event) error {
	var errs []error
	for _, s := range f.sinks {
		if err := s.Write(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
		}
	}
	return errors.Join(errs...)
}

func (f *FanoutSink) Sync(ctx context.Context) error {
	var errs []error
	for _, s := range f.sinks {
		if err := s.Sync(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
		}
	}
	return errors.Join(errs...)
}

func (f *FanoutSink) Close(ctx context.Context) error {
	var errs []error
	for _, s := range f.sinks {
		if err := s.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
		}
	}
	return errors.Join(errs...)
}
