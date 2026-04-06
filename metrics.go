package unilog

import (
	"context"
	"sync/atomic"
)

type Metrics struct {
	Writes    atomic.Uint64
	Successes atomic.Uint64
	Failures  atomic.Uint64
	Dropped   atomic.Uint64
}

type MetricSnapshot struct {
	Writes    uint64
	Successes uint64
	Failures  uint64
	Dropped   uint64
}

func (m *Metrics) Snapshot() MetricSnapshot {
	if m == nil {
		return MetricSnapshot{}
	}
	return MetricSnapshot{
		Writes:    m.Writes.Load(),
		Successes: m.Successes.Load(),
		Failures:  m.Failures.Load(),
		Dropped:   m.Dropped.Load(),
	}
}

type MetricsSink struct {
	next    Sink
	metrics *Metrics
}

func NewMetricsSink(next Sink, metrics *Metrics) *MetricsSink {
	if next == nil {
		next = NopSink{}
	}
	if metrics == nil {
		metrics = &Metrics{}
	}
	return &MetricsSink{
		next:    next,
		metrics: metrics,
	}
}

func (s *MetricsSink) Name() string {
	return "Metrics->" + s.next.Name()
}

func (s *MetricsSink) Write(ctx context.Context, event Event) error {
	s.metrics.Writes.Add(1)

	err := s.next.Write(ctx, event)
	if err != nil {
		if IsDropEvent(err) {
			s.metrics.Dropped.Add(1)
		} else {
			s.metrics.Failures.Add(1)
		}
		return err
	}

	s.metrics.Successes.Add(1)
	return nil
}

func (s *MetricsSink) Sync(ctx context.Context) error {
	return s.next.Sync(ctx)
}

func (s *MetricsSink) Close(ctx context.Context) error {
	return s.next.Close(ctx)
}
